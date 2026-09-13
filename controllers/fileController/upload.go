package fileController

import (
	"bytes"
	"coblog-backend/common/exception"
	configreader "coblog-backend/configs/configReader"
	"coblog-backend/controllers/accountControllers"
	"coblog-backend/services/fileService"
	"coblog-backend/services/userService"
	"coblog-backend/utils"
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
	if fileHeader.Size > int64(10240000) { // 图片限制 10 MiB
		c.Error(exception.ApiFileTooLarge)
		return
	}
	data, ext, err := readFileData(fileHeader)
	if err != nil {
		c.Error(err)
		return
	}

	result, err := fileService.SaveImageWithCompression(data, ext)
	if err != nil {
		log.Printf("[ERROR][FileSvc] 不能保存图片 %v", err)
		c.Error(exception.ApiFileNotSaved)
		return
	}

	// 拼接对外可访问的绝对 URL（RSS、跨域等场景需要绝对地址；站内浏览器也兼容）
	// public_base_url 未配置时退化为相对路径，保证站内仍可用
	baseURL := strings.TrimRight(configreader.GetConfig().FileObject.PublicBaseURL, "/")
	buildURL := func(name string) string {
		return baseURL + "/static/uploads/" + name
	}

	// url 指原图：正文里存它，展示时加 ?thumb=1 由服务端换成压缩图；
	// thumb_url 就是压缩图地址，没有压缩图时服务端自动回退到原图。
	originalURL := buildURL(result.OriginalName)
	utils.JsonSuccessResponse(c, "上传成功", gin.H{
		"id":         result.OriginalName,
		"url":        originalURL,
		"thumb_url":  originalURL + "?thumb=1",
		"compressed": result.CompressedName != "",
	})
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
