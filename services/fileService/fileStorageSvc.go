package fileService

import (
	"bytes"
	"coblog-backend/common/exception"
	configreader "coblog-backend/configs/configReader"
	"crypto/rand"
	"encoding/binary"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"math/big"
	"os"
	"path/filepath"

	"github.com/disintegration/imaging"
	_ "golang.org/x/image/webp" // 注册 webp 解码器
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

// imageTypeByContent 按魔数判定图片类型，返回规范化的后缀。
// 只看内容不看文件名：既防住改名的 .svg/.html 触发存储型 XSS，也避免后缀与内容不符时解码静默失败。
func imageTypeByContent(data []byte) (string, bool) {
	switch {
	case bytes.HasPrefix(data, []byte("\xff\xd8\xff")):
		return ".jpg", true
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		return ".png", true
	case bytes.HasPrefix(data, []byte("GIF87a")), bytes.HasPrefix(data, []byte("GIF89a")):
		return ".gif", true
	case isWebPContainer(data):
		return ".webp", true
	}
	return "", false
}

// SaveImageWithCompression 保存图片，并在满足条件时额外生成压缩版本。
// 原图始终保存；超过阈值（阈值为 0 表示始终压）时生成 <base>_c<后缀>，
// 后缀由压缩结果决定（有透明通道为 png，否则 jpg）。
// data 为完整图片字节，类型按内容判定，与文件名后缀无关。
func SaveImageWithCompression(data []byte) (ImageSaveResult, error) {
	ext, ok := imageTypeByContent(data)
	if !ok {
		return ImageSaveResult{}, exception.ApiFileNotSupported
	}

	cfg := configreader.GetConfig().FileObject

	// 落盘前先卡像素上限，避免解压炸弹把内存打满
	if err := checkPixels(data, cfg.MaxPixels); err != nil {
		return ImageSaveResult{}, err
	}

	dir := filepath.Join(cfg.Dir, "img")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ImageSaveResult{}, err
	}

	baseName := randStrGenerater(32)
	origName := baseName + ext
	stored := stripMetadataSafely(data, ext)
	if err := os.WriteFile(filepath.Join(dir, origName), stored, 0o644); err != nil {
		return ImageSaveResult{}, err
	}
	log.Printf("[INFO][FileSvc] 原图已保存: %s (%d bytes)", origName, len(stored))

	threshold := cfg.CompressThreshold
	if threshold != 0 && int64(len(stored)) <= threshold {
		return ImageSaveResult{OriginalName: origName}, nil
	}

	compressed, compExt, err := compressImage(data, cfg.CompressMaxWidth, cfg.CompressMaxHeight, cfg.CompressQuality)
	if err != nil {
		// 压缩失败不影响原图，记录日志继续
		log.Printf("[WARN][FileSvc] 压缩失败，仅保留原图: %v", err)
		return ImageSaveResult{OriginalName: origName}, nil
	}
	if !worthKeeping(len(data), len(compressed)) {
		log.Printf("[INFO][FileSvc] 压缩收益不足，仅保留原图: %s", origName)
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

// #####PRIVATE#####

// keepPercent 压缩图体积低于原图的这个百分比才值得单独存一份
const keepPercent = 95

// stripMetadataSafely 去掉原图里的位置信息。若清理结果尺寸对不上（改坏了），
// 退回原始字节，宁可不清理也不写出半损坏的文件。
func stripMetadataSafely(data []byte, ext string) []byte {
	stripped := stripMetadata(data, ext)
	if len(stripped) == len(data) {
		return data
	}
	if !sameImageSize(data, stripped) {
		log.Printf("[WARN][FileSvc] 元数据清理后尺寸校验不通过，保留原始字节")
		return data
	}
	return stripped
}

// sameImageSize 只比头部信息，确认清理没动像素尺寸（动图 webp 读不出头部，改用首帧尺寸）
func sameImageSize(before, after []byte) bool {
	if w1, h1, ok1 := webpFirstFrameSize(before); ok1 {
		w2, h2, ok2 := webpFirstFrameSize(after)
		return ok2 && w1 == w2 && h1 == h2
	}
	c1, _, err1 := image.DecodeConfig(bytes.NewReader(before))
	c2, _, err2 := image.DecodeConfig(bytes.NewReader(after))
	if err1 != nil || err2 != nil {
		return err1 != nil && err2 != nil // 两边都读不出：没得比，放行
	}
	return c1.Width == c2.Width && c1.Height == c2.Height
}

// worthKeeping 压缩收益是否值得单独存一份
func worthKeeping(orig, comp int) bool {
	return comp*100 < orig*keepPercent
}

// checkPixels 落盘前按头部信息卡像素数上限，避免「解压炸弹」把内存打满。
// maxPixels <= 0 表示不限制；头部读不出时不拦，交给后续解码报错。
func checkPixels(data []byte, maxPixels int64) error {
	if maxPixels <= 0 {
		return nil
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		// 动图 webp 连头部都读不出，退而看首帧尺寸
		w, h, ok := webpFirstFrameSize(data)
		if !ok {
			return nil
		}
		return checkPixelCount(w, h, maxPixels)
	}
	return checkPixelCount(cfg.Width, cfg.Height, maxPixels)
}

func checkPixelCount(w, h int, maxPixels int64) error {
	if int64(w)*int64(h) > maxPixels {
		return exception.ApiImageTooManyPixels
	}
	return nil
}

// compressImage 解码、等比缩小后重新编码，后缀由输出格式决定。
// 有真实透明像素 → PNG（保留透明通道），否则 → JPEG（同尺寸下体积小得多）。
// 返回压缩后的字节及其对应的文件后缀（".png" / ".jpg"）。
func compressImage(data []byte, maxWidth, maxHeight, quality int) ([]byte, string, error) {
	img, err := decodeImage(data)
	if err != nil {
		return nil, "", err
	}

	// 必须在缩放前判断：Resize 的产物是 *image.NRGBA，会被误判成有透明像素
	transparent := hasAlpha(img)
	img = resizeToFit(img, maxWidth, maxHeight)

	var buf bytes.Buffer
	if transparent {
		if err := png.Encode(&buf, img); err != nil {
			return nil, "", err
		}
		return buf.Bytes(), ".png", nil
	}

	if quality <= 0 || quality > 100 {
		quality = jpeg.DefaultQuality
	}
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), ".jpg", nil
}

// resizeToFit 按最大宽高等比缩小，不放大；上限为 0 表示不限
func resizeToFit(img image.Image, maxWidth, maxHeight int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	scale := 1.0
	if maxWidth > 0 && w > maxWidth {
		scale = float64(maxWidth) / float64(w)
	}
	if maxHeight > 0 && h > maxHeight {
		if s := float64(maxHeight) / float64(h); s < scale {
			scale = s
		}
	}
	if scale >= 1 {
		return img
	}
	nw, nh := int(float64(w)*scale+0.5), int(float64(h)*scale+0.5)
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	return imaging.Resize(img, nw, nh, imaging.Lanczos)
}

// hasAlpha 判断是否存在真实透明像素；颜色模型不含 alpha 时直接返回 false
func hasAlpha(img image.Image) bool {
	switch m := img.(type) {
	case *image.NRGBA:
		return hasTransparentByte(m.Pix)
	case *image.RGBA:
		return hasTransparentByte(m.Pix)
	case *image.NRGBA64, *image.RGBA64, *image.Paletted, *image.Alpha, *image.Alpha16, *image.NYCbCrA:
	default:
		return false
	}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a < 0xffff {
				return true
			}
		}
	}
	return false
}

// hasTransparentByte 检查 8 位 RGBA 像素的 alpha 字节是否有非满值
func hasTransparentByte(pix []byte) bool {
	for i := 3; i < len(pix); i += 4 {
		if pix[i] != 0xff {
			return true
		}
	}
	return false
}

// decodeImage 解码并按 EXIF 方向转正；动图 webp 退回首帧
func decodeImage(data []byte) (image.Image, error) {
	img, err := decodeOriented(data)
	if err == nil {
		return img, nil
	}
	if frame, ok := firstWebPFrame(data); ok {
		return decodeOriented(frame)
	}
	return nil, err
}

func decodeOriented(data []byte) (image.Image, error) {
	return imaging.Decode(bytes.NewReader(data), imaging.AutoOrientation(true))
}

// #####WEBP#####
// x/image 的 webp 解码器不支持动图（VP8X 带 ANIM 会直接报错），所以自己剥出首帧
// 重新包成最小 RIFF 容器。

const (
	webpHeaderLen  = 12 // "RIFF" + 长度 + "WEBP"
	anmfHeaderLen  = 16 // ANMF 载荷开头的帧头：位置(6) + 尺寸(6) + 时长(3) + 标志(1)
	webpAlphaFlags = 0x10
)

// riffChunk 一个 RIFF 子块
type riffChunk struct {
	id   string
	body []byte
}

// isWebPContainer 是否是 RIFF/WEBP 容器
func isWebPContainer(data []byte) bool {
	return len(data) >= webpHeaderLen && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP"
}

// readChunks 从 off 起按「fourCC + 小端长度」遍历子块，长度越界视为结构异常
func readChunks(data []byte, off int) ([]riffChunk, bool) {
	var chunks []riffChunk
	for off+8 <= len(data) {
		size := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		if size < 0 || off+8+size > len(data) {
			return nil, false
		}
		chunks = append(chunks, riffChunk{id: string(data[off : off+4]), body: data[off+8 : off+8+size]})
		off += 8 + size + size%2 // 载荷按偶数对齐
	}
	return chunks, len(chunks) > 0
}

// webpTopChunks 返回 webp 容器的顶层子块
func webpTopChunks(data []byte) ([]riffChunk, bool) {
	if !isWebPContainer(data) {
		return nil, false
	}
	return readChunks(data, webpHeaderLen)
}

// firstWebPFrame 取动图 webp 的首帧，重新包成可解码的最小 RIFF 容器；非动图返回 false
func firstWebPFrame(data []byte) ([]byte, bool) {
	chunks, ok := webpTopChunks(data)
	if !ok {
		return nil, false
	}
	for _, c := range chunks {
		if c.id != "ANMF" || len(c.body) <= anmfHeaderLen {
			continue
		}
		parts, ok := frameParts(c.body)
		if !ok {
			return nil, false
		}
		return packWebP(parts), true
	}
	return nil, false
}

// webpFirstFrameSize 只读首帧显示尺寸，供像素上限校验用
func webpFirstFrameSize(data []byte) (int, int, bool) {
	chunks, ok := webpTopChunks(data)
	if !ok {
		return 0, 0, false
	}
	for _, c := range chunks {
		if c.id == "ANMF" && len(c.body) >= anmfHeaderLen {
			return int(readUint24(c.body[6:9])) + 1, int(readUint24(c.body[9:12])) + 1, true
		}
	}
	return 0, 0, false
}

// frameParts 从 ANMF 载荷中挑出可独立解码的位流子块。
// 有 ALPH 时必须补一个只声明 alpha 的 VP8X：解码器要靠 VP8X 才会接受后面的 ALPH。
func frameParts(anmf []byte) ([]riffChunk, bool) {
	sub, ok := readChunks(anmf, anmfHeaderLen)
	if !ok {
		return nil, false
	}
	var alpha, bitstream *riffChunk
	for i := range sub {
		switch sub[i].id {
		case "ALPH":
			alpha = &sub[i]
		case "VP8 ", "VP8L":
			if bitstream == nil {
				bitstream = &sub[i]
			}
		}
	}
	if bitstream == nil {
		return nil, false
	}
	// 无损位流自带透明通道，带上 ALPH/VP8X 反而会被解码器拒绝
	if alpha == nil || bitstream.id == "VP8L" {
		return []riffChunk{*bitstream}, true
	}
	return []riffChunk{{id: "VP8X", body: vp8xBody(anmf)}, *alpha, *bitstream}, true
}

// vp8xBody 按帧头里的尺寸造 VP8X 载荷：标志(1) + 保留(3) + 宽-1(3) + 高-1(3)
func vp8xBody(anmf []byte) []byte {
	wMinusOne, hMinusOne := readUint24(anmf[6:9]), readUint24(anmf[9:12])
	body := make([]byte, 10)
	body[0] = webpAlphaFlags
	body[4], body[5], body[6] = byte(wMinusOne), byte(wMinusOne>>8), byte(wMinusOne>>16)
	body[7], body[8], body[9] = byte(hMinusOne), byte(hMinusOne>>8), byte(hMinusOne>>16)
	return body
}

// packWebP 按 RIFF 规范打包子块（载荷奇数长度补 1 字节）
func packWebP(parts []riffChunk) []byte {
	body := make([]byte, 0, 64)
	for _, p := range parts {
		var size [4]byte
		binary.LittleEndian.PutUint32(size[:], uint32(len(p.body)))
		body = append(body, p.id...)
		body = append(body, size[:]...)
		body = append(body, p.body...)
		if len(p.body)%2 == 1 {
			body = append(body, 0)
		}
	}

	out := make([]byte, 0, webpHeaderLen+len(body))
	var size [4]byte
	binary.LittleEndian.PutUint32(size[:], uint32(4+len(body))) // "WEBP" + 子块
	out = append(out, "RIFF"...)
	out = append(out, size[:]...)
	out = append(out, "WEBP"...)
	return append(out, body...)
}

func readUint24(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16
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
