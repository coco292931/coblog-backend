package rssController

import (
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
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
	// 图片对外基础地址，用于把封面等相对路径补成绝对 URL
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

	// 文章按 ID 降序排列（新文章在前，避免依赖 pubDate 排序及 DB 返回顺序）
	sort.Slice(list.Articles, func(i, j int) bool {
		return list.Articles[i].ID > list.Articles[j].ID
	})

	// 文章 -> RSS Item
	items := make([]*feeds.Item, 0, len(list.Articles))
	for i := range list.Articles {
		items = append(items, postToItem(&list.Articles[i], cfg.BaseURL, imageBase))
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
// Description 放摘要（阅读器列表预览），Content 放全文 HTML（映射为 <content:encoded>）
// 有封面图时附加 <enclosure>，供阅读器作缩略图展示
func postToItem(p *models.Post, baseURL, imageBase string) *feeds.Item {
	link := fmt.Sprintf("%s/articles/%d", baseURL, p.ID)
	item := &feeds.Item{
		Title:       p.Title,
		Link:        &feeds.Link{Href: link},
		Description: p.Summary,
		Content:     p.Content,
		Created:     p.CreatedAt,
		Updated:     p.UpdatedAt,
		Id:          link,
	}

	if p.CoverImage != "" {
		coverURL := absURL(p.CoverImage, imageBase)
		item.Enclosure = &feeds.Enclosure{
			Url:    coverURL,
			Type:   imageMIME(coverURL),
			Length: "0", // 文件大小未知，RSS 规范允许填 0；gorilla/feeds 要求非空才渲染
		}
	}

	return item
}

// absURL 把相对路径（以 / 开头）补成基于 base 的绝对 URL；已是绝对地址则原样返回
func absURL(raw, base string) string {
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw
	}
	if base == "" {
		return raw
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(raw, "/")
}

// imageMIME 按扩展名推断图片 MIME 类型，未知时回退到通用类型
func imageMIME(url string) string {
	switch strings.ToLower(filepath.Ext(url)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
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
