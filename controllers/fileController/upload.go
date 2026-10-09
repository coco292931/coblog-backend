package fileController

import (
	"bytes"
	"coblog-backend/common/exception"
	configreader "coblog-backend/configs/configReader"
	"coblog-backend/controllers/accountControllers"
	"coblog-backend/services/fileService"
	"coblog-backend/services/userService"
	"coblog-backend/utils"
	"errors"
	"io"
	"log"
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

func UpdateAvatar(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.Error(exception.ApiNoFormFile)
		return
	}
	if fileHeader.Size > int64(1024090) { // 头像限制 1 MiB
		c.Error(exception.ApiFileTooLarge)
		return
	}
	data, _, err := readFileData(fileHeader)
	if err != nil {
		c.Error(err)
		return
	}
	accountID, err := accountControllers.GetAccountIDFromContext(c)
	if err != nil {
		c.Error(err)
		return
	}
	if err := userService.UploadAvatar(accountID, toReader(data)); err != nil {
		c.Error(err)
		return
	}
	utils.JsonSuccessResponse(c, "上传成功", nil)
}

func UploadImage(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.Error(exception.ApiNoFormFile)
		return
	}
	result, err := saveImage(fileHeader)
	if err != nil {
		c.Error(err)
		return
	}

	// url 指原图：正文里存它，展示时加 ?thumb=1 由服务端换成压缩图；
	// thumb_url 就是压缩图地址，没有压缩图时服务端自动回退到原图。
	originalURL := PublicImageURL(result.OriginalName)
	utils.JsonSuccessResponse(c, "上传成功", gin.H{
		"id":         result.OriginalName,
		"url":        originalURL,
		"thumb_url":  originalURL + "?thumb=1",
		"compressed": result.CompressedName != "",
	})
}

// maxImageSize 单张图片的大小上限（10 MiB）
const maxImageSize = int64(10240000)

// saveImage 校验大小并保存一张上传的图片（含压缩），返回存储结果。
func saveImage(fileHeader *multipart.FileHeader) (fileService.ImageSaveResult, error) {
	if fileHeader.Size > maxImageSize {
		return fileService.ImageSaveResult{}, exception.ApiFileTooLarge
	}
	data, _, err := readFileData(fileHeader)
	if err != nil {
		return fileService.ImageSaveResult{}, err
	}

	// 图片类型由服务层按内容判定，不再传文件名后缀
	result, err := fileService.SaveImageWithCompression(data)
	if err != nil {
		var apiErr *exception.Exception
		if errors.As(err, &apiErr) {
			// 类型不支持、像素超限这类可以直接告诉用户
			return fileService.ImageSaveResult{}, err
		}
		log.Printf("[ERROR][FileSvc] 不能保存图片 %v", err)
		return fileService.ImageSaveResult{}, exception.ApiFileNotSaved
	}
	return result, nil
}

// SaveImageFile 保存一张图片并返回原图的对外 URL。
// 给 /lite 写作页的表单上传用：它没有脚本，不能调 /api/upload/image，
// 图片随文章表单一起以 multipart 提交。
func SaveImageFile(fileHeader *multipart.FileHeader) (string, error) {
	result, err := saveImage(fileHeader)
	if err != nil {
		return "", err
	}
	return PublicImageURL(result.OriginalName), nil
}

// PublicImageURL 拼接上传图片对外可访问的 URL。
// 用绝对地址：RSS、跨域等场景需要；站内浏览器也兼容。
// public_base_url 未配置时退化为相对路径，保证站内仍可用。
// /lite 写作页的上传也走这里，两边存进文章的地址格式一致。
func PublicImageURL(name string) string {
	baseURL := strings.TrimRight(configreader.GetConfig().FileObject.PublicBaseURL, "/")
	return baseURL + "/static/uploads/" + name
}

// readFileData 读取 multipart 文件的全部字节并返回小写扩展名
func readFileData(fileHeader *multipart.FileHeader) ([]byte, string, error) {
	f, err := fileHeader.Open()
	if err != nil {
		return nil, "", exception.ApiFileCannotOpen
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, "", exception.ApiFileCannotOpen
	}

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	return data, ext, nil
}

func toReader(data []byte) io.Reader {
	return bytes.NewReader(data)
}
