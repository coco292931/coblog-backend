package middleware

import (
	"coblog-backend/common/exception"
	"coblog-backend/models"
	"coblog-backend/services/userService"
	"coblog-backend/utils"
	"fmt"

	"github.com/gin-gonic/gin"
)

// 鉴权成功时额外放在 context 上的几项
const (
	AuthTokenKey      = "AuthToken"      // string：本次请求使用的登录 token（仅 LooseAuth）
	AuthViaCookieKey  = "AuthViaCookie"  // bool：token 是否取自 cookie（仅 LooseAuth）
	CurrentAccountKey = "CurrentAccount" // *models.AccountInfo：鉴权时从数据库读到的当前账号
)

// CurrentAccount 取鉴权时已经读出的当前账号；未登录时为 nil。
//
// 鉴权本来就要按 ID 查一次用户表（校验 token 版本号、取当前权限组），
// 同一请求里需要账号信息的地方都从这里拿，不要再按 ID 查一遍。
// 返回的是共享对象，只读；要改字段请先复制。
func CurrentAccount(c *gin.Context) *models.AccountInfo {
	raw, ok := c.Get(CurrentAccountKey)
	if !ok {
		return nil
	}
	user, _ := raw.(*models.AccountInfo)
	return user
}

func Auth(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.Error(exception.UsrNotLogin)
		c.Abort()
		fmt.Println("鉴权失败: 未登录")
		return
	}
	// 签名、有效期、登出黑名单、改密后的版本号一并校验；权限组取数据库当前值
	user, err := userService.ValidateSession(authHeader)
	if err != nil {
		c.Error(exception.UsrLoginInvalid)
		c.Abort()
		return
	}

	fmt.Println("鉴权成功")
	c.Set("AccountID", user.ID)
	c.Set("PermissionGroupID", user.PermGroupID)
	c.Set(CurrentAccountKey, user)
	c.Next()
}

func LooseAuth(c *gin.Context) {
	// 默认未登录状态
	c.Set("AccountID", uint64(0))
	c.Set("PermissionGroupID", uint64(0))

	token := c.GetHeader("Authorization")
	viaCookie := false
	if token == "" {
		// 浏览器地址栏导航（后端直出的 /lite 就是这种）带不了自定义头，
		// 所以兜底读一次 cookie —— cookie 由登录接口写入。
		// 注意只有松鉴权会读 cookie：/api 的写操作仍然以 Authorization 头为凭据。
		// /lite 的写操作以 cookie 为凭据，由 liteControllers.Guard 另做 CSRF 校验。
		token = utils.AuthTokenFromCookie(c)
		viaCookie = true
	}
	if token == "" {
		c.Next()
		return
	}
	user, err := userService.ValidateSession(token)
	if err != nil {
		fmt.Println("松鉴权失败: 用户登录无效，已放行")
		c.Next()
		return
	}

	fmt.Println("松鉴权成功")
	c.Set("AccountID", user.ID)
	c.Set("PermissionGroupID", user.PermGroupID)
	c.Set(CurrentAccountKey, user)
	// 供 /lite 派生 CSRF token：只有凭据来自 cookie 时才需要防 CSRF
	// （跨站请求设不了 Authorization 头）
	c.Set(AuthTokenKey, token)
	c.Set(AuthViaCookieKey, viaCookie)
	c.Next()
}
