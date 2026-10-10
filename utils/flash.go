package utils

import (
	"encoding/base64"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

// FlashCookieName 一次性提示（PRG 的 POST → 302 → GET 之间捎带结果文案）的 cookie 名。
//
// 为什么不放查询串：GET 页分不清 ?msg= 是自己重定向时写的还是别人构造的，
// 任何人都能发一条 /lite/me?ok=1&msg=任意文字 的链接，让官方页面以成功样式显示它。
const FlashCookieName = "coblog_flash"

const (
	flashPath   = "/lite"
	flashMaxAge = 60 // 秒：只需撑过一次重定向
	// flashMaxRunes 读出时的长度上限，防止被塞进超长文本
	flashMaxRunes = 200
)

// SetFlash 写入一条一次性提示，ok 决定提示框样式（成功 / 失败）。msg 为空时不写。
func SetFlash(c *gin.Context, msg string, ok bool) {
	if msg == "" {
		return
	}
	flag := "0"
	if ok {
		flag = "1"
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name: FlashCookieName,
		// base64url：cookie 值里不能直接放中文与分号
		Value:    base64.RawURLEncoding.EncodeToString([]byte(flag + msg)),
		Path:     flashPath,
		MaxAge:   flashMaxAge,
		Expires:  time.Now().Add(flashMaxAge * time.Second),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   requestIsHTTPS(c),
	})
}

// TakeFlash 读出并清除一次性提示；没有或格式不对时返回空串。
func TakeFlash(c *gin.Context) (msg string, ok bool) {
	ck, err := c.Request.Cookie(FlashCookieName)
	if err != nil || ck == nil {
		return "", false
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     FlashCookieName,
		Value:    "",
		Path:     flashPath,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   requestIsHTTPS(c),
	})

	raw, err := base64.RawURLEncoding.DecodeString(ck.Value)
	if err != nil || len(raw) < 2 || (raw[0] != '0' && raw[0] != '1') {
		return "", false
	}
	msg = string(raw[1:])
	if !utf8.ValidString(msg) || utf8.RuneCountInString(msg) > flashMaxRunes {
		return "", false
	}
	return msg, raw[0] == '1'
}
