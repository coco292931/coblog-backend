package middleware

import (
	"JHETBackend/common/exception"
	"JHETBackend/common/webtoken"

	"github.com/gin-gonic/gin"
	"fmt"
)

func Auth(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.Error(exception.UsrNotLogin)
		c.Abort()
		fmt.Println("鉴权失败: 未登录")
		return
	}
	if !webtoken.VerifyWt(authHeader) {
		c.Error(exception.UsrLoginInvalid)
		c.Abort()
		return
	}
	uid, pgid, err := webtoken.GetWtPayload(authHeader)
	if err != nil {
		c.Error(exception.UsrLoginInvalid)
		c.Abort()
		return
	}
	fmt.Println("鉴权成功")
	c.Set("AccountID", uid)
	c.Set("PermissionGroupID", pgid)
	c.Next()
}

func LooseAuth(c *gin.Context) { //松校验，针对无登录的文章访问情况，为了深度返回账户和权限
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.Set("AccountID", 0)
		c.Set("PermissionGroupID", 0)
		c.Next()
		fmt.Println("松鉴权失败: 未登录，已放行")
		return
	}
	if !webtoken.VerifyWt(authHeader) {
		c.Error(exception.UsrLoginInvalid)
		c.Abort()
		return
	}
	uid, pgid, err := webtoken.GetWtPayload(authHeader)
	if err != nil {
		c.Error(exception.UsrLoginInvalid)
		c.Abort()
		return
	}
	fmt.Println("松鉴权成功")
	c.Set("AccountID", uid)
	c.Set("PermissionGroupID", pgid)
	c.Next()
}
