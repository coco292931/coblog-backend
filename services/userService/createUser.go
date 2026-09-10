package userService

import (
	"coblog-backend/common/exception"
	"coblog-backend/configs/database"
	"coblog-backend/models"
	"crypto/sha256"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// 向数据库保存用户信息
func CreateUser(
	password string,
	email string,
	userName string,
	permGroupID uint32,
) (*models.AccountInfo, error) {

	// var userID uint64
	var err error
	// userID, err = strconv.ParseUint(studentID, 10, 64)
	// if err != nil {
	// 	//返回的学生id有误
	// 	fmt.Println("参数错误2:", err)
	// 	return nil, exception.ApiParamError
	// }

	_, err = GetUserByUserName(userName) //判断昵称是否重复
	if err == nil {
		return nil, exception.UsrAlreadyExisted
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	_, err = GetUserByEmail(email) //判断邮箱是否重复
	if err == nil {
		return nil, exception.UsrAlreadyExisted
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	//hashedPassword, err := bcrypt.GenerateFromPassword(hashPassword, 12) //cost
	hash := sha256.Sum256([]byte(password))

	// 将哈希值转换为十六进制字符串
	hashedPassword := fmt.Sprintf("%x", hash)
	// if err != nil {
	// 	//fmt.Println(err)
	// 	return nil, exception.SysPwdHashFailed
	// }
	//fmt.Println(string(hashedPassword))

	user := &models.AccountInfo{
		PasswordHash: string(hashedPassword),
		Email:        email,
		UserName:     userName,
		PermGroupID:  permGroupID,
		RSSToken:     GenToken(email),
		// 未激活哨兵值（非空、不等于 activated）：
		// 新激活令牌存于 Redis，数据库不再保存任何可用凭证。
		// 这里必须非空，否则 database 启动时的 backfillActivation 会把
		// activation='' 的账户当成存量用户直接标记为已激活。
		Activation: pendingMark,
	}

	res := database.DataBase.Create(user)

	return user, res.Error
}
