package liteview

import (
	"bytes"
	"strings"
	"testing"
)

// RSS 页的展示项与措辞照搬主站 rss/index.vue，只少了那个「复制」按钮。
func TestRSSTemplateRenders(t *testing.T) {
	const target = "https://api.coco-29.wang/api/rss?token=abc123"

	var buf bytes.Buffer
	if err := liteTemplates.ExecuteTemplate(&buf, "rss", RSSView{URL: target}); err != nil {
		t.Fatalf("rss 模板渲染失败: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		target,
		"RSS 订阅",
		"复制下面的订阅地址，或在新标签页打开。",
		"打开 RSS",
		`href="/lite/me"`,
		"查看 Token",
		"已登录用户会自动携带 RSS Token，你可以使用token访问深度模式等的文章。",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rss 页面缺少 %q", want)
		}
	}

	if strings.Contains(out, "<script") {
		t.Error("rss 页面出现了 script 标签")
	}
	// 剪贴板 API 在老设备上不存在，主站那个「复制」按钮就不做
	if strings.Contains(out, "复制</button>") {
		t.Error("rss 页面不应有复制按钮")
	}
}
