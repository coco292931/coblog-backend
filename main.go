package main

import (
	"log"

	"coblog-backend/configs/cache"
	"coblog-backend/configs/router"
	"coblog-backend/services/mailService"
	"coblog-backend/services/userService"
)

//import "coblog-backend/configs/database"

func main() {
	// 初始化 Redis（幂等）。连接失败只告警不退出：
	// 依赖它的功能（激活链接、邮件验证码）会明确报错，而非静默降级。
	cache.Init()
	if cache.Available() {
		store := cache.NewStore(cache.Client)
		mailService.SetStore(store)
		userService.SetActivationStore(store)
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
