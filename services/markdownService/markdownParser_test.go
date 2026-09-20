package markdownService

import "testing"

func TestParseMarkdownToHTML(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "标题 id 由标题文本推导",
			in:   "# Hello World",
			want: "<h1 id=\"hello-world\">Hello World</h1>\n",
		},
		{
			name: "中文标题保留中文",
			in:   "# 中文标题",
			want: "<h1 id=\"中文标题\">中文标题</h1>\n",
		},
		{
			name: "中英混排只折叠分隔符",
			in:   "### Mixed 中文 English 标题",
			want: "<h3 id=\"mixed-中文-english-标题\">Mixed 中文 English 标题</h3>\n",
		},
		{
			name: "标题重名依次加后缀",
			in:   "# 标题\n\n## 标题\n\n### 标题",
			want: "<h1 id=\"标题\">标题</h1>\n<h2 id=\"标题-1\">标题</h2>\n<h3 id=\"标题-2\">标题</h3>\n",
		},
		{
			name: "取的是可见文本：行内代码",
			in:   "# `code` 标题",
			want: "<h1 id=\"code-标题\"><code>code</code> 标题</h1>\n",
		},
		{
			name: "取的是可见文本：链接不带地址",
			in:   "# [链接文字](https://example.com/a/b?c=d)",
			want: "<h1 id=\"链接文字\"><a href=\"https://example.com/a/b?c=d\">链接文字</a></h1>\n",
		},
		{
			name: "标题全是标点时兜底为 section",
			in:   "# !!!",
			want: "<h1 id=\"section\">!!!</h1>\n",
		},
		{
			name: "标点折叠成单个连字符",
			in:   "# 带标点：标题（括号）#号",
			want: "<h1 id=\"带标点-标题-括号-号\">带标点：标题（括号）#号</h1>\n",
		},
		{
			name: "中文软换行不插入空格",
			in:   "中文第一行\n中文第二行",
			want: "<p>中文第一行中文第二行</p>\n",
		},
		{
			name: "非中文软换行保持换行",
			in:   "line one\nline two",
			want: "<p>line one\nline two</p>\n",
		},
		{
			name: "GFM 表格",
			in:   "| a | b |\n|---|---|\n| 1 | 2 |",
			want: "<table>\n<thead>\n<tr>\n<th>a</th>\n<th>b</th>\n</tr>\n</thead>\n<tbody>\n<tr>\n<td>1</td>\n<td>2</td>\n</tr>\n</tbody>\n</table>\n",
		},
		{
			name: "GFM 删除线与自动链接",
			in:   "~~del~~ and https://example.com",
			want: "<p><del>del</del> and <a href=\"https://example.com\">https://example.com</a></p>\n",
		},
		{
			name: "GFM 任务列表",
			in:   "- [x] done\n- [ ] todo",
			want: "<ul>\n<li><input checked=\"\" disabled=\"\" type=\"checkbox\"> done</li>\n<li><input disabled=\"\" type=\"checkbox\"> todo</li>\n</ul>\n",
		},
		{
			// 前端代码块的「语言标签 + 复制按钮」依赖这里的 language-* class
			name: "围栏代码块带语言 class",
			in:   "```go\nfmt.Println(1)\n```",
			want: "<pre><code class=\"language-go\">fmt.Println(1)\n</code></pre>\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseMarkdownToHTML(c.in)
			if err != nil {
				t.Fatalf("ParseMarkdownToHTML(%q) 返回错误: %v", c.in, err)
			}
			if got != c.want {
				t.Errorf("ParseMarkdownToHTML(%q)\n得到: %q\n期望: %q", c.in, got, c.want)
			}
		})
	}
}

func TestParseMarkdownMath(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "行内公式",
			in:   "质能方程 $E = mc^2$ 成立。",
			want: "<p>质能方程 <span class=\"math-inline\">E = mc^2</span> 成立。</p>\n",
		},
		{
			// 公式里的 _ * \ 若不被当成原始文本保护起来，
			// 会被 Markdown 的行内语法吃掉（_…_ 变强调、字母被转义）
			name: "下标与星号不被 Markdown 解析",
			in:   "$a_1 * b_2 + c_{n+1} \\le \\infty$",
			want: "<p><span class=\"math-inline\">a_1 * b_2 + c_{n+1} \\le \\infty</span></p>\n",
		},
		{
			name: "块级公式：$$ 独占行",
			in:   "$$\n\\int_0^1 x^2 \\, dx = \\frac{1}{3}\n$$",
			want: "<div class=\"math-block\">\\int_0^1 x^2 \\, dx = \\frac{1}{3}</div>",
		},
		{
			// 块级解析器在 Open 阶段结束不了自己，写错就会把后面的正文卷进来
			name: "$$ 与收尾同行时不吞后续内容",
			in:   "前文\n\n$$ a = b $$\n\n后文",
			want: "<p>前文</p>\n<p><span class=\"math-block\">a = b</span></p>\n<p>后文</p>\n",
		},
		{
			name: "多行对齐环境里的 & 与反斜杠被 HTML 转义",
			in:   "$$\n\\begin{aligned}\na &= b + c \\\\\nd &= e - f\n\\end{aligned}\n$$",
			want: "<div class=\"math-block\">\\begin{aligned}\na &amp;= b + c \\\\\nd &amp;= e - f\n\\end{aligned}</div>",
		},
		{
			name: "货币写法不被误判成公式",
			in:   "价格是 $5 到 $10 之间",
			want: "<p>价格是 $5 到 $10 之间</p>\n",
		},
		{
			name: "转义的美元符按字面输出",
			in:   "\\$100 不打折",
			want: "<p>$100 不打折</p>\n",
		},
		{
			name: "代码块里的 $ 不当公式",
			in:   "```sh\necho \"$HOME\"\n```",
			want: "<pre><code class=\"language-sh\">echo &quot;$HOME&quot;\n</code></pre>\n",
		},
		{
			name: "列表与引用里的公式",
			in:   "- 列表 $x^2$\n\n> 引用 $$y = kx + b$$",
			want: "<ul>\n<li>列表 <span class=\"math-inline\">x^2</span></li>\n</ul>\n<blockquote>\n<p>引用 <span class=\"math-block\">y = kx + b</span></p>\n</blockquote>\n",
		},
		{
			name: "标题里的公式参与目录 id",
			in:   "# 公式 $e^{i\\pi}$",
			want: "<h1 id=\"公式\">公式 <span class=\"math-inline\">e^{i\\pi}</span></h1>\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseMarkdownToHTML(c.in)
			if err != nil {
				t.Fatalf("ParseMarkdownToHTML(%q) 返回错误: %v", c.in, err)
			}
			if got != c.want {
				t.Errorf("ParseMarkdownToHTML(%q)\n得到: %q\n期望: %q", c.in, got, c.want)
			}
		})
	}
}

func TestParseMarkdownFootnote(t *testing.T) {
	got, err := ParseMarkdownToHTML("正文[^1]。\n\n[^1]: 脚注内容。")
	if err != nil {
		t.Fatalf("返回错误: %v", err)
	}
	want := "<p>正文<sup id=\"fnref:1\"><a href=\"#fn:1\" class=\"footnote-ref\" role=\"doc-noteref\">1</a></sup>。</p>\n" +
		"<div class=\"footnotes\" role=\"doc-endnotes\">\n<hr>\n<ol>\n<li id=\"fn:1\">\n" +
		"<p>脚注内容。&#160;<a href=\"#fnref:1\" class=\"footnote-backref\" role=\"doc-backlink\">&#x21a9;&#xfe0e;</a></p>\n" +
		"</li>\n</ol>\n</div>\n"
	if got != want {
		t.Errorf("脚注渲染结果不符\n得到: %q\n期望: %q", got, want)
	}
}

func TestSlugify(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Hello World", "hello-world"},
		{"  前后空白  ", "前后空白"},
		{"a--b  c", "a-b-c"},
		{"a_b", "ab"},              // 下划线是行内标记，直接丢弃
		{"emoji 🎉 后缀", "emoji-后缀"}, // 非字母数字折叠成连字符
		{"🎉", ""},                  // 无可用字符
		{"中文，标点。", "中文-标点"},
	}
	for _, c := range cases {
		if got := slugify(c.in); got != c.want {
			t.Errorf("slugify(%q) = %q, 期望 %q", c.in, got, c.want)
		}
	}
}

func TestUniqueHeadingID(t *testing.T) {
	used := map[string]bool{}
	if got := uniqueHeadingID("", used); got != "section" {
		t.Errorf("空标题应兜底为 section，得到 %q", got)
	}
	if got := uniqueHeadingID("", used); got != "section-1" {
		t.Errorf("section 重名应追加后缀，得到 %q", got)
	}
	if got := uniqueHeadingID("标题", used); got != "标题" {
		t.Errorf("首次出现的标题不应加后缀，得到 %q", got)
	}
	if got := uniqueHeadingID("标题", used); got != "标题-1" {
		t.Errorf("重名标题应追加后缀，得到 %q", got)
	}
}
