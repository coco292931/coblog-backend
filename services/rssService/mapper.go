package rssService

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"coblog-backend/models"
)

// 本文件承载「文章模型 → RSS 条目」的纯映射逻辑。
// 放在 rssService 而非 controller，一是职责归属（这是 RSS 领域知识），
// 二是 controller 传递性依赖 database 包，其 init() 会立即建立数据库连接，
// 导致任何在 controller 层的测试都无法运行。

// PostToItem 把文章转换为 RSS 条目。
// Description 放摘要（阅读器列表预览），Content 放全文 HTML（映射为 <content:encoded>）；
// 有封面图时附加 Enclosure，供阅读器作缩略图展示。
func PostToItem(p *models.Post, baseURL, imageBase string) *Item {
	link := fmt.Sprintf("%s/articles/%d", baseURL, p.ID)
	item := &Item{
		Title:       p.Title,
		Link:        link,
		Description: p.Summary,
		Content:     p.Content,
		Created:     p.CreatedAt,
		Updated:     p.UpdatedAt,
		ID:          link,
		Categories:  Categories(p.Category, p.Tags),
	}

	if p.CoverImage != "" {
		coverURL := AbsURL(p.CoverImage, imageBase)
		item.Enclosure = &Enclosure{
			URL:    coverURL,
			Type:   ImageMIME(coverURL),
			Length: "0", // 文件大小未知，RSS 规范允许填 0
		}
	}

	return item
}

// Categories 从文章的分类与标签字段中提取 RSS category。
// 入参为原始字段值（形如 `["tech","music"]`），去重交给 uniqueNonEmpty 处理。
func Categories(rawValues ...string) []string {
	result := make([]string, 0)
	for _, raw := range rawValues {
		result = append(result, ParseStringList(raw)...)
	}
	return result
}

// ParseStringList 解析 JSON 数组字符串；非 JSON 时回退为单元素列表。
func ParseStringList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err == nil {
		return values
	}
	return []string{raw}
}

// LatestCreated 取文章中最新的创建时间作为频道时间，列表为空则返回零值。
// 与条目 pubDate 保持同一口径：若改用 UpdatedAt，编辑旧文章会把频道时间推到现在。
func LatestCreated(posts []models.Post) (t time.Time) {
	for i := range posts {
		if posts[i].CreatedAt.After(t) {
			t = posts[i].CreatedAt
		}
	}
	return t
}

// AbsURL 把相对路径补成基于 base 的绝对 URL；已是绝对地址则原样返回。
func AbsURL(raw, base string) string {
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw
	}
	if base == "" {
		return raw
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(raw, "/")
}

// ImageMIME 按扩展名推断图片 MIME 类型，未知时回退到通用类型。
func ImageMIME(url string) string {
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

// SelfURL 依据请求路径与查询串构造 atom:link 自引用地址。
// 必须保留查询串：带 token 的请求返回 deep 内容，self 若指向无 token 的地址
// 会变成 def 内容，支持自引用校验的阅读器会认为 feed 不一致。
// baseURL 未配置时返回空串，此时调用方不输出 atom:link。
func SelfURL(baseURL, requestURI string) string {
	if baseURL == "" {
		return ""
	}
	// 防御非法取值；正常情况为 "/api/rss" 或以 "/" 开头的路径
	if !strings.HasPrefix(requestURI, "/") {
		return ""
	}
	return AbsURL(requestURI, baseURL)
}
