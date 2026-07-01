package articleService

import (
	"coblog-backend/configs/database"
	"coblog-backend/models"
	"strings"
)

// EscapeLike 转义LIKE查询中的特殊字符，防止通配符注入和性能攻击
func EscapeLike(s string) string {
	// 先转义反斜杠（必须最先处理）
	s = strings.ReplaceAll(s, "\\", "\\\\")
	// 转义 SQL LIKE 的特殊字符 % 和 _
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}

type RequestParams struct {
	Page     uint64 `form:"page"`
	PageSize uint64 `form:"pageSize"`
	Category string `form:"category"` //列表
	Tag      string `form:"tag"`      //标签列表
	Q        string `form:"q"`        //搜索关键词
}

type ArticleListResponse struct {
	Articles []models.Post `json:"articles"`
	Total    int64         `json:"total"`
	Page     uint64        `json:"page"`
	PageSize uint64        `json:"page_size"`
}

// GetArticleList 获取文章列表。
// keepContent 为 true 时保留富文本 content 字段（如 RSS 全文输出需要）；
// 为 false 时清空 content 以节省带宽（文章列表场景）。
func GetArticleList(status string, requestParams RequestParams, keepContent bool) (*ArticleListResponse, error) {
	var articles []models.Post
	var total int64

	// 构建基础查询
	query := database.DataBase.Model(&models.Post{})

	// 根据 status 筛选 ISDEEP
	if status == "def" {
		query = query.Where("is_deep = ?", false)
	}

	// 根据 category 筛选（JSON 数组包含查询）
	if requestParams.Category != "" {
		// 转义特殊字符防止LIKE注入
		escapedCategory := EscapeLike(requestParams.Category)
		// MySQL 使用 LIKE 查询 JSON 数组（默认用反斜杠转义）
		query = query.Where("category LIKE ?", "%\""+escapedCategory+"\"%")
	}

	// 根据 tag 筛选（JSON 数组包含查询）
	if requestParams.Tag != "" {
		// 转义特殊字符防止LIKE注入
		escapedTag := EscapeLike(requestParams.Tag)
		query = query.Where("tags LIKE ?", "%\""+escapedTag+"\"%")
	}

	// 根据 q 搜索关键词（在标题或内容中搜索）
	if requestParams.Q != "" {
		// 限制搜索关键词长度，防止超长查询攻击
		if len(requestParams.Q) > 100 {
			return nil, nil // 或返回特定错误
		}
		// 转义特殊字符防止LIKE注入和性能DoS攻击
		escapedQ := EscapeLike(requestParams.Q)
		searchPattern := "%" + escapedQ + "%"
		// MySQL 默认用反斜杠转义，不需要 ESCAPE 子句
		query = query.Where("title LIKE ? OR subtitle LIKE ? OR content LIKE ? OR summary LIKE ? OR category LIKE ? OR tags LIKE ?",
			searchPattern, searchPattern, searchPattern, searchPattern, searchPattern, searchPattern)
	}

	// 获取总数
	countQuery := query
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, err
	}

	// 设置默认分页参数
	page := requestParams.Page
	pageSize := requestParams.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10 // 默认每页 10 条
	}

	// 分页处理
	offset := (page - 1) * pageSize
	query = query.Offset(int(offset)).Limit(int(pageSize))

	// 执行查询
	if err := query.Find(&articles).Error; err != nil {
		return nil, err
	}

	//删去 content 字段，节省带宽（RSS 等需要全文的场景通过 keepContent 保留）
	if !keepContent {
		for i := range articles {
			articles[i].Content = ""
		}
	}
	// 构建响应
	response := &ArticleListResponse{
		Articles: articles,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}

	return response, nil
}
