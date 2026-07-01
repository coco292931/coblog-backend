package articleService

import (
	"coblog-backend/common/exception"
	"coblog-backend/configs/database"
	"coblog-backend/models"
)

func GetArticle(status string, id string) (models.Post, error) {
	var article models.Post
	result := database.DataBase.First(&article, id)
	if result.Error != nil {
		return models.Post{}, result.Error
	}

	// 隐藏文章对所有人不可见（优先级最高，对外等同不存在）
	if article.Hidden {
		return models.Post{}, exception.UsrNotPermitted
	}

	if status == "def" && article.IsDeep {
		// def 模式：不返回 isdeep 文章
		return models.Post{}, exception.UsrNotPermitted
	}

	// 公开详情不返回 Markdown 原文，省带宽；编辑回填请走编辑专用接口
	article.MdContent = ""
	return article, nil
}

// GetArticleForEdit 返回文章完整内容（含 Markdown 原文），供编辑器回填使用
func GetArticleForEdit(id string) (models.Post, error) {
	var article models.Post
	if err := database.DataBase.First(&article, id).Error; err != nil {
		return models.Post{}, exception.SysCannotGetArticle
	}
	return article, nil
}
