package models

import (
	"database/sql"
	"time"
)

// 数据库中用户信息模型
type AccountInfo struct {

	// 用户账号信息

	ID           uint64 `json:"id" gorm:"column:id;primaryKey"`        // 默认为主键
	Email        string `json:"email" gorm:"column:email;index"`       // 邮箱
	PasswordHash string `json:"-" gorm:"column:password_hash"`         // 密码的哈希值
	UserName     string `json:"username" gorm:"column:username;index"` // 用户名
	PermGroupID  uint32 `json:"permGroupID" gorm:"index"`              // 用户所在权限组
	Activation   string `json:"-"`                                     // 账户激活状态(保留,用于验证邮箱是否存在)
	Deepable     bool   `json:"deepable"`                              // 是否允许启用深度
	IsDeep       bool   `json:"isDeep"`                                // 是否已经启用深度
	RSSToken     string `json:"rssToken"`                              // RSS特征秘钥

	AvatarFile string `json:"avatarFile"` // 头像文件名
	Sex        string `json:"sex"`        // 性别
	SexInfo    string `json:"sexInfo"`    // 自定义

	Behaviors []string `json:"behaviors"` // 喜欢的主页标签，保留
	RequestTime int64 `json:"requestTime"` // 请求文章次数 暂时不用
	likes []string `json:"likes"` // 喜欢的文章列表
	//stars []string `json:"stars"` // 收藏的文章列表

	// 用户关联信息
	CreatedAt time.Time    `json:"createdAt"`
	UpdatedAt time.Time    `json:"updatedAt"`
	DeletedAt sql.NullTime `json:"deletedAt,omitempty"`

	// 备用
	TwoFactorAuth string `json:"twoFactorAuth"` // 双因素认证密钥  F:7J64V3P3E77J3LKNUGSZ5QANTLRLTKVL
	GithubOpenID  string `json:"githubOpenID"`  // GitHubopenid，留给第三方做的
}