package userService

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log"
	"time"

	"coblog-backend/common/exception"
	"coblog-backend/common/webtoken"
	"coblog-backend/configs/cache"
	configreader "coblog-backend/configs/configReader"
	"coblog-backend/models"
)

// 登录会话：签发、校验、吊销。
//
// token 本身是无状态的签名串，单靠它做不到「登出即失效」「改密后旧设备下线」。
// 这里补两层：
//   - 登出：把这一个 token 的哈希写进 Redis 黑名单，有效期等于 token 剩余寿命
//   - 改密 / 重置：账户的 token_version +1，旧版本的 token 一律作废（见 updatePasswordHash）
//
// 校验时顺带取数据库里当前的权限组，而不是 token 里签发时的快照：
// 账户激活后权限组从 GUEST 升到 USER，不用重新登录就生效。

// IssueSession 为账户签发登录 token，返回 token 与有效秒数。
func IssueSession(user *models.AccountInfo) (token string, validSecs uint64) {
	validSecs = configreader.GetConfig().Account.ValidSecs
	return webtoken.GenerateWt(user.ID, user.PermGroupID, user.TokenVersion, validSecs), validSecs
}

// ValidateSession 校验登录 token：签名、有效期、黑名单、版本号，
// 通过时返回数据库中的当前账号（权限组以它为准）。
//
// 这是每个已登录请求唯一一次查用户表：中间件会把返回的账号放进请求上下文，
// 同一请求里的其他地方直接复用，不要再按 ID 查一遍。
func ValidateSession(token string) (*models.AccountInfo, error) {
	payload, ok := webtoken.ParseWt(token)
	if !ok {
		return nil, exception.UsrLoginInvalid
	}

	if revoked, rerr := isRevoked(token); rerr == nil && revoked {
		return nil, exception.UsrLoginInvalid
	} else if rerr != nil && !errors.Is(rerr, cache.ErrUnavailable) {
		// Redis 出错时放行：黑名单只管「主动登出」，cookie 那一份登出时已经清掉了
		log.Printf("[WARN][Session] 读取 token 黑名单失败: %v", rerr)
	}

	user, err := GetUserByID(payload.UID)
	if err != nil {
		// 账号不存在（含软删除）或数据库读不到，都按登录无效处理
		return nil, exception.UsrLoginInvalid
	}
	if user.TokenVersion != payload.Version {
		return nil, exception.UsrLoginInvalid
	}
	return user, nil
}

// RevokeSession 吊销一个 token（登出）。无效或已过期的 token 直接忽略。
func RevokeSession(token string) {
	payload, ok := webtoken.ParseWt(token)
	if !ok || !authStore.Available() {
		return
	}
	ttl := time.Until(payload.ExpireAt)
	if ttl <= 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := authStore.Set(ctx, keyRevoked(token), "1", ttl); err != nil {
		log.Printf("[WARN][Session] 写入 token 黑名单失败: %v", err)
	}
}

func isRevoked(token string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return authStore.Exists(ctx, keyRevoked(token))
}

// keyRevoked 黑名单键。存哈希不存原文：Redis 被读到也拿不到可用的 token
func keyRevoked(token string) string {
	sum := sha256.Sum256([]byte(token))
	return cache.Key("coblog", "revoked", hex.EncodeToString(sum[:]))
}

// ReissueSession 改密后给当前设备换一个新版本的 token（旧版本此时已全部作废）。
func ReissueSession(accountID uint64) (token string, validSecs uint64, err error) {
	user, err := GetUserByID(accountID)
	if err != nil {
		return "", 0, err
	}
	token, validSecs = IssueSession(user)
	return token, validSecs, nil
}
