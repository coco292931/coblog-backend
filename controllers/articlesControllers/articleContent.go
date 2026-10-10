package articlesControllers

import (
	"coblog-backend/common/exception"
	"coblog-backend/controllers/accountControllers"
	middleware "coblog-backend/middlewares"
	"coblog-backend/services/articleService"
	"coblog-backend/services/userService"
	"coblog-backend/utils"
	"errors"

	"github.com/gin-gonic/gin"
)

func GetArticleContent(c *gin.Context) {
	// 只确认鉴权中间件跑过（LooseAuth 一定会设置 AccountID）；账号本身下面直接取
	if _, err := accountControllers.GetAccountIDFromContext(c); err != nil {
		// 这里不应该报错
		c.Error(exception.SysUknExc)
		return
	}

	// 隐藏文章（hidden）与深度文章（is_deep）的可见性统一由 userService 判定：
	// 匿名 / 无深度权限 → def，与原先内联在此处的逻辑一致。
	// 账号已由鉴权中间件读出，直接复用，不再按 ID 查一次库
	status := userService.ContentStatusFor(middleware.CurrentAccount(c))

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
