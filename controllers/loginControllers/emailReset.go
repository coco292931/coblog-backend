package loginControllers

import (
	"coblog-backend/common/exception"
	"coblog-backend/services/mailService"
	"coblog-backend/services/userService"
	"coblog-backend/utils"

	"github.com/gin-gonic/gin"
)

type resetPwdForm struct {
	Email            string `json:"email" binding:"required"`
	VerificationCode string `json:"verificationCode" binding:"required"`
	NewPassword      string `json:"newPassword" binding:"required"`
}

// ResetPwdByEmail 通过邮箱验证码找回（重置）密码，无需登录。
func ResetPwdByEmail(c *gin.Context) {
	var form resetPwdForm
	if err := c.ShouldBindJSON(&form); err != nil {
		c.Error(exception.ApiParamError)
		return
	}

	if !utils.IsValidEmail(form.Email) {
		c.Error(exception.ApiParamError)
		return
	}

	// 校验验证码
	if !mailService.VerifyCode(mailService.PurposeReset, form.Email, form.VerificationCode) {
		c.Error(exception.UsrCodeInvalid)
		return
	}

	if err := userService.ResetPwdByEmail(form.Email, form.NewPassword); err != nil {
		c.Error(err)
		return
	}

	utils.JsonSuccessResponse(c, "密码重置成功", nil)
}
