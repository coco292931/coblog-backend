package loginControllers

import (
	"coblog-backend/common/exception"
	"coblog-backend/services/userService"
	"coblog-backend/utils"

	"github.com/gin-gonic/gin"
)

// ActivateAccount 处理邮件激活链接：GET /api/auth/activate?token=xxx
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
