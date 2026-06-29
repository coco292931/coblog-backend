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
	utils.JsonSuccessResponse(c, "修改成功", nil)
}

func EditAccountInfoUser(c *gin.Context) { 
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