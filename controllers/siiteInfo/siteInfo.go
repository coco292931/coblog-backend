package siteInfoControllers

import (
	"coblog-backend/models"
	"coblog-backend/services/siteInfoService"
)

func GetSiteInfo() models.SiteInfo {
	siteInfo ,err := siteInfoService.GetSiteInfo()
	if err != nil {
		
	}
	return siteInfo
}
