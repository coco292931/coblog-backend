package userService

import (
	"context"
	"strconv"
	"time"

	"coblog-backend/common/exception"
	"coblog-backend/configs/cache"
	"coblog-backend/configs/database"
	"coblog-backend/models"
)

// activationStore 激活令牌所用存储。默认降级实例（不可用），由 main 注入真实 Redis。
var activationStore = cache.NewStore(nil)

// SetActivationStore 注入激活令牌所用存储
func SetActivationStore(store *cache.Store) {
	if store != nil {
		activationStore = store
	}
}

// 账户激活状态标记
const (
	activatedMark        = "activated"
	activatedPermGroupID = uint32(2) // USER 权限组
	// pendingMark 未激活哨兵值。数据库侧只用它表达「尚未激活」，
	// 必须非空，否则会被 database 启动时的存量回填逻辑误判为老账户。
	pendingMark = "pending"
)

// ActivationLinkTTL 激活链接有效期。
// 过期由 Redis TTL 负责：邮箱里只有一个不可猜的随机串，
// 不含任何可篡改的过期信息，也无需签名。
const ActivationLinkTTL = 24 * time.Hour

// keyActivation 激活令牌键：值为待激活账户的 UID
func keyActivation(token string) string {
	return cache.Key("coblog", "activation", token)
}

// IssueActivationToken 为指定账户签发激活令牌并写入 Redis。
// Redis 不可用时返回 SysUknExc，调用方应提示稍后重试，而不是发出无法使用的链接。
func IssueActivationToken(accountID uint64) (string, error) {
	if !activationStore.Available() {
		return "", exception.SysUknExc
	}

	token, err := cache.NewToken()
	if err != nil {
		return "", exception.SysUknExc
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 写入后由 TTL 自动回收，无需任何清理任务
	if err := activationStore.Set(ctx, keyActivation(token),
		strconv.FormatUint(accountID, 10), ActivationLinkTTL); err != nil {
		return "", exception.SysUknExc
	}

	return token, nil
}

// ActivateByToken 消费激活令牌并激活对应账户。
// 令牌为一次性：取出即删除（Redis 侧原子完成），重复点击会得到「令牌无效」。
func ActivateByToken(token string) error {
	if token == "" {
		return exception.UsrTokenInvalid
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	uidValue, hit, err := activationStore.Take(ctx, keyActivation(token))
	if err != nil {
		return exception.SysUknExc
	}
	if !hit {
		// 令牌不存在、已被使用或已过期
		return exception.UsrTokenInvalid
	}

	accountID, err := strconv.ParseUint(uidValue, 10, 64)
	if err != nil {
		return exception.UsrTokenInvalid
	}

	res := database.DataBase.Model(&models.AccountInfo{}).
		Where("id = ?", accountID).
		Updates(map[string]interface{}{
			"activation":    activatedMark,
			"perm_group_id": activatedPermGroupID,
		})
	if res.Error != nil {
		return exception.SysCannotUpdate
	}
	if res.RowsAffected == 0 {
		// 账户不存在或已处于激活态（值未变化时 RowsAffected 也可能为 0）
		return exception.UsrNotExisted
	}
	return nil
}

// IsActivated 检查账户是否已激活
func IsActivated(account *models.AccountInfo) bool {
	return account.Activation == activatedMark
}
