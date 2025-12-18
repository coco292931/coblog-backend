package models

import (
	"time"
)

type SiteInfo struct {
	Articles    string    `json:"total_articles"`
	Words       string    `json:"total_words"`
	Visits      string    `json:"total_visits"`   //总查看次数
	Visitors    string    `json:"total_visitors"` //先按账户数算
	Uptime      time.Time `json:"uptime"`
	StartedTime time.Time `json:"started_time"`
}
