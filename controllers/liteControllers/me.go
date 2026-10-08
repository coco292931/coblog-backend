package liteControllers

import (
	"errors"
	"log"
	"net/http"
	"net/url"

	"coblog-backend/common/exception"
	"coblog-backend/common/permission"
	"coblog-backend/controllers/accountControllers"
	"coblog-backend/controllers/liteControllers/liteview"
	"coblog-backend/services/mailService"
	"coblog-backend/services/userService"

	"github.com/gin-gonic/gin"
)

// 个人中心。
//
//	GET  /lite/me                    ← /api/user/info/     需要 Perm_GetProfile
//	POST /lite/me/password           ← /api/user/pwd/      需要 Perm_ChangePassword
//	POST /lite/me/rss                ← /api/user/rst-rss/  需要 Perm_UpdateProfile
//	POST /lite/me/resend-activation  ← /api/auth/activate/resend（无需权限）
//
// 之所以自己判而不是挂 middlewares.NeedPerm：后者失败时输出 JSON，
// 而这些页面必须回 HTML。

// MePage GET /lite/me
func MePage(c *gin.Context) {
	accountID, ok := requireAccount(c)
	if !ok {
		return
	}

	// 已登录但权限组读不了个人信息（典型是未激活账户）→ 主站的 permDenied 分支
	if !hasPerm(c, permission.Perm_GetProfile) {
		liteview.Render(c, http.StatusOK, "me", liteview.MeView{
			BaseView: newBaseView(c),
			Denied:   true,
		})
		return
	}

	view := loadMeView(c, accountID)
	view.Message = c.Query("msg")
	view.OK = c.Query("ok") != ""
	liteview.Render(c, http.StatusOK, "me", view)
}

// MeChangePassword POST /lite/me/password
func MeChangePassword(c *gin.Context) {
	accountID, ok := requireAccount(c)
	if !ok {
		return
	}
	if !hasPerm(c, permission.Perm_ChangePassword) {
		redirectMe(c, exception.UsrNotPermitted.Msg, false)
		return
	}

	newPwd := c.PostForm("newPassword")
	// 与主站同一套规则（constants/account.js 的 validateNewPassword）
	if msg := liteview.ValidateNewPassword(newPwd, c.PostForm("confirmPassword")); msg != "" {
		redirectMe(c, msg, false)
		return
	}

	if err := userService.ChangePwd(accountID, c.PostForm("oldPassword"), newPwd); err != nil {
		redirectMe(c, errMsg(err), false)
		return
	}
	// 与 /api/user/pwd/ 的成功文案一致
	redirectMe(c, "修改成功", true)
}

// MeResetRSSToken POST /lite/me/rss
func MeResetRSSToken(c *gin.Context) {
	accountID, ok := requireAccount(c)
	if !ok {
		return
	}
	if !hasPerm(c, permission.Perm_UpdateProfile) {
		redirectMe(c, exception.UsrNotPermitted.Msg, false)
		return
	}

	if _, err := userService.RstRSSToken(accountID); err != nil {
		redirectMe(c, errMsg(err), false)
		return
	}
	// 与 /api/user/rst-rss/ 的成功文案一致
	redirectMe(c, "重置成功", true)
}

// MeResendActivation POST /lite/me/resend-activation
//
// 文案与 /api/auth/activate/resend 一致。这里用当前登录用户自己的邮箱，
// 不接受表单传入的邮箱（否则就成了任意用户触发发信口）。
func MeResendActivation(c *gin.Context) {
	accountID, ok := requireAccount(c)
	if !ok {
		return
	}

	user, err := userService.GetUserByID(accountID)
	if err != nil {
		redirectMe(c, errMsg(err), false)
		return
	}

	if userService.IsActivated(user) {
		redirectMe(c, "账户已激活，无需重复发送", true)
		return
	}

	token, err := userService.IssueActivationToken(user.ID)
	if err != nil {
		log.Printf("[LITE] 签发激活令牌失败: %v", err)
		redirectMe(c, exception.SysCannotSendMail.Msg, false)
		return
	}

	cooldown, err := mailService.SendActivationEmail(user.Email, token)
	if err != nil {
		log.Printf("[LITE] 发送激活邮件失败: %v", err)
		redirectMe(c, exception.SysCannotSendMail.Msg, false)
		return
	}
	if cooldown {
		redirectMe(c, "发送过于频繁，请稍后再试", false)
		return
	}
	redirectMe(c, "激活邮件已重新发送，请前往邮箱查收", true)
}

// ────────────────────────────── 辅助 ──────────────────────────────

// requireAccount 取当前登录账号；未登录时按主站的登录守卫跳到登录页并带上回跳地址。
func requireAccount(c *gin.Context) (uint64, bool) {
	accountID, err := accountControllers.GetAccountIDFromContext(c)
	if err != nil || accountID == 0 {
		c.Redirect(http.StatusFound, "/lite/login?redirect="+url.QueryEscape(c.Request.URL.RequestURI()))
		return 0, false
	}
	return accountID, true
}

// hasPerm 与 middlewares.NeedPerm 的判定保持一致
// （PermissionGroupID 由 LooseAuth 从 token 里取出后放在 context 上）。
func hasPerm(c *gin.Context, needed permission.PermissionID) bool {
	raw, exists := c.Get("PermissionGroupID")
	if !exists {
		return false
	}
	pgid, ok := raw.(uint32)
	if !ok || pgid == 0 {
		return false
	}
	return permission.IsPermSatisfied(pgid, needed)
}

// loadMeView 组装个人中心的数据。不设置 Message，由调用方补。
func loadMeView(c *gin.Context, accountID uint64) liteview.MeView {
	view := liteview.MeView{
		BaseView:     newBaseView(c),
		PasswordRule: liteview.PasswordRuleText,
	}

	user, err := userService.GetUserByID(accountID)
	if err != nil {
		view.FetchErr = errMsg(err)
		return view
	}

	view.Username = user.UserName
	view.Email = user.Email
	view.Activated = userService.IsActivated(user)
	view.Deepable = user.Deepable
	view.IsDeep = user.IsDeep
	view.RSSToken = user.RSSToken
	return view
}

// redirectMe 回到个人中心并捎一条结果提示。
// 用 PRG（POST → 302 → GET）而不是直接渲染，避免刷新时重复提交。
func redirectMe(c *gin.Context, msg string, ok bool) {
	values := url.Values{}
	if msg != "" {
		values.Set("msg", msg)
	}
	if ok {
		values.Set("ok", "1")
	}

	target := "/lite/me"
	if len(values) > 0 {
		target += "?" + values.Encode()
	}
	c.Redirect(http.StatusFound, target)
}

// errMsg 把 service 层返回的错误转成给用户看的文案（就是业务错误的原文）。
func errMsg(err error) string {
	var apiErr *exception.Exception
	if errors.As(err, &apiErr) {
		return apiErr.Msg
	}
	return exception.SysUknExc.Msg
}
