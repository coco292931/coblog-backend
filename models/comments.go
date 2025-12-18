package models

import (
	"gorm.io/gorm"
	"time"
)

type Comments struct {
	ID        uint64         `json:"id" gorm:"column:uid"`                    //默认为主键
	Creator   uint64         `json:"creator" gorm:"column:creator"`           //作者ID
	ArticleID uint64         `json:"articleID" gorm:"column:articleID;index"` //评论的文章ID
	Content   string         `json:"content" gorm:"column:content"`           //内容，需要支持换行
	IsPublic  bool           `json:"-"`                                       //保留
	LikedBy   string         `json:"likedBy" gorm:"column:likedBy"`           //点赞的id列表 `[1,2,"sports"]`
	CreatedAt time.Time      `json:"createdAt"`
	DeletedAt gorm.DeletedAt `json:"-"`
}
