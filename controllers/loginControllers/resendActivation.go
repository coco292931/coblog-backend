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

	// 每次都签发新令牌：旧令牌仍受 Redis TTL 约束，用户拿最新邮件即可
	activationToken, issueErr := userService.IssueActivationToken(user.ID)
	if issueErr != nil {
		fmt.Println("签发激活令牌失败:", issueErr)
		c.Error(exception.SysCannotSendMail)
		return
	}

	cooldown, err := mailService.SendActivationEmail(user.Email, activationToken)
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
