package middleware

import (
	"coblog-backend/common/exception"
	"coblog-backend/common/webtoken"
	"coblog-backend/services/userService"
	"fmt"

	"github.com/gin-gonic/gin"
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

	// 检查账户激活状态，未激活视为访客
	account, err := userService.GetUserByID(uid)
	if err != nil || !userService.IsActivated(account) {
		c.Error(exception.UsrNotActivated)
		c.Abort()
		return
	}

	fmt.Println("鉴权成功")
	c.Set("AccountID", uid)
	c.Set("PermissionGroupID", pgid)
	c.Next()
}

func LooseAuth(c *gin.Context) {
	// 默认未登录状态
	c.Set("AccountID", uint64(0))
	c.Set("PermissionGroupID", uint64(0))

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" || !webtoken.VerifyWt(authHeader) {
		fmt.Println("松鉴权失败: 用户登录无效，已放行")
		c.Next()
		return
	}

	uid, pgid, err := webtoken.GetWtPayload(authHeader)
	if err != nil {
		c.Next()
		return
	}

	// 未激活账户降级为访客
	account, err := userService.GetUserByID(uid)
	if err != nil || !userService.IsActivated(account) {
		fmt.Println("松鉴权: 账户未激活，降级为访客")
		c.Next()
		return
	}

	fmt.Println("松鉴权成功")
	c.Set("AccountID", uid)
	c.Set("PermissionGroupID", pgid)
	c.Next()
}
