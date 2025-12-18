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

	if status == "def" && article.IsDeep {
		// def 模式：不返回 isdeep 文章
		return models.Post{}, exception.UsrNotPermitted
	}
	return article, nil
}
