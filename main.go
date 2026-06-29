package main

import (
	"coblog-backend/configs/router"
	"coblog-backend/services/mailService"
)

//import "coblog-backend/configs/database"

func main() {
	mailService.InitCodeStore() // 启动验证码过期清理
	ginEng := router.InitEngine()
	// CORS配置已经在 router.InitEngine() 中完成
	ginEng.Run(":8080")
}
