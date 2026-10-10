package loginControllers

import (
	"coblog-backend/common/exception"
	"coblog-backend/models"
	"coblog-backend/services/mailService"
	"coblog-backend/services/userService"
	"coblog-backend/utils"
	"errors"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type passwordLoginForm struct {
	Account  string `json:"account" binding:"required"` //返回的姓名或id
	Password string `json:"password" binding:"required"`
}

// AuthByPassword 通过密码认证
func AuthByCombo(c *gin.Context) {
	var postForm passwordLoginForm
	err := c.ShouldBindJSON(&postForm) //验证数据完整性
	if err != nil {
		c.Error(exception.ApiParamError)
		return
	}
	// 不打印密码
	fmt.Println("登录账号:", postForm.Account)

	ip := c.ClientIP()
	if err := userService.CheckLoginAllowed(ip, postForm.Account); err != nil {
		c.Error(err)
		return
	}

	var user interface{}
	var userErr error
	//matched, _ := regexp.MatchString(`^\d+$`, postForm.Account) 正则,已弃用
	//_, err = strconv.ParseUint(postForm.Account, 10, 64)
	//if err != nil {
	// Convert id string to uint64

	// if err != nil {
	// 	c.Error(exception.ApiParamError)
	// 	return
	// }

	fmt.Println("邮箱登录:", postForm.Account)
	user, userErr = userService.GetUserByEmail(postForm.Account) //从数据库获取用户信息,判断用户存在
	//}

	if errors.Is(userErr, gorm.ErrRecordNotFound) {
		userService.RecordLoginFailure(ip, postForm.Account)
		c.Error(exception.UsrNotExisted)
		return
	}
	if userErr != nil {
		c.Error(exception.SysUknExc)
		return
	}

	accountInfo, ok := user.(*models.AccountInfo)
	if !ok {
		c.Error(exception.SysUknExc)
		return
	}

	if err := userService.VerifyPwd(accountInfo, postForm.Password); err != nil { //验证密码
		var apiErr *exception.Exception
		if errors.As(err, &apiErr) {
			fmt.Println("密码错误0:", err)
			userService.RecordLoginFailure(ip, postForm.Account)
			c.Error(exception.UsrPasswordErr)
		} else {
			fmt.Println("密码错误1:", err)
			c.Error(exception.SysCannotLoadFromDB)
		}
		return
	}

	activated := userService.IsActivated(accountInfo)
	// 账户未激活：补发激活邮件，但不阻止登录
	if !activated {
		if token, issueErr := userService.IssueActivationToken(accountInfo.ID); issueErr != nil {
			fmt.Println("签发激活令牌失败:", issueErr)
		} else if cooldown, err := mailService.SendActivationEmail(accountInfo.Email, token); err != nil {
			fmt.Println("激活邮件发送失败:", err)
		} else if cooldown {
			fmt.Println("激活邮件发送过于频繁，已跳过")
		}
	}

	userService.ClearLoginFailures(postForm.Account)

	token, validSecs := userService.IssueSession(accountInfo)
	// 一并种一份 cookie：后端直出的 /lite 走浏览器导航、带不了 Authorization 头，
	// 只能靠 cookie 识别身份（深度文章的可见性依赖它）。
	utils.SetAuthCookie(c, token, int(validSecs))

	utils.JsonSuccessResponse(c, "登录成功", map[string]interface{}{
		"token":     token, //100000000 194年
		"userID":    accountInfo.ID,
		"username":  accountInfo.UserName,
		"userType":  strconv.FormatUint(uint64(accountInfo.PermGroupID), 10),
		"activated": activated,
	})
}
