package mailService

import (
	"context"
	"testing"
	"time"

	"coblog-backend/configs/cache"

	"github.com/redis/go-redis/v9"
)

// 依赖本机 Redis，不可用时自动跳过
func useRealStore(t *testing.T) *cache.Store {
	t.Helper()

	c := redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:6379",
		DialTimeout: 500 * time.Millisecond,
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := c.Ping(ctx).Err(); err != nil {
		_ = c.Close()
		t.Skipf("本机 Redis 不可用，跳过: %v", err)
	}
	t.Cleanup(func() { c.Close() })

	s := cache.NewStore(c)
	SetStore(s)
	return s
}

func uniqueEmail(t *testing.T) string {
	t.Helper()
	return "test-" + time.Now().Format("150405.000000") + "@example.com"
}

// cleanupCode 注册清理，避免测试在真实 Redis 中留下残留键
func cleanupCode(t *testing.T, email string, purposes ...CodePurpose) {
	t.Helper()
	s := store
	t.Cleanup(func() {
		if !s.Available() {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		for _, p := range purposes {
			_ = s.Delete(ctx, keyCode(p, email))
			_ = s.Delete(ctx, keyCooldown(string(p), email))
		}
	})
}

func TestIssueAndVerifyCode(t *testing.T) {
	useRealStore(t)
	email := uniqueEmail(t)
	cleanupCode(t, email, PurposeRegister)

	code, cooldown, err := IssueCode(PurposeRegister, email)
	if err != nil {
		t.Fatalf("签发验证码失败: %v", err)
	}
	if cooldown {
		t.Fatal("首次签发不应触发冷却")
	}
	if len(code) != codeLength {
		t.Fatalf("验证码长度应为 %d，实际 %d", codeLength, len(code))
	}

	if !VerifyCode(PurposeRegister, email, code) {
		t.Fatal("正确的验证码应校验通过")
	}
	// 一次性：重复使用必须失败
	if VerifyCode(PurposeRegister, email, code) {
		t.Fatal("验证码应一次性使用，第二次必须失败")
	}
}

func TestVerifyCodeRejectsWrongCodeWithoutConsuming(t *testing.T) {
	useRealStore(t)
	email := uniqueEmail(t)
	cleanupCode(t, email, PurposeReset)

	code, _, err := IssueCode(PurposeReset, email)
	if err != nil {
		t.Fatalf("签发验证码失败: %v", err)
	}

	if VerifyCode(PurposeReset, email, "000000") && code != "000000" {
		t.Fatal("错误验证码不应通过")
	}
	// 关键：错误尝试不能消耗掉记录，否则暴力枚举可让合法用户无法验证
	if !VerifyCode(PurposeReset, email, code) {
		t.Fatal("错误尝试后，正确验证码仍应可用")
	}
}

func TestVerifyCodeIsPurposeScoped(t *testing.T) {
	useRealStore(t)
	email := uniqueEmail(t)
	cleanupCode(t, email, PurposeRegister, PurposeReset)

	code, _, err := IssueCode(PurposeRegister, email)
	if err != nil {
		t.Fatalf("签发验证码失败: %v", err)
	}

	// 注册验证码不能用于找回密码
	if VerifyCode(PurposeReset, email, code) {
		t.Fatal("验证码不应跨用途使用")
	}
	// 原用途仍然可用（未被错误尝试消耗）
	if !VerifyCode(PurposeRegister, email, code) {
		t.Fatal("原用途的验证码应仍然有效")
	}
}

func TestVerifyCodeEmailIsCaseInsensitive(t *testing.T) {
	useRealStore(t)
	cleanupCode(t, "Mixed.Case@Example.com", PurposeLogin)

	code, _, err := IssueCode(PurposeLogin, "Mixed.Case@Example.com")
	if err != nil {
		t.Fatalf("签发验证码失败: %v", err)
	}

	if !VerifyCode(PurposeLogin, "mixed.case@example.com", code) {
		t.Fatal("邮箱大小写不同应视为同一账户")
	}
}

func TestIssueCodeRespectsCooldown(t *testing.T) {
	useRealStore(t)
	email := uniqueEmail(t)
	cleanupCode(t, email, PurposeRegister)

	if _, cooldown, err := IssueCode(PurposeRegister, email); err != nil || cooldown {
		t.Fatalf("首次签发应成功，实际 cooldown=%v err=%v", cooldown, err)
	}
	if _, cooldown, err := IssueCode(PurposeRegister, email); err != nil || !cooldown {
		t.Fatalf("冷却期内应返回 cooldown=true，实际 cooldown=%v err=%v", cooldown, err)
	}
}

func TestCooldownIsPerPurpose(t *testing.T) {
	useRealStore(t)
	email := uniqueEmail(t)
	cleanupCode(t, email, PurposeRegister, PurposeReset)

	if _, cooldown, _ := IssueCode(PurposeRegister, email); cooldown {
		t.Fatal("首次签发应成功")
	}
	// 不同用途互不影响冷却
	if _, cooldown, err := IssueCode(PurposeReset, email); err != nil || cooldown {
		t.Fatalf("不同用途不应共享冷却，实际 cooldown=%v err=%v", cooldown, err)
	}
}

func TestVerifyCodeExpires(t *testing.T) {
	s := useRealStore(t)
	email := uniqueEmail(t)
	cleanupCode(t, email, PurposeReset)

	code, _, err := IssueCode(PurposeReset, email)
	if err != nil {
		t.Fatalf("签发验证码失败: %v", err)
	}

	// 直接改短有效期以模拟过期
	ctx := context.Background()
	if err := s.Set(ctx, keyCode(PurposeReset, email), code, 50*time.Millisecond); err != nil {
		t.Fatalf("重设有效期失败: %v", err)
	}
	time.Sleep(150 * time.Millisecond)

	if VerifyCode(PurposeReset, email, code) {
		t.Fatal("过期验证码不应通过")
	}
}

func TestGenerateCodeFormat(t *testing.T) {
	for i := 0; i < 50; i++ {
		code, err := GenerateCode()
		if err != nil {
			t.Fatalf("生成验证码失败: %v", err)
		}
		if len(code) != codeLength {
			t.Fatalf("长度应为 %d，实际 %q", codeLength, code)
		}
		for _, ch := range code {
			if ch < '0' || ch > '9' {
				t.Fatalf("验证码应为纯数字，实际 %q", code)
			}
		}
	}
}

func TestDiscardCodeReleasesCooldown(t *testing.T) {
	useRealStore(t)
	email := uniqueEmail(t)
	cleanupCode(t, email, PurposeReset)

	code, cooldown, err := IssueCode(PurposeReset, email)
	if err != nil || cooldown {
		t.Fatalf("首次签发应成功，实际 cooldown=%v err=%v", cooldown, err)
	}

	// 模拟邮件发送失败后的清理
	DiscardCode(PurposeReset, email)

	// 验证码应已被丢弃
	if VerifyCode(PurposeReset, email, code) {
		t.Error("丢弃后验证码不应再可用")
	}
	// 冷却应已释放，允许立即重试（否则用户要白等 60 秒）
	if _, cooldown, err := IssueCode(PurposeReset, email); err != nil || cooldown {
		t.Errorf("丢弃后应可立即重新签发，实际 cooldown=%v err=%v", cooldown, err)
	}
}

func TestUnavailableStoreFailsClosed(t *testing.T) {
	// Redis 不可用时必须「失败关闭」：不签发、不通过校验
	SetStore(cache.NewStore(nil))
	t.Cleanup(func() { SetStore(cache.NewStore(nil)) })

	email := uniqueEmail(t)
	if code, _, err := IssueCode(PurposeRegister, email); code != "" || err == nil {
		t.Errorf("不可用时不应签发验证码，实际 code=%q err=%v", code, err)
	}
	if VerifyCode(PurposeRegister, email, "123456") {
		t.Error("不可用时不应让任何验证码通过")
	}
	if Available() {
		t.Error("不可用时 Available() 应为 false")
	}
}
