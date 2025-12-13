package middleware

import (
	"JHETBackend/common/exception"
	"JHETBackend/common/webtoken"

	"github.com/gin-gonic/gin"
)

func Auth(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.Error(exception.UsrNotLogin)
		c.Abort()
		//fmt.Println("鉴权失败: 未登录")
		return
	}
	if !webtoken.VerifyWt(authHeader) {
		c.Error(exception.UsrLoginInvalid)
		c.Abort()
		return
	}
	uid, pgid, err := webtoken.GetWtPayload(authHeader)
	if err != nil {
		c.Error(exception.UsrLoginInvalid)
		c.Abort()
		return
	}
	//fmt.Println("鉴权成功")
	c.Set("AccountID", uid)
	c.Set("PermissionGroupID", pgid)
	c.Next()
}
