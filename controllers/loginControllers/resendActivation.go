package loginControllers

import (
	"coblog-backend/common/exception"
	middleware "coblog-backend/middlewares"
	"coblog-backend/models"
	"coblog-backend/services/mailService"
	"coblog-backend/services/userService"
	"coblog-backend/utils"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type resendActivationForm struct {
	// 邮箱可选：不带时用当前登录的账号发信
	Email string `json:"email"`
}

// ResendActivationEmail 重新发送账户激活邮件。
//
// 两种调用方式：
//   - 不带 email：必须已登录（路由器上挂了 LooseAuth），用会话里的账号发信。
//     未激活账户读不了 /api/user/info（1002），前端拿不到自己的邮箱，只能走这条；
//   - 带 email：沿用旧行为（注册页等知道邮箱的调用方）。
//
// 不带 email 时不接受请求体里的账号，避免这个接口变成任意用户的发信触发器。
func ResendActivationEmail(c *gin.Context) {
	var form resendActivationForm
	// 允许空请求体：不带 email 时按当前登录账号处理
	if err := c.ShouldBindJSON(&form); err != nil && !errors.Is(err, io.EOF) {
		c.Error(exception.ApiParamError)
		return
	}

	user, apiErr := resendTargetUser(c, strings.TrimSpace(form.Email))
	if apiErr != nil {
		c.Error(apiErr)
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

// resendTargetUser 决定给谁发信：给了邮箱就按邮箱查，没给就取当前登录账号。
func resendTargetUser(c *gin.Context, email string) (*models.AccountInfo, error) {
	if email == "" {
		user := middleware.CurrentAccount(c)
		if user == nil {
			return nil, exception.UsrNotLogin
		}
		return user, nil
	}

	if !utils.IsValidEmail(email) {
		return nil, exception.ApiParamError
	}
	user, err := userService.GetUserByEmail(email)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, exception.UsrNotExisted
	}
	if err != nil {
		return nil, exception.SysCannotReadDB
	}
	return user, nil
}
