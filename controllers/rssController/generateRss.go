package rssController

import (
	"fmt"
	"net/http"

	"coblog-backend/common/exception"
	configreader "coblog-backend/configs/configReader"
	"coblog-backend/services/articleService"
	"coblog-backend/services/rssService"
	"coblog-backend/services/userService"

	"github.com/gin-gonic/gin"
)

// 默认 RSS 输出条数，config 未配置时使用
const defaultRSSMaxItems = 30

// GenerateRSSHandler 动态生成 RSS Feed
// 支持：token 鉴权决定是否包含深度内容；category / tag 筛选
func GenerateRSSHandler(c *gin.Context) {
	cfg := configreader.GetConfig().Site
	// 图片对外基础地址，同时用于封面绝对化与 RSS 的 atom:link
	imageBase := configreader.GetConfig().FileObject.PublicBaseURL

	// 解析筛选参数（与文章列表保持一致）
	maxItems := cfg.RSSMaxItems
	if maxItems <= 0 {
		maxItems = defaultRSSMaxItems
	}
	params := articleService.RequestParams{
		Page:     1,
		PageSize: uint64(maxItems),
		Category: c.Query("category"),
		Tag:      c.Query("tag"),
	}

	// 根据 RSSToken 鉴权，决定返回 def / deep 内容
	status := resolveRSSStatus(c.Query("token"))

	// 复用文章列表服务，自动按 def/deep 与分类筛选；keepContent=true 以携带全文
	list, err := articleService.GetArticleList(status, params, true)
	if err != nil || list == nil {
		c.Error(exception.SysCannotGetArticle)
		return
	}

	// 服务层已按发布时间倒序（created_at DESC）取最新的 maxItems 篇，此处不再重排

	// 文章 -> RSS Item
	items := make([]*rssService.Item, 0, len(list.Articles))
	for i := range list.Articles {
		items = append(items, rssService.PostToItem(&list.Articles[i], cfg.BaseURL, imageBase))
	}

	meta := rssService.FeedMeta{
		Title:       cfg.Title,
		Link:        cfg.BaseURL,
		Description: cfg.Description,
		Author:      cfg.Author,
		Email:       cfg.Email,
		Created:     rssService.LatestCreated(list.Articles),
		// 保留本次请求的查询串，保证自引用地址与当前内容一致
		SelfURL:  rssService.SelfURL(imageBase, c.Request.URL.RequestURI()),
		Language: "zh-CN",
	}

	rss, err := rssService.GenerateRSS(meta, items)
	if err != nil {
		fmt.Println("生成RSS失败:", err)
		c.Error(exception.SysUknExc)
		return
	}

	c.Data(http.StatusOK, "application/rss+xml; charset=utf-8", []byte(rss))
}

// resolveRSSStatus 根据 RSSToken 判定内容级别：
// 无 token / token 无效 / 无深度权限 -> "def"；具备深度权限 -> "deep"
func resolveRSSStatus(token string) string {
	if token == "" {
		return "def"
	}
	account, err := userService.GetUserByToken(token)
	if err != nil {
		return "def"
	}
	if account.Deepable && account.IsDeep {
		return "deep"
	}
	return "def"
}
