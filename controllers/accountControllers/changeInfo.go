package accountControllers

import (
	"coblog-backend/services/userService"
	"coblog-backend/utils"

	"github.com/gin-gonic/gin"
)

func ChangePwd(c *gin.Context) {
	accountID, err := GetAccountIDFromContext(c)
	if err != nil {
		c.Error(err)
		return
	}

	//var err error
	err = userService.ChangePwd(accountID, c.PostForm("oldPassword"), c.PostForm("newPassword"))
	if err != nil {
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