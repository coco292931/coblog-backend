package mailService

import (
	"context"
	"fmt"

	"coblog-backend/common/exception"
	configreader "coblog-backend/configs/configReader"
)

// 各用途的邮件主题与正文模板
var purposeText = map[CodePurpose]struct {
	subject string
	action  string
}{
	PurposeRegister:  {subject: "【%s】注册验证码", action: "注册账号"},
	PurposeReset:     {subject: "【%s】找回密码验证码", action: "找回密码"},
	PurposeChangePwd: {subject: "【%s】修改密码验证码", action: "修改密码"},
	PurposeLogin:     {subject: "【%s】登录验证码", action: "登录"},
}

// SendVerificationCode 为指定邮箱+用途签发并发送验证码。
// 返回 cooldown=true 表示触发频率限制、本次未发送。
func SendVerificationCode(p CodePurpose, email string) (cooldown bool, err error) {
	code, cooldown, err := IssueCode(p, email)
	if err != nil {
		return false, err
	}
	if cooldown {
		return true, nil
	}

	siteTitle := configreader.GetConfig().Site.Title
	if siteTitle == "" {
		siteTitle = "CoBlog"
	}

	text, ok := purposeText[p]
	if !ok {
		text = struct {
			subject string
			action  string
		}{subject: "【%s】验证码", action: "操作"}
	}

	subject := fmt.Sprintf(text.subject, siteTitle)
	body := buildCodeEmailHTML(siteTitle, text.action, code)

	if err := SendMail(email, subject, body); err != nil {
		// 发送失败：丢弃已写入的验证码并释放冷却，
		// 否则用户会白等 60 秒，且验证码虽在 Redis 却永远不会到达邮箱
		DiscardCode(p, email)
		return false, err
	}
	return false, nil
}

// SendActivationEmail 发送账户激活邮件，邮件中包含激活链接。
// activationToken 由调用方（userService）签发并存入 Redis，本函数只负责投递。
// 返回 cooldown=true 表示同一邮箱短时间内已发送过，本次未重复发送。
func SendActivationEmail(email, activationToken string) (cooldown bool, err error) {
	if !store.Available() {
		return false, exception.SysUknExc
	}
	if activationToken == "" {
		return false, exception.UsrTokenInvalid
	}

	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	cooldownKey := keyCooldown("activation", email)
	// SET NX：抢到冷却标记才发送，天然防止并发重复投递
	if !store.AcquireCooldown(ctx, cooldownKey, resendCooldown) {
		return true, nil
	}

	sent := false
	defer func() {
		if sent {
			return
		}
		// 发送失败：释放冷却，允许用户立即重试
		releaseCtx, releaseCancel := context.WithTimeout(context.Background(), opTimeout)
		defer releaseCancel()
		_ = store.ReleaseCooldown(releaseCtx, cooldownKey)
	}()

	cfg := configreader.GetConfig()
	siteTitle := cfg.Site.Title
	if siteTitle == "" {
		siteTitle = "CoBlog"
	}
	baseURL := cfg.Site.BaseURL
	if baseURL == "" {
		baseURL = "https://localhost"
	}

	activationURL := fmt.Sprintf("%s/activate?token=%s", baseURL, activationToken)
	subject := fmt.Sprintf("【%s】激活你的账户", siteTitle)
	body := buildActivationEmailHTML(siteTitle, activationURL)

	if err := SendMail(email, subject, body); err != nil {
		return false, err
	}
	sent = true
	return false, nil
}

// buildCodeEmailHTML 渲染验证码邮件正文
func buildCodeEmailHTML(siteTitle, action, code string) string {
	return fmt.Sprintf(
		`<div style="max-width:480px;margin:0 auto;font-family:Arial,sans-serif;">`+
			`<h2 style="color:#667eea;">%s</h2>`+
			`<p>你正在%s，验证码为：</p>`+
			`<p style="font-size:32px;font-weight:bold;letter-spacing:6px;color:#333;margin:20px 0;">%s</p>`+
			`<p style="color:#888;font-size:13px;">验证码 %d 分钟内有效，请勿泄露给他人。如非本人操作，请忽略此邮件。</p>`+
			`</div>`,
		siteTitle, action, code, int(codeTTL.Minutes()),
	)
}

// buildActivationEmailHTML 渲染账户激活邮件正文
func buildActivationEmailHTML(siteTitle, activationURL string) string {
	return fmt.Sprintf(
		`<div style="max-width:480px;margin:0 auto;font-family:Arial,sans-serif;">`+
			`<h2 style="color:#667eea;">欢迎加入 %s</h2>`+
			`<p>请点击下方按钮激活你的账户：</p>`+
			`<p style="margin:24px 0;">`+
			`<a href="%s" style="background:linear-gradient(135deg,#667eea,#764ba2);color:white;padding:12px 28px;border-radius:6px;text-decoration:none;font-size:16px;">激活账户</a>`+
			`</p>`+
			`<p style="color:#888;font-size:13px;">若按钮无法点击，请复制以下链接到浏览器：<br>%s</p>`+
			`<p style="color:#aaa;font-size:12px;">如非本人操作，请忽略此邮件。</p>`+
			`</div>`,
		siteTitle, activationURL, activationURL,
	)
}
