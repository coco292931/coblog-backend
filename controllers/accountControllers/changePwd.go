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
