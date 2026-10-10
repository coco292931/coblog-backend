package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
)

// CSRFFieldName 表单里承载 CSRF token 的隐藏字段名。
//
// 为什么 /lite 需要它：/lite 以 cookie 作登录凭据，而目标设备（老 Kindle，WebKit 534）
// 早于 SameSite 出现，不认识这个属性，跨站表单 POST 照样会带上 cookie。
const CSRFFieldName = "_csrf"

// CSRFToken 由登录 token 派生 CSRF token：HMAC-SHA256(key, session)。
// 无状态：不用存储，登录 token 一换（重新登录）旧表单自然失效。
// session 为空（未登录）时返回空串。
func CSRFToken(key []byte, session string) string {
	if session == "" {
		return ""
	}
	m := hmac.New(sha256.New, key)
	m.Write([]byte(session))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

// CSRFValid 常量时间比对提交上来的 CSRF token。
func CSRFValid(key []byte, session, got string) bool {
	want := CSRFToken(key, session)
	if want == "" || got == "" {
		return false
	}
	return hmac.Equal([]byte(want), []byte(got))
}
