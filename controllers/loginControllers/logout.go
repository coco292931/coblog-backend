package loginControllers

import (
	"coblog-backend/utils"

	"github.com/gin-gonic/gin"
)

// Logout POST /api/auth/logout
//
// 前端登出时会把 localStorage / sessionStorage 里的 token 删掉，但登录 cookie
// 是 HttpOnly 的，JS 删不掉 —— 不调这个接口的话，登出之后 /lite 仍然认为用户
// 处于登录态（共用设备上这是个真实的信息泄露）。
//
// 这里刻意不做鉴权：cookie 已经失效时清一次也无害，加鉴权反而会让
// 「token 过期了想登出」这种场景卡住。
func Logout(c *gin.Context) {
	utils.ClearAuthCookie(c)
	utils.JsonSuccessResponse(c, "已退出登录", nil)
}
