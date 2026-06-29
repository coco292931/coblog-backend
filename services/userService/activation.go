package userService

import (
	"coblog-backend/common/exception"
	"coblog-backend/configs/database"
	"coblog-backend/models"
	"crypto/rand"
	"encoding/hex"
)

const activatedMark = "activated"

// GenActivationToken 生成一个 32 字节随机 hex token，用于邮件激活链接
func GenActivationToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// IsActivated 检查账户是否已激活
func IsActivated(account *models.AccountInfo) bool {
	return account.Activation == activatedMark
}

// ActivateByToken 通过激活 token 激活账户，激活后将 Activation 置为 "activated"
func ActivateByToken(token string) error {
	res := database.DataBase.Model(&models.AccountInfo{}).
		Where("activation = ?", token).
		Update("activation", activatedMark)
	if res.Error != nil {
		return exception.SysCannotUpdate
	}
	if res.RowsAffected == 0 {
		return exception.UsrTokenInvalid
	}
	return nil
}

// SetActivationToken 将指定账户的 Activation 字段设置为给定 token（注册时调用）
func SetActivationToken(accountID uint64, token string) error {
	res := database.DataBase.Model(&models.AccountInfo{}).
		Where("id = ?", accountID).
		Update("activation", token)
	if res.Error != nil {
		return exception.SysCannotUpdate
	}
	return nil
}
