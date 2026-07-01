package siteInfoService

import (
	"coblog-backend/configs/database"
	"coblog-backend/models"
	"strconv"
	"sync"
	"time"
)

// 开港时间（站点开放时间），恒定常量
var siteStartedTime = time.Date(2025, 12, 20, 0, 0, 0, 0, time.UTC)

// 内存缓存：读路径零计算，仅在文章增删改时重算刷新
var (
	cachedSiteInfo *models.SiteInfo
	cacheMutex     sync.RWMutex
)

// GetSiteInfo 获取站点信息
// 优先返回内存缓存；缓存为空（如冷启动）时从数据库拉取最新一条填充；
// 数据库也没有记录时，实时重算一次。
func GetSiteInfo() (models.SiteInfo, error) {
	// 1. 命中内存缓存
	cacheMutex.RLock()
	if cachedSiteInfo != nil {
		info := *cachedSiteInfo
		cacheMutex.RUnlock()
		return info, nil
	}
	cacheMutex.RUnlock()

	// 2. 冷启动：从数据库拉最新一条
	var siteInfo models.SiteInfo
	if err := database.DataBase.Last(&siteInfo).Error; err == nil {
		// 开港时间以常量为准，避免历史数据污染
		siteInfo.StartedTime = siteStartedTime
		cacheMutex.Lock()
		cachedSiteInfo = &siteInfo
		cacheMutex.Unlock()
		return siteInfo, nil
	}

	// 3. 数据库无记录：实时重算并写入
	return UpdateSiteInfo()
}

// UpdateSiteInfo 重新统计站点信息，刷新内存缓存并追加一条历史记录。
// 在文章增、删、改之后调用，读路径因此无需重算。
func UpdateSiteInfo() (models.SiteInfo, error) {
	var articleCount int64
	var totalWords int64
	var totalViews int64
	var visitorCount int64

	// 文章数（排除隐藏与显式不计入统计的文章）
	if err := database.DataBase.Model(&models.Post{}).
		Where("hidden = ? AND no_stats = ?", false, false).
		Count(&articleCount).Error; err != nil {
		return models.SiteInfo{}, err
	}
	// 总字数（SUM(words)，无记录时为 NULL，用 COALESCE 兜底 0）
	if err := database.DataBase.Model(&models.Post{}).
		Where("hidden = ? AND no_stats = ?", false, false).
		Select("COALESCE(SUM(words), 0)").Scan(&totalWords).Error; err != nil {
		return models.SiteInfo{}, err
	}
	// 总浏览量（SUM(views)）
	if err := database.DataBase.Model(&models.Post{}).
		Where("hidden = ? AND no_stats = ?", false, false).
		Select("COALESCE(SUM(views), 0)").Scan(&totalViews).Error; err != nil {
		return models.SiteInfo{}, err
	}
	// 访客数：暂按注册用户数计（README 已注明）
	if err := database.DataBase.Model(&models.AccountInfo{}).Count(&visitorCount).Error; err != nil {
		return models.SiteInfo{}, err
	}

	newInfo := models.SiteInfo{
		Articles:    strconv.FormatInt(articleCount, 10),
		Words:       strconv.FormatInt(totalWords, 10),
		Visits:      strconv.FormatInt(totalViews, 10),
		Visitors:    strconv.FormatInt(visitorCount, 10),
		Uptime:      time.Now().UTC(),
		StartedTime: siteStartedTime,
	}

	// 追加一条历史记录（SiteInfo 表作为历史流水）
	if err := database.DataBase.Create(&newInfo).Error; err != nil {
		return models.SiteInfo{}, err
	}

	// 刷新内存缓存
	cacheMutex.Lock()
	cachedSiteInfo = &newInfo
	cacheMutex.Unlock()

	return newInfo, nil
}
