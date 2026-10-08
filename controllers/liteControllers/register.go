package liteControllers

import (
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
)

// 注册与激活。

// RegisterPage GET /lite/register
func RegisterPage(c *gin.Context) {
	liteview.Render(c, http.StatusOK, "register", liteview.RegisterView{
		BaseView:     newBaseView(c),
		PasswordRule: liteview.PasswordRuleText,
		Error:        c.Query("err"),
		Notice:       c.Query("msg"),
	})
}

// RegisterSubmit POST /lite/register
//
// 校验顺序与 /api/auth/register 保持一致：参数齐全 → 邮箱格式 → 密码规则 → 建号。
func RegisterSubmit(c *gin.Context) {
	username := strings.TrimSpace(c.PostForm("username"))
	email := strings.TrimSpace(c.PostForm("email"))
	password := c.PostForm("password")
	confirm := c.PostForm("confirmPassword")

	renderFail := func(msg string) {
		liteview.Render(c, http.StatusOK, "register", liteview.RegisterView{
			BaseView:     newBaseView(c),
			PasswordRule: liteview.PasswordRuleText,
			Username:     username,
			Email:        email,
			Error:        msg,
		})
	}

	if username == "" || email == "" || password == "" {
		renderFail(exception.ApiParamError.Msg)
		return
	}
	if !utils.IsValidEmail(email) {
		renderFail(exception.ApiParamError.Msg)
		return
	}
	// 与「我的」页共用同一套密码规则（主站也是共用 constants/account.js）
	if msg := liteview.ValidateNewPassword(password, confirm); msg != "" {
		renderFail(msg)
		return
	}

	// 权限组固定 GUEST(1)，与 /api/auth/register 一致
	user, err := userService.CreateUser(password, email, username, 1)
	if err != nil {
		renderFail(errMsg(err))
		return
	}

	// 三种结果文案与 JSON 注册接口逐字一致
	msg := "注册成功，激活邮件已发送，请前往邮箱完成激活"
	if token, issueErr := userService.IssueActivationToken(user.ID); issueErr != nil {
		msg = "注册成功，但激活邮件发送失败，请在“我的”页面重新发送"
		log.Printf("[LITE] 签发激活令牌失败: %v", issueErr)
	} else if cooldown, sendErr := mailService.SendActivationEmail(user.Email, token); sendErr != nil {
		msg = "注册成功，但激活邮件发送失败，请在“我的”页面重新发送"
		log.Printf("[LITE] 激活邮件发送失败: %v", sendErr)
	} else if cooldown {
		msg = "注册成功，激活邮件已在近期发送，请前往邮箱查收"
	}

	// PRG：成功后重定向回去，避免刷新时重复注册
	c.Redirect(http.StatusFound, "/lite/register?msg="+url.QueryEscape(msg))
}

// ActivatePage GET /lite/activate?token=xxx
//
// 令牌是一次性的（Redis 侧取出即删），所以刷新会变成「激活失败」
func ActivatePage(c *gin.Context) {
	token := strings.TrimSpace(c.Query("token"))
	view := liteview.ActivateView{BaseView: newBaseView(c)}

	if token == "" {
		view.Title = "激活链接无效"
		view.Message = "缺少激活参数，请重新从邮件中打开完整链接。"
		liteview.Render(c, http.StatusOK, "activate", view)
		return
	}

	if err := userService.ActivateByToken(token); err != nil {
		view.Title = "账户激活失败"
		view.Message = "激活链接已失效，请返回“我的”页面重新发送激活邮件。"
		liteview.Render(c, http.StatusOK, "activate", view)
		return
	}

	view.Success = true
	view.Title = "账户激活成功"
	view.Message = "你的账户已成功激活，现在可以正常登录。"
	liteview.Render(c, http.StatusOK, "activate", view)
}
