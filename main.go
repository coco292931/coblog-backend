package main

import (
	"log"

	"coblog-backend/configs/cache"
	"coblog-backend/configs/router"
	"coblog-backend/services/mailService"
)

//import "coblog-backend/configs/database"

func main() {
	// 初始化 Redis（幂等）。连接失败只告警不退出：
	// 依赖它的功能会明确报错，而非静默降级到不安全行为。
	cache.Init()
	if cache.Available() {
		mailService.SetStore(cache.NewStore(cache.Client))
	}
	defer func() {
		if err := cache.Close(); err != nil {
			log.Printf("[WARN][Redis] 关闭连接失败: %v", err)
		}
	}()

	ginEng := router.InitEngine()
	// CORS配置已经在 router.InitEngine() 中完成
	ginEng.Run(":8080")
}
