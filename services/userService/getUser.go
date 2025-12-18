package userService

import (
	"coblog-backend/configs/database"
	"coblog-backend/dao"
	"coblog-backend/models"
)

// GetAccountInfoByUID 根据AccountID 获取用户信息

func GetUserByID(uid uint64) (*models.AccountInfo, error) {
	accountInfo, err := dao.GetAccountInfoByID(uid)
	if err != nil {
		return nil, err
	}
	return accountInfo, nil
}

// GetUserByEmail 根据用户邮箱获取用户
func GetUserByEmail(email string) (*models.AccountInfo, error) {
	user := models.AccountInfo{}
	result := database.DataBase.Where(
		&models.AccountInfo{
			Email: email,
		},
	).First(&user)
	if result.Error != nil {
		return nil, result.Error
	}
	return &user, nil
}

// GetUserByEmail 根据用户邮箱获取用户
func GetUserByUserName(userName string) (*models.AccountInfo, error) {
	user := models.AccountInfo{}
	result := database.DataBase.Where(
		&models.AccountInfo{
			UserName: userName,
		},
	).First(&user)
	if result.Error != nil {
		return nil, result.Error
	}
	return &user, nil
}

// GetUserByToken 根据用户token获取用户
func GetUserByToken(token string) (*models.AccountInfo, error) {
	user := models.AccountInfo{}
	result := database.DataBase.Where(
		&models.AccountInfo{
			RSSToken: token,
		},
	).First(&user)
	if result.Error != nil {
		return nil, result.Error
	}
	return &user, nil
}