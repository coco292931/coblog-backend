package rssService

import (
	"testing"
	"time"

	"coblog-backend/models"
)

func TestPostToItemMapsFields(t *testing.T) {
	post := &models.Post{
		ID:         42,
		Title:      "标题",
		Summary:    "摘要",
		Content:    "<p>正文</p>",
		Category:   `["tech"]`,
		Tags:       `["go"]`,
		CoverImage: "/images/cover.png",
	}

	item := PostToItem(post, "https://blog.coco-29.wang", "https://api.coco-29.wang")

	if item.Link != "https://blog.coco-29.wang/articles/42" {
		t.Errorf("链接错误: %s", item.Link)
	}
	if item.ID != item.Link {
		t.Errorf("guid 应与链接一致: %s / %s", item.ID, item.Link)
	}
	if item.Enclosure == nil {
		t.Fatal("有封面图时应输出 enclosure")
	}
	if item.Enclosure.URL != "https://api.coco-29.wang/images/cover.png" {
		t.Errorf("封面地址未补全: %s", item.Enclosure.URL)
	}
	if item.Enclosure.Type != "image/png" {
		t.Errorf("封面 MIME 错误: %s", item.Enclosure.Type)
	}
	if len(item.Categories) != 2 {
		t.Errorf("分类应为 2 个，实际 %v", item.Categories)
	}
}

func TestPostToItemWithoutCover(t *testing.T) {
	item := PostToItem(&models.Post{ID: 1}, "https://blog.coco-29.wang", "")
	if item.Enclosure != nil {
		t.Error("无封面图时不应输出 enclosure")
	}
	if item.Categories == nil {
		t.Error("分类应为空切片而非 nil")
	}
}

func TestThumbURL(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"本站上传图", "https://api.coco-29.wang/static/uploads/AB.png", "https://api.coco-29.wang/static/uploads/AB.png?thumb=1"},
		{"已带查询串", "https://api.coco-29.wang/static/uploads/AB.png?v=2", "https://api.coco-29.wang/static/uploads/AB.png?v=2&thumb=1"},
		{"外链原样返回", "https://cdn.example.com/a.png", "https://cdn.example.com/a.png"},
		{"空串", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ThumbURL(c.raw); got != c.want {
				t.Errorf("ThumbURL(%q) = %q，期望 %q", c.raw, got, c.want)
			}
		})
	}
}

func TestParseStringList(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want []string
	}{
		{name: "JSON 数组", raw: `["tech","music"]`, want: []string{"tech", "music"}},
		{name: "单元素数组", raw: `["tech"]`, want: []string{"tech"}},
		{name: "非 JSON 回退为单值", raw: "tech", want: []string{"tech"}},
		{name: "空字符串返回 nil", raw: "   ", want: nil},
		{name: "首尾空白被裁剪", raw: `  ["tech"]  `, want: []string{"tech"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseStringList(tc.raw)
			if len(got) != len(tc.want) {
				t.Fatalf("ParseStringList(%q) = %v，期望 %v", tc.raw, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("ParseStringList(%q) = %v，期望 %v", tc.raw, got, tc.want)
				}
			}
		})
	}
}

func TestCategoriesMergesCategoryAndTags(t *testing.T) {
	got := Categories(`["tech","go"]`, `["backend","go"]`)
	want := []string{"tech", "go", "backend", "go"} // 去重交由 uniqueNonEmpty 处理

	if len(got) != len(want) {
		t.Fatalf("Categories 结果 %v，期望 %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("Categories 结果 %v，期望 %v", got, want)
		}
	}
}

func TestImageMIME(t *testing.T) {
	cases := map[string]string{
		"https://x/a.jpg":  "image/jpeg",
		"https://x/a.JPEG": "image/jpeg",
		"https://x/a.png":  "image/png",
		"https://x/a.gif":  "image/gif",
		"https://x/a.webp": "image/webp",
		"https://x/a.bmp":  "application/octet-stream",
		"https://x/a":      "application/octet-stream",
	}
	for url, want := range cases {
		if got := ImageMIME(url); got != want {
			t.Errorf("ImageMIME(%q) = %q，期望 %q", url, got, want)
		}
	}
}

func TestAbsURL(t *testing.T) {
	cases := []struct{ raw, base, want string }{
		{"/images/a.png", "https://api.coco-29.wang", "https://api.coco-29.wang/images/a.png"},
		{"/images/a.png", "https://api.coco-29.wang/", "https://api.coco-29.wang/images/a.png"},
		{"https://other.com/a.png", "https://api.coco-29.wang", "https://other.com/a.png"},
		{"http://other.com/a.png", "https://api.coco-29.wang", "http://other.com/a.png"},
		{"/images/a.png", "", "/images/a.png"},
	}
	for _, tc := range cases {
		if got := AbsURL(tc.raw, tc.base); got != tc.want {
			t.Errorf("AbsURL(%q, %q) = %q，期望 %q", tc.raw, tc.base, got, tc.want)
		}
	}
}

func TestSelfURLKeepsQuery(t *testing.T) {
	base := "https://api.coco-29.wang"

	cases := []struct {
		name       string
		baseURL    string
		requestURI string
		want       string
	}{
		{
			name:       "保留 token 查询串",
			baseURL:    base,
			requestURI: "/api/rss?token=abc123",
			want:       "https://api.coco-29.wang/api/rss?token=abc123",
		},
		{
			name:       "保留多个筛选参数",
			baseURL:    base,
			requestURI: "/api/rss?token=abc123&category=tech&tag=go",
			want:       "https://api.coco-29.wang/api/rss?token=abc123&category=tech&tag=go",
		},
		{
			name:       "无查询串",
			baseURL:    base,
			requestURI: "/api/rss",
			want:       "https://api.coco-29.wang/api/rss",
		},
		{
			name:       "base 末尾多余斜杠被归一",
			baseURL:    base + "/",
			requestURI: "/api/rss?token=abc123",
			want:       "https://api.coco-29.wang/api/rss?token=abc123",
		},
		{
			name:       "base 未配置时留空，不输出 atom:link",
			baseURL:    "",
			requestURI: "/api/rss?token=abc123",
			want:       "",
		},
		{
			name:       "异常请求路径被忽略",
			baseURL:    base,
			requestURI: "api/rss",
			want:       "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SelfURL(tc.baseURL, tc.requestURI); got != tc.want {
				t.Errorf("SelfURL(%q, %q) = %q，期望 %q", tc.baseURL, tc.requestURI, got, tc.want)
			}
		})
	}
}

func TestLatestCreatedPrefersCreatedAt(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	// 第一篇创建较新；第二篇创建较早但刚被编辑过；第三篇两者都最新
	posts := []models.Post{
		{CreatedAt: base.Add(24 * time.Hour), UpdatedAt: base},
		{CreatedAt: base, UpdatedAt: base.Add(100 * time.Hour)},
		{CreatedAt: base.Add(48 * time.Hour), UpdatedAt: base.Add(200 * time.Hour)},
	}

	// 频道时间必须取最新创建时间，不能被 UpdatedAt 推后（否则编辑旧文章会导致频道重排）
	want := base.Add(48 * time.Hour)
	if got := LatestCreated(posts); !got.Equal(want) {
		t.Errorf("频道时间应为 %v，实际 %v", want, got)
	}

	if !LatestCreated(nil).IsZero() {
		t.Error("空列表应返回零值")
	}
}
