package fileController

import (
	"coblog-backend/common/exception"
	"coblog-backend/controllers/accountControllers"
	"coblog-backend/services/fileService"
	"coblog-backend/services/userService"
	"coblog-backend/utils"
	"io"
	"log"
	"mime/multipart"

	//"crypto/md5" hash算法库 <<< 请使用sha256!(MucheXD)

	"github.com/gin-gonic/gin"
)

// UploadFile 处理单文件上传  POST /upload

// var initOnce sync.Once

func UpdateAvatar(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.Error(exception.ApiNoFormFile)
		return
	}
	if fileHeader.Size > int64(1024090) { // 对头像文件限制 1Mib
		c.Error(exception.ApiFileTooLarge)
	}
	fileHandler, err := getFileHandler(fileHeader)
	if err != nil {
		c.Error(err) // 由于 getFileHandler 也使用统一错误，因此可以直接返回
		return
	}
	accountID, err := accountControllers.GetAccountIDFromContext(c)
	if err != nil {
		c.Error(err)
		return
	}
	userService.UploadAvatar(accountID, fileHandler)
}

func UploadImage(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.Error(exception.ApiNoFormFile)
		return
	}
	if fileHeader.Size > int64(10240000) { // 对文件限制 10Mib
		c.Error(exception.ApiFileTooLarge)
	}
	fileHandler, err := getFileHandler(fileHeader)
	if err != nil {
		c.Error(err) // 由于 getFileHandler 也使用统一错误，因此可以直接返回
		return
	}
	//TODO
	fileName, err := fileService.SaveUploadedFile(&fileHandler)
	if err != nil {
		log.Printf("[ERROR][FileSvc] 不能保存图片 %v", err)
		c.Error(exception.ApiFileNotSaved) // 转换成统一错误返回，原error信息丢失
		return
	}
	utils.JsonSuccessResponse(c, "上传成功", gin.H{
		"imageId": fileName,
		"url":     "/static/uploads/" + fileName,
	})

}

func getFileHandler(fileHeader *multipart.FileHeader) (io.Reader, error) {
	// initOnce.Do(initFileController)
	// 打开文件
	fileHandler, err := fileHeader.Open()
	if err != nil {
		return nil, exception.ApiFileCannotOpen
	}

	defer fileHandler.Close() // 返回时关闭文件

	return fileHandler, nil
}
