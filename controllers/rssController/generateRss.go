package rssController

import (
	"fmt"
	"net/http"
	"time"

	"coblog-backend/common/exception"
	configreader "coblog-backend/configs/configReader"
	"coblog-backend/models"
	"coblog-backend/services/articleService"
	"coblog-backend/services/rssService"
	"coblog-backend/services/userService"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/feeds"
)

// 默认 RSS 输出条数，config 未配置时使用
const defaultRSSMaxItems = 30

// GenerateRSSHandler 动态生成 RSS Feed
// 支持：token 鉴权决定是否包含深度内容；category / tag 筛选
func GenerateRSSHandler(c *gin.Context) {
	cfg := configreader.GetConfig().Site

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

	// 复用文章列表服务，自动按 def/deep 与分类筛选
	list, err := articleService.GetArticleList(status, params)
	if err != nil || list == nil {
		c.Error(exception.SysCannotGetArticle)
		return
	}

	// 文章 -> RSS Item
	items := make([]*feeds.Item, 0, len(list.Articles))
	for i := range list.Articles {
		items = append(items, postToItem(&list.Articles[i], cfg.BaseURL))
	}

	meta := rssService.FeedMeta{
		Title:       cfg.Title,
		Link:        cfg.BaseURL,
		Description: cfg.Description,
		Author:      cfg.Author,
		Email:       cfg.Email,
		Created:     latestCreated(list.Articles),
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

// postToItem 把文章转换为 RSS 条目
func postToItem(p *models.Post, baseURL string) *feeds.Item {
	return &feeds.Item{
		Title:       p.Title,
		Link:        &feeds.Link{Href: fmt.Sprintf("%s/articles/%d", baseURL, p.ID)},
		Description: p.Summary,
		Created:     p.CreatedAt,
		Updated:     p.UpdatedAt,
		Id:          fmt.Sprintf("%s/articles/%d", baseURL, p.ID),
	}
}

// latestCreated 取文章中最新的创建时间作为频道时间，列表为空则用零值
func latestCreated(posts []models.Post) (t time.Time) {
	for i := range posts {
		if posts[i].CreatedAt.After(t) {
			t = posts[i].CreatedAt
		}
	}
	return t
}
