package models

import (
	"time"
)

// SiteInfo 站点统计。
// 时间列在库中为 datetime（无小数秒），需显式声明 type，
// 否则 AutoMigrate 会按 Go time.Time 的默认精度改写为 datetime(3)。
type SiteInfo struct {
	Id          uint64    `json:"-"` //更新次数，没啥用
	Articles    string    `json:"total_articles"`
	Words       string    `json:"total_words"`
	Visits      string    `json:"total_visits"`   //总查看次数,保留
	Visitors    string    `json:"total_visitors"` //先按账户数算
	Uptime      time.Time `json:"uptime" gorm:"type:datetime"`
	StartedTime time.Time `json:"started_time" gorm:"type:datetime"`

	//不予理睬
	CreatedAt time.Time `json:"-" gorm:"type:datetime"`
	UpdatedAt time.Time `json:"-" gorm:"type:datetime"`
}
