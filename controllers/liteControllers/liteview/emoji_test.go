package liteview

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 老设备（老 Kindle、老安卓 WebView）普遍没有 emoji 字体，
// 这些字符会渲染成一个空方框 —— 比不显示还难看。
// 主站页脚的 🖊️🍵、文章统计的 📊👁️👍💬、提示条的 ⚠️✅ 都已去掉，
// 这条用例守住以后又手滑加回来的情况。
func TestTemplatesHaveNoEmoji(t *testing.T) {
	banned := []string{
		"⚠", "✅", "❌", "🖊", "🍵", "📊", "👁", "👍", "💬",
		"🔗", "📖", "✨", "🎉", "👉", "🌟", "📌", "🔍", "⌛",
	}

	files, err := filepath.Glob(filepath.Join("assets", "templates", "*.html"))
	if err != nil {
		t.Fatalf("枚举模板失败: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("没有找到任何模板文件，路径可能变了")
	}

	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("读 %s 失败: %v", f, err)
		}
		content := string(b)
		for _, bad := range banned {
			if strings.Contains(content, bad) {
				t.Errorf("%s 里含 emoji %q —— 老设备会把它渲染成方框", filepath.Base(f), bad)
			}
		}
	}
}
