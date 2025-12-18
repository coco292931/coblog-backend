package middleware

import (
	"coblog-backend/common/exception"
	"coblog-backend/common/permission"

	"fmt"

	"github.com/gin-gonic/gin"
)

func NeedPerm(needed ...permission.PermissionID) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 直接使用类型断言获取 uint32 类型的值
		rawVal, exists := c.Get("PermissionGroupID")
		if !exists {
			fmt.Println("权限校验失败: 未找到权限组ID")
			c.Error(exception.UsrNotLogin)
			c.AbortWithStatus(401)
			return
		}
		
		pgid, ok := rawVal.(uint32)
		if !ok {
			fmt.Printf("权限校验失败: 权限组ID类型错误, 实际类型: %T\n", rawVal)
			c.Error(exception.UsrNotLogin)
			c.AbortWithStatus(401)
			return
		}
		
		if pgid == 0 {
			//c.Redirect(401, "/login") 感觉放进错误处理中间件?
			fmt.Println("权限校验失败: 未登录")
			c.Error(exception.UsrNotLogin)
			c.AbortWithStatus(401) // 直接返回401
			return
		}
		if !permission.IsPermSatisfied(pgid, needed...) {
			c.Error(exception.UsrNotPermitted)
			c.Abort()
			return
		}
		c.Next()
	}
}
