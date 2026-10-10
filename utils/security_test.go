package utils

import (
	"net/http"
	"strings"
	"testing"
)

func TestCSRFToken(t *testing.T) {
	key := []byte("k1")
	tok := CSRFToken(key, "session-a")
	if tok == "" {
		t.Fatal("已登录时应生成非空 token")
	}
	if CSRFToken(key, "session-a") != tok {
		t.Error("同一会话应得到同一个 token（无状态派生）")
	}
	if CSRFToken(key, "session-b") == tok {
		t.Error("不同会话的 token 不能相同")
	}
	if CSRFToken([]byte("k2"), "session-a") == tok {
		t.Error("换密钥后 token 应变化")
	}
	if CSRFToken(key, "") != "" {
		t.Error("未登录时不应生成 token")
	}
}

func TestCSRFValid(t *testing.T) {
	key := []byte("k1")
	tok := CSRFToken(key, "session-a")
	if !CSRFValid(key, "session-a", tok) {
		t.Error("正确的 token 应通过")
	}
	for _, bad := range []string{"", "x", tok + "x", CSRFToken(key, "session-b")} {
		if CSRFValid(key, "session-a", bad) {
			t.Errorf("token %q 不应通过", bad)
		}
	}
	if CSRFValid(key, "", "") {
		t.Error("未登录时空 token 不应通过")
	}
}

func TestFlashRoundTrip(t *testing.T) {
	c, w := newTestCtx("/lite/me/rss")
	SetFlash(c, "重置成功", true)

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("应写入 1 个 cookie，实际 %d", len(cookies))
	}
	ck := cookies[0]
	if ck.Name != FlashCookieName || !ck.HttpOnly || ck.Path != "/lite" || ck.MaxAge <= 0 {
		t.Errorf("flash cookie 属性不对: %+v", ck)
	}
	if strings.Contains(ck.Value, "重置") {
		t.Error("cookie 值应编码，不能直接放中文")
	}

	c2, w2 := newTestCtx("/lite/me")
	c2.Request.AddCookie(ck)
	msg, ok := TakeFlash(c2)
	if msg != "重置成功" || !ok {
		t.Errorf("TakeFlash = (%q, %v)，期望 (重置成功, true)", msg, ok)
	}
	cleared := w2.Result().Cookies()
	if len(cleared) != 1 || cleared[0].MaxAge >= 0 {
		t.Error("读出后应立即清除 flash cookie")
	}
}

func TestFlashFailureAndEmpty(t *testing.T) {
	c, w := newTestCtx("/lite/me/password")
	SetFlash(c, "用户密码错误", false)
	c2, _ := newTestCtx("/lite/me")
	c2.Request.AddCookie(w.Result().Cookies()[0])
	if msg, ok := TakeFlash(c2); msg != "用户密码错误" || ok {
		t.Errorf("失败提示应原样读出且 ok=false，实际 (%q, %v)", msg, ok)
	}

	c3, w3 := newTestCtx("/lite/me")
	SetFlash(c3, "", true)
	if len(w3.Result().Cookies()) != 0 {
		t.Error("空提示不应写 cookie")
	}

	c4, _ := newTestCtx("/lite/me")
	if msg, ok := TakeFlash(c4); msg != "" || ok {
		t.Error("没有 flash cookie 时应返回空")
	}
}

func TestFlashRejectsForged(t *testing.T) {
	for _, v := range []string{"!!!", "", "MQ", strings.Repeat("A", 2000)} {
		c, _ := newTestCtx("/lite/me")
		c.Request.AddCookie(&http.Cookie{Name: FlashCookieName, Value: v})
		if msg, _ := TakeFlash(c); msg != "" {
			t.Errorf("格式不对的值 %q 不应读出内容，实际 %q", v, msg)
		}
	}
}

func TestIsAllowedOrigin(t *testing.T) {
	cases := map[string]bool{
		"http://localhost":            true,
		"http://localhost:5173":       true,
		"https://127.0.0.1:8080":      true,
		"http://192.168.1.20:5173":    true,
		"https://coco-29.wang":        true,
		"https://blog.coco-29.wang":   true,
		"https://a.b.coco-29.wang":    true,
		"http://localhost.evil.com":   false,
		"http://127.0.0.1.evil.com":   false,
		"http://192.168.evil.com":     false,
		"http://192.168.1.1.evil.com": false,
		"https://evilcoco-29.wang":    false,
		"https://coco-29.wang.evil":   false,
		"http://10.0.0.1":             false,
		"null":                        false,
		"":                            false,
		"ftp://localhost":             false,
		"http://localhost/path":       false,
		"http://user@localhost":       false,
	}
	for origin, want := range cases {
		if got := IsAllowedOrigin(origin); got != want {
			t.Errorf("IsAllowedOrigin(%q) = %v，期望 %v", origin, got, want)
		}
	}
}
