package loginControllers

import (
	"coblog-backend/common/exception"
	"coblog-backend/common/webtoken"
	"coblog-backend/configs/configReader"
	"coblog-backend/services/mailService"
	"coblog-backend/services/userService"
	"coblog-backend/utils"
	"errors"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type emailLoginForm struct {
	Email            string `json:"email" binding:"required"`
	VerificationCode string `json:"verificationCode" binding:"required"`
}

// AuthByEmail POST /api/auth/login/email — 邮箱验证码登录
func AuthByEmail(c *gin.Context) {
	var form emailLoginForm
	if err := c.ShouldBindJSON(&form); err != nil {
		c.Error(exception.ApiParamError)
		return
	}

	// 验证码校验
	if !mailService.VerifyCode(mailService.PurposeLogin, form.Email, form.VerificationCode) {
		c.Error(exception.UsrCodeInvalid)
		return
	}

	user, err := userService.GetUserByEmail(form.Email)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.Error(exception.UsrNotExisted)
		return
	}
	if err != nil {
		c.Error(exception.SysUknExc)
		return
	}

	// 未激活：重发激活邮件
	if !userService.IsActivated(user) {
		if sendErr := mailService.SendActivationEmail(user.Email, user.Activation); sendErr != nil {
			fmt.Println("激活邮件发送失败:", sendErr)
		}
		c.Error(exception.UsrNotActivated)
		return
	}

	utils.JsonSuccessResponse(c, "登录成功", map[string]interface{}{
		"token":    webtoken.GenerateWt(user.ID, user.PermGroupID, configreader.GetConfig().Account.ValidSecs),
		"userID":   user.ID,
		"username": user.UserName,
		"userType": strconv.FormatUint(uint64(user.PermGroupID), 10),
	})
}
