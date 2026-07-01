package articleService

import (
	"coblog-backend/services/siteInfoService"
	"log"
)

// refreshSiteInfo 在文章增删改后刷新站点统计。
// 统计失败仅记录日志，不影响文章操作主流程。
func refreshSiteInfo() {
	if _, err := siteInfoService.UpdateSiteInfo(); err != nil {
		log.Printf("[WARN][articleService] 刷新站点统计失败: %v", err)
	}
}
