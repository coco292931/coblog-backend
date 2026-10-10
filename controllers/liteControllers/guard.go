package liteControllers

import (
	"errors"
	"net/http"

	"coblog-backend/common/webtoken"
	middleware "coblog-backend/middlewares"
	"coblog-backend/utils"

	"github.com/gin-gonic/gin"
)

// /lite 的安全守卫，挂在 LooseAuth 之后。
//
// /lite 以 cookie 作登录凭据，而目标设备（老 Kindle，WebKit 534）不认识 SameSite，
// 跨站表单 POST 照样带 cookie；老 WebKit 的表单 POST 也不一定带 Origin 头。
// 所以写操作只能靠表单里的 CSRF token 来确认「这次提交出自本站页面」。

// csrfHeader 脚本（write.js）上传时用请求头带 token，不必先解析整个请求体
const csrfHeader = "X-CSRF-Token"

// csrfExpiredMsg CSRF 校验失败时给用户看的文案。
// 正常用户撞上它的典型情形：页面打开后在别处重新登录过，旧页面里的 token 已过期。
const csrfExpiredMsg = "页面已过期，请重新提交"

// defaultFormBody 普通表单（登录、改密码等）的请求体上限
const defaultFormBody = int64(1 << 20)

// Guard 统一设置安全响应头，并校验 cookie 登录态下 POST 的 CSRF token。
func Guard(c *gin.Context) {
	h := c.Writer.Header()
	// 防点击劫持：老 WebKit 认 X-Frame-Options，新浏览器认 frame-ancestors
	h.Set("X-Frame-Options", "DENY")
	h.Set("Content-Security-Policy", "frame-ancestors 'none'")

	session := sessionToken(c)
	if session != "" {
		// 登录态的页面含邮箱、RSS Token、深度文章等，不让浏览器 / 中间层缓存
		h.Set("Cache-Control", "private, no-store")
	}

	// 只有凭据来自 cookie 时才有 CSRF 风险：跨站请求设不了 Authorization 头。
	// 未登录的 POST（登录、注册、找回密码）没有可被冒用的身份，放行。
	viaCookie, _ := c.Get(middleware.AuthViaCookieKey)
	if c.Request.Method != http.MethodPost || session == "" || viaCookie != true {
		c.Next()
		return
	}

	got := c.GetHeader(csrfHeader)
	if got == "" {
		// 读表单前先限好大小：这里是第一个解析请求体的地方
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, bodyLimit(c))
		err := c.Request.ParseMultipartForm(32 << 20)
		if err != nil && !errors.Is(err, http.ErrNotMultipart) {
			c.Abort()
			bodyParseFail(c, err)
			return
		}
		got = c.Request.PostForm.Get(utils.CSRFFieldName)
	}

	if !utils.CSRFValid(webtoken.CSRFKey(), session, got) {
		c.Abort()
		csrfFail(c)
		return
	}
	c.Next()
}

// sessionToken 当前请求的登录 token；未登录时为空串。
func sessionToken(c *gin.Context) string {
	raw, _ := c.Get(middleware.AuthTokenKey)
	token, _ := raw.(string)
	return token
}

// csrfTokenFor 给页面表单用的 CSRF token；未登录时为空串（模板据此不输出隐藏字段）。
func csrfTokenFor(c *gin.Context) string {
	session := sessionToken(c)
	if session == "" {
		return ""
	}
	return utils.CSRFToken(webtoken.CSRFKey(), session)
}

// bodyLimit 按路由给请求体上限，与各 handler 自己的上限一致。
func bodyLimit(c *gin.Context) int64 {
	switch c.FullPath() {
	case "/lite/upload":
		return uploadBody
	case "/lite/write", "/lite/write/:id":
		return maxWriteBody
	}
	return defaultFormBody
}

// isWriteForm 是否为文章表单的提交（失败时要保住用户已经写的内容）
func isWriteForm(c *gin.Context) bool {
	p := c.FullPath()
	return p == "/lite/write" || p == "/lite/write/:id"
}

// bodyParseFail 请求体读不出来（超限或格式不对）
func bodyParseFail(c *gin.Context, err error) {
	switch {
	case c.FullPath() == "/lite/upload":
		uploadJSON(c, http.StatusOK, uploadParseErrMsg(err), nil)
	case isWriteForm(c):
		renderWriteParseError(c, err)
	default:
		backWithFlash(c, "请求无效，请重新提交")
	}
}

// csrfFail CSRF 校验失败。文章表单原样回填，其余回到对应页面并提示。
func csrfFail(c *gin.Context) {
	switch {
	case c.FullPath() == "/lite/upload":
		uploadJSON(c, http.StatusForbidden, csrfExpiredMsg, nil)
	case isWriteForm(c):
		// 正文可能很长，丢了代价大：用提交上来的内容重渲染，新页面带的是新 token
		view := writeViewFromForm(c)
		view.Error = csrfExpiredMsg + "（选择的图片需要重新选择）"
		renderWrite(c, view)
	default:
		backWithFlash(c, csrfExpiredMsg)
	}
}

// backWithFlash 带着失败提示回到当前 POST 对应的 GET 页面。
func backWithFlash(c *gin.Context, msg string) {
	utils.SetFlash(c, msg, false)
	c.Redirect(http.StatusFound, getPathFor(c))
}

// getPathFor 当前请求对应的 GET 页面。
//
// 登录页的回跳、提示的回跳都要用它：直接用 POST 的地址，
// 对 /lite/me/password 这类只有 POST 的路由会以 GET 打过去，得到 404。
func getPathFor(c *gin.Context) string {
	if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead {
		return c.Request.URL.RequestURI()
	}
	switch c.FullPath() {
	case "/lite/login", "/lite/register", "/lite/forgot-password",
		"/lite/write", "/lite/write/:id", "/lite/write/:id/delete":
		// 这几个 POST 路由都有同地址的 GET 页面
		return c.Request.URL.Path
	case "/lite/logout":
		return "/lite/"
	}
	return "/lite/me"
}
