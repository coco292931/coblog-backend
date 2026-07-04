package registerControllers

import (
	"coblog-backend/common/exception"
	"coblog-backend/services/mailService"
	"coblog-backend/services/userService"
	"coblog-backend/utils"
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
)

type UserInfo struct {
	Email    string `json:"email"  binding:"required"`
	UserName string `json:"username" binding:"required"`
	Password string `json:"password"   binding:"required"`
	//PermGroupID
}

func CreateNormalUser(c *gin.Context) { //用户注册,默认权限组PermGroupID=1(GUEST)
	var postForm UserInfo
	err := c.ShouldBindJSON(&postForm)
	if err != nil {
		c.Error(exception.ApiParamError)
		fmt.Println("参数错误0:", err)
		return
	}

	if !utils.IsValidEmail(postForm.Email) {
		c.Error(exception.ApiParamError)
		fmt.Println("邮箱格式错误:", postForm.Email)
		return
	}

	fmt.Println("注册信息:", postForm)
	user, err := userService.CreateUser(
		postForm.Password,
		postForm.Email,
		postForm.UserName,
		1, // GUEST
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

	activationMailSent := true
	msg := "注册成功，激活邮件已发送，请前往邮箱完成激活"
	if cooldown, err := mailService.SendActivationEmail(user.Email, user.Activation); err != nil {
		activationMailSent = false
		msg = "注册成功，但激活邮件发送失败，请在“我的”页面重新发送"
		fmt.Println("激活邮件发送失败:", err)
	} else if cooldown {
		activationMailSent = false
		msg = "注册成功，激活邮件已在近期发送，请前往邮箱查收"
	}

	utils.JsonSuccessResponse(c, msg, map[string]interface{}{
		"userID":             user.ID,
		"email":              user.Email,
		"activationMailSent": activationMailSent,
	})
}
