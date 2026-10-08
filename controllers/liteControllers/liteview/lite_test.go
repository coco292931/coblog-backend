package liteview

import (
	"bytes"
	"html/template"
	"regexp"
	"strings"
	"testing"
	"time"

	"coblog-backend/models"
)

// 这一组测试关注三件事：
//   1. 各页面模板能渲染出内容（语法正确、字段接得上）；
//   2. 展示项忠于主站 —— 只搬运、不发明，不少也不多；
//   3. 产出物符合老 Kindle 的约束（内容直出、不依赖脚本、样式不含引擎不认的写法）。
//
// 第 2 点是重点：这里刻意留了一个「护栏用例」（TestFooterItemsMatchMainSite），
// 把用户明确否掉的几项写成断言，防止以后又被加回去。
//
// 本包不依赖 configReader / database，因此这些用例不需要 MySQL 就能跑。

// ────────────────────────────── 辅助 ──────────────────────────────

func testBase() BaseView {
	return BaseView{
		CurrentPath: "/lite/",
		Stats: &StatsView{
			TotalWords:  "34.6k",
			ReadingTime: "1:56",
			Uptime:      "已避风 292天5时30分12秒",
		},
	}
}

func renderToString(t *testing.T, name string, data any) string {
	t.Helper()
	var buf bytes.Buffer
	if err := liteTemplates.ExecuteTemplate(&buf, name, data); err != nil {
		t.Fatalf("渲染模板 %s 失败: %v", name, err)
	}
	return buf.String()
}

func mustContain(t *testing.T, html, want, why string) {
	t.Helper()
	if !strings.Contains(html, want) {
		t.Fatalf("%s\n期望包含: %q\n实际输出:\n%s", why, want, html)
	}
}

func mustNotContain(t *testing.T, html, bad, why string) {
	t.Helper()
	if strings.Contains(html, bad) {
		t.Fatalf("%s\n不应出现: %q", why, bad)
	}
}

func sampleItem() ListItemView {
	return ListItemView{
		ID:         7,
		Title:      "第一篇",
		Summary:    "这是摘要",
		Cover:      "https://api.example.com/static/uploads/x.jpg?thumb=1",
		Date:       "2026-01-02",
		Categories: []string{"tech", "music"},
	}
}

// ────────────────────────────── 模板渲染 ──────────────────────────────

func TestHomeTemplateRendersContent(t *testing.T) {
	html := renderToString(t, "home", HomeView{
		BaseView: testBase(),
		Articles: []ListItemView{sampleItem()},
	})

	mustContain(t, html, "第一篇", "首页应直出文章标题")
	mustContain(t, html, "/lite/articles/7", "首页卡片应链接到文章详情")
	mustContain(t, html, "2026-01-02", "应显示日期")
	mustContain(t, html, "海内存知己，天涯若比邻_", "首屏是主站的那句欢迎语")
}

func TestCardShowsOnlyMainSiteItems(t *testing.T) {
	html := renderToString(t, "home", HomeView{
		BaseView: testBase(),
		Articles: []ListItemView{sampleItem()},
	})

	// 主站 ArticleTimeline 的 .article 只有：封面 / 标题 / 描述 / 日期 / 分类
	mustContain(t, html, "这是摘要", "应显示描述（主站的 article.description）")
	mustContain(t, html, "tech", "应显示分类")

	// 这些是之前自作主张加的，主站列表里都没有
	mustNotContain(t, html, " 字", "列表卡片不应显示字数（主站没有）")
	mustNotContain(t, html, "深度", "列表卡片不应有深度标记（主站没有）")
}

func TestListTemplateMatchesMainSiteItems(t *testing.T) {
	html := renderToString(t, "list", ListView{
		BaseView:   testBase(),
		Articles:   []ListItemView{sampleItem()},
		Total:      25,
		HasFilter:  true,
		FilterText: "搜索“koko” · 分类“tech”",
		Page:       2,
		TotalPages: 3,
		HasPrev:    true,
		HasNext:    true,
		PrevURL:    "/lite/articles?page=1",
		NextURL:    "/lite/articles?page=3",
		Keyword:    "koko",

		SortPublishedURL: "/lite/articles?q=koko&page=1",
		SortUpdatedURL:   "/lite/articles?q=koko&page=1&sort=updated",
	})

	// 主站 search 页的展示项与措辞
	mustContain(t, html, "符合条件的文章：", "有筛选时用主站的措辞")
	mustContain(t, html, "<strong>25</strong> 篇", "结果数用主站的写法")
	mustContain(t, html, "搜索“koko”", "应显示生效的筛选条件描述")
	mustContain(t, html, `placeholder="搜索文章标题、内容或标签..."`, "搜索框占位符照搬主站")
	mustContain(t, html, `placeholder="按分类筛选，如：随笔"`, "分类占位符照搬主站")
	mustContain(t, html, `placeholder="按标签筛选，如：vue"`, "标签占位符照搬主站")
	mustContain(t, html, ">应用<", "按钮文案照搬主站")
	mustContain(t, html, "重置", "按钮文案照搬主站")
	mustContain(t, html, "最新发布", "排序选项照搬主站")
	mustContain(t, html, "最近修改", "排序选项照搬主站")
	mustContain(t, html, `method="get"`, "筛选必须走 GET 表单（不依赖脚本）")

	// 分页是 MPA 必需的（主站是无限滚动，无对应物）
	mustContain(t, html, "/lite/articles?page=1", "上一页应是链接")
	mustContain(t, html, "/lite/articles?page=3", "下一页应是链接")
}

func TestListTemplateAllArticlesWording(t *testing.T) {
	html := renderToString(t, "list", ListView{
		BaseView:         testBase(),
		Articles:         []ListItemView{sampleItem()},
		Total:            3,
		TotalPages:       1,
		Page:             1,
		SortPublishedURL: "/lite/articles?page=1",
		SortUpdatedURL:   "/lite/articles?page=1&sort=updated",
	})
	mustContain(t, html, "全部文章：", "无筛选时用主站的措辞")
	mustContain(t, html, "lite-btn-disabled", "首页的「上一页」应是禁用态")
	mustNotContain(t, html, `href=""`, "不应产生空链接")
}

func TestListTemplateEmptyState(t *testing.T) {
	html := renderToString(t, "list", ListView{
		BaseView:   testBase(),
		TotalPages: 1,
		Page:       1,
	})
	mustContain(t, html, "没有找到匹配的文章", "空状态文案照搬主站")
}

func TestArticleTemplateMatchesMainSiteItems(t *testing.T) {
	html := renderToString(t, "article", ArticleView{
		BaseView:   testBase(),
		Title:      "第一篇",
		Subtitle:   "副标题",
		Content:    template.HTML(`<h2>小节</h2><p>正文内容</p>`),
		CreateTime: "2026/01/02 15:04",
		UpdateTime: "2026/01/05 09:30",
		Categories: []string{"tech"},
		ReadingMin: 3,
		Author:     "coco_29",
		ArticleURL: "https://blog.coco-29.wang/articles/7",
		Views:      12,
		Likes:      3,
	})

	mustContain(t, html, "<h2>小节</h2>", "正文 HTML 必须原样输出、不能被转义")
	mustContain(t, html, "副标题", "主站详情页有副标题")
	mustContain(t, html, "创建：2026/01/02 15:04", "创建时间用主站的措辞与格式")
	mustContain(t, html, "修改：2026/01/05 09:30", "修改时间用主站的措辞与格式")
	mustContain(t, html, "3 min", "阅读时长用主站的写法")
	mustContain(t, html, "版权声明", "主站有版权声明块")
	mustContain(t, html, "本文作者：coco_29", "版权作者与主站一致")
	mustContain(t, html, "https://blog.coco-29.wang/articles/7", "本文链接指向主站地址")
	mustContain(t, html, "CC BY-NC-SA 4.0", "许可协议与主站一致")
	mustContain(t, html, "📊 文章统计", "主站有文章统计块")
	mustContain(t, html, "👁️ 浏览量：", "统计项与主站一致")
	mustContain(t, html, "👍 点赞量：", "统计项与主站一致")
	mustContain(t, html, "💬 评论数：", "统计项与主站一致")

	// 之前自作主张加的
	mustNotContain(t, html, " 字", "详情页不应显示字数（主站显示的是阅读时长）")
	mustNotContain(t, html, "深度", "详情页不应有深度标记（主站没有）")
	mustNotContain(t, html, "改于", "主站用的是「修改：」")
}

func TestArticleTemplateUncategorizedFallback(t *testing.T) {
	html := renderToString(t, "article", ArticleView{
		BaseView: testBase(),
		Title:    "第一篇",
		Content:  template.HTML("<p>x</p>"),
	})
	mustContain(t, html, "未分类", "无分类时与主站一样显示「未分类」")
}

func TestLoginTemplate(t *testing.T) {
	html := renderToString(t, "login", LoginView{BaseView: testBase()})

	// 字段与文案照搬主站 regAlogin 页
	mustContain(t, html, "登录", "标题照搬主站")
	mustContain(t, html, "邮箱", "字段照搬主站")
	mustContain(t, html, "密码", "字段照搬主站")
	mustContain(t, html, "记住我", "选项照搬主站")
	mustContain(t, html, "忘记密码？", "入口照搬主站")
	mustContain(t, html, "立即注册", "切换入口照搬主站")
	mustContain(t, html, `method="post"`, "表单必须是 POST（不依赖脚本）")
	mustContain(t, html, `action="/lite/login"`, "提交到表单自己的地址")
	mustNotContain(t, html, "lite-alert", "没有错误时不应渲染提示框")
}

func TestLoginTemplateShowsErrorAndKeepsInput(t *testing.T) {
	html := renderToString(t, "login", LoginView{
		BaseView: testBase(),
		Account:  "someone@example.com",
		Error:    "用户密码错误",
		Redirect: "/lite/me",
	})

	mustContain(t, html, "用户密码错误", "错误文案应原样显示后端的 msg")
	mustContain(t, html, "lite-alert", "有错误时应渲染提示框")
	mustContain(t, html, `value="someone@example.com"`, "邮箱应回填")
	mustContain(t, html, `name="redirect" value="/lite/me"`, "应保留回跳地址")
}

func TestSafeRedirect(t *testing.T) {
	cases := map[string]string{
		"":                      "",
		"/lite/me":              "/lite/me",
		"/lite/articles?page=2": "/lite/articles?page=2",
		" /lite/me ":            "/lite/me",
		"https://evil.com":      "",
		"//evil.com":            "",
		"/\\evil":               "",
		"javascript:alert(1)":   "",
		"lite/me":               "",
	}
	for raw, want := range cases {
		if got := SafeRedirect(raw); got != want {
			t.Errorf("SafeRedirect(%q) = %q，期望 %q", raw, got, want)
		}
	}
}

func TestRegisterTemplate(t *testing.T) {
	html := renderToString(t, "register", RegisterView{
		BaseView:     testBase(),
		PasswordRule: PasswordRuleText,
	})

	// 字段与文案照搬主站 regAlogin 的注册态
	mustContain(t, html, "注册", "标题照搬主站")
	mustContain(t, html, "用户名", "字段照搬主站")
	mustContain(t, html, "邮箱", "字段照搬主站")
	mustContain(t, html, "确认密码", "字段照搬主站")
	mustContain(t, html, "注册成功后请使用邮件中的链接完成账户激活。", "提示语照搬主站")
	mustContain(t, html, "立即登录", "切换入口照搬主站")
	mustContain(t, html, `action="/lite/register"`, "提交到表单自己的地址")
	mustContain(t, html, `method="post"`, "表单必须是 POST")
}

func TestRegisterTemplateShowsMessages(t *testing.T) {
	bad := renderToString(t, "register", RegisterView{BaseView: testBase(), Username: "u", Email: "e@x.com", Error: "用户已存在"})
	mustContain(t, bad, "用户已存在", "应显示后端错误原文")
	mustContain(t, bad, `value="u"`, "用户名应回填")
	mustContain(t, bad, `value="e@x.com"`, "邮箱应回填")

	ok := renderToString(t, "register", RegisterView{BaseView: testBase(), Notice: "注册成功，激活邮件已发送，请前往邮箱完成激活"})
	mustContain(t, ok, "激活邮件已发送", "应显示成功提示")
	mustContain(t, ok, "lite-alert-ok", "成功提示用成功样式")
}

func TestActivateTemplate(t *testing.T) {
	ok := renderToString(t, "activate", ActivateView{
		BaseView: testBase(),
		Success:  true,
		Title:    "账户激活成功",
		Message:  "你的账户已成功激活，现在可以正常登录。",
	})
	mustContain(t, ok, "已激活", "状态标签照搬主站")
	mustContain(t, ok, "账户激活成功", "标题照搬主站")
	mustContain(t, ok, "去登录", "按钮照搬主站")

	bad := renderToString(t, "activate", ActivateView{
		BaseView: testBase(),
		Title:    "账户激活失败",
		Message:  "激活链接已失效，请返回“我的”页面重新发送激活邮件。",
	})
	mustContain(t, bad, "激活失败", "状态标签照搬主站")
	mustContain(t, bad, "返回我的页面", "按钮照搬主站")
	mustContain(t, bad, "返回首页", "按钮照搬主站")
}

func TestMeTemplate(t *testing.T) {
	html := renderToString(t, "me", MeView{
		BaseView:     testBase(),
		Username:     "coco",
		Email:        "coco@example.com",
		Activated:    true,
		Deepable:     true,
		IsDeep:       false,
		RSSToken:     "tok-abc",
		PasswordRule: PasswordRuleText,
	})

	// 展示项与文案照搬主站 pages/me/index.vue
	mustContain(t, html, "coco", "应显示用户名")
	mustContain(t, html, "📧 邮箱：", "字段与主站一致")
	mustContain(t, html, "✅ 账户激活状态：", "字段与主站一致")
	mustContain(t, html, "📝 深度模式权限：", "字段与主站一致")
	mustContain(t, html, "🔓 深度模式状态：", "字段与主站一致")
	mustContain(t, html, "🔑 RSS Token：", "字段与主站一致")
	mustContain(t, html, "已激活", "状态文案与主站一致")
	mustContain(t, html, "已开通", "状态文案与主站一致")
	mustContain(t, html, "未启用", "状态文案与主站一致")
	mustContain(t, html, "🔒 修改密码", "操作项与主站一致")
	mustContain(t, html, "🔄 重置 RSS Token", "操作项与主站一致")
	mustContain(t, html, "确认修改", "按钮文案与主站一致")
	mustContain(t, html, "确认重置", "按钮文案与主站一致")
	mustContain(t, html, "退出登录", "登出入口")
	mustContain(t, html, PasswordRuleText, "新密码提示与主站一致")
	mustContain(t, html, `method="post"`, "这些操作必须走 POST")
	mustNotContain(t, html, "账户待激活", "已激活时不显示激活提示")
}

func TestMeTemplateActivationPrompt(t *testing.T) {
	html := renderToString(t, "me", MeView{BaseView: testBase(), Username: "u"})
	mustContain(t, html, "账户待激活", "未激活提示照搬主站")
	mustContain(t, html, "请先完成邮箱激活", "标题照搬主站")
	mustContain(t, html, "未激活的账户的权限跟未登录时一致", "说明照搬主站")
	mustContain(t, html, "重新发送激活邮件", "按钮照搬主站")
}

func TestMeTemplateDenied(t *testing.T) {
	html := renderToString(t, "me", MeView{BaseView: testBase(), Denied: true})
	mustContain(t, html, "权限不足", "照搬主站 permDenied 分支")
	mustContain(t, html, "无法查看个人信息", "标题照搬主站")
	mustContain(t, html, "去激活账户", "按钮照搬主站")
}

func TestValidateNewPassword(t *testing.T) {
	cases := []struct {
		pwd, confirm, want string
	}{
		{"", "", "请输入新密码"},
		{"12345", "12345", "新密码长度至少需要6位字符"},
		{"123456", "123457", "两次输入的密码不一致，请重新输入"},
		{"123456", "123456", ""},
		{"密码密码密码", "密码密码密码", ""}, // 6 个字符，按字符数而非字节数计
	}
	for _, c := range cases {
		if got := ValidateNewPassword(c.pwd, c.confirm); got != c.want {
			t.Errorf("ValidateNewPassword(%q,%q) = %q，期望 %q", c.pwd, c.confirm, got, c.want)
		}
	}
}

func TestAboutTemplateTabs(t *testing.T) {
	us := renderToString(t, "about", AboutView{BaseView: testBase(), Tab: "us"})
	mustContain(t, us, "关于本港湾", "默认页签应是「我们的避风港」")
	mustContain(t, us, "is-active", "应标出当前页签")

	friends := renderToString(t, "about", AboutView{BaseView: testBase(), Tab: "friends"})
	mustContain(t, friends, "好朋友们", "friends 页签应切换正文")
	mustContain(t, friends, "/lite/about/us", "应能切回另一个页签")
}

func TestErrorTemplateUsesMainSiteCopy(t *testing.T) {
	html := renderToString(t, "error", ErrorView{
		BaseView: testBase(),
		Code:     404,
	})

	// 与主站 pages/notFound/index.vue 完全一致
	mustContain(t, html, "404", "应显示状态码")
	mustContain(t, html, "这里什么都没有", "标题照搬主站")
	mustContain(t, html, "你要找的页面可能已被移动、删除，或者从未存在过。", "说明照搬主站")
	mustContain(t, html, "也可能只是coco太饿，已经吃到肚子里了", "吐槽照搬主站")
	mustContain(t, html, "当前地址：/lite/", "主站会显示当前地址")
	mustContain(t, html, "返回首页", "按钮照搬主站")
	mustContain(t, html, "浏览全部文章", "按钮照搬主站")
}

// TestFooterItemsMatchMainSite 是护栏用例：
// 页脚必须与主站 Footer.vue 展示同一组信息，既不能少也不能自己加。
func TestFooterItemsMatchMainSite(t *testing.T) {
	html := renderToString(t, "home", HomeView{BaseView: testBase()})

	// 主站页脚有的
	mustContain(t, html, "© 2025-2026 coco_29. All Rights Reserved.", "版权行照搬主站")
	mustContain(t, html, "🖊️ 站点总字数 ≈ 34.6k", "站点总字数照搬主站")
	mustContain(t, html, "🍵 阅读时长 ≈ 1:56", "阅读时长照搬主站")
	mustContain(t, html, "已避风 292天5时30分12秒", "避风时长照搬主站")
	mustContain(t, html, "Powered by Vue & GO", "主站的署名行")

	// ⚠️ 以下是用户明确否掉的「自作主张」项，加回去这个用例就会红
	banned := []string{"位读者", "篇文章", "已开港", "总访客"}
	for _, bad := range banned {
		mustNotContain(t, html, bad, "页脚不得显示主站没有的数据")
	}
}

// TestTemplatesHaveNoLiteralEscapes 挡一类手滑：把换行写成字面的 \n。
// 模板里出现它时不会报错，页面会原样显示「\n」—— 而且只影响观感，
// 单测里那些 mustContain 断言照样通过，所以需要单独钉住。
func TestTemplatesHaveNoLiteralEscapes(t *testing.T) {
	dir := "assets/templates"
	entries, err := templatesFS.ReadDir(dir)
	if err != nil {
		t.Fatalf("读模板目录失败: %v", err)
	}
	for _, e := range entries {
		b, err := templatesFS.ReadFile(dir + "/" + e.Name())
		if err != nil {
			t.Fatalf("读 %s 失败: %v", e.Name(), err)
		}
		if strings.Contains(string(b), `\n`) {
			t.Errorf("%s 里出现字面的 \\n，应改为真实换行", e.Name())
		}
	}
}

// TestTemplatesHaveNoScript 是老 Kindle 的硬要求：内容写在 HTML 里，别指望脚本填。
func TestTemplatesHaveNoScript(t *testing.T) {
	pages := map[string]any{
		"home":    HomeView{BaseView: testBase(), Articles: []ListItemView{sampleItem()}},
		"list":    ListView{BaseView: testBase(), Articles: []ListItemView{sampleItem()}, Page: 1, TotalPages: 1},
		"article": ArticleView{BaseView: testBase(), Title: "t", Content: template.HTML("<p>x</p>")},
		"about":   AboutView{BaseView: testBase(), Tab: "us"},
		"error":   ErrorView{BaseView: testBase(), Code: 404},
	}

	for name, data := range pages {
		html := renderToString(t, name, data)
		mustNotContain(t, html, "<script", name+" 页面不应含脚本：老 Kindle 上脚本是奢侈品，内容必须直出")
		mustNotContain(t, html, "onclick=", name+" 页面不应依赖内联事件")
	}
}

// ────────────────────────────── 纯函数 ──────────────────────────────

func TestParsePage(t *testing.T) {
	cases := map[string]uint64{
		"":            1,
		"0":           1,
		"abc":         1,
		"-3":          1,
		"2":           2,
		" 3 ":         3,
		"99999999999": MaxPage,
	}
	for input, want := range cases {
		if got := ParsePage(input); got != want {
			t.Errorf("ParsePage(%q) = %d, 期望 %d", input, got, want)
		}
	}
}

func TestPrevPage(t *testing.T) {
	cases := map[uint64]uint64{0: 1, 1: 1, 2: 1, 3: 2}
	for input, want := range cases {
		if got := PrevPage(input); got != want {
			t.Errorf("PrevPage(%d) = %d, 期望 %d", input, got, want)
		}
	}
}

func TestParseJSONList(t *testing.T) {
	cases := []struct {
		raw  string
		want []string
	}{
		{`["tech","music"]`, []string{"tech", "music"}},
		{`["tech", "music"]`, []string{"tech", "music"}},
		{`[""]`, nil},
		{`tech,music`, []string{"tech", "music"}}, // 老数据是逗号串
		{`tech，music`, []string{"tech", "music"}},
		{``, nil},
		{`[]`, nil},
	}
	for _, c := range cases {
		got := ParseJSONList(c.raw)
		if len(got) != len(c.want) {
			t.Errorf("ParseJSONList(%q) = %v, 期望 %v", c.raw, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("ParseJSONList(%q) = %v, 期望 %v", c.raw, got, c.want)
				break
			}
		}
	}
}

func TestThumbURL(t *testing.T) {
	base := "https://api.example.com/static/uploads/a.jpg"

	if got := ThumbURL(base); got != base+"?thumb=1" {
		t.Errorf("本站图应加 thumb=1，实际 %q", got)
	}
	if got := ThumbURL(base + "?thumb=1"); got != base+"?thumb=1" {
		t.Errorf("ThumbURL 应当幂等，实际 %q", got)
	}
	if got := ThumbURL(base + "?v=2"); got != base+"?v=2&thumb=1" {
		t.Errorf("已有查询串时应改用 &，实际 %q", got)
	}
	external := "https://cdn.example.com/a.jpg"
	if got := ThumbURL(external); got != external {
		t.Errorf("外链不应改写，实际 %q", got)
	}
	if got := ThumbURL(""); got != "" {
		t.Errorf("空值应原样返回，实际 %q", got)
	}
}

func TestRewriteContentImages(t *testing.T) {
	local := `<p>看这张：<img src="https://api.example.com/static/uploads/a.jpg" alt="图"></p>`
	got := RewriteContentImages(local)
	if !strings.Contains(got, "a.jpg?thumb=1") {
		t.Errorf("本站图应改写为压缩图地址，实际: %s", got)
	}
	if !strings.Contains(got, `alt="图"`) {
		t.Errorf("改写不应丢掉其它属性，实际: %s", got)
	}

	webp := `<img src="https://cdn.example.com/pic.webp">`
	got = RewriteContentImages(webp)
	if strings.Contains(got, "<img") {
		t.Errorf("webp 外链应降级为链接（老 Kindle 不显示 WebP），实际: %s", got)
	}
	if !strings.Contains(got, "https://cdn.example.com/pic.webp") {
		t.Errorf("降级后仍应保留原地址，实际: %s", got)
	}

	webpQS := `<img src="https://cdn.example.com/pic.webp?x=1">`
	if got = RewriteContentImages(webpQS); strings.Contains(got, "<img") {
		t.Errorf("带查询串的 webp 也应降级，实际: %s", got)
	}

	externalJpg := `<img src="https://cdn.example.com/pic.jpg">`
	if got = RewriteContentImages(externalJpg); got != externalJpg {
		t.Errorf("常见外链图应原样保留，实际: %s", got)
	}

	if got = RewriteContentImages(""); got != "" {
		t.Errorf("空内容应原样返回，实际: %q", got)
	}
	plain := "<p>没有图片</p>"
	if got = RewriteContentImages(plain); got != plain {
		t.Errorf("无图片的内容不应被改动，实际: %q", got)
	}
}

func TestBuildPageURL(t *testing.T) {
	got := BuildPageURL("/lite/articles", 3, "koko", "tech", "go", "updated")
	for _, want := range []string{"page=3", "q=koko", "category=tech", "tag=go", "sort=updated"} {
		if !strings.Contains(got, want) {
			t.Errorf("分页链接应保留筛选条件 %q，实际: %s", want, got)
		}
	}
	if !strings.HasPrefix(got, "/lite/articles?") {
		t.Errorf("分页链接应带上基准路径，实际: %s", got)
	}

	empty := BuildPageURL("/lite/articles", 1, "", "", "", "")
	if empty != "/lite/articles?page=1" {
		t.Errorf("无筛选时只应带页码，实际: %s", empty)
	}
}

func TestFormatDayAndLatestTime(t *testing.T) {
	if got := FormatDay(time.Time{}); got != "" {
		t.Errorf("零值应返回空串，实际 %q", got)
	}
	day := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if got := FormatDay(day); got != "2026-01-02" {
		t.Errorf("FormatDay = %q, 期望 2026-01-02", got)
	}

	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	updated := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	if got := LatestTime(models.Post{CreatedAt: created, UpdatedAt: updated}); !got.Equal(updated) {
		t.Errorf("应取较晚的时间，实际 %v", got)
	}
	if got := LatestTime(models.Post{CreatedAt: updated, UpdatedAt: created}); !got.Equal(updated) {
		t.Errorf("应取较晚的时间，实际 %v", got)
	}
	if got := LatestTime(models.Post{CreatedAt: created}); !got.Equal(created) {
		t.Errorf("updatedAt 为零值时应取 createdAt，实际 %v", got)
	}
}

// ────── 以下四个换算规则必须与主站前端逐字一致 ──────

func TestFormatCompactNumber(t *testing.T) {
	cases := map[string]string{
		"293":     "293",
		"999":     "999",
		"1000":    "1.0k",
		"34567":   "34.6k",
		"999999":  "1000.0k", // 与主站一致：只按下一档判断，不做进位
		"1000000": "1.0m",
		"1234567": "1.2m",
		"":        "",
		"abc":     "abc",
	}
	for raw, want := range cases {
		if got := FormatCompactNumber(raw); got != want {
			t.Errorf("FormatCompactNumber(%q) = %q, 期望 %q", raw, got, want)
		}
	}
}

func TestFormatSiteReadingTime(t *testing.T) {
	cases := map[string]string{
		"293":   "0:01",
		"0":     "0:00",
		"300":   "0:01",
		"600":   "0:02",
		"34567": "1:56",
		"18000": "1:00",
		"abc":   "0:00",
		"":      "0:00",
	}
	for raw, want := range cases {
		if got := FormatSiteReadingTime(raw); got != want {
			t.Errorf("FormatSiteReadingTime(%q) = %q, 期望 %q", raw, got, want)
		}
	}
}

func TestReadingMinutes(t *testing.T) {
	cases := map[uint64]int{
		0:   1, // 不足 200 字一律 1 分钟
		100: 1,
		199: 1,
		200: 1,
		201: 2,
		400: 2,
		401: 3,
		450: 3,
	}
	for words, want := range cases {
		if got := ReadingMinutes(words); got != want {
			t.Errorf("ReadingMinutes(%d) = %d, 期望 %d", words, got, want)
		}
	}
}

func TestFormatDateTime(t *testing.T) {
	if got := FormatDateTime(time.Time{}); got != "" {
		t.Errorf("零值应返回空串，实际 %q", got)
	}
	// 主站对早于 2000 年的日期返回空串（如 0001-01-01）
	if got := FormatDateTime(time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)); got != "" {
		t.Errorf("早于 2000 年的日期应返回空串，实际 %q", got)
	}
	got := FormatDateTime(time.Date(2026, 1, 2, 15, 4, 0, 0, time.UTC))
	if got != "2026/01/02 15:04" {
		t.Errorf("FormatDateTime = %q, 期望 2026/01/02 15:04", got)
	}
}

func TestFormatUptime(t *testing.T) {
	started := time.Date(2025, 12, 20, 0, 0, 0, 0, time.UTC)

	ok := started.AddDate(0, 0, 292).Add(5*time.Hour + 30*time.Minute + 12*time.Second)
	if got := FormatUptime(started, ok); got != "已避风 292天5时30分12秒" {
		t.Errorf("已开港时应是「已避风 …」，实际 %q", got)
	}

	// 还没开港：主站显示「本港湾还有 … 开放」
	before := started.Add(-48*time.Hour - 30*time.Minute - 5*time.Second)
	if got := FormatUptime(started, before); got != "本港湾还有 2天0时30分5秒 开放" {
		t.Errorf("未开港时应是「本港湾还有 … 开放」，实际 %q", got)
	}
}

func TestFormatFilterText(t *testing.T) {
	cases := []struct {
		q, category, tag string
		want             string
	}{
		{"", "", "", ""},
		{"koko", "", "", "搜索“koko”"},
		{"", "tech", "", "分类“tech”"},
		{"", "", "go", "标签“go”"},
		{"koko", "tech", "go", "搜索“koko” · 分类“tech” · 标签“go”"},
	}
	for _, c := range cases {
		if got := FormatFilterText(c.q, c.category, c.tag); got != c.want {
			t.Errorf("FormatFilterText(%q,%q,%q) = %q, 期望 %q", c.q, c.category, c.tag, got, c.want)
		}
	}
}

// ────────────────────────────── 样式约束 ──────────────────────────────

// TestLiteCSSAvoidsUnsupportedSyntax 把「发布前过一遍」那份清单固化成断言：
// 老 Kindle 的 CSS 引擎不认识这些写法，写了不报错、只静默降级，很难在真机上发现。
func TestLiteCSSAvoidsUnsupportedSyntax(t *testing.T) {
	css := string(liteCSS)
	if len(css) == 0 {
		t.Fatal("lite.css 应当被内嵌进二进制，实际为空")
	}

	banned := []struct {
		pattern *regexp.Regexp
		why     string
	}{
		{regexp.MustCompile(`(?i)display\s*:\s*flex`), "flex 容器在这台机器上会退化成块级堆叠"},
		{regexp.MustCompile(`(?i)display\s*:\s*grid`), "grid 完全不支持"},
		{regexp.MustCompile(`(?i)flex-direction|flex-wrap|justify-content|align-items`), "现代 flex 属性不支持，应改用 -webkit-box-*"},
		{regexp.MustCompile(`(?i)\bgap\s*:`), "gap 无效，间隔要用 padding / margin"},
		{regexp.MustCompile(`[0-9.]+rem`), "没有 rem，尺寸只能用 px / em / %"},
		{regexp.MustCompile(`[0-9.]+v(w|h)\b`), "没有 vw / vh"},
		{regexp.MustCompile(`var\s*\(`), "CSS 变量不解析，颜色必须写死"},
		{regexp.MustCompile(`:has\s*\(`), ":has() 在没有 JS 的老引擎上不存在"},
		{regexp.MustCompile(`(?i)position\s*:\s*sticky`), "不支持 sticky"},
		{regexp.MustCompile(`(?i)\btransition\s*:`), "不写过渡（不解析，且墨水屏上会留残影）"},
		{regexp.MustCompile(`(?i)\banimation\s*:`), "不写动画"},
	}

	for _, b := range banned {
		if loc := b.pattern.FindStringIndex(css); loc != nil {
			line := 1 + strings.Count(css[:loc[0]], "\n")
			t.Errorf("lite.css:%d 出现禁用的写法 %q —— %s", line, css[loc[0]:loc[1]], b.why)
		}
	}
}

// TestLiteCSSKeepsWebkitPrefixes 圆角 / 阴影在老引擎上需要 -webkit- 前缀，
// 现代浏览器需要标准写法，两个都得写。
func TestLiteCSSKeepsWebkitPrefixes(t *testing.T) {
	css := string(liteCSS)
	pairs := []struct{ webkit, standard string }{
		{"-webkit-border-radius", "border-radius"},
		{"-webkit-box-shadow", "box-shadow"},
	}

	for _, p := range pairs {
		if !strings.Contains(css, p.webkit) {
			t.Errorf("lite.css 缺少 %s（老引擎只认带前缀的写法）", p.webkit)
		}
		if !strings.Contains(css, p.standard) {
			t.Errorf("lite.css 缺少 %s（现代浏览器需要标准写法）", p.standard)
		}
	}

	if !strings.Contains(css, "display: -webkit-box") {
		t.Error("lite.css 应使用 display: -webkit-box 做并排布局")
	}
}

// TestLiteCSSTouchTargets 老 Kindle 只有 click、没有 hover，
// 可点区域的高度必须够大（指南建议 ≥48px）。
func TestLiteCSSTouchTargets(t *testing.T) {
	css := string(liteCSS)
	for _, selector := range []string{".lite-btn {", ".lite-nav a {", ".lite-about-link {", ".lite-sort-option {"} {
		idx := strings.Index(css, selector)
		if idx < 0 {
			t.Fatalf("lite.css 里找不到 %s", selector)
		}
		end := strings.Index(css[idx:], "}")
		block := css[idx : idx+end]
		if !strings.Contains(block, "min-height: 48px") {
			t.Errorf("%s 的触摸目标应至少 48px，实际:\n%s", selector, block)
		}
	}
}

// TestLiteCSSFontStacksHaveFallback 字体名必须带通用族兜底，
// 否则只写 Georgia 之类会直接掉进默认字体。
func TestLiteCSSFontStacksHaveFallback(t *testing.T) {
	css := string(liteCSS)
	fontRe := regexp.MustCompile(`font-family\s*:\s*([^;]+);`)
	matches := fontRe.FindAllStringSubmatch(css, -1)
	if len(matches) == 0 {
		t.Fatal("lite.css 里没有 font-family 声明？")
	}
	for _, m := range matches {
		stack := strings.TrimSpace(m[1])
		if !strings.Contains(stack, "serif") && !strings.Contains(stack, "sans-serif") && !strings.Contains(stack, "monospace") {
			t.Errorf("字体栈 %q 缺少通用族兜底", stack)
		}
	}
}
