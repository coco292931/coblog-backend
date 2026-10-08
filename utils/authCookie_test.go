package utils

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// 这一组只测 cookie 本身的读写与安全属性。
// LooseAuth 如何用它（以及只有松鉴权读 cookie 这条约定）由端到端验证覆盖 ——
// middlewares 包传递性依赖 database，包内单测会连库。

func newTestCtx(target string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, target, nil)
	return c, w
}

func TestSetAuthCookieWritesHttpOnlyCookie(t *testing.T) {
	c, w := newTestCtx("/api/auth/login/combo")
	SetAuthCookie(c, "tok-123", 3600)

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("应写入 1 个 cookie，实际 %d", len(cookies))
	}

	ck := cookies[0]
	if ck.Name != AuthCookieName {
		t.Errorf("cookie 名 = %q，期望 %q", ck.Name, AuthCookieName)
	}
	if ck.Value != "tok-123" {
		t.Errorf("cookie 值 = %q，期望 tok-123", ck.Value)
	}
	if !ck.HttpOnly {
		t.Error("必须是 HttpOnly：token 不该被 JS 读到（XSS 时少一条路）")
	}
	if ck.Path != "/" {
		t.Errorf("Path = %q，期望 / —— 否则 /lite 收不到这个 cookie", ck.Path)
	}
	if ck.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v，期望 Lax（站内导航会带、跨站 POST 不带）", ck.SameSite)
	}
	if ck.Secure {
		t.Error("本机 http 下不应带 Secure，否则浏览器根本不会把 cookie 发回来")
	}
	if ck.MaxAge != 3600 {
		t.Errorf("MaxAge = %d，期望 3600（应与 token 有效期一致）", ck.MaxAge)
	}
}

func TestSetAuthCookieSecureBehindTLSProxy(t *testing.T) {
	c, w := newTestCtx("/api/auth/login/combo")
	c.Request.Header.Set("X-Forwarded-Proto", "https")
	SetAuthCookie(c, "tok", 60)

	if ck := w.Result().Cookies()[0]; !ck.Secure {
		t.Error("反代终止 TLS 时（X-Forwarded-Proto: https）应带 Secure")
	}
}

func TestSetAuthCookieSkipsEmptyToken(t *testing.T) {
	c, w := newTestCtx("/api/auth/login/combo")
	SetAuthCookie(c, "", 60)

	if got := len(w.Result().Cookies()); got != 0 {
		t.Errorf("空 token 不应写 cookie，实际写了 %d 个", got)
	}
}

func TestSetAuthCookieFallsBackToDefaultMaxAge(t *testing.T) {
	c, w := newTestCtx("/api/auth/login/combo")
	SetAuthCookie(c, "tok", 0)

	if ck := w.Result().Cookies()[0]; ck.MaxAge != defaultAuthCookieSecs {
		t.Errorf("未给出有效期时应回落到默认值 %d，实际 %d", defaultAuthCookieSecs, ck.MaxAge)
	}
}

func TestClearAuthCookie(t *testing.T) {
	c, w := newTestCtx("/api/auth/logout")
	ClearAuthCookie(c)

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("应写入 1 个 cookie，实际 %d", len(cookies))
	}

	ck := cookies[0]
	if ck.Name != AuthCookieName {
		t.Errorf("cookie 名 = %q，期望 %q", ck.Name, AuthCookieName)
	}
	if ck.Value != "" {
		t.Errorf("登出时 cookie 值应为空，实际 %q", ck.Value)
	}
	if ck.MaxAge >= 0 {
		t.Errorf("登出应把 MaxAge 置为负数，实际 %d", ck.MaxAge)
	}
	if !ck.HttpOnly {
		t.Error("清 cookie 时的属性应与写入时一致，否则可能删不掉")
	}
}

func TestAuthTokenFromCookie(t *testing.T) {
	c, _ := newTestCtx("/lite/")
	if got := AuthTokenFromCookie(c); got != "" {
		t.Errorf("没有 cookie 时应返回空串，实际 %q", got)
	}

	c2, _ := newTestCtx("/lite/")
	c2.Request.AddCookie(&http.Cookie{Name: AuthCookieName, Value: "tok-9"})
	if got := AuthTokenFromCookie(c2); got != "tok-9" {
		t.Errorf("应读到 tok-9，实际 %q", got)
	}

	// 名字不对的 cookie 不应被当成凭证
	c3, _ := newTestCtx("/lite/")
	c3.Request.AddCookie(&http.Cookie{Name: "other", Value: "nope"})
	if got := AuthTokenFromCookie(c3); got != "" {
		t.Errorf("无关 cookie 不应被读取，实际 %q", got)
	}
}
