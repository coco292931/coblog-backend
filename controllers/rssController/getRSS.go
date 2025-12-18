package rssController

import (
	"coblog-backend/common/exception"
	"coblog-backend/services/SSRService"
	"coblog-backend/services/userService"
	"fmt"

	"github.com/gin-gonic/gin"
)

func GetRSS(c *gin.Context) {
	if c.Query("token") == "" {
		returnRss(c, "def")
		return
	}
	//剩下的都是有token的
	//校验token是否合法
	accountInfo, err := userService.GetUserByToken(c.Query("token"))
	if err != nil {
		c.Error(exception.UsrTokenInvalid)
		return
	}

	//校验深度权限
	if !(accountInfo.Deepable && accountInfo.IsDeep) {
		//用户无深度权限，返回默认RSS
		returnRss(c, "def")
		return
	}
	//通过深度RSS校验用户权限，返回深度RSS
	returnRss(c, "deep")
}

func returnRss(c *gin.Context, status string) {
	var content string
	var err error
	if status == "def" {
		//默认RSS
		content, err = SSRService.GetDefRSS(c)
	} else if status == "deep" {
		//深度RSS
		content, err = SSRService.GetDeepRSS(c)
	} else {
		fmt.Println("status不准确！")
		c.Error(exception.SysUknExc)
		return
	}
	if err != nil {
		c.Error(exception.SysCannotLoadRssFile)
		return
	}
	// 设置响应头为 XML
	c.Header("Content-Type", "application/xml; charset=utf-8")
	c.String(200, content) //http状态码200，返回内容
}
