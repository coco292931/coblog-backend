package siteInfoControllers

import (
	"coblog-backend/models"
	"coblog-backend/services/siteInfoService"
)

func GetSiteInfo() models.SiteInfo {
	// 这里是模拟数据，实际应用中应从数据库或其他数据源获取
	siteInfo := siteInfoService.GetSiteInfo()
	return siteInfo
}
