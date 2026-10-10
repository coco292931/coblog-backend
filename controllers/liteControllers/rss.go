package liteControllers

import (
	"net/http"
	"net/url"
	"strings"

	"coblog-backend/configs/configReader"
	"coblog-backend/controllers/liteControllers/liteview"
	middleware "coblog-backend/middlewares"

	"github.com/gin-gonic/gin"
)

// RSSPage GET /lite/rss
//
// 与主站 /rss 一致：展示订阅地址，已登录时自动带上该账号的 RSS Token

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

	user := middleware.CurrentAccount(c)
	if user == nil {
		return target
	}
	token := user.RSSToken
	if token == "" {
		return target
	}
	return target + "?token=" + url.QueryEscape(token)
}
