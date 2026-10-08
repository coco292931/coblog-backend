package liteControllers

import (
	"net/http"
	"net/url"
	"strings"

	"coblog-backend/configs/configReader"
	"coblog-backend/controllers/accountControllers"
	"coblog-backend/controllers/liteControllers/liteview"

	"github.com/gin-gonic/gin"
)

// RSSPage GET /lite/rss
//
// 与主站 /rss 一致：展示订阅地址，已登录时自动带上该账号的 RSS Token
// （主站 getRSSUrl 也是这么拼的：/api/rss?token=...）。
// 唯一的降级是没有「复制」按钮 —— 剪贴板 API 在无脚本环境下拿不到。
func RSSPage(c *gin.Context) {
	liteview.Render(c, http.StatusOK, "rss", liteview.RSSView{
		BaseView: newBaseView(c),
		URL:      rssSubscribeURL(c),
	})
}

// rssSubscribeURL 拼订阅地址。基址用配置里的 PublicBaseURL（接口域名）。
func rssSubscribeURL(c *gin.Context) string {
	base := strings.TrimRight(configreader.GetConfig().FileObject.PublicBaseURL, "/")
	target := base + "/api/rss"

	accountID, err := accountControllers.GetAccountIDFromContext(c)
	if err != nil || accountID == 0 {
		return target
	}
	token := loadMeView(c, accountID).RSSToken
	if token == "" {
		return target
	}
	return target + "?token=" + url.QueryEscape(token)
}
