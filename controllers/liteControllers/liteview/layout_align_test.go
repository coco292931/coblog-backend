package liteview

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// cssBlock 取出 CSS 里某个选择器的规则体。
// 只匹配顶格的选择器（媒体查询里的版本是缩进的，不会命中）。
func cssBlock(t *testing.T, css, sel string) string {
	t.Helper()
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(sel) + `\s*\{([^}]*)\}`)
	m := re.FindStringSubmatch(css)
	if m == nil {
		return ""
	}
	return m[1]
}

// 页头、页脚的内容容器必须和 .lite-shell 用同一套「居中 + 左右留白」。
//
// 这条用例是真出事之后补的：`.lite-header-inner` 当时只写了 display:-webkit-box
// 那三行，漏了 max-width / margin / padding —— 页头本体是通栏的，内容容器
// 却没有留白，宽屏下品牌直接贴在 x=0，和下面居中的内容区左右错开。
// 漏改的原因是一次批量编辑里这条替换失败，而我漏看了失败项。
func TestLayoutContainersShareSameInsets(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("assets", "lite.css"))
	if err != nil {
		t.Fatalf("读 lite.css 失败: %v", err)
	}
	css := string(b)

	for _, sel := range []string{".lite-shell", ".lite-header-inner", ".lite-footer-inner"} {
		block := cssBlock(t, css, sel)
		if block == "" {
			t.Errorf("找不到规则 %s", sel)
			continue
		}
		for _, prop := range []string{"max-width", "margin", "padding"} {
			if !strings.Contains(block, prop) {
				t.Errorf("%s 缺少 %s —— 内容会贴边，与 .lite-shell 对不齐", sel, prop)
			}
		}
	}
}

// 通栏的那一层（.lite-header / .lite-footer）自己不能带左右内边距，
// 否则会叠在内容容器的留白之上，让页头比内容区更窄。
func TestFullBleedBarsHaveNoSidePadding(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("assets", "lite.css"))
	if err != nil {
		t.Fatalf("读 lite.css 失败: %v", err)
	}
	css := string(b)

	for _, sel := range []string{".lite-header", ".lite-footer"} {
		block := cssBlock(t, css, sel)
		if block == "" {
			t.Errorf("找不到规则 %s", sel)
			continue
		}
		padding := ""
		for _, line := range strings.Split(block, ";") {
			if strings.Contains(line, "padding") {
				padding = strings.TrimSpace(line)
			}
		}
		if padding == "" {
			continue
		}
		// padding: 26px 0 40px 这种左右为 0 的写法可以接受
		parts := strings.Fields(strings.TrimPrefix(padding, "padding:"))
		if len(parts) >= 2 && parts[1] != "0" {
			t.Errorf("%s 的 padding 带左右值（%q）—— 它是通栏层，留白应交给 -inner", sel, padding)
		}
	}
}
