package accountControllers

import (
	"coblog-backend/common/exception"
	"coblog-backend/models"
	"coblog-backend/services/userService"
	"coblog-backend/utils"
	"errors"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func ChangePwd(c *gin.Context) {
	if c.AccountID == "" { //如果没有传入id参数.其实肯定会传入的，没用
		c.Error(exception.ApiParamError)
		//utils.JsonSuccessResponse(c, "查询失败", models.AccountInfo{})
		return
	}

	//var err error
	_, err := userService.ChangePwd(c.AccountID, c.PostForm("oldPassword"), c.PostForm("newPassword"))
	if err != nil {
		c.Error(err)
		return
	}
	utils.JsonSuccessResponse(c, "修改成功")
}
