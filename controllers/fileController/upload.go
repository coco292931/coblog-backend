package fileController

import (
	"bytes"
	"coblog-backend/common/exception"
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

	// 优先返回压缩图 URL；无压缩图时返回原图
	serveURL := "/static/uploads/" + result.OriginalName
	if result.CompressedName != "" {
		serveURL = "/static/uploads/" + result.CompressedName
	}

	utils.JsonSuccessResponse(c, "上传成功", gin.H{
		"imageId":       result.OriginalName,
		"url":           serveURL,
		"original_url":  "/static/uploads/" + result.OriginalName,
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
