package userService

import ()
func ChangePwd(accountID uint64,oldPwd string, newPwd string) error {
	//先读库，取出后校验
	return nil
}

func RstRSSToken(accountID uint64) (string, error) {
	postForm,err := GetUserByID(accountID)
	if err != nil {
		return "", err
	}
	newToken := GenToken(postForm.Email)
	//写库
	return newToken, nil
}