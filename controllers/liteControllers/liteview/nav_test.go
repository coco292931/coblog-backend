package liteview

import (
	"strings"
	"testing"
)

// 导航高亮：路径 → 当前栏目。
// 语义与主站 router-link-active 一致 —— 详情页归它的上级栏目。
func TestNavKeyFor(t *testing.T) {
	cases := []struct{ path, want string }{
		{"/lite/", ""},
		{"/lite/articles", "articles"},
		{"/lite/articles/4", "articles"},
		{"/lite/write", "write"},
		{"/lite/write/7", "write"},
		{"/lite/write/7/delete", "write"},
		{"/lite/rss", "rss"},
		{"/lite/about", "about"},
		{"/lite/about/us", "about"},
		{"/lite/about/friends", "about"},
		{"/lite/me", "me"},
		// 这些不属于任何导航项
		{"/lite/login", ""},
		{"/lite/register", ""},
		{"/lite/forgot-password", ""},
		{"/lite/activate", ""},
		{"/lite/nope", ""},
		{"/", ""},
		// 前缀相同但不是同一栏目，不能被误吸进去
		{"/lite/articlesx", ""},
		{"/lite/medical", ""},
	}
	for _, c := range cases {
		if got := NavKeyFor(c.path); got != c.want {
			t.Errorf("NavKeyFor(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

// 模板确实把 is-active 加到了对应项上，而且只加一项
func TestNavHighlightsExactlyOneItem(t *testing.T) {
	// testBase() 的路径是 /lite/，不该高亮任何导航项
	html := renderToString(t, "home", HomeView{BaseView: testBase()})
	if n := strings.Count(html, "is-active"); n != 0 {
		t.Errorf("首页不应高亮任何导航项，实际 %d 处", n)
	}

	base := testBase()
	base.NavKey = "articles"
	html = renderToString(t, "home", HomeView{BaseView: base})
	if n := strings.Count(html, "is-active"); n != 1 {
		t.Fatalf("应恰好高亮一项，实际 %d 处", n)
	}
	if !strings.Contains(html, `/lite/articles" class="is-active"`) {
		t.Error("高亮应加在「足迹」上")
	}
}
