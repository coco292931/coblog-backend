package middleware

import (
	"coblog-backend/common/exception"
	"coblog-backend/common/webtoken"
	"coblog-backend/utils"
	"fmt"

	"github.com/gin-gonic/gin"
)

// LooseAuth 鉴权成功时额外放在 context 上的两项
const (
	AuthTokenKey     = "AuthToken"     // string：本次请求使用的登录 token
	AuthViaCookieKey = "AuthViaCookie" // bool：token 是否取自 cookie
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
	viaCookie := false
	if token == "" {
		// 浏览器地址栏导航（后端直出的 /lite 就是这种）带不了自定义头，
		// 所以兜底读一次 cookie —— cookie 由登录接口写入。
		// 注意只有松鉴权会读 cookie：/api 的写操作仍然以 Authorization 头为凭据。
		// /lite 的写操作以 cookie 为凭据，由 liteControllers.Guard 另做 CSRF 校验。
		token = utils.AuthTokenFromCookie(c)
		viaCookie = true
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
	// 供 /lite 派生 CSRF token：只有凭据来自 cookie 时才需要防 CSRF
	// （跨站请求设不了 Authorization 头）
	c.Set(AuthTokenKey, token)
	c.Set(AuthViaCookieKey, viaCookie)
	c.Next()
}
