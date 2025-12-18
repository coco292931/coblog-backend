package siteInfoService

import (
	"coblog-backend/models"
	"time"
)

func GetSiteInfo() models.SiteInfo {
	// 这里是模拟数据，实际应用中应从数据库或其他数据源获取
	uptimeTime, _ := time.Parse(time.RFC3339, "2024-01-01T00:00:00Z")
	startedTime, _ := time.Parse(time.RFC3339, "2025-12-20T00:00:00Z")
	
	siteInfo := models.SiteInfo{
		Articles:    "1234",
		Words:       "567890",
		Visits:      "23456",
		Visitors:    "3456",
		Uptime:      uptimeTime,
		StartedTime: startedTime,
	}
	return siteInfo
}
