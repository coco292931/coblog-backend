package models

import (
	"time"

	"gorm.io/datatypes"
)

type Post struct {
	ID         uint64 `json:"id" gorm:"column:uid"` //默认为主键
	Title      string `json:"title" gorm:"column:title;not null"`
	Subtitle   string `json:"subtitle" gorm:"column:subtitle"`
	Summary    string `json:"summary" gorm:"column:summary"`          //简介，放在列表里面。提交时手动或自动生成
	CoverImage string `json:"cover_image" gorm:"column:coverImage"`   //没有默认主图
	Content    string `json:"content" gorm:"column:content;not null"` //富文本内容
	MdContent  string `json:"md_content" gorm:"column:md_content"`    //Markdown内容，保留位
	// 分类与标签存 JSON 数组字符串。类型需与库中一致（category varchar(255)、tags longtext），
	// 否则 AutoMigrate 会按模型声明改写列定义，可能缩减容量。
	Category string `json:"category" gorm:"column:category;index;type:varchar(255)"` //`["tech","music","sports"]`
	Tags     string `json:"tags"`                                                    //`["tech","music","sports"]`
	IsDeep   bool   `json:"is_deep" gorm:"index"`
	// hidden / no_stats 在库中为非空且默认 0，显式声明以避免 AutoMigrate 抹掉默认值
	Hidden  bool   `json:"hidden" gorm:"column:hidden;index;not null;default:0"`
	NoStats bool   `json:"no_stats" gorm:"column:no_stats;not null;default:0"`
	Words   uint64 `json:"words" gorm:"column:words"`

	Views uint64 `json:"views" gorm:"column:views"`
	Likes uint64 `json:"likes" gorm:"column:likes"`

	Comments datatypes.JSON `json:"-"` //保留，目前使用comment组件 //{"id":1,"content":"xxxx","creator":{"content":1,"images":{和上面的格式一样},"createdAt":"time.Time"}

	// created_at / updated_at 在库中为 datetime（无小数秒）并带
	// DEFAULT CURRENT_TIMESTAMP、updated_at 另有 ON UPDATE CURRENT_TIMESTAMP。
	// 显式声明 type 与 default 使其与库定义一致：AutoMigrate 不会改写该列，
	// 从而保留 ON UPDATE 语义（GORM 标签无法表达 ON UPDATE）。
	CreatedAt time.Time `json:"createdAt" gorm:"type:datetime;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"type:datetime;default:CURRENT_TIMESTAMP"`
}
