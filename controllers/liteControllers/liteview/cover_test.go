package liteview

import (
	"regexp"
	"strings"
	"testing"
)

// 封面必须用「比例容器」而不是像素限高。
//
// 像素限高（max-height: 560px）在不同宽度下会给出不同比例：同一张图在窄屏
// 被裁成接近方形、在宽屏被裁成长条。改成 padding-top 百分比撑出的固定比例后，
// 任何宽度下裁出来的形状都一致。
//
// 比例本身要对齐主站：
//   - 列表封面取 .article-photo 的 165 x 110 = 3:2
//   - 详情封面取 .main-photo-article 的 max-height: min(60vh, 100vw * 0.7) = 10:7
func TestCoverUsesAspectRatio(t *testing.T) {
	css := string(liteCSS)
	ratioRe := regexp.MustCompile(`padding(-top)?:\s*[0-9.]+%`)

	for _, sel := range []string{".lite-card-cover", ".lite-cover"} {
		block := cssBlock(t, css, sel)
		if block == "" {
			t.Errorf("找不到 %s", sel)
			continue
		}
		if !ratioRe.MatchString(block) {
			t.Errorf("%s 应该用 padding 百分比撑出固定比例，实际:\n%s", sel, block)
		}
	}

	if block := cssBlock(t, css, ".lite-card-cover"); !strings.Contains(block, "66.67%") {
		t.Errorf("列表封面应是主站 .article-photo 的 3:2 比例（padding-top: 66.67%%），实际:\n%s", block)
	}
	if block := cssBlock(t, css, ".lite-cover"); !strings.Contains(block, "70%") {
		t.Errorf("详情页封面应是主站 .main-photo-article 的 10:7 比例（padding-top: 70%%），实际:\n%s", block)
	}
}

// 页头在极窄屏必须折成两行。
//
// 实测 375px 视口下「品牌 + 四个导航项」横排需要 395px（品牌 170 + 导航 211），
// 直接撑出屏幕。断点 560px 里把 .lite-header-inner 改成 display: block，
// 品牌占一行、导航占一行，导航项自身可以继续折行。
func TestHeaderWrapsOnNarrowScreens(t *testing.T) {
	css := string(liteCSS)

	re := regexp.MustCompile(`@media \(max-width: 560px\) \{([\s\S]*?)\n\}`)
	m := re.FindStringSubmatch(css)
	if m == nil {
		t.Fatal("找不到 max-width: 560px 的断点")
	}
	block := m[1]

	if !strings.Contains(block, ".lite-header-inner") {
		t.Error("窄屏断点里应覆盖 .lite-header-inner")
	}
	if !strings.Contains(block, "display: block") {
		t.Error("窄屏断点里应把 .lite-header-inner 改成 display: block（品牌与导航折成两行）")
	}
	// 横排时靠 -webkit-box-pack 两端对齐；折行后必须靠 margin 拉开导航项间距，
	// 否则四项会挤成一坨。
	if !strings.Contains(block, "margin-right") {
		t.Error("窄屏折行后，导航项要靠 margin-right 拉开间距（原来的 margin-left 会让首项贴边）")
	}
}
