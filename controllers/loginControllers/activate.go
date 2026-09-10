package loginControllers

import (
	"coblog-backend/common/exception"
	"coblog-backend/services/userService"
	"coblog-backend/utils"

	"github.com/gin-gonic/gin"
)

// ActivateAccount 处理邮件激活链接：GET /api/auth/activate?token=xxx
// 令牌为写入 Redis 的一次性随机串，过期由 Redis TTL 控制；
// 取出即删除（Redis 侧原子完成），因此重复点击会得到「令牌无效」。
func ActivateAccount(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.Error(exception.ApiParamError)
		return
	}

	if err := userService.ActivateByToken(token); err != nil {
		c.Error(err)
		return
	}

	utils.JsonSuccessResponse(c, "账户激活成功", nil)
}
