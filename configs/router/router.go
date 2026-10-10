package router

import (
	"coblog-backend/common/permission"
	configreader "coblog-backend/configs/configReader"
	middleware "coblog-backend/middlewares"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"coblog-backend/controllers/accountControllers"
	"coblog-backend/controllers/articlesControllers"
	"coblog-backend/controllers/fileController"
	"coblog-backend/controllers/liteControllers"
	"coblog-backend/controllers/loginControllers"
	"coblog-backend/controllers/markdownController"
	"coblog-backend/controllers/registerControllers"
	"coblog-backend/controllers/rssController"
	"coblog-backend/controllers/siteInfoController"
	"coblog-backend/utils"

	"github.com/gin-contrib/cors"
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

	// 真实客户端 IP（登录限流用）：生产经 Cloudflare 隧道进来，取 CF-Connecting-IP。
	// 不信任任何代理头（X-Forwarded-For 等）：没有 CF 头时（本地开发）退回直连地址。
	// 注意：源站若能绕过 Cloudflare 直连，这个头可以伪造 —— 账号维度的限流不受影响。
	ginEngine.TrustedPlatform = gin.PlatformCloudflare
	if err := ginEngine.SetTrustedProxies(nil); err != nil {
		panic(err)
	}

	// CORS配置 - 必须在所有路由之前配置
	corsConfig := cors.Config{
		// 白名单按解析后的 hostname 精确比对，见 utils.IsAllowedOrigin
		AllowOriginFunc: utils.IsAllowedOrigin,
		AllowMethods: []string{
			"GET",
			"POST",
			"PUT",
			"DELETE",
			"OPTIONS",
			"PATCH",
		},
		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Authorization",
			"Accept",
			"X-Requested-With",
			"Content-Length",
		},
		ExposeHeaders: []string{
			"Content-Length",
			"Content-Type",
			"Authorization",
			"X-Image-Variant",
		},
		AllowCredentials: true,      // 允许携带cookie和Authorization
		MaxAge:           12 * 3600, // 预检请求缓存12小时
	}
	ginEngine.Use(cors.New(corsConfig))

	// 静态文件服务：把上传目录挂到 /static/uploads，供图片等资源直接访问
	// 与 fileController 返回的 URL 前缀保持一致；带 ?thumb=1 时取压缩图
	serveUpload := fileController.ServeUpload(filepath.Join(configreader.GetConfig().FileObject.Dir, "img"))
	ginEngine.GET("/static/uploads/*filepath", serveUpload)
	ginEngine.HEAD("/static/uploads/*filepath", serveUpload)

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

		auth.POST("/register", registerControllers.CreateNormalUser)

		// 发送邮箱验证码（登录 / 找回密码），purpose 区分用途
		auth.POST("/code/send", loginControllers.SendCode)
		// 通过邮箱验证码重置密码（找回密码，无需登录）
		auth.POST("/pwd/reset", loginControllers.ResetPwdByEmail)
		// 邮件激活账户
		auth.GET("/activate", loginControllers.ActivateAccount)
		// 重发激活邮件
		auth.POST("/activate/resend", loginControllers.ResendActivationEmail)

		// 登出：清掉 HttpOnly 的登录 cookie（那是前端删不掉的部分），
		// 否则登出后 /lite 仍然认为用户处于登录态。
		auth.POST("/logout", loginControllers.Logout)
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
			accountControllers.ChangePwd)

		//普通用户重置RSS Token
		user.PUT("/rst-rss/", middleware.NeedPerm(permission.Perm_UpdateProfile),
			accountControllers.RstRSSToken)
	}

	//文章这一块
	ginEngine.GET("/api/articles", middleware.UnifiedErrorHandler(), middleware.LooseAuth,
		articlesControllers.GetArticleList) //文章列表
	ginEngine.GET("/api/articles/:id", middleware.UnifiedErrorHandler(), middleware.LooseAuth,
		articlesControllers.GetArticleContent) //文章页面
	ginEngine.POST("/api/articles", middleware.UnifiedErrorHandler(), middleware.Auth,
		middleware.NeedPerm(permission.Perm_PostPost),
		articlesControllers.CreateArticle) //发布文章
	ginEngine.PUT("/api/articles/:id", middleware.UnifiedErrorHandler(), middleware.Auth,
		middleware.NeedPerm(permission.Perm_PostPost),
		articlesControllers.UpdateArticle) //修改文章
	ginEngine.DELETE("/api/articles/:id", middleware.UnifiedErrorHandler(), middleware.Auth,
		middleware.NeedPerm(permission.Perm_PostPost),
		articlesControllers.DeleteArticle) //删除文章
	ginEngine.GET("/api/articles/:id/edit", middleware.UnifiedErrorHandler(), middleware.Auth,
		middleware.NeedPerm(permission.Perm_PostPost),
		articlesControllers.GetArticleForEdit) //获取文章原文(含md)供编辑回填

	//Markdown 渲染预览（需登录且有发帖权限）
	ginEngine.POST("/api/markdown/render", middleware.UnifiedErrorHandler(), middleware.Auth,
		middleware.NeedPerm(permission.Perm_PostPost),
		markdownController.MarkdownToHTMLHandler) //Markdown转HTML预览

	//站点信息
	ginEngine.GET("/api/site/info", middleware.UnifiedErrorHandler(),
		middleware.LooseAuth,
		siteInfoController.GetSiteInfo) //底栏,

	ginEngine.GET("/api/rss", middleware.UnifiedErrorHandler(),
		middleware.LooseAuth,
		rssController.GenerateRSSHandler) //RSS订阅

	//管理员相关
	//获取用户列表
	ginEngine.GET("/api/admin/users/", middleware.UnifiedErrorHandler(),
		middleware.Auth,
		middleware.NeedPerm(permission.Perm_GetAnyProfile),
		accountControllers.GetAccountInfoAdmin)

	//老设备（Kindle 等）只读页面
	//
	//与主站 SPA 同构的路径层级，只多一个 /lite 前缀。全部由后端直出 HTML：
	//老 Kindle 的浏览器跑不动 Vue 运行时（缺 Proxy / Promise / Map / Set），
	//客户端渲染的页面在它上面只会白屏，而且它没有 devtools，白屏无从排查。
	//
	//样式表不进 LooseAuth 组：它是静态资源，没必要每次请求都做一次鉴权与日志。
	ginEngine.GET("/lite/lite.css", liteControllers.ServeLiteCSS)
	ginEngine.GET("/lite/write.js", liteControllers.ServeLiteWriteJS)
	ginEngine.GET("/lite/icon.ico", liteControllers.ServeLiteIcon)

	lite := ginEngine.Group("/lite", middleware.LooseAuth, liteControllers.Guard)
	{
		lite.GET("/", liteControllers.HomePage)
		lite.GET("/articles", liteControllers.ArticleListPage)
		lite.GET("/articles/:id", liteControllers.ArticlePage)
		lite.GET("/about", liteControllers.AboutPage)
		lite.GET("/about/us", liteControllers.AboutPage)
		lite.GET("/about/friends", liteControllers.AboutPage)
		lite.GET("/rss", liteControllers.RSSPage)

		// 账户：表单页走 POST + 302，不经过 JSON 接口
		lite.GET("/login", liteControllers.LoginPage)
		lite.POST("/login", liteControllers.LoginSubmit)
		lite.POST("/logout", liteControllers.Logout)

		lite.GET("/register", liteControllers.RegisterPage)
		lite.POST("/register", liteControllers.RegisterSubmit)
		lite.GET("/activate", liteControllers.ActivatePage)

		lite.GET("/forgot-password", liteControllers.ForgotPasswordPage)
		lite.POST("/forgot-password", liteControllers.ForgotPasswordSubmit)

		lite.GET("/me", liteControllers.MePage)
		lite.POST("/me/password", liteControllers.MeChangePassword)
		lite.POST("/me/rss", liteControllers.MeResetRSSToken)
		lite.POST("/me/resend-activation", liteControllers.MeResendActivation)

		// 写作：新增 / 编辑 / 删除
		lite.GET("/write", liteControllers.WritePage)
		lite.POST("/write", liteControllers.WriteSubmit)
		lite.GET("/write/:id", liteControllers.WriteEditPage)
		lite.POST("/write/:id", liteControllers.WriteSubmit)
		lite.GET("/write/:id/delete", liteControllers.WriteDeleteConfirm)
		lite.POST("/write/:id/delete", liteControllers.WriteDelete)
		lite.POST("/upload", liteControllers.UploadImage)
	}

	// /lite 下的未匹配路径给出同风格的 404 页面；其余依旧交给前端处理。
	ginEngine.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/lite") {
			liteControllers.NotFoundPage(c)
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "接口不存在"})
	})

	return ginEngine

}
