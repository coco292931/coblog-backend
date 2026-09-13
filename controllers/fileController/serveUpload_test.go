package fileController

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

const (
	originalBody  = "ORIGINAL-IMAGE-BYTES-0123456789"
	thumbJpgBody  = "THUMB-JPG-BYTES"
	thumbPngBody  = "THUMB-PNG-BYTES"
	thumbWebpBody = "THUMB-WEBP-BYTES"
	secretBody    = "SECRET-OUTSIDE"
)

// 建临时上传目录：bg.jpg 有压缩图、small.jpg 没有、avatar.png 同时存在两种压缩图
func newUploadFixture(t *testing.T) string {
	t.Helper()
	parent := t.TempDir()
	dir := filepath.Join(parent, "img")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	// 目录外的文件，用来验证路径穿越不会读到它
	if err := os.WriteFile(filepath.Join(parent, "secret.txt"), []byte(secretBody), 0o644); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"bg.jpg":        originalBody,
		"bg_c.jpg":      thumbJpgBody,
		"small.jpg":     originalBody,
		"plain.webp":    originalBody,
		"avatar.png":    originalBody,
		"avatar_c.png":  thumbPngBody,
		"avatar_c.jpg":  thumbJpgBody, // 早期残留，必须优先取 _c.png
		"noalpha.png":   originalBody, // png 原图但压缩图是 jpg（无透明通道）
		"noalpha_c.jpg": thumbJpgBody,
		"future.png":    originalBody, // 将来换 webp 之类的压缩后缀也要能认
		"future_c.webp": thumbWebpBody,
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func newUploadEngine(dir string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	handler := ServeUpload(dir)
	engine.GET("/static/uploads/*filepath", handler)
	engine.HEAD("/static/uploads/*filepath", handler)
	return engine
}

func request(t *testing.T, engine *gin.Engine, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(method, target, nil))
	return w
}

func TestServeUploadReturnsVariant(t *testing.T) {
	engine := newUploadEngine(newUploadFixture(t))

	cases := []struct {
		name        string
		target      string
		wantBody    string
		wantVariant string
	}{
		{"不带参数取原图", "/static/uploads/bg.jpg", originalBody, "original"},
		{"thumb=1 取压缩图", "/static/uploads/bg.jpg?thumb=1", thumbJpgBody, "thumb"},
		{"没有压缩图时回退原图", "/static/uploads/small.jpg?thumb=1", originalBody, "original"},
		{"webp 原图也能回退", "/static/uploads/plain.webp?thumb=1", originalBody, "original"},
		{"png 原图优先 _c.png", "/static/uploads/avatar.png?thumb=1", thumbPngBody, "thumb"},
		{"png 只压出 jpg 时也命中", "/static/uploads/noalpha.png?thumb=1", thumbJpgBody, "thumb"},
		{"未知压缩后缀也认", "/static/uploads/future.png?thumb=1", thumbWebpBody, "thumb"},
		{"其它参数不影响", "/static/uploads/bg.jpg?thumb=0", originalBody, "original"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := request(t, engine, http.MethodGet, c.target)
			if w.Code != http.StatusOK {
				t.Fatalf("状态码 = %d", w.Code)
			}
			if w.Body.String() != c.wantBody {
				t.Fatalf("响应体 = %q，期望 %q", w.Body.String(), c.wantBody)
			}
			if got := w.Header().Get("X-Image-Variant"); got != c.wantVariant {
				t.Fatalf("X-Image-Variant = %q，期望 %q", got, c.wantVariant)
			}
		})
	}
}

func TestServeUploadHead(t *testing.T) {
	engine := newUploadEngine(newUploadFixture(t))

	w := request(t, engine, http.MethodHead, "/static/uploads/bg.jpg?thumb=1")
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d", w.Code)
	}
	if w.Header().Get("X-Image-Variant") != "thumb" {
		t.Fatalf("HEAD 也要带 X-Image-Variant，实际 = %q", w.Header().Get("X-Image-Variant"))
	}
	if w.Body.Len() != 0 {
		t.Fatalf("HEAD 不应有响应体，实际 %d 字节", w.Body.Len())
	}
}

// 文件名随机且内容永不改变，可以长缓存；带 ETag 的重复请求要能走 304
func TestServeUploadCacheHeaders(t *testing.T) {
	engine := newUploadEngine(newUploadFixture(t))

	w := request(t, engine, http.MethodGet, "/static/uploads/bg.jpg?thumb=1")
	if got, want := w.Header().Get("Cache-Control"), "public, max-age=31536000, immutable"; got != want {
		t.Errorf("Cache-Control = %q，期望 %q", got, want)
	}
	etag := w.Header().Get("ETag")
	if etag != `"bg_c.jpg"` {
		t.Errorf("ETag = %q，期望指向实际返回的压缩图", etag)
	}

	req := httptest.NewRequest(http.MethodGet, "/static/uploads/bg.jpg?thumb=1", nil)
	req.Header.Set("If-None-Match", etag)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusNotModified {
		t.Errorf("状态码 = %d，期望 304", w.Code)
	}

	// 404 不能带长缓存，否则边缘会把「文件不存在」钉住
	if w := request(t, engine, http.MethodGet, "/static/uploads/nope.jpg"); w.Header().Get("Cache-Control") != "" {
		t.Errorf("404 不该带 Cache-Control，实际 = %q", w.Header().Get("Cache-Control"))
	}
}

func TestServeUploadMissingAndTraversal(t *testing.T) {
	engine := newUploadEngine(newUploadFixture(t))

	cases := []string{
		"/static/uploads/nope.jpg",
		"/static/uploads/nope.jpg?thumb=1",
		"/static/uploads/../secret.txt",
		"/static/uploads/../secret.txt?thumb=1",
		"/static/uploads/%2e%2e/secret.txt",
		"/static/uploads/",
		"/static/uploads/a/b.jpg?thumb=1",
		"/static/uploads/*?thumb=1", // 通配符不能当成目录枚举用
	}

	for _, target := range cases {
		t.Run(target, func(t *testing.T) {
			w := request(t, engine, http.MethodGet, target)
			if w.Code == http.StatusOK {
				t.Fatalf("不应返回 200，响应体 = %q", w.Body.String())
			}
			if w.Body.String() == secretBody {
				t.Fatal("越权读到了目录外文件")
			}
		})
	}
}

func TestServeUploadRange(t *testing.T) {
	engine := newUploadEngine(newUploadFixture(t))

	req := httptest.NewRequest(http.MethodGet, "/static/uploads/bg.jpg?thumb=1", nil)
	req.Header.Set("Range", "bytes=0-4")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusPartialContent {
		t.Fatalf("状态码 = %d，期望 206（带 ?thumb=1 时 Range 也要正常）", w.Code)
	}
	if w.Body.String() != thumbJpgBody[:5] {
		t.Fatalf("响应体 = %q", w.Body.String())
	}
}
