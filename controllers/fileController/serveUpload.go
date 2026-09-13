package fileController

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// 上传文件对外挂载的路径前缀，与 fileController 返回的 URL 前缀保持一致
const uploadURLPrefix = "/static/uploads"

// 压缩图命名：<原文件名去扩展名> + thumbMarker + 压缩后的扩展名。
// 后缀由压缩策略决定且会变（png 无透明通道时也会压成 jpg），
// 所以不写死后缀列表；同时存在多份时优先 png（无损、可带透明通道）。
const thumbMarker = "_c"
const preferredThumbExt = ".png"

// ServeUpload 提供 /static/uploads 下的静态文件。
// 带 ?thumb=1 时优先返回同名的压缩图，没有则回退原图，
// 这样前端只用一个地址加参数就能取到两种变体，不必知道文件命名规则。
// 实际返回的是哪一种由响应头 X-Image-Variant 告知（thumb / original）。
func ServeUpload(dir string) gin.HandlerFunc {
	// 下面会先把 Path 重写成带前缀的形式，再由 StripPrefix 摘掉前缀交给文件服务。
	// gin.Dir(dir, false) 关掉目录列举；http.Dir 自身还会做一次路径安全校验，
	// 所以 uploadFileName 是「第一道」而非唯一一道防线，别把 gin.Dir 换成裸 os.Open。
	fileServer := http.StripPrefix(uploadURLPrefix, http.FileServer(gin.Dir(dir, false)))

	return func(c *gin.Context) {
		name, ok := uploadFileName(c.Param("filepath"))
		if !ok {
			c.Status(http.StatusNotFound)
			return
		}

		variant := "original"
		if c.Query("thumb") == "1" {
			if real, ok := thumbName(dir, name); ok {
				name = real
				variant = "thumb"
			}
		}
		c.Header("X-Image-Variant", variant)
		// 文件名随机且内容永不改变，可以长缓存；ETag 用文件名，省掉到期后的回源协商
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
		c.Header("ETag", `"`+name+`"`)

		// 路径换成实际要返回的那个文件，再交给文件服务
		c.Request.URL.Path = uploadURLPrefix + "/" + name
		c.Request.URL.RawPath = ""
		fileServer.ServeHTTP(c.Writer, c.Request)
	}
}

// uploadFileName 把路由参数收敛成一个纯文件名，含路径分隔或上跳的一律拒绝
func uploadFileName(raw string) (string, bool) {
	// ".." 挡普通上跳；"%2e" 挡双重编码——gin 只解码一次，%252e 到这里才变成 %2e
	if strings.Contains(raw, "..") || strings.Contains(strings.ToLower(raw), "%2e") {
		return "", false
	}

	// 归一并去掉前导斜杠，把 "/a"、"//a"、"a/" 这类写法拉到同一形态，
	// 结果必须只剩一段，还带分隔符就说明没收敛干净
	name := strings.TrimPrefix(path.Clean("/"+raw), "/")
	if name == "" || strings.Contains(name, "/") {
		return "", false
	}
	return name, true
}

// thumbName 找出原图对应的压缩图文件名，没有则返回 false（调用方回退原图）
func thumbName(dir, name string) (string, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}

	// 压缩图后缀不定（png/jpg），所以按 "base_c." 前缀扫目录项匹配。
	// 用前缀而非 Contains，免得 bg_c_old.jpg 这类残留文件被误认。
	prefix := strings.TrimSuffix(name, filepath.Ext(name)) + thumbMarker + "."
	fallback := ""
	for _, entry := range entries {
		candidate := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(candidate, prefix) {
			continue
		}
		if strings.EqualFold(filepath.Ext(candidate), preferredThumbExt) {
			return candidate, true
		}
		// 不是 png 的先留着当备选，继续往后面找 png
		if fallback == "" {
			fallback = candidate
		}
	}
	// 有 png 就用 png，否则退到备选；一个都没有则交回调用方回退原图
	return fallback, fallback != ""
}
