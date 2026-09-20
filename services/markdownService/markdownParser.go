package markdownService

import (
	"bytes"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/util"
)

// md 是包级单例：构建一个 Markdown 实例要装配整套 block/inline 解析器与渲染器，
// 而 ParseMarkdownToHTML 每个请求都会调用，因此只构建一次复用。
// 复用是安全的：goldmark 的 Convert 每次都会新建 text.Reader 与 parser.Context，
// 解析状态不跨调用共享（goldmark 包内自带的 Convert 就是这么复用一个包级实例的）。
var md = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM,
		// CJK：东亚换行处理。默认（CommonMark）会把软换行渲染成空白，
		// 中文段落里作者随手换行就会多出一个空格，中文排版不需该空格。
		extension.CJK,
		// 脚注 [^1]（非标准语法，但写作常用）
		extension.Footnote,
		// 注意：没有启用 extension.DefinitionList。它在「多条定义 + 前后含多字节字符」时
		// 会按字节偏移切错文本段，往中文里插入一个空格（实测："一种草本植物的果实" →
		// "一种草本植物 的果实"，纯英文列表正常），属上游缺陷，等修复后再开。
		// LaTeX 公式：这里只做原样保护，排版由前端 KaTeX 完成（见 math.go）
		&mathExtender{},
	),
	goldmark.WithParserOptions(
		// 标题id由 headingIDTransformer 生成（见 headingID.go）
		parser.WithASTTransformers(util.Prioritized(&headingIDTransformer{}, 100)),
	),
)

// ParseMarkdownToHTML parses a Markdown string and returns the HTML string
func ParseMarkdownToHTML(markdown string) (string, error) {
	var buf bytes.Buffer
	if err := md.Convert([]byte(markdown), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}
