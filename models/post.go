package models

import (
	"time"

	"gorm.io/datatypes"
)

type Post struct {
	ID          uint64         `json:"id" gorm:"column:uid"` //默认为主键
	Title       string         `json:"title" gorm:"column:title"`
	Subtitle   string         `json:"subtitle" gorm:"column:subtitle"` 
	summary     string         `json:"summary" gorm:"column:summary"` //简介，放在列表里面。提交时手动或自动生成
	coverImage string         `json:"cover_image" gorm:"column:coverImage"`//没有默认主图
	Content     string         `json:"content" gorm:"column:content"` //富文本内容
	MdContent  string         `json:"md_content" gorm:"column:md_content"` //Markdown内容，保留位
	Category    []string         `json:"category" gorm:"column:category;index"`
	tags     []string         `json:"tags" gorm:"index"`
	IsPublic bool           `json:"is_public" gorm:"index"`
	Words   uint64         `json:"words" gorm:"column:words"`

	Views     uint64         `json:"views" gorm:"column:views"`
	Likes      uint64         `json:"likes" gorm:"column:likes"`

	Comments     datatypes.JSON `json:"-"`//{"id":1,"content":"xxxx","creator":{"content":1,"images":{和上面的格式一样},"createdAt":"time.Time"}

	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}
