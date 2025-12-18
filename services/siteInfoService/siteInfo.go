package siteInfoService

import (
	"coblog-backend/configs/database"
	"coblog-backend/models"
	"time"
)

func GetSiteInfo() (models.SiteInfo, error) {
	//模拟数据
	var siteInfo models.SiteInfo
	uptimeTime, _ := time.Parse(time.RFC3339, "2024-01-01T00:00:00Z")
	startedTime, _ := time.Parse(time.RFC3339, "2025-12-20T00:00:00Z")

	defSiteInfo := models.SiteInfo{
		Articles:    "1234",
		Words:       "567890",
		Visits:      "23456",
		Visitors:    "3456",
		Uptime:      uptimeTime,
		StartedTime: startedTime,
	}

	result := database.DataBase.Last(&siteInfo)
	if result.Error != nil {
		return defSiteInfo, result.Error //目前返回def其实是无效的，因为直接返回err
	}
	return siteInfo, nil
}

// 更新站点信息，上传文章后自动调用
func UpdateSiteInfo() error {
	//先获取上次数据
	oldSiteInfo, err := GetSiteInfo()
	if err != nil {
		return err
	}

	startedTime := oldSiteInfo.StartedTime.UTC()
	uptimeTime := time.Now().UTC()

	newInfo := &models.SiteInfo{
		Articles:    "1234",
		Words:       "567890",
		Visits:      "23456",
		Visitors:    "3456",
		Uptime:      uptimeTime,
		StartedTime: startedTime,
	}

	res := database.DataBase.Create(newInfo)
	if res.Error != nil {
		return res.Error
	}
	return nil
}
