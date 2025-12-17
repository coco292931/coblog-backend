package router

import (
	"coblog-backend/common/permission"
	middleware "coblog-backend/middlewares"
	"fmt"

	"coblog-backend/controllers/accountControllers"
	"coblog-backend/controllers/fileController"
	"coblog-backend/controllers/loginControllers"
	"coblog-backend/controllers/registerControllers"

	"github.com/gin-gonic/gin"
	//"github.com/silenceper/wechat/v2/openplatform/account"
)

func SayHello(c *gin.Context) {
	// 200 表示 HTTP 响应状态码（<=> http.StatusOK）
	// 使用 Context 的 String 函数将 "Hello 精弘!" 这句话以纯文本（字符串）的形式返回给前端
	// 实际上是对返回响应的封装
	c.String(200, "Hello go!")
}

func InitEngine() *gin.Engine {
	ginEngine := gin.Default()

	fmt.Println(gin.Context{})
	// // 添加中间件处理字符编码
	// ginEngine.Use(func(c *gin.Context) {
	// 	c.Header("Content-Type", "application/json; charset=utf-8")
	// 	c.Next()
	// })

	ginEngine.GET("/test", middleware.UnifiedErrorHandler(),
		middleware.Auth,
		middleware.NeedPerm(
			permission.Perm_ForTestOnly1,
			permission.Perm_ForTestOnly2), SayHello)

	// API分组
	api := ginEngine.Group("/api", middleware.UnifiedErrorHandler())
	{
		// 认证相关分组
		auth := api.Group("/auth")
		{
			auth.POST("/login/combo", loginControllers.AuthByCombo)
			auth.GET("/login/combo", SayHello)
			auth.POST("/register", registerControllers.CreateNormalUser)
		}

		// 上传相关分组
		upload := api.Group("/upload", middleware.Auth, middleware.NeedPerm(permission.Perm_UploadFile))
		{
			upload.POST("/image", fileController.UploadImage)
		}

		// 普通用户相关分组
		user := api.Group("/user", middleware.Auth)
		{
			user.GET("/info/", middleware.NeedPerm(permission.Perm_GetProfile), accountControllers.GetAccountInfoUser)
			user.PUT("/info/", middleware.NeedPerm(permission.Perm_UpdateProfile), accountControllers.EditAccountInfoUser)
			user.PUT("/pwd/", middleware.NeedPerm(permission.Perm_ChangePassword), accountControllers.EditAccountInfoUser)
		}

		// 文章相关分组
		articles := api.Group("/articles", middleware.LooseAuth)
		{
			articles.GET("", SayHello)              // 文章列表
			articles.GET("/{article_id}", SayHello) // 文章页面
		}

		// 站点信息分组
		site := api.Group("/site", middleware.LooseAuth)
		{
			site.GET("/info", SayHello) // 底栏
		}

		// 管理员相关分组
		admin := api.Group("/admin", middleware.Auth)
		{
			admin.GET("/users/", middleware.NeedPerm(permission.Perm_GetAnyProfile), accountControllers.GetAccountInfoAdmin)
		}
	}

	return ginEngine

}
