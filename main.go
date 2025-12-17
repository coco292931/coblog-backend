package main

import (
	"coblog-backend/configs/router"

	"github.com/gin-gonic/gin"
)

//import "coblog-backend/configs/database"

func main() {
	ginEng := router.InitEngine()
	ginEng.Run(":8080")
	ginEng.GET("/user", userHandler)

}

func userHandler(c *gin.Context) {
}
