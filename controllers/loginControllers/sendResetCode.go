package loginControllers

import (
	"coblog-backend/common/exception"
	"coblog-backend/services/mailService"
	"coblog-backend/services/userService"
	"coblog-backend/utils"
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type sendCodeForm struct {
	Email   string `json:"email" binding:"required"`
	Purpose string `json:"purpose" binding:"required"` // register / reset
}

// SendCode 发送邮箱验证码。
// purpose=register：要求邮箱未注册；purpose=reset：要求邮箱已注册。
func SendCode(c *gin.Context) {
	var form sendCodeForm
	if err := c.ShouldBindJSON(&form); err != nil {
		c.Error(exception.ApiParamError)
		return
	}

	purpose := mailService.CodePurpose(form.Purpose)
	switch purpose {
	case mailService.PurposeRegister:
		// 注册场景：邮箱不应已存在
		_, err := userService.GetUserByEmail(form.Email)
		if err == nil {
			c.Error(exception.UsrAlreadyExisted)
			return
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			c.Error(exception.SysCannotReadDB)
			return
		}
	case mailService.PurposeReset, mailService.PurposeLogin:
		// 找回密码 / 邮箱登录：邮箱必须存在
		_, err := userService.GetUserByEmail(form.Email)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.Error(exception.UsrNotExisted)
			return
		}
		if err != nil {
			c.Error(exception.SysCannotReadDB)
			return
		}
	default:
		c.Error(exception.ApiParamError)
		return
	}

	cooldown, err := mailService.SendVerificationCode(purpose, form.Email)
	if err != nil {
		fmt.Println("发送验证码失败:", err)
		c.Error(exception.SysCannotSendMail)
		return
	}
	if cooldown {
		c.Error(exception.UsrCodeTooFreq)
		return
	}

	utils.JsonSuccessResponse(c, "验证码已发送", nil)
}
