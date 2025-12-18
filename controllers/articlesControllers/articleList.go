package articlesControllers

import (
	"coblog-backend/common/exception"
	"coblog-backend/controllers/accountControllers"
	"coblog-backend/services/articleService"
	"coblog-backend/services/userService"
	"coblog-backend/utils"
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

	accountId, err := accountControllers.GetAccountIDFromContext(c)
	if err != nil {
		// 这里不应该报错
		c.Error(exception.SysUknExc)
		return
	}

	var data any
	// 未登录用户，返回默认列表
	if accountId == 0 {
		data, err = articleService.GetArticleList("def", requestForm)
		if err != nil {
			c.Error(exception.SysUknExc)
			return
		}
		utils.JsonSuccessResponse(c, "获取成功", data)
		return
	}

	// 已登录用户，检查深度权限
	accountInfo, err := userService.GetUserByID(accountId)
	if err != nil {
		fmt.Println("获取用户信息失败")
		c.Error(exception.SysUknExc)
		return
	}

	// 校验深度权限
	if !(accountInfo.Deepable && accountInfo.IsDeep) {
		data, err = articleService.GetArticleList("def", requestForm)
	} else {
		// 通过深度权限校验，返回深度文章列表
		data, err = articleService.GetArticleList("deep", requestForm)
	}

	if err != nil {
		c.Error(exception.SysUknExc)
		return
	}
	utils.JsonSuccessResponse(c, "获取成功", data)
}
