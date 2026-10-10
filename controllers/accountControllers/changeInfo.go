package accountControllers

import (
	"coblog-backend/common/exception"
	"coblog-backend/services/userService"
	"coblog-backend/utils"

	"github.com/gin-gonic/gin"
)

type changePwdForm struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required"`
}

func ChangePwd(c *gin.Context) {
	accountID, err := GetAccountIDFromContext(c)
	if err != nil {
		c.Error(err)
		return
	}

	var form changePwdForm
	if err := c.ShouldBindJSON(&form); err != nil {
		c.Error(exception.ApiParamError)
		return
	}

	if err := userService.ChangePwd(accountID, form.OldPassword, form.NewPassword); err != nil {
		c.Error(err)
		return
	}
	// 改密让该账号所有旧 token 失效（含当前这个），给当前设备换一个新的，
	// 前端据此更新本地存储，否则下一次请求就会被踢回登录页
	token, validSecs, err := userService.ReissueSession(accountID)
	if err != nil {
		utils.JsonSuccessResponse(c, "修改成功，请重新登录", nil)
		return
	}
	utils.SetAuthCookie(c, token, int(validSecs))
	utils.JsonSuccessResponse(c, "修改成功", gin.H{"token": token})
}

func EditAccountInfoUser(c *gin.Context) { 
	// todo 普通用户修改自己的账户信息
	return
}

func RstRSSToken(c *gin.Context) {
	accountID, err := GetAccountIDFromContext(c)
	if err != nil {
		c.Error(err)
		return
	}
	newToken, err := userService.RstRSSToken(accountID)
	if err != nil {
		c.Error(err)
		return
	}
	utils.JsonSuccessResponse(c, "重置成功", map[string]interface{}{
		"newToken": newToken,
	})
}