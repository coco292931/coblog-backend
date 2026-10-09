package liteControllers

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"coblog-backend/common/exception"
	"coblog-backend/common/webtoken"
	configreader "coblog-backend/configs/configReader"
	"coblog-backend/controllers/liteControllers/liteview"
	"coblog-backend/services/mailService"
	"coblog-backend/services/userService"
	"coblog-backend/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// /lite 的账户表单页，走 POST + 302（PRG）。
// 错误文案直接复用 exception 的 msg，与 JSON 接口保持一致。

// LoginPage GET /lite/login
func LoginPage(c *gin.Context) {
	liteview.Render(c, http.StatusOK, "login", liteview.LoginView{
		BaseView: newBaseView(c),
		Redirect: liteview.SafeRedirect(c.Query("redirect")),
	})
}

// LoginSubmit POST /lite/login
//
// 成功后 302 回跳（避免刷新重复提交）；失败直接重渲染表单并带上后端的错误文案。
func LoginSubmit(c *gin.Context) {
	account := strings.TrimSpace(c.PostForm("account"))
	password := c.PostForm("password")
	redirect := liteview.SafeRedirect(c.PostForm("redirect"))
	remember := c.PostForm("rememberMe") != ""

	renderFail := func(msg string) {
		liteview.Render(c, http.StatusOK, "login", liteview.LoginView{
			BaseView: newBaseView(c),
			Account:  account,
			Error:    msg,
			Redirect: redirect,
		})
	}

	if account == "" || password == "" {
		renderFail(exception.ApiParamError.Msg)
		return
	}

	user, err := userService.GetUserByEmail(account)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			renderFail(exception.UsrNotExisted.Msg)
			return
		}
		log.Printf("[LITE] 登录时查询用户失败: %v", err)
		renderFail(exception.SysUknExc.Msg)
		return
	}

	if err := userService.VerifyPwd(user, password); err != nil {
		renderFail(exception.UsrPasswordErr.Msg)
		return
	}

	// 未激活：补发激活邮件，但不阻止登录（与 /api/auth/login/combo 行为一致）
	if !userService.IsActivated(user) {
		if token, issueErr := userService.IssueActivationToken(user.ID); issueErr != nil {
			log.Printf("[LITE] 签发激活令牌失败: %v", issueErr)
		} else if cooldown, sendErr := mailService.SendActivationEmail(user.Email, token); sendErr != nil {
			log.Printf("[LITE] 激活邮件发送失败: %v", sendErr)
		} else if cooldown {
			log.Printf("[LITE] 激活邮件发送过于频繁，已跳过")
		}
	}

	validSecs := configreader.GetConfig().Account.ValidSecs
	token := webtoken.GenerateWt(user.ID, user.PermGroupID, validSecs)

	// 未勾「记住我」写会话 cookie，勾了才写持久的 ——
	// 对应主站把 token 存 sessionStorage 还是 localStorage 的差别。
	maxAge := 0
	if remember {
		maxAge = int(validSecs)
	}
	utils.SetAuthCookie(c, token, maxAge)

	target := redirect
	if target == "" {
		target = "/lite/"
	}
	c.Redirect(http.StatusFound, target)
}

// Logout POST /lite/logout。JSON 侧的登出是 /api/auth/logout，
func Logout(c *gin.Context) {
	utils.ClearAuthCookie(c)
	c.Redirect(http.StatusFound, "/lite/")
}
