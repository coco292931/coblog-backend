package siteInfoController

import (
	"coblog-backend/common/exception"
	"coblog-backend/services/siteInfoService"
	"coblog-backend/utils"
	"fmt"

	"github.com/gin-gonic/gin"
)

func GetSiteInfo(c *gin.Context) {
	fmt.Println("获取站点信息")
	siteInfo, err := siteInfoService.GetSiteInfo()
	if err != nil {
		fmt.Println("无法获取站点信息", err)
		c.Error(exception.SysCannotGetSiteInfo)
		return
	}

	utils.JsonSuccessResponse(c, "获取站点信息成功", siteInfo)
}
