package articleService

import (
	"coblog-backend/configs/database"
	"coblog-backend/models"
)

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

func GetArticleList(status string, requestParams RequestParams) (*ArticleListResponse, error) {
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
		// SQL Server 使用 JSON_VALUE 或 LIKE 查询 JSON 数组
		query = query.Where("category LIKE ?", "%\""+requestParams.Category+"\"%")
	}

	// 根据 tag 筛选（JSON 数组包含查询）
	if requestParams.Tag != "" {
		query = query.Where("tags LIKE ?", "%\""+requestParams.Tag+"\"%")
	}
	// 根据 q 搜索关键词（在标题或内容中搜索）
	if requestParams.Q != "" {
		query = query.Where("title LIKE ? OR content LIKE ? OR subtitle LIKE ? OR summary LIKE ?",
			"%"+requestParams.Q+"%", "%"+requestParams.Q+"%", "%"+requestParams.Q+"%", "%"+requestParams.Q+"%")
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

	//删去 content 字段，节省带宽
	for i := range articles {
		articles[i].Content = ""
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
