package liteview

import (
	"bytes"
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
		"正文", "标题", "删除这篇文章",
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
