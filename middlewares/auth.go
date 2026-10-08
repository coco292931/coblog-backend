package middleware

import (
	"coblog-backend/common/exception"
	"coblog-backend/common/webtoken"
	"coblog-backend/utils"
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
	c.Set("AccountID", uid)
	c.Set("PermissionGroupID", pgid)
	c.Next()
}

func LooseAuth(c *gin.Context) {
	// 默认未登录状态
	c.Set("AccountID", uint64(0))
	c.Set("PermissionGroupID", uint64(0))

	token := c.GetHeader("Authorization")
	if token == "" {
		// 浏览器地址栏导航（后端直出的 /lite 就是这种）带不了自定义头，
		// 所以兜底读一次 cookie —— cookie 由登录接口写入。
		// 注意只有松鉴权会读 cookie：写操作仍然以 Authorization 头为凭据，
		// 这样跨站请求即使带上 cookie 也无法触发写操作。
		token = utils.AuthTokenFromCookie(c)
	}
	if token == "" || !webtoken.VerifyWt(token) {
		fmt.Println("松鉴权失败: 用户登录无效，已放行")
		c.Next()
		return
	}

	uid, pgid, err := webtoken.GetWtPayload(token)
	if err != nil {
		c.Next()
		return
	}

	fmt.Println("松鉴权成功")
	c.Set("AccountID", uid)
	c.Set("PermissionGroupID", pgid)
	c.Next()
}
