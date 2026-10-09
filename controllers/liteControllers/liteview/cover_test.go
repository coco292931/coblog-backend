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

// 封面必须「等比铺满、超出裁掉」，不能拉伸。
//
// 老引擎不认 object-fit，<img> 塞进固定比例框只会被拉伸 / 压扁。
// 所以封面走背景图：background-size: cover（带 -webkit- 前缀）。
func TestCoverCropsInsteadOfStretching(t *testing.T) {
	css := string(liteCSS)
	for _, sel := range []string{".lite-card-cover", ".lite-cover"} {
		block := cssBlock(t, css, sel)
		for _, want := range []string{"-webkit-background-size: cover", "background-size: cover", "background-position: center"} {
			if !strings.Contains(block, want) {
				t.Errorf("%s 缺少 %q，实际:\n%s", sel, want, block)
			}
		}
	}

	card := renderToString(t, "home", HomeView{BaseView: testBase(), Articles: []ListItemView{sampleItem()}})
	mustContain(t, card, `class="lite-card-cover"`, "列表应有封面")
	mustContain(t, card, "background-image: url(", "列表封面应走背景图")
	mustContain(t, card, "x.jpg?thumb=1", "封面地址应原样带上")
	mustNotContain(t, card, "<img", "列表封面不应再用 <img>（老引擎会拉伸）")

	art := renderToString(t, "article", ArticleView{BaseView: testBase(), Title: "t", Cover: "https://api.example.com/static/uploads/y.jpg"})
	mustContain(t, art, "background-image: url(", "详情封面应走背景图")
	mustContain(t, art, "y.jpg", "封面地址应原样带上")
}

// 560px 断点必须写在 700px 之后。
//
// 窄屏两个断点同时命中，后写的同名规则胜出。之前 560 写在前面，
// 里面对页头的覆盖全被 700 盖掉了。
func TestNarrowBreakpointComesLast(t *testing.T) {
	css := string(liteCSS)
	i560 := strings.Index(css, "@media (max-width: 560px)")
	i700 := strings.Index(css, "@media (max-width: 700px)")
	if i560 < 0 || i700 < 0 {
		t.Fatal("找不到 560px / 700px 断点")
	}
	if i560 < i700 {
		t.Error("560px 断点必须写在 700px 断点之后，否则会被覆盖")
	}
}

// 页头必须「天生不溢出」：品牌左、导航右都用 float，放不下导航自己掉行，
// 不依赖任何断点。
//
// 之前用 -webkit-box 两端对齐，它不折行，375px 下「品牌 + 导航」要 395px，
// 直接撑出屏幕。
func TestHeaderIsFluidByDefault(t *testing.T) {
	css := string(liteCSS)

	inner := cssBlock(t, css, ".lite-header-inner")
	if inner == "" {
		t.Fatal("找不到 .lite-header-inner")
	}
	if strings.Contains(inner, "-webkit-box") {
		t.Error(".lite-header-inner 不能用 -webkit-box 横排（不折行，窄屏会溢出）")
	}

	if !strings.Contains(inner, "overflow: hidden") {
		t.Error(".lite-header-inner 要 overflow: hidden 清浮动，否则页头高度塌陷")
	}

	brand := cssBlock(t, css, ".lite-brand")
	if !strings.Contains(brand, "float: left") {
		t.Errorf(".lite-brand 应 float: left，实际:\n%s", brand)
	}

	nav := cssBlock(t, css, ".lite-nav")
	if !strings.Contains(nav, "float: right") {
		t.Errorf(".lite-nav 应 float: right，实际:\n%s", nav)
	}

	// 导航项 inline-block + margin-right：
	// 太窄时自己折行，且折行后首项不贴边，末尾不会多出空白
	link := cssBlock(t, css, ".lite-nav a")
	if !strings.Contains(link, "inline-block") {
		t.Error(".lite-nav a 应是 inline-block（可自然折行）")
	}
	if !strings.Contains(link, "margin-right") {
		t.Error(".lite-nav a 应该用 margin-right 拉开间距（margin-left 会让折行后的首项贴边）")
	}
}

// 另外四处曾经用 -webkit-box 硬横排的地方，全部改成 float / 块级流。
// 它们同样不能依赖断点才不溢出。
func TestNoFlexHacksForLayout(t *testing.T) {
	css := string(liteCSS)
	for _, sel := range []string{
		".lite-search-row",
		".lite-stats-bar",
		".lite-card-info",
		".lite-article-info",
	} {
		block := cssBlock(t, css, sel)
		if block == "" {
			t.Errorf("找不到 %s", sel)
			continue
		}
		if strings.Contains(block, "-webkit-box") {
			t.Errorf("%s 不能用 -webkit-box 硬横排（窄屏会溢出）", sel)
		}
	}
}
