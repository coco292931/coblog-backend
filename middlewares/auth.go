package middleware

import (
	"coblog-backend/common/exception"
	"coblog-backend/common/webtoken"

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
	fmt.Println("鉴权成功")
	//fmt.Println("用户ID:", uid, "权限组ID:", pgid)
	c.Set("AccountID", uid)
	c.Set("PermissionGroupID", pgid)
	c.Next()
}

func LooseAuth(c *gin.Context) { //松校验，针对无登录的文章访问情况，为了深度返回账户和权限
	authHeader := c.GetHeader("Authorization")

	// 默认设置为未登录状态
	//c.Set("IsAuthenticated", false)
	c.Set("AccountID", uint64(0))
	c.Set("PermissionGroupID", uint64(0))

	// 如果有token且验证通过，才设置真实信息
	if authHeader != "" && webtoken.VerifyWt(authHeader) {
		uid, pgid, err := webtoken.GetWtPayload(authHeader)
		if err == nil {
			fmt.Println("松鉴权成功")
			//c.Set("IsAuthenticated", true)
			c.Set("AccountID", uid)
			c.Set("PermissionGroupID", pgid)
		}
	}else {
		fmt.Println("松鉴权失败: 用户登录无效，已放行")
	}

	c.Next()
}
