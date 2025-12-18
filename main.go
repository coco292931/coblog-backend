package main

import (
	"coblog-backend/configs/router"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

//import "coblog-backend/configs/database"

func main() {
	ginEng := router.InitEngine()
	ginEng.Run(":8080")
	ginEng.GET("/user", userHandler)

	// CORS配置
    config := cors.DefaultConfig()
    config.AllowOrigins = []string{
        "http://localhost:5173",        // 本地开发
        "https://blog.coco-29.wang",    // 生产环境
    }
    config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
    config.AllowHeaders = []string{"Origin", "Content-Type", "Authorization"}
    
    ginEng.Use(cors.New(config))

}

func userHandler(c *gin.Context) {
}
