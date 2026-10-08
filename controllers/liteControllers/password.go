package liteControllers

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"

	"coblog-backend/common/exception"
	"coblog-backend/controllers/liteControllers/liteview"
	"coblog-backend/services/mailService"
	"coblog-backend/services/userService"
	"coblog-backend/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 找回密码（M4 最后一块）。
//
// 校验与文案逐条对齐 /api/auth/code/send 的 reset 分支与 /api/auth/pwd/reset。

// ForgotPasswordPage GET /lite/forgot-password
func ForgotPasswordPage(c *gin.Context) {
	liteview.Render(c, http.StatusOK, "forgotPassword", liteview.ForgotPasswordView{
		BaseView:     newBaseView(c),
		PasswordRule: liteview.PasswordRuleText,
		Email:        c.Query("email"),
		Error:        c.Query("err"),
		Notice:       c.Query("msg"),
	})
}

// ForgotPasswordSubmit POST /lite/forgot-password
//
// 同一个表单两个动作：action=send 发验证码，其余（含 action=reset）走重置。
func ForgotPasswordSubmit(c *gin.Context) {
	email := strings.TrimSpace(c.PostForm("email"))
	if c.PostForm("action") == "send" {
		sendResetCode(c, email)
		return
	}
	resetPasswordByCode(c, email)
}

// sendResetCode 发验证码。与 /api/auth/code/send 的 reset 分支一致：邮箱必须已注册。
func sendResetCode(c *gin.Context, email string) {
	if !utils.IsValidEmail(email) {
		backToForgot(c, email, exception.ApiParamError.Msg, false)
		return
	}

	if _, err := userService.GetUserByEmail(email); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			backToForgot(c, email, exception.UsrNotExisted.Msg, false)
			return
		}
		backToForgot(c, email, exception.SysCannotReadDB.Msg, false)
		return
	}

	cooldown, err := mailService.SendVerificationCode(mailService.PurposeReset, email)
	if err != nil {
		log.Printf("[LITE] 发送验证码失败: %v", err)
		backToForgot(c, email, exception.SysCannotSendMail.Msg, false)
		return
	}
	if cooldown {
		backToForgot(c, email, exception.UsrCodeTooFreq.Msg, false)
		return
	}
	backToForgot(c, email, "验证码已发送", true)
}

// resetPasswordByCode 用验证码重置密码。与 /api/auth/pwd/reset 一致：先验码，再改密。
func resetPasswordByCode(c *gin.Context, email string) {
	newPwd := c.PostForm("newPassword")

	if !utils.IsValidEmail(email) {
		backToForgot(c, email, exception.ApiParamError.Msg, false)
		return
	}
	// 与「我的」页共用同一套密码规则
	if msg := liteview.ValidateNewPassword(newPwd, c.PostForm("confirmPassword")); msg != "" {
		backToForgot(c, email, msg, false)
		return
	}

	if !mailService.VerifyCode(mailService.PurposeReset, email, strings.TrimSpace(c.PostForm("verificationCode"))) {
		backToForgot(c, email, exception.UsrCodeInvalid.Msg, false)
		return
	}

	if err := userService.ResetPwdByEmail(email, newPwd); err != nil {
		backToForgot(c, email, errMsg(err), false)
		return
	}
	// 与 /api/auth/pwd/reset 的成功文案一致
	backToForgot(c, email, "密码重置成功", true)
}

// backToForgot 回到表单页并捎上结果提示与已填的邮箱（PRG，避免刷新重复提交）。
func backToForgot(c *gin.Context, email, msg string, ok bool) {
	values := url.Values{}
	if email != "" {
		values.Set("email", email)
	}
	if msg != "" {
		values.Set("msg", msg)
	}
	if ok {
		values.Set("ok", "1")
	}
	c.Redirect(http.StatusFound, "/lite/forgot-password?"+values.Encode())
}
