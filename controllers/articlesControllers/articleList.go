package articlesControllers

import (
	"coblog-backend/common/exception"
	"coblog-backend/controllers/accountControllers"
	middleware "coblog-backend/middlewares"
	"coblog-backend/services/articleService"
	"coblog-backend/services/userService"
	"coblog-backend/utils"
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
)

func GetArticleList(c *gin.Context) {
	var err error
	var requestForm articleService.RequestParams
	// 绑定查询参数
	err = c.ShouldBindQuery(&requestForm)
	if err != nil {
		c.Error(exception.ApiParamError)
		fmt.Println("参数错误:", err)
		return
	}
	fmt.Println(requestForm.Q)

	// 只确认鉴权中间件跑过（LooseAuth 一定会设置 AccountID）；账号本身下面直接取
	if _, err := accountControllers.GetAccountIDFromContext(c); err != nil {
		// 这里不应该报错
		c.Error(exception.SysUknExc)
		return
	}

	// 内容级别（是否包含深度文章）由 userService 统一判定：
	// 未登录 → def；已登录但不具备深度权限 → def；具备 → deep。
	// 这段判定原先在列表 / 详情 / RSS 三处各写一遍，改权限规则时很容易漏改。
	// 账号已由鉴权中间件读出，直接复用，不再按 ID 查一次库
	status := userService.ContentStatusFor(middleware.CurrentAccount(c))

	data, err := articleService.GetArticleList(status, requestForm, false)
	if err != nil {
		if errors.Is(err, exception.UsrNotPermitted) {
			c.Error(exception.UsrNotPermitted)
			return
		}
		c.Error(exception.SysUknExc)
		return
	}
	utils.JsonSuccessResponse(c, "获取成功", data)
}
