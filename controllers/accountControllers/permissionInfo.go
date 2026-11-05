package accountControllers

import (
	"coblog-backend/common/exception"
	"coblog-backend/common/permission"
	"coblog-backend/utils"

	"github.com/gin-gonic/gin"
)

// GetMyPermissions GET /api/user/perm
//
// 只要求登录、不要求任何具体权限：前端据此决定"写作"入口要不要给，
// 免得用户写完整篇才在提交时收到 1002。
//
// 返回的是权限位里的开关，不是权限组 ID —— 组 ID 对前端没有意义。
func GetMyPermissions(c *gin.Context) {
	raw, exists := c.Get("PermissionGroupID")
	pgid, ok := raw.(uint32)
	if !exists || !ok || pgid == 0 {
		c.Error(exception.UsrNotLogin)
		return
	}

	granted := func(p permission.PermissionID) bool {
		return permission.IsPermSatisfied(pgid, p)
	}

	utils.JsonSuccessResponse(c, "查询成功", gin.H{
		"postPost":      granted(permission.Perm_PostPost),
		"uploadFile":    granted(permission.Perm_UploadFile),
		"getProfile":    granted(permission.Perm_GetProfile),
		"changePwd":     granted(permission.Perm_ChangePassword),
		"updateProfile": granted(permission.Perm_UpdateProfile),
	})
}
