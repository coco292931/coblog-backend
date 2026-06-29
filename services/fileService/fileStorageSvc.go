package fileService

import (
	"bytes"
	"coblog-backend/common/exception"
	configreader "coblog-backend/configs/configReader"
	"crypto/rand"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"math/big"
	"os"
	"path/filepath"

	"github.com/disintegration/imaging"
)

// ImageSaveResult 保存图片后返回的文件名信息
type ImageSaveResult struct {
	OriginalName   string // 原图文件名（始终保存）
	CompressedName string // 压缩图文件名，为空表示未生成压缩图
}

// SaveUploadedFile 统一从 io.Reader 读文件存盘（非图片通用，保持向后兼容）
func SaveUploadedFile(ior *io.Reader) (string, error) {
	dir := configreader.GetConfig().FileObject.Dir
	fileName := randStrGenerater(32)
	filePath := filepath.Join(dir, fileName)
	dst, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer dst.Close()
	if _, err = io.Copy(dst, *ior); err != nil {
		return "", err
	}
	log.Printf("[INFO][FileCtrl] New file uploaded, file: %v", dst.Name())
	return fileName, nil
}

// allowedImageExt 允许保存的图片后缀白名单，杜绝 .svg/.html 等可执行脚本的存储型 XSS
var allowedImageExt = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
	".webp": true,
}

// SaveImageWithCompression 保存图片，并在满足条件时生成压缩版本。
// 原图始终保存；满足阈值时额外保存一份压缩图（加 _c 后缀，JPEG 格式）。
// data 为完整图片字节，ext 为小写扩展名（如 ".jpg" ".png"）。
func SaveImageWithCompression(data []byte, ext string) (ImageSaveResult, error) {
	if !allowedImageExt[ext] {
		return ImageSaveResult{}, exception.ApiParamError
	}

	cfg := configreader.GetConfig().FileObject
	dir := cfg.Dir

	baseName := randStrGenerater(32)
	origName := baseName + ext
	if err := os.WriteFile(filepath.Join(dir, origName), data, 0o644); err != nil {
		return ImageSaveResult{}, err
	}
	log.Printf("[INFO][FileSvc] 原图已保存: %s (%d bytes)", origName, len(data))

	threshold := cfg.CompressThreshold
	if threshold == 0 || int64(len(data)) > threshold {
		compressed, compExt, err := compressImage(data, ext, cfg.CompressMaxWidth, cfg.CompressQuality)
		if err != nil {
			// 压缩失败不影响原图，记录日志继续
			log.Printf("[WARN][FileSvc] 压缩失败，仅保留原图: %v", err)
			return ImageSaveResult{OriginalName: origName}, nil
		}
		compName := baseName + "_c" + compExt
		if err := os.WriteFile(filepath.Join(dir, compName), compressed, 0o644); err != nil {
			log.Printf("[WARN][FileSvc] 压缩图写盘失败，仅保留原图: %v", err)
			return ImageSaveResult{OriginalName: origName}, nil
		}
		log.Printf("[INFO][FileSvc] 压缩图已保存: %s (%d bytes)", compName, len(compressed))
		return ImageSaveResult{OriginalName: origName, CompressedName: compName}, nil
	}

	return ImageSaveResult{OriginalName: origName}, nil
}

// #####PRIVATE#####

// compressImage 解码图片、按需缩放后重新编码。
// PNG 保持 PNG 编码以保留透明通道，其余格式编码为 JPEG。
// 返回压缩后的字节及其对应的文件后缀（".png" / ".jpg"）。
func compressImage(data []byte, ext string, maxWidth int, quality int) ([]byte, string, error) {
	if quality <= 0 || quality > 100 {
		quality = 80
	}

	img, err := decodeImage(data, ext)
	if err != nil {
		return nil, "", err
	}

	if maxWidth > 0 && img.Bounds().Dx() > maxWidth {
		img = imaging.Resize(img, maxWidth, 0, imaging.Lanczos)
	}

	var buf bytes.Buffer
	if ext == ".png" {
		if err := png.Encode(&buf, img); err != nil {
			return nil, "", err
		}
		return buf.Bytes(), ".png", nil
	}
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), ".jpg", nil
}

func decodeImage(data []byte, ext string) (image.Image, error) {
	switch ext {
	case ".png":
		return png.Decode(bytes.NewReader(data))
	default:
		// jpg / jpeg / webp 等，统一用 imaging.Decode
		return imaging.Decode(bytes.NewReader(data))
	}
}

func randStrGenerater(length int) string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	buf := make([]byte, length)
	for i := range buf {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		buf[i] = charset[num.Int64()]
	}
	return string(buf)
}
