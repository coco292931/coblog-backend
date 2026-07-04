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

type resendActivationForm struct {
	Email string `json:"email" binding:"required"`
}

// ResendActivationEmail 重新发送账户激活邮件。
func ResendActivationEmail(c *gin.Context) {
	var form resendActivationForm
	if err := c.ShouldBindJSON(&form); err != nil {
		c.Error(exception.ApiParamError)
		return
	}

	if !utils.IsValidEmail(form.Email) {
		c.Error(exception.ApiParamError)
		return
	}

	user, err := userService.GetUserByEmail(form.Email)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.Error(exception.UsrNotExisted)
		return
	}
	if err != nil {
		c.Error(exception.SysCannotReadDB)
		return
	}

	if userService.IsActivated(user) {
		utils.JsonSuccessResponse(c, "账户已激活，无需重复发送", map[string]interface{}{
			"alreadyActivated": true,
		})
		return
	}

	cooldown, err := mailService.SendActivationEmail(user.Email, user.Activation)
	if err != nil {
		fmt.Println("发送激活邮件失败:", err)
		c.Error(exception.SysCannotSendMail)
		return
	}
	if cooldown {
		c.Error(exception.UsrCodeTooFreq)
		return
	}

	utils.JsonSuccessResponse(c, "激活邮件已重新发送，请前往邮箱查收", nil)
}
