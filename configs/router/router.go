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

	
	//登录注册这一块
	auth := ginEngine.Group("/api/auth", middleware.UnifiedErrorHandler())
	{
		auth.POST("/login/combo", loginControllers.AuthByCombo)
		auth.GET("/login/combo", SayHello)

		auth.POST("/login/email", loginControllers.AuthByEmail)
		auth.POST("/login/email/verify/", loginControllers.AuthByEmail) //发送验证码

		auth.POST("/register", registerControllers.CreateNormalUser)
	}

	//上传图片这一块,暂时和文件共用权限
	ginEngine.POST("/api/upload/image", middleware.UnifiedErrorHandler(),
		middleware.Auth,
		middleware.NeedPerm(permission.Perm_UploadFile), 
		fileController.UploadImage)

	//普通用户获取用户信息
	user := ginEngine.Group("/api/user", middleware.UnifiedErrorHandler(), middleware.Auth)
	{
		user.GET("/info/", middleware.NeedPerm(permission.Perm_GetProfile), 
		accountControllers.GetAccountInfoUser)

		//普通用户更新自己信息
		user.PUT("/info/", middleware.NeedPerm(permission.Perm_UpdateProfile), 
		accountControllers.EditAccountInfoUser)
		user.PUT("/pwd/", middleware.NeedPerm(permission.Perm_ChangePassword), 
		accountControllers.EditAccountInfoUser)
	}

	//文章这一块
	ginEngine.GET("/api/articles", middleware.UnifiedErrorHandler(), middleware.LooseAuth)              //文章列表
	ginEngine.GET("/api/articles/{article_id}", middleware.UnifiedErrorHandler(), middleware.LooseAuth) //文章页面

	//站点信息
	ginEngine.GET("/api/site/info", middleware.UnifiedErrorHandler(), middleware.LooseAuth) //底栏

	//管理员相关
	//获取用户信息
	ginEngine.GET("/api/admin/users/", middleware.UnifiedErrorHandler(),
		middleware.Auth,
		middleware.NeedPerm(permission.Perm_GetAnyProfile),
		accountControllers.GetAccountInfoAdmin)

	return ginEngine

}
