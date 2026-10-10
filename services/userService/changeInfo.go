package userService

import (
	"coblog-backend/common/exception"
	"coblog-backend/configs/database"
	"coblog-backend/models"
	"coblog-backend/utils"
	"crypto/sha256"
	"fmt"

	"gorm.io/gorm"
)

// HashPwd 计算密码哈希，与登录/注册保持一致（SHA256 十六进制）
func HashPwd(password string) string {
	hash := sha256.Sum256([]byte(password))
	return fmt.Sprintf("%x", hash)
}

// ChangePwd 校验旧密码后修改为新密码（用于已登录用户主动改密）
func ChangePwd(accountID uint64, oldPwd string, newPwd string) error {
	if err := checkPasswordRule(newPwd); err != nil {
		return err
	}
	user, err := GetUserByID(accountID)
	if err != nil {
		return exception.SysCannotReadDB
	}
	if err := VerifyPwd(user, oldPwd); err != nil {
		return exception.UsrPasswordErr
	}
	return updatePasswordHash(user.ID, HashPwd(newPwd))
}

// ResetPwdByEmail 直接将指定邮箱用户的密码重置为新密码（用于验证码找回，调用方需先校验验证码）
func ResetPwdByEmail(email string, newPwd string) error {
	if err := checkPasswordRule(newPwd); err != nil {
		return err
	}
	user, err := GetUserByEmail(email)
	if err != nil {
		return exception.UsrNotExisted
	}
	return updatePasswordHash(user.ID, HashPwd(newPwd))
}

// checkPasswordRule 新密码规则在 service 层兜底：
// 页面（主站与 /lite）都校验过，但 JSON 接口可以绕过页面直接调用。
func checkPasswordRule(password string) error {
	if msg := utils.ValidatePasswordRule(password); msg != "" {
		if msg == utils.PasswordTooShortMsg {
			return exception.UsrPasswordWeak
		}
		return exception.ApiParamError
	}
	return nil
}

// updatePasswordHash 更新密码哈希，同时让该账户之前签发的所有登录 token 失效
func updatePasswordHash(accountID uint64, hash string) error {
	res := database.DataBase.Model(&models.AccountInfo{}).
		Where("id = ?", accountID).
		Updates(map[string]interface{}{
			"password_hash": hash,
			"token_version": gorm.Expr("token_version + 1"),
		})
	if res.Error != nil {
		return exception.SysCannotUpdate
	}
	return nil
}

func RstRSSToken(accountID uint64) (string, error) {
	user, err := GetUserByID(accountID)
	if err != nil {
		return "", exception.SysCannotReadDB
	}
	newToken := GenToken(user.Email)
	res := database.DataBase.Model(&models.AccountInfo{}).
		Where("id = ?", accountID).
		Update("rss_token", newToken)
	if res.Error != nil {
		return "", exception.SysCannotUpdate
	}
	return newToken, nil
}
