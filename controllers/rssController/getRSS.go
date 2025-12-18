package rssController

import (
	"github.com/gin-gonic/gin"
	"coblog-backend/controllers/accountControllers"
	"coblog-backend/utils"
)

func GetRSS(c *gin.Context) {
	accountID,err:=accountControllers.GetAccountIDFromContext(c)
	if err!=nil{
		c.Error(err)
		return
	}
	if accountID==0{
		//未登录用户，返回默认RSS
		utils.JsonSuccessResponse(c,"获取RSS成功",accountID)
		return
	}
	//获取账户深度权限
}