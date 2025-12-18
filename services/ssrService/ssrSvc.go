package ssrService

import (
	"fmt"
	"io"
	"os"

	"github.com/gin-gonic/gin"
)

func GetDefRSS(c *gin.Context) (string, error) {
	// 读取 RSS
	filePath := "./rss_def_feed.xml"
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Println("无法打开默认RSS", err)
		return "", err
	}
	defer file.Close()

	return loadFile(file)
}

func GetDeepRSS(c *gin.Context) (string, error) {
	// 读取 RSS
	filePath := "./rss_deep_feed.xml"
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Println("无法打开深度RSS", err)
		return "", err
	}
	defer file.Close()

	return loadFile(file)
}

func loadFile(file *os.File) (string, error) {
	content, err := io.ReadAll(file)
	if err != nil {
		fmt.Println("无法读取RSS内容", err)
		return "", err
	}
	return string(content), nil

}
