package liteview

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

// 分类 / 标签的逗号分隔 ↔ JSON 数组转换。
// 主站前端 buildArticlePayload 也是 trim 后 JSON 化，这里必须对得上。
func TestToJSONListMatchesFrontend(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"   ", ""},
		{"tech", `["tech"]`},
		{"tech, music", `["tech","music"]`},
		{"tech，music", `["tech","music"]`},
		{",,tech,", `["tech"]`},
		{" a , b , c ", `["a","b","c"]`},
	}
	for _, c := range cases {
		if got := ToJSONList(c.in); got != c.want {
			t.Errorf("ToJSONList(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// 编辑回填：库里的 JSON 数组 → 逗号分隔输入框
func TestJoinListRoundTrip(t *testing.T) {
	raw := `["tech","music"]`
	if got := JoinList(ParseJSONList(raw)); got != "tech,music" {
		t.Errorf("JoinList(ParseJSONList(%s)) = %q, want %q", raw, got, "tech,music")
	}
	if got := JoinList(ParseJSONList("")); got != "" {
		t.Errorf("空分类应回填空串，得到 %q", got)
	}
}

func TestWriteTemplateRenders(t *testing.T) {
	var buf bytes.Buffer
	data := WriteView{
		IsEdit:   true,
		ID:       7,
		Title:    `标题 <script>alert(1)</script>`,
		Category: "tech",
		MdBody:   "# hello",
		IsDeep:   true,
	}
	if err := liteTemplates.ExecuteTemplate(&buf, "write", data); err != nil {
		t.Fatalf("write 模板渲染失败: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		`action="/lite/write/7"`, "md_content", "is_deep", "hidden", "no_stats",
		"正文", "标题", "删除文章",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("write 页面缺少 %q", want)
		}
	}
	if strings.Contains(out, "<script>alert(1)</script>") {
		t.Error("标题未转义")
	}
	if strings.Contains(out, "<script") {
		t.Error("write 页面出现了 script 标签")
	}
}

// 正文里的 Markdown 原文要原样落进 textarea（只是 HTML 转义，不做别的处理）
func TestWriteTemplateKeepsMarkdown(t *testing.T) {
	body := "# 标题\n\n- a < b\n- c & d"
	var buf bytes.Buffer
	if err := liteTemplates.ExecuteTemplate(&buf, "write", WriteView{MdBody: body}); err != nil {
		t.Fatalf("write 模板渲染失败: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "- a &lt; b") {
		t.Error("Markdown 正文没有正确转义后落进 textarea")
	}
	if !strings.Contains(out, "c &amp; d") {
		t.Error("& 应转义为 &amp;")
	}
}

// 有上传权限时：表单必须是 multipart，带封面 / 正文图两个文件框，并加载 write.js
func TestWriteTemplateUploadFields(t *testing.T) {
	var buf bytes.Buffer
	data := WriteView{CanUpload: true, MaxUploads: 5, CoverPreview: "https://api.example.com/static/uploads/c.jpg?thumb=1"}
	if err := liteTemplates.ExecuteTemplate(&buf, "write", data); err != nil {
		t.Fatalf("write 模板渲染失败: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		`enctype="multipart/form-data"`,
		`id="cover_file" name="cover_file"`,
		`name="images"`, "multiple",
		"最多 5 张",
		"background-image: url(", "c.jpg?thumb=1",
		`<script src="/lite/write.js"></script>`,
		// 脚本里按 id 找元素，模板改名会让增强静默失效
		`id="write-form"`, `id="md_content"`, `id="cover_image"`, `id="cover-preview"`,
		`id="upload-status"`, `id="char-count"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("write 页面缺少 %q", want)
		}
	}
	// 回车提交的是第一个提交按钮，必须是「发表」
	if strings.Count(out, `type="submit"`) != 1 {
		t.Error("写作页只应有一个提交按钮")
	}
	// WebP 在老设备上显示不了，不该出现在可选类型里
	if strings.Contains(out, "webp") {
		t.Error("文件框不应接受 webp")
	}
}

// 字段顺序、文案与 placeholder 对齐主站 pages/write/index.vue
func TestWriteTemplateMatchesMainSite(t *testing.T) {
	out := renderToString(t, "write", WriteView{})
	order := []string{
		`id="title"`, `id="subtitle"`, `id="summary"`, `id="category"`, `id="tags"`,
		`id="cover_image"`, `name="is_deep"`, `name="hidden"`, `name="no_stats"`, `id="md_content"`,
	}
	last := -1
	for _, f := range order {
		i := strings.Index(out, f)
		if i < 0 {
			t.Errorf("缺少字段 %s", f)
			continue
		}
		if i < last {
			t.Errorf("字段 %s 的位置与主站不一致", f)
		}
		last = i
	}
	for _, want := range []string{
		`placeholder="给文章起个标题"`, `placeholder="可选"`, `placeholder="列表页展示的简介"`,
		`placeholder="多个用逗号分隔，如：技术, 随笔"`, `placeholder="多个用逗号分隔，如：vue, go"`,
		`placeholder="在这里用 Markdown 写作…"`, ">发表文章</button>",
	} {
		mustContain(t, out, want, "文案与主站一致")
	}
}

// 没有上传权限时不显示文件框
func TestWriteTemplateHidesUploadWithoutPerm(t *testing.T) {
	var buf bytes.Buffer
	if err := liteTemplates.ExecuteTemplate(&buf, "write", WriteView{}); err != nil {
		t.Fatalf("write 模板渲染失败: %v", err)
	}
	out := buf.String()
	for _, bad := range []string{`type="file"`, `<script`} {
		if strings.Contains(out, bad) {
			t.Errorf("无上传权限时不应出现 %q", bad)
		}
	}
}

func TestAppendImageMarkdown(t *testing.T) {
	cases := []struct {
		md   string
		urls []string
		want string
	}{
		{"正文", nil, "正文"},
		{"", []string{"a.jpg"}, "![](a.jpg)\n"},
		{"正文\n\n\n", []string{"a.jpg", "b.png"}, "正文\n\n![](a.jpg)\n\n![](b.png)\n"},
		{"正文  ", []string{"a.jpg"}, "正文\n\n![](a.jpg)\n"},
	}
	for _, c := range cases {
		if got := AppendImageMarkdown(c.md, c.urls); got != c.want {
			t.Errorf("AppendImageMarkdown(%q, %v) = %q, want %q", c.md, c.urls, got, c.want)
		}
	}
}

func TestConfirmDeleteTemplateRenders(t *testing.T) {
	var buf bytes.Buffer
	data := ConfirmDeleteView{ID: 7, Title: `旧文章 <b>x</b>`}
	if err := liteTemplates.ExecuteTemplate(&buf, "confirm-delete", data); err != nil {
		t.Fatalf("confirm-delete 模板渲染失败: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `action="/lite/write/7/delete"`) {
		t.Error("删除表单 action 不对")
	}
	if !strings.Contains(out, "旧文章 &lt;b&gt;x&lt;/b&gt;") {
		t.Error("标题未转义")
	}
	if strings.Contains(out, "<script") {
		t.Error("confirm-delete 页面出现了 script 标签")
	}
}

// write.js 必须是 ES5：老 Kindle 遇到任何一个 ES6 写法，整段脚本在解析阶段就失败。
// 黑名单对照 kindle-web-dev.md 第七节；先剥掉注释和字符串，避免误报。
func TestWriteJSIsES5(t *testing.T) {
	src := string(writeJS)
	if src == "" {
		t.Fatal("write.js 应当被内嵌进二进制，实际为空")
	}
	code := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(src, "")
	code = regexp.MustCompile(`(?m)//.*$`).ReplaceAllString(code, "")
	code = regexp.MustCompile(`'(?:[^'\\n]|\.)*'`).ReplaceAllString(code, "''")

	banned := []struct{ re, why string }{
		{"`", "模板字符串"},
		{`\blet\s`, "let"},
		{`\bconst\s`, "const"},
		{`=>`, "箭头函数"},
		{`\bclass\s`, "class"},
		{`\?\.`, "?."},
		{`\?\?`, "??"},
		{`\.\.\.`, "剩余 / 展开"},
		{`\bfor\s*\([^)]*\bof\b`, "for...of"},
		{`\basync\b|\bawait\b`, "async / await"},
		{`\*\*`, "**"},
		{`\bfetch\s*\(`, "fetch（没有）"},
		{`\bPromise\b`, "Promise（没有）"},
		{`\.closest\s*\(|\.matches\s*\(`, "closest / matches（没有）"},
		{`\.bind\s*\(`, "Function.bind（没有）"},
		{`\.responseType\b`, "responseType（设了抛 SYNTAX_ERR）"},
		{`\.includes\s*\(|Array\.from|Object\.assign`, "ES6 内置方法（没有）"},
	}
	for _, b := range banned {
		if loc := regexp.MustCompile(b.re).FindStringIndex(code); loc != nil {
			line := 1 + strings.Count(code[:loc[0]], "\n")
			t.Errorf("write.js 约第 %d 行用了 %s —— 老 Kindle 不支持", line, b.why)
		}
	}
}
