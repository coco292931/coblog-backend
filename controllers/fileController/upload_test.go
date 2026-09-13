package fileController

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configreader "coblog-backend/configs/configReader"

	"github.com/gin-gonic/gin"
)

// 生成一张足够大（超过压缩阈值）的 JPEG，用噪声保证压不小
func noiseJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	rnd := rand.New(rand.NewSource(1))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(rnd.Intn(256)), uint8(rnd.Intn(256)), uint8(rnd.Intn(256)), 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func multipartImage(t *testing.T, filename string, data []byte) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return &body, writer.FormDataContentType()
}

// 上传接口要返回原图与压缩图两个地址，且两者都能按约定取到
func TestUploadImageReturnsBothVariants(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := filepath.Join(configreader.GetConfig().FileObject.Dir, "img")
	source := noiseJPEG(t, 1600, 1200)

	body, contentType := multipartImage(t, "probe.jpg", source)
	req := httptest.NewRequest(http.MethodPost, "/api/upload/image", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()

	// 只测 handler 本身的响应，鉴权中间件不在本测试范围内
	engine := gin.New()
	engine.POST("/api/upload/image", UploadImage)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d，响应 = %s", w.Code, w.Body.String())
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			ID         string `json:"id"`
			URL        string `json:"url"`
			ThumbURL   string `json:"thumb_url"`
			Compressed bool   `json:"compressed"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v，原始响应 = %s", err, w.Body.String())
	}
	if resp.Code != 200 {
		t.Fatalf("业务码 = %d，响应 = %s", resp.Code, w.Body.String())
	}
	t.Cleanup(func() {
		os.Remove(filepath.Join(dir, resp.Data.ID))
		os.Remove(filepath.Join(dir, strings.TrimSuffix(resp.Data.ID, filepath.Ext(resp.Data.ID))+"_c.jpg"))
	})

	if resp.Data.ID == "" || resp.Data.URL == "" || resp.Data.ThumbURL == "" {
		t.Fatalf("响应字段不完整: %+v", resp.Data)
	}
	if strings.Contains(resp.Data.URL, "?") {
		t.Fatalf("url 应当是干净的原图地址，实际 = %s", resp.Data.URL)
	}
	if !strings.HasSuffix(resp.Data.ThumbURL, "?thumb=1") {
		t.Fatalf("thumb_url 应当是原图地址加 ?thumb=1，实际 = %s", resp.Data.ThumbURL)
	}
	if !resp.Data.Compressed {
		t.Fatalf("噪声大图应当生成压缩图，compressed = false")
	}

	// 通过同一个静态服务分别取两种变体，验证命名约定与回退都成立
	serve := newUploadEngine(dir)
	path := "/static/uploads/" + resp.Data.ID

	original := request(t, serve, http.MethodGet, path)
	if original.Body.Len() != len(source) {
		t.Fatalf("原图字节数 = %d，期望 %d", original.Body.Len(), len(source))
	}
	if got := original.Header().Get("X-Image-Variant"); got != "original" {
		t.Fatalf("原图 X-Image-Variant = %q", got)
	}

	thumb := request(t, serve, http.MethodGet, path+"?thumb=1")
	if thumb.Body.Len() == 0 || thumb.Body.Len() >= len(source) {
		t.Fatalf("压缩图字节数 = %d，应当小于原图 %d", thumb.Body.Len(), len(source))
	}
	if got := thumb.Header().Get("X-Image-Variant"); got != "thumb" {
		t.Fatalf("压缩图 X-Image-Variant = %q", got)
	}
}
