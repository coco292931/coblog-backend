// Package liteview 是 /lite 的视图层：模板、样式、视图模型与几个纯函数。
// 不依赖 configReader / database，单测不需要 MySQL。
package liteview

import (
	"bytes"
	"embed"
	"html/template"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// 模板与样式与 Go 源码同包，用 embed 打进二进制。
//
// 为什么必须内嵌：生产镜像是 FROM scratch，运行时既没有源码目录也没有
// templates/ 目录，LoadHTMLGlob 这类按文件系统路径加载的方式在容器里会直接失败。
// 内嵌之后部署仍然只有一个二进制文件，Dockerfile 不需要改动。
//
//go:embed assets/templates/*.html
var templatesFS embed.FS

//go:embed assets/lite.css
var liteCSS []byte

// 写作页的增强脚本。必须是 ES5（老 Kindle 遇到 ES6 语法整段不执行），见 write_test.go。
//
//go:embed assets/write.js
var writeJS []byte

// 站点图标。复制自前端 src/assets/icon.ico。
//
//go:embed assets/icon.ico
var liteIcon []byte

// liteTemplates 在进程启动时解析一次。模板有语法错误会在这里直接 panic，
// 属于「启动即失败」，比运行到某个页面才报错更容易发现。
var liteTemplates = template.Must(
	template.New("lite").ParseFS(templatesFS, "assets/templates/*.html"),
)

// Render 渲染页面。
//
// 先渲染到内存缓冲再写出：这样模板执行中途出错时还能改发错误页，
// 不会出现「响应头已发出、页面缺一半」的情况（老 Kindle 上没有 devtools，
// 半截页面极难排查）。
func Render(c *gin.Context, status int, name string, data any) {
	var buf bytes.Buffer
	if err := liteTemplates.ExecuteTemplate(&buf, name, data); err != nil {
		log.Printf("[LITE] 模板 %s 渲染失败: %v", name, err)
		c.String(http.StatusInternalServerError, "页面渲染出错，请稍后再试")
		return
	}
	c.Data(status, "text/html; charset=utf-8", buf.Bytes())
}

// ServeCSS 提供 /lite/lite.css。
//
// 样式表同样来自内嵌资源。走 CDN / 反向代理时建议给这个地址加长缓存，
// 但不要依赖文件系统上的 static 目录。
func ServeCSS(c *gin.Context) {
	c.Data(http.StatusOK, "text/css; charset=utf-8", liteCSS)
}

// ServeWriteJS 提供 /lite/write.js。
func ServeWriteJS(c *gin.Context) {
	c.Data(http.StatusOK, "application/javascript; charset=utf-8", writeJS)
}

// ServeIcon 提供 /lite/icon.ico。
func ServeIcon(c *gin.Context) {
	c.Data(http.StatusOK, "image/x-icon", liteIcon)
}
