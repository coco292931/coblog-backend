package userService

import (
	"context"
	"log"
	"strings"
	"time"

	"coblog-backend/common/exception"
	"coblog-backend/configs/cache"
)

// 密码登录的失败次数限制。
//
// 没有它的话，密码可以被无限次尝试。按两个维度计数：
//   - 账号：防止针对某一个账号的爆破
//   - IP：防止换着账号试（同时拖慢「用户不存在 / 密码错误」两种文案带来的邮箱枚举）
//
// 只数失败；登录成功清掉该账号的计数。窗口内超限即拒绝，窗口到期自动解除。

const (
	loginFailWindow     = 15 * time.Minute
	maxLoginFailAccount = 10 // 同一账号窗口内最多失败次数
	maxLoginFailIP      = 30 // 同一 IP 窗口内最多失败次数（多个账号共用一个出口时留余量）
)

// authStore 登录限流与 token 吊销所用存储。默认降级实例（不可用），由 main 注入真实 Redis。
var authStore = cache.NewStore(nil)

// SetAuthStore 注入登录限流与 token 吊销所用存储
func SetAuthStore(store *cache.Store) {
	if store != nil {
		authStore = store
	}
}

func keyLoginFailAccount(email string) string {
	return cache.Key("coblog", "loginfail", "acct", strings.ToLower(strings.TrimSpace(email)))
}

func keyLoginFailIP(ip string) string {
	return cache.Key("coblog", "loginfail", "ip", ip)
}

// CheckLoginAllowed 在校验密码之前调用：该账号或该 IP 失败过多时返回 UsrLoginTooFreq。
//
// Redis 不可用时放行（失败开放）：限流是附加防线，不能因为缓存故障让所有人都登录不了。
func CheckLoginAllowed(ip, email string) error {
	if !authStore.Available() {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if n, err := authStore.Count(ctx, keyLoginFailAccount(email)); err == nil && n >= maxLoginFailAccount {
		return exception.UsrLoginTooFreq
	}
	if ip != "" {
		if n, err := authStore.Count(ctx, keyLoginFailIP(ip)); err == nil && n >= maxLoginFailIP {
			return exception.UsrLoginTooFreq
		}
	}
	return nil
}

// RecordLoginFailure 记录一次失败（用户不存在、密码错误都算）。
func RecordLoginFailure(ip, email string) {
	if !authStore.Available() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if _, err := authStore.IncrWindow(ctx, keyLoginFailAccount(email), loginFailWindow); err != nil {
		log.Printf("[WARN][Login] 记录登录失败次数出错: %v", err)
	}
	if ip != "" {
		_, _ = authStore.IncrWindow(ctx, keyLoginFailIP(ip), loginFailWindow)
	}
}

// ClearLoginFailures 登录成功后清掉该账号的失败计数（IP 的不清，避免用一个自己的账号给 IP 洗白）。
func ClearLoginFailures(email string) {
	if !authStore.Available() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = authStore.Delete(ctx, keyLoginFailAccount(email))
}
