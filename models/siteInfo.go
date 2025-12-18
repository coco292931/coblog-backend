package models

import (
	"time"
)

type SiteInfo struct {
	Id          uint64    `json:"-" ` //更新次数，没啥用
	Articles    string    `json:"total_articles"`
	Words       string    `json:"total_words"`
	Visits      string    `json:"total_visits"`   //总查看次数,保留
	Visitors    string    `json:"total_visitors"` //先按账户数算
	Uptime      time.Time `json:"uptime"`
	StartedTime time.Time `json:"started_time"`

	//不予理睬
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}
