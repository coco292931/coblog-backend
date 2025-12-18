package articlesControllers

import (
	"coblog-backend/common/exception"
	"coblog-backend/controllers/accountControllers"
	"coblog-backend/models"
	"coblog-backend/services/articleService"
	"coblog-backend/services/userService"
	"coblog-backend/utils"
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
)
func GetArticleContent(c *gin.Context) {
	var data models.Post
	var err error
	accountId, err := accountControllers.GetAccountIDFromContext(c)
	if err != nil {
		// 这里不应该报错
		c.Error(exception.SysUknExc)
		return
	}

	// 未登录用户，返回默认列表
	if accountId == 0 {
		data, err = articleService.GetArticle("def", c.Param("id"))
		if errors.Is(err, exception.UsrNotPermitted) {
			c.Error(exception.UsrNotPermitted)
			return
		} else if err != nil {
			c.Error(exception.SysCannotGetArticle)
			return
		}
		utils.JsonSuccessResponse(c, "获取成功", data)
		return
	}

	// 已登录用户，检查深度权限
	accountInfo, err := userService.GetUserByID(accountId)
	if err != nil {
		fmt.Println("获取用户信息失败")
		c.Error(exception.SysCannotReadDB)
		return
	}

	// 校验深度权限
	if !(accountInfo.Deepable && accountInfo.IsDeep) {
		data, err = articleService.GetArticle("def", c.Param("id"))
	} else {
		// 通过深度权限校验，返回深度文章列表
		data, err = articleService.GetArticle("deep", c.Param("id"))
	}

	if err != nil {
		c.Error(exception.SysCannotGetArticle)
		return
	}
	utils.JsonSuccessResponse(c, "获取成功", data)
}