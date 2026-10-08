package articlesControllers

import (
	"coblog-backend/common/exception"
	"coblog-backend/controllers/accountControllers"
	"coblog-backend/services/articleService"
	"coblog-backend/services/userService"
	"coblog-backend/utils"
	"errors"

	"github.com/gin-gonic/gin"
)

func GetArticleContent(c *gin.Context) {
	accountId, err := accountControllers.GetAccountIDFromContext(c)
	if err != nil {
		// 这里不应该报错
		c.Error(exception.SysUknExc)
		return
	}

	// 隐藏文章（hidden）与深度文章（is_deep）的可见性统一由 userService 判定：
	// 匿名 / 无深度权限 → def，与原先内联在此处的逻辑一致。
	status := userService.ResolveContentStatus(accountId)

	data, err := articleService.GetArticle(status, c.Param("id"))
	if err != nil {
		if errors.Is(err, exception.UsrNotPermitted) {
			c.Error(exception.UsrNotPermitted)
			return
		}
		c.Error(exception.SysCannotGetArticle)
		return
	}
	utils.JsonSuccessResponse(c, "获取成功", data)
}
