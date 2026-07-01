package articlesControllers

import (
	"fmt"
	"strings"

	"coblog-backend/common/exception"
	"coblog-backend/services/articleService"
	"coblog-backend/utils"

	"github.com/gin-gonic/gin"
)

// CreateArticleRequest 创建文章的请求体
type CreateArticleRequest struct {
	Title      string `json:"title"`
	Subtitle   string `json:"subtitle"`
	Summary    string `json:"summary"`
	CoverImage string `json:"cover_image"`
	Content    string `json:"content"`    // 富文本(HTML)，可选
	MdContent  string `json:"md_content"` // Markdown，可选
	Category   string `json:"category"`   // JSON 数组字符串
	Tags       string `json:"tags"`       // JSON 数组字符串
	IsDeep     bool   `json:"is_deep"`
	Hidden     bool   `json:"hidden"`   // true=对所有人隐藏
	NoStats    bool   `json:"no_stats"` // true=不计入站点统计
}

// CreateArticle 创建文章 POST /api/articles
// 需要 Perm_PostPost 权限（由路由中间件保证）
func CreateArticle(c *gin.Context) {
	var req CreateArticleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fmt.Println("创建文章参数错误:", err)
		c.Error(exception.ApiParamError)
		return
	}

	// 基本校验：标题必填，且 content / md_content 至少有一个
	if strings.TrimSpace(req.Title) == "" {
		c.Error(exception.ApiParamError)
		return
	}
	if strings.TrimSpace(req.Content) == "" && strings.TrimSpace(req.MdContent) == "" {
		c.Error(exception.ApiParamError)
		return
	}

	post, err := articleService.CreatePost(articleService.CreatePostInput{
		Title:      req.Title,
		Subtitle:   req.Subtitle,
		Summary:    req.Summary,
		CoverImage: req.CoverImage,
		Content:    req.Content,
		MdContent:  req.MdContent,
		Category:   req.Category,
		Tags:       req.Tags,
		IsDeep:     req.IsDeep,
		Hidden:     req.Hidden,
		NoStats:    req.NoStats,
	})
	if err != nil {
		fmt.Println("创建文章失败:", err)
		c.Error(exception.SysCannotSaveArticle)
		return
	}

	utils.JsonSuccessResponse(c, "创建成功", post)
}
