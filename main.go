package main

import (
	"coblog-backend/configs/router"
)

//import "coblog-backend/configs/database"

func main() {
	ginEng := router.InitEngine()
	// CORS配置已经在 router.InitEngine() 中完成
	ginEng.Run(":8080")
}
