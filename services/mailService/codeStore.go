package mailService

import (
	"crypto/rand"
	"sync"
	"time"
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
	codeTTL       = 10 * time.Minute // 验证码有效期
	resendCooldown = 60 * time.Second // 同一邮箱+用途的最短重发间隔
	codeLength    = 6                 // 验证码位数
)

// 存储条目
type codeEntry struct {
	code     string
	expireAt time.Time
	sentAt   time.Time
}

// key = purpose + ":" + email
type codeStore struct {
	mu sync.Mutex
	m  map[string]codeEntry
}

var store = &codeStore{m: make(map[string]codeEntry)}

func storeKey(p CodePurpose, email string) string {
	return string(p) + ":" + email
}

// GenerateCode 生成一个 codeLength 位的纯数字验证码（加密安全随机）
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

// IssueCode 为指定邮箱+用途签发一个验证码并存储。
// 若距上次发送不足冷却时间，返回 cooldown=true 拒绝下发，用于频率限制。
func IssueCode(p CodePurpose, email string) (code string, cooldown bool, err error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	key := storeKey(p, email)
	now := time.Now()
	if existing, ok := store.m[key]; ok {
		if now.Sub(existing.sentAt) < resendCooldown {
			return "", true, nil
		}
	}

	code, err = GenerateCode()
	if err != nil {
		return "", false, err
	}
	store.m[key] = codeEntry{
		code:     code,
		expireAt: now.Add(codeTTL),
		sentAt:   now,
	}
	return code, false, nil
}

// VerifyCode 校验验证码是否匹配且未过期。校验成功后立即失效（一次性）。
func VerifyCode(p CodePurpose, email, code string) bool {
	store.mu.Lock()
	defer store.mu.Unlock()

	key := storeKey(p, email)
	entry, ok := store.m[key]
	if !ok {
		return false
	}
	if time.Now().After(entry.expireAt) {
		delete(store.m, key)
		return false
	}
	if entry.code != code {
		return false
	}
	// 一次性使用，校验通过即删除
	delete(store.m, key)
	return true
}

// CooldownSeconds 返回提示用的冷却秒数
func CooldownSeconds() int {
	return int(resendCooldown / time.Second)
}

// startCleanup 定期清理过期条目，避免内存堆积
func startCleanup() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now()
			store.mu.Lock()
			for k, v := range store.m {
				if now.After(v.expireAt) {
					delete(store.m, k)
				}
			}
			store.mu.Unlock()
		}
	}()
}

var cleanupOnce sync.Once

// InitCodeStore 启动后台清理（幂等），可在程序启动时调用
func InitCodeStore() {
	cleanupOnce.Do(startCleanup)
}
