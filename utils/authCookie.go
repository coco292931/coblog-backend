package utils

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// AuthCookieName 承载登录 token 的 cookie 名。
//
// 为什么需要它：token 平时走 Authorization 头（前端 axios 注入），
// 但浏览器地址栏导航带不了自定义头 —— 后端直出的 /lite 就是这种情况。
// 没有 cookie 时，/lite 永远只能以匿名身份取内容，深度文章一篇都看不到。
const AuthCookieName = "coblog_token"

// 未显式给出有效期时的兜底值，与 account.valid_secs 的默认量级一致。
const defaultAuthCookieSecs = 30 * 24 * 3600

// SetAuthCookie 写入登录 cookie。maxAgeSecs 应当与 token 自身的有效期一致。
func SetAuthCookie(c *gin.Context, token string, maxAgeSecs uint64) {
	if token == "" {
		return
	}
	maxAge := int(maxAgeSecs)
	if maxAge <= 0 {
		maxAge = defaultAuthCookieSecs
	}

	http.SetCookie(c.Writer, &http.Cookie{
		Name:  AuthCookieName,
		Value: token,
		Path:  "/",
		// 不给 JS 读：token 是长周期凭证，少一条被 XSS 取走的路
		HttpOnly: true,
		// Lax：站内导航会带上，跨站 POST 不带，构成最基本的 CSRF 防护。
		// 老设备若不认识这个属性会直接忽略，忽略后行为仍等同「同站发送」。
		SameSite: http.SameSiteLaxMode,
		Secure:   requestIsHTTPS(c),
		MaxAge:   maxAge,
		Expires:  time.Now().Add(time.Duration(maxAge) * time.Second),
	})
}

// ClearAuthCookie 清除登录 cookie（登出）。
//
// 注意：cookie 是 HttpOnly 的，前端 JS 删不掉，所以登出必须经过后端。
func ClearAuthCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     AuthCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   requestIsHTTPS(c),
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

// AuthTokenFromCookie 从请求里读登录 token，没有则返回空串。
func AuthTokenFromCookie(c *gin.Context) string {
	ck, err := c.Request.Cookie(AuthCookieName)
	if err != nil || ck == nil {
		return ""
	}
	return strings.TrimSpace(ck.Value)
}

// requestIsHTTPS 判断当前请求是否走 HTTPS。
//
// 生产是 nginx 反代，后端看到的仍是 http，所以还要看 X-Forwarded-Proto。
// 判错的代价不对称：本机 http 上误加 Secure，cookie 会根本发不出去；
// 反过来漏加，只是少一层传输层保护。
func requestIsHTTPS(c *gin.Context) bool {
	if c.Request.TLS != nil {
		return true
	}
	return strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
}
