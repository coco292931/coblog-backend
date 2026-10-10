package webtoken

import (
	"encoding/base64"
	"testing"
	"time"
)

// 测试里不读配置文件：直接放一把固定密钥，并把 readKeyOnce 标记为已执行
func init() {
	for i := range wtSigkey {
		wtSigkey[i] = byte(i + 1)
	}
	readKeyOnce.Do(func() {})
}

func TestParseWtRoundTrip(t *testing.T) {
	tok := GenerateWt(42, 2, 7, 3600)
	p, ok := ParseWt(tok)
	if !ok {
		t.Fatal("刚签发的 token 应能通过校验")
	}
	if p.UID != 42 || p.PermGroupID != 2 || p.Version != 7 {
		t.Errorf("载荷不对: %+v", p)
	}
	if d := time.Until(p.ExpireAt); d < 3590*time.Second || d > 3600*time.Second {
		t.Errorf("过期时间不对: 剩余 %v", d)
	}
}

// 版本号在签名范围内：改了它签名就对不上，不能自己把旧 token 改成新版本
func TestParseWtRejectsTamperedVersion(t *testing.T) {
	tok := GenerateWt(42, 2, 1, 3600)
	raw, _ := base64.RawURLEncoding.DecodeString(tok)
	raw[20] = 2
	if _, ok := ParseWt(base64.RawURLEncoding.EncodeToString(raw)); ok {
		t.Error("篡改版本号后不应通过校验")
	}
}

// 历史 token 的预留位是 0，应解析为版本 0（与新列的默认值一致，上线不踢人）
func TestParseWtLegacyVersionZero(t *testing.T) {
	p, ok := ParseWt(GenerateWt(42, 2, 0, 3600))
	if !ok || p.Version != 0 {
		t.Errorf("版本 0 的 token 应正常解析，实际 ok=%v version=%d", ok, p.Version)
	}
}

func TestParseWtRejectsExpiredAndGarbage(t *testing.T) {
	if _, ok := ParseWt(GenerateWt(42, 2, 0, 0)); ok {
		t.Error("已过期的 token 不应通过")
	}
	for _, bad := range []string{"", "abc", "!!!!", GenerateWt(42, 2, 0, 3600) + "x"} {
		if _, ok := ParseWt(bad); ok {
			t.Errorf("格式不对的 token %q 不应通过", bad)
		}
	}
}
