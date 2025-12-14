package registerControllers

import (
	"JHETBackend/common/exception"
	"JHETBackend/services/userService"
	"JHETBackend/utils"
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
)

type UserInfo struct {
	Email            string `json:"email"  binding:"required"`
	VerificationCode string `json:"verificationCode"` //验证码，还没想好是先验证还是后验证注册
	UserName         string `json:"username"`
	Password         string `json:"password"   binding:"required"`
	//PermGroupID
}

func CreateNormalUser(c *gin.Context) { //用户注册,强制绑定权限组PermGroupID=2
	var postForm UserInfo
	err := c.ShouldBindJSON(&postForm)
	if err != nil {
		c.Error(exception.ApiParamError)
		fmt.Println("参数错误0:", err)
		return
	}

	//在此处加入验证码判断逻辑（如有

	fmt.Println("注册信息:", postForm)
	user, err := userService.CreateUser(
		postForm.Password,
		postForm.Email, 
		postForm.UserName,
		2, //用户类型， PermGroupID 需要修改
	)
	if err != nil {
		if errors.Is(err, exception.ApiParamError) {
			fmt.Println("参数错误1:", err)
			c.Error(exception.ApiParamError)
		} else if errors.Is(err, exception.UsrAlreadyExisted) {
			fmt.Println("用户已存在:", err)
			c.Error(exception.UsrAlreadyExisted)
		} else {
			fmt.Println("读取失败0:", err)
			c.Error(exception.SysCannotLoadFromDB)
		}
		return
	}

	utils.JsonSuccessResponse(c, "注册成功", map[string]interface{}{
		"userID": user.ID,
	})
}
