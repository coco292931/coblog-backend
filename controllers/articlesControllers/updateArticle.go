package articlesControllers

import (
	"errors"
	"fmt"
	"strings"

	"coblog-backend/common/exception"
	"coblog-backend/services/articleService"
	"coblog-backend/utils"

	"github.com/gin-gonic/gin"
)

// UpdateArticleRequest 更新文章的请求体（整体替换，PUT 语义）
type UpdateArticleRequest struct {
	Title      string `json:"title"`
	Subtitle   string `json:"subtitle"`
	Summary    string `json:"summary"`
	CoverImage string `json:"cover_image"`
	Content    string `json:"content"`    // 富文本(HTML)，可选
	MdContent  string `json:"md_content"` // Markdown，可选
	Category   string `json:"category"`   // JSON 数组字符串
	Tags       string `json:"tags"`       // JSON 数组字符串
	IsDeep     bool   `json:"is_deep"`
}

// UpdateArticle 更新文章 PUT /api/articles/:id
// 需要 Perm_PostPost 权限（由路由中间件保证）
func UpdateArticle(c *gin.Context) {
	var req UpdateArticleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fmt.Println("更新文章参数错误:", err)
		c.Error(exception.ApiParamError)
		return
	}

	if strings.TrimSpace(req.Title) == "" {
		c.Error(exception.ApiParamError)
		return
	}
	if strings.TrimSpace(req.Content) == "" && strings.TrimSpace(req.MdContent) == "" {
		c.Error(exception.ApiParamError)
		return
	}

	post, err := articleService.UpdatePost(c.Param("id"), articleService.UpdatePostInput{
		Title:      req.Title,
		Subtitle:   req.Subtitle,
		Summary:    req.Summary,
		CoverImage: req.CoverImage,
		Content:    req.Content,
		MdContent:  req.MdContent,
		Category:   req.Category,
		Tags:       req.Tags,
		IsDeep:     req.IsDeep,
	})
	if err != nil {
		if errors.Is(err, exception.SysCannotGetArticle) {
			c.Error(exception.SysCannotGetArticle)
			return
		}
		fmt.Println("更新文章失败:", err)
		c.Error(exception.SysCannotSaveArticle)
		return
	}

	utils.JsonSuccessResponse(c, "更新成功", post)
}

// DeleteArticle 删除文章 DELETE /api/articles/:id
// 需要 Perm_PostPost 权限（由路由中间件保证）
func DeleteArticle(c *gin.Context) {
	if err := articleService.DeletePost(c.Param("id")); err != nil {
		if errors.Is(err, exception.SysCannotGetArticle) {
			c.Error(exception.SysCannotGetArticle)
			return
		}
		fmt.Println("删除文章失败:", err)
		c.Error(exception.SysCannotSaveArticle)
		return
	}

	utils.JsonSuccessResponse(c, "删除成功", nil)
}

// GetArticleForEdit 获取文章完整内容（含 Markdown 原文）供编辑回填 GET /api/articles/:id/edit
// 需要 Perm_PostPost 权限（由路由中间件保证）
func GetArticleForEdit(c *gin.Context) {
	article, err := articleService.GetArticleForEdit(c.Param("id"))
	if err != nil {
		c.Error(exception.SysCannotGetArticle)
		return
	}
	utils.JsonSuccessResponse(c, "获取成功", article)
}
