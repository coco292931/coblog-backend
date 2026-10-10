package utils

import "unicode/utf8"

// 新密码规则。与前端 constants/account.js 的 validateNewPassword 保持一致。
//
// 放在 utils（不依赖数据库）是为了让 service 层与 /lite 视图层共用同一份：
// 之前只有页面在校验，JSON 的注册 / 找回密码接口直接收任何密码。

// PasswordMinLen 新密码最少字符数（按字符计，不是字节）
const PasswordMinLen = 6

// PasswordRuleText 新密码强度提示
const PasswordRuleText = "至少 6 位，建议同时包含字母与数字"

// PasswordTooShortMsg 新密码过短时的文案
const PasswordTooShortMsg = "新密码长度至少需要6位字符"

// ValidatePasswordRule 校验新密码是否满足规则，返回错误文案；通过时返回空串。
func ValidatePasswordRule(password string) string {
	if password == "" {
		return "请输入新密码"
	}
	if utf8.RuneCountInString(password) < PasswordMinLen {
		return PasswordTooShortMsg
	}
	return ""
}
