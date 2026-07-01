// permission.go
package permission

import (
	//"coblog-backend/common/basics"
	"coblog-backend/common/exception"
	"coblog-backend/dao"
	"coblog-backend/models"
	"fmt"
	"log"
	"sync"

	"github.com/bits-and-blooms/bitset"
)

// 权限枚举，以 Perm_ 开头，与数据库列名一致
type PermissionID uint32

const (
	_                 PermissionID = 0
	Perm_ForTestOnly1 PermissionID = 255
	Perm_ForTestOnly2 PermissionID = 254

	// 登录
	Perm_Login PermissionID = 001 // 登录

	// 个人信息
	Perm_GetProfile     PermissionID = 010 // 获取个人信息
	Perm_UpdateProfile  PermissionID = 011 // 修改个人信息
	Perm_UpdateAvatar   PermissionID = 012 // 更改头像
	Perm_ChangePassword PermissionID = 013 // 修改密码

	//文件相关
	Perm_UploadFile PermissionID = 021 // 上传图片
	//Perm_UploadFile PermissionID = 020 // 上传文件（含图片）
	//Perm_DownloadImage PermissionID = 022 // 下载图片  要考虑权限继承，暂时不做
	Perm_DownloadFile PermissionID = 027 // 下载文件（含图片）

	//帖子（文章）相关
	Perm_PostPost    PermissionID = 031 // 发帖
	Perm_ViewDeep    PermissionID = 032 // 查看深度。一般不使用该权限，而通过账户数据来判断。这个仅适用于需要深度才能访问的路由
	Perm_Like        PermissionID = 033 // 点赞
	Perm_CommentPost PermissionID = 034 // 评论

	Perm_DownloadArticle PermissionID = 037 // 下载md文章

	// 管理面板
	Perm_GetUserList        PermissionID = 100 // 获取用户列表
	Perm_AddUser            PermissionID = 101 // 新增用户
	Perm_EditUser           PermissionID = 102 // 编辑用户
	Perm_DeleteUser         PermissionID = 103 // 删除用户
	Perm_GetUserPermission  PermissionID = 104 // 获取用户权限
	Perm_EditUserPermission PermissionID = 105 // 编辑用户权限
	Perm_GetAnyProfile      PermissionID = 106 // 获取个人的所有信息

	// 往下继续加...
)

var permissionGroups = map[uint32]models.PermissionGroup{} // permGroupID -> 权限组 对应表

// 加载数据库中的权限组权限表
func loadFromDB() {
	var err error
	if permissionGroups, err = dao.GetAllPermissionGroup(); err != nil {
		panic(exception.SysCannotLoadPermGroup)
	}
}

var loadDBOnce sync.Once

func GetPermissionByGroupID(permGroupId uint32) (models.PermissionGroup, error) {
	loadDBOnce.Do(loadFromDB) // 懒加载：从数据库获取权限组权限表

	premGroupResult, ok := permissionGroups[permGroupId]
	if !ok {
		log.Print("[ERROR][PERM] 尝试获取一个不存在的权限组权限")
		return models.PermissionGroup{}, fmt.Errorf("permission group with ID %d not found", permGroupId)
	}
	return premGroupResult, nil
}

func GetAllPermissionGroups() *map[uint32]models.PermissionGroup {
	loadDBOnce.Do(loadFromDB) // 懒加载：从数据库获取权限组权限表
	return &permissionGroups
}

// IsPermSatisfied 判断指定权限组是否满足一组所需权限。
//
// 仅以数据库中的权限组与权限位图作为判定依据：
// 1. 先根据权限组 ID 读取数据库中的权限组定义；
// 2. 再逐项检查 needed 中的权限位是否被置位。
//
// 只要 needed 中任意一个权限不满足，立即返回 false；
// 只有全部权限都满足时，才返回 true。
// 若权限组在数据库中不存在，则直接视为无权。
func IsPermSatisfied(permGroupId uint32, needed ...PermissionID) bool {
	log.Printf("[INFO][PERM] 开始校验权限组 %d，需求权限=%v", permGroupId, needed)
	tocheck, err := GetPermissionByGroupID(permGroupId)
	if err != nil {
		log.Print("[ERROR][PERM] 该权限组不存在 视为无权")
		return false
	}
	for _, perm := range needed {
		if !tocheck.Permissions.Test(uint(perm)) {
			log.Printf("[WARN][PERM] 权限组 %d 缺少数据库权限 %d", permGroupId, perm)
			return false
		}
		log.Printf("[INFO][PERM] 权限组 %d 通过数据库权限 %d", permGroupId, perm)
	}
	log.Printf("[INFO][PERM] 权限组 %d 权限校验通过，需求权限=%v", permGroupId, needed)
	return true
}

func AddPermissionGroup(name string, permissions ...PermissionID) error {
	var newPG models.PermissionGroup
	newPG.Name = name
	newPG.Permissions = *bitset.New(255)
	for _, perm := range permissions {
		newPG.Permissions.Set(uint(perm))
	}
	dao.AddPermissionGroup(newPG)
	loadFromDB() // 重新从数据库载入权限表
	return nil
}

// AddSuperAdminGroup 创建超级管理员权限组，自动拥有所有权限（0-255位全部设置为1）
func AddSuperAdminGroup(name string) error {
	/*
		var newPG models.PermissionGroup
		newPG.Name = name
		newPG.Permissions = *bitset.New(256) // 创建0-255的位集
		// 设置所有位为1，表示拥有所有权限
		for i := uint(0); i <= 255; i++ {
			newPG.Permissions.Set(i)
		}
		dao.AddPermissionGroup(newPG)
		loadFromDB() // 重新从数据库载入权限表
	*/
	return nil
}
