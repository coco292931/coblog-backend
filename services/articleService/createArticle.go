package articleService

import (
	"unicode/utf8"

	"coblog-backend/configs/database"
	"coblog-backend/models"
	"coblog-backend/services/markdownService"
)

// CreatePostInput 创建文章的输入参数
type CreatePostInput struct {
	Title      string
	Subtitle   string
	Summary    string
	CoverImage string
	Content    string // 富文本(HTML)内容，可选
	MdContent  string // Markdown 内容，可选
	Category   string // JSON 数组字符串，如 ["tech","music"]
	Tags       string // JSON 数组字符串
	IsDeep     bool
	Hidden     bool // true=对所有人隐藏
	NoStats    bool // true=不计入站点统计
}

// CreatePost 创建一篇文章
// 内容处理规则（Markdown 为单一信源）：
//   - 提供了 MdContent 时，Content(HTML) 一律由 goldmark 渲染得到，忽略传入的 Content
//   - 未提供 MdContent 时，按纯 HTML 文章处理，直接保存传入的 Content
//   - Words 根据 Markdown（优先）或 HTML 内容的字符数估算
func CreatePost(input CreatePostInput) (*models.Post, error) {
	content := input.Content

	// Markdown 优先且唯一信源：有 md 就以渲染结果为准，覆盖传入的 html
	if input.MdContent != "" {
		html, err := markdownService.ParseMarkdownToHTML(input.MdContent)
		if err != nil {
			return nil, err
		}
		content = html
	}

	// 字数统计：优先按 Markdown 原文，否则按 HTML
	wordSource := input.MdContent
	if wordSource == "" {
		wordSource = content
	}

	post := models.Post{
		Title:      input.Title,
		Subtitle:   input.Subtitle,
		Summary:    input.Summary,
		CoverImage: input.CoverImage,
		Content:    content,
		MdContent:  input.MdContent,
		Category:   input.Category,
		Tags:       input.Tags,
		IsDeep:     input.IsDeep,
		Hidden:     input.Hidden,
		NoStats:    input.NoStats,
		Words:      uint64(utf8.RuneCountInString(wordSource)),
	}

	if err := database.DataBase.Create(&post).Error; err != nil {
		return nil, err
	}

	// 文章数变化，刷新站点统计（失败不影响发文）
	refreshSiteInfo()

	return &post, nil
}
