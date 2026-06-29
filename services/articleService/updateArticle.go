package articleService

import (
	"unicode/utf8"

	"coblog-backend/common/exception"
	"coblog-backend/configs/database"
	"coblog-backend/models"
	"coblog-backend/services/markdownService"
)

// UpdatePostInput 更新文章的输入参数（整体替换，PUT 语义）
type UpdatePostInput struct {
	Title      string
	Subtitle   string
	Summary    string
	CoverImage string
	Content    string // 富文本(HTML)，纯 html 文章使用
	MdContent  string // Markdown，markdown 文章使用
	Category   string // JSON 数组字符串
	Tags       string // JSON 数组字符串
	IsDeep     bool
}

// UpdatePost 更新一篇文章
// 内容处理规则与创建一致（Markdown 为单一信源）：
//   - 提供了 MdContent 时，Content(HTML) 一律由 goldmark 重新渲染
//   - 未提供 MdContent 时，按纯 HTML 文章处理，直接保存 Content
func UpdatePost(id string, input UpdatePostInput) (*models.Post, error) {
	var post models.Post
	if err := database.DataBase.First(&post, id).Error; err != nil {
		return nil, exception.SysCannotGetArticle
	}

	content := input.Content
	if input.MdContent != "" {
		html, err := markdownService.ParseMarkdownToHTML(input.MdContent)
		if err != nil {
			return nil, err
		}
		content = html
	}

	wordSource := input.MdContent
	if wordSource == "" {
		wordSource = content
	}

	// 整体替换字段
	post.Title = input.Title
	post.Subtitle = input.Subtitle
	post.Summary = input.Summary
	post.CoverImage = input.CoverImage
	post.Content = content
	post.MdContent = input.MdContent
	post.Category = input.Category
	post.Tags = input.Tags
	post.IsDeep = input.IsDeep
	post.Words = uint64(utf8.RuneCountInString(wordSource))

	if err := database.DataBase.Save(&post).Error; err != nil {
		return nil, err
	}

	return &post, nil
}

// DeletePost 删除一篇文章
func DeletePost(id string) error {
	result := database.DataBase.Delete(&models.Post{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return exception.SysCannotGetArticle
	}
	return nil
}
