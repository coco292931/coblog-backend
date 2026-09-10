package mailService

import (
	"context"
	"crypto/rand"
	"strings"
	"sync"
	"time"

	"coblog-backend/common/exception"
	"coblog-backend/configs/cache"
)

// 验证码用途，区分注册 / 改密 / 找回，避免一码多用
type CodePurpose string

const (
	PurposeRegister  CodePurpose = "register"   // 注册验证
	PurposeReset     CodePurpose = "reset"      // 找回密码
	PurposeChangePwd CodePurpose = "change_pwd" // 修改密码
	PurposeLogin     CodePurpose = "login"      // 邮箱验证码登录
)

const (
	codeTTL        = 10 * time.Minute // 验证码有效期，由 Redis TTL 控制
	resendCooldown = 60 * time.Second // 同一邮箱+用途的最短重发间隔
	codeLength     = 6                // 验证码位数
	opTimeout      = 2 * time.Second  // 单次 Redis 操作超时
)

// store 验证码所用存储，默认降级实例（不可用），由 main 注入真实 Redis。
// 不做内存兜底：验证码属一次性短期凭证，进程重启后失效是正确语义，
// 且内存方案在多实例下会各自为政。
var store = cache.NewStore(nil)

// SetStore 注入验证码所用存储
func SetStore(s *cache.Store) {
	if s != nil {
		store = s
	}
}

// Available 报告验证码功能是否可用（Redis 是否就绪）
func Available() bool {
	return store.Available()
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// keyCode 验证码键：用途与邮箱共同决定，避免一码多用
func keyCode(p CodePurpose, email string) string {
	return cache.Key("coblog", "code", string(p), normalizeEmail(email))
}

// keyCooldown 重发冷却键（scope 区分验证码用途 / 激活邮件）
func keyCooldown(scope, email string) string {
	return cache.Key("coblog", "cooldown", scope, normalizeEmail(email))
}

// GenerateCode 生成 codeLength 位纯数字验证码（加密安全随机）
func GenerateCode() (string, error) {
	const digits = "0123456789"
	buf := make([]byte, codeLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	for i := range buf {
		buf[i] = digits[int(buf[i])%len(digits)]
	}
	return string(buf), nil
}

// IssueCode 为指定邮箱+用途签发验证码并写入 Redis。
// 若距上次发送不足冷却时间，返回 cooldown=true 拒绝下发。
func IssueCode(p CodePurpose, email string) (code string, cooldown bool, err error) {
	if !store.Available() {
		return "", false, exception.SysUknExc
	}

	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	// SET NX：抢到冷却标记才允许发送，天然防止并发重复下发
	cooldownKey := keyCooldown(string(p), email)
	if !store.AcquireCooldown(ctx, cooldownKey, resendCooldown) {
		return "", true, nil
	}

	code, err = GenerateCode()
	if err != nil {
		// 生成失败：释放冷却，避免用户被无谓地锁 60 秒
		_ = store.ReleaseCooldown(ctx, cooldownKey)
		return "", false, err
	}

	if err := store.Set(ctx, keyCode(p, email), code, codeTTL); err != nil {
		_ = store.ReleaseCooldown(ctx, cooldownKey)
		return "", false, exception.SysUknExc
	}

	return code, false, nil
}

// VerifyCode 校验验证码是否匹配且未过期。校验成功后立即失效（一次性）。
func VerifyCode(p CodePurpose, email, code string) bool {
	if !store.Available() {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	// Lua 保证「比对 + 删除」原子完成：正确才消费，
	// 错误时不消耗记录，避免暴力枚举直接把记录打掉
	_, hit, err := store.TakeIfMatch(ctx, keyCode(p, email), strings.TrimSpace(code))
	return err == nil && hit
}

// DiscardCode 丢弃指定邮箱+用途的验证码并释放重发冷却。
// 用于「验证码已签发但邮件发送失败」的场景：否则用户会白等 60 秒冷却，
// 且邮箱永远收不到那个已写入 Redis 的验证码。
func DiscardCode(p CodePurpose, email string) {
	if !store.Available() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	_ = store.Delete(ctx, keyCode(p, email))
	_ = store.ReleaseCooldown(ctx, keyCooldown(string(p), email))
}

// ---- 激活邮件重发冷却 ----
// TODO: 待激活令牌迁移到 Redis 后，与验证码统一使用 Redis 的 SET NX 实现

type mailSendStore struct {
	mu sync.Mutex
	m  map[string]time.Time
}

var sendStore = &mailSendStore{m: make(map[string]time.Time)}

func reserveActivationMail(email string) (cooldown bool, release func(success bool)) {
	key := "activation:" + normalizeEmail(email)
	now := time.Now()

	sendStore.mu.Lock()
	if sentAt, ok := sendStore.m[key]; ok && now.Sub(sentAt) < resendCooldown {
		sendStore.mu.Unlock()
		return true, nil
	}
	sendStore.m[key] = now
	sendStore.mu.Unlock()

	return false, func(success bool) {
		if success {
			return
		}
		sendStore.mu.Lock()
		delete(sendStore.m, key)
		sendStore.mu.Unlock()
	}
}
