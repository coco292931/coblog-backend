package utils

import "testing"

func TestValidatePasswordRule(t *testing.T) {
	cases := map[string]string{
		"":         "请输入新密码",
		"12345":    PasswordTooShortMsg,
		"123456":   "",
		"abc def!": "",
		"密码密码密码":   "", // 按字符计：6 个汉字算 6 位
		"密码密码密":    PasswordTooShortMsg,
	}
	for pwd, want := range cases {
		if got := ValidatePasswordRule(pwd); got != want {
			t.Errorf("ValidatePasswordRule(%q) = %q，期望 %q", pwd, got, want)
		}
	}
}
