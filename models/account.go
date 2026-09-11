package models

import (
	"database/sql"
	"time"
)

// 数据库中用户信息模型
type AccountInfo struct {

	// 用户账号信息

	ID uint64 `json:"id" gorm:"column:id;primaryKey"` // 默认为主键
	// email 在库中为 varchar(256) 且字符集为 utf8mb3，需显式声明以免 AutoMigrate
	// 将其改写为默认的 191（会缩减容量并使含 emoji 的邮箱写入失败）。
	Email        string `json:"email" gorm:"column:email;index;type:varchar(256);not null"`
	PasswordHash string `json:"-" gorm:"column:password_hash"`
	// username 在库中为非空，声明 not null 以匹配
	UserName    string `json:"username" gorm:"column:username;index;not null"`
	PermGroupID uint32 `json:"permGroupID" gorm:"index;not null"` // 用户所在权限组
	Activation  string `json:"-"`                                 // 账户激活状态(保留,用于验证邮箱是否存在)
	Activated   bool   `json:"activated" gorm:"-"`                // 对外暴露的激活状态
	Deepable    bool   `json:"deepable" gorm:"default:0"`         // 是否允许启用深度
	IsDeep      bool   `json:"isDeep" gorm:"default:0"`           // 是否已经启用深度
	RSSToken    string `json:"rssToken"`                          // RSS特征秘钥

	AvatarFile string `json:"avatarFile"` // 头像文件名
	Sex        string `json:"sex"`        // 性别
	SexInfo    string `json:"sexInfo"`    // 自定义

	// behaviors / likes 在库中为 longtext（非默认的 text），
	// 显式声明 type 以避免 AutoMigrate 将列缩减为 text（上限 64KB）。
	Behaviors   string `json:"behaviors" gorm:"type:longtext"` // 喜欢的主页标签，保留  `["tech","music","sports"]`
	RequestTime int64  `json:"requestTime"`                    // 请求文章次数 暂时不用
	Likes       string `json:"likes" gorm:"type:longtext"`     // 喜欢的文章id列表 `[1,2]`
	//stars string `json:"stars"` // 收藏的文章列表

	// 备用
	TwoFactorAuth string `json:"twoFactorAuth"` // 双因素认证密钥  F:7J64V3P3E77J3LKNUGSZ5QANTLRLTKVL
	GithubOpenID  string `json:"githubOpenID"`  // GitHubopenid，留给第三方做的

	// 用户关联信息
	CreatedAt time.Time    `json:"createdAt"`
	UpdatedAt time.Time    `json:"updatedAt"`
	DeletedAt sql.NullTime `json:"deletedAt,omitempty"`
}
