package userService

import (
	"coblog-backend/common/exception"
	"coblog-backend/configs/database"
	"coblog-backend/models"
	"crypto/sha256"
	"fmt"
)

// HashPwd 计算密码哈希，与登录/注册保持一致（SHA256 十六进制）
func HashPwd(password string) string {
	hash := sha256.Sum256([]byte(password))
	return fmt.Sprintf("%x", hash)
}

// ChangePwd 校验旧密码后修改为新密码（用于已登录用户主动改密）
func ChangePwd(accountID uint64, oldPwd string, newPwd string) error {
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
	user, err := GetUserByEmail(email)
	if err != nil {
		return exception.UsrNotExisted
	}
	return updatePasswordHash(user.ID, HashPwd(newPwd))
}

// updatePasswordHash 仅更新密码哈希字段
func updatePasswordHash(accountID uint64, hash string) error {
	res := database.DataBase.Model(&models.AccountInfo{}).
		Where("id = ?", accountID).
		Update("password_hash", hash)
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
