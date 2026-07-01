package permission_test

import (
	"coblog-backend/common/permission"
	"log"
	"testing"
)

func Test_TryGetPermission(t *testing.T) {
	log.Print(permission.GetPermissionByGroupID(9999))
	log.Print(permission.GetPermissionByGroupID(1))
	log.Printf("%v", permission.GetAllPermissionGroups())
}

func Test_AddPermission(t *testing.T) {
	log.Printf("%v", permission.AddPermissionGroup("TEST", permission.Perm_ForTestOnly1))
	log.Printf("%v", permission.GetAllPermissionGroups())
	log.Printf("%v", permission.IsPermSatisfied(28, permission.Perm_ForTestOnly1))
	log.Printf("%v", permission.IsPermSatisfied(28, permission.Perm_ForTestOnly2))
}

func Test_AddUserPG(t *testing.T) {
	permission.AddPermissionGroup("USER", permission.Perm_GetProfile,
		permission.Perm_Login,
		permission.Perm_UpdateProfile,
		permission.Perm_UpdateAvatar,
		permission.Perm_ChangePassword,
		permission.Perm_DownloadFile,)
	log.Printf("%v", permission.GetAllPermissionGroups())
}

func Test_AddAdminPG(t *testing.T) {
	// 使用超级管理员权限组，自动拥有所有权限（0-255）
	permission.AddSuperAdminGroup("ADMIN")
	log.Printf("管理员权限组创建完成（拥有全部权限）: %v", permission.GetAllPermissionGroups())
}

func Test_AddGuestPG(t *testing.T) {
	permission.AddPermissionGroup("GUEST",permission.Perm_GetProfile,
		permission.Perm_Login,
		permission.Perm_ChangePassword,
		permission.Perm_UpdateProfile,)
	log.Printf("%v", permission.GetAllPermissionGroups())
}