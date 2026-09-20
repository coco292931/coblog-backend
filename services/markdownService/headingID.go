package markdownService

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// headingIDTransformer 给每个标题写上 id，供文章目录锚点使用。
//
// 没有用 goldmark 自带的 parser.WithAutoHeadingID()：那套生成规则会把长度不为 1 的
// UTF-8 字节直接丢掉，于是中文标题一律退化成 "heading"、"heading-1"、"heading-2"…，
// 对中文博客等于没有意义（锚点不可读，也没法分享）。
// 这里改成保留中日韩文字、把其余字符折叠成 "-"，并在同一篇文档内去重。
//
// 该 transformer 在块级与行内解析都完成之后运行，所以 heading.Text() 拿到的是
// 解析后的可见文本：`code` 取到 code、[文字](url) 取到文字，不会把链接地址带进 id。
type headingIDTransformer struct{}

// Transform 实现 parser.ASTTransformer。
func (t *headingIDTransformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	source := reader.Source()
	used := make(map[string]bool)

	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		heading, ok := node.(*ast.Heading)
		if !ok {
			return ast.WalkContinue, nil
		}

		// 标题已经带 id（例如将来启用属性语法让它显式指定）就沿用，只登记占用防重名
		if id, ok := headingID(heading); ok {
			used[id] = true
			return ast.WalkSkipChildren, nil
		}

		heading.SetAttributeString("id", []byte(uniqueHeadingID(slugify(string(heading.Text(source))), used)))
		return ast.WalkSkipChildren, nil
	})
}

// headingID 读取标题上已有的 id 属性（goldmark 里可能是 []byte，也可能是 string）。
func headingID(heading *ast.Heading) (string, bool) {
	value, ok := heading.AttributeString("id")
	if !ok {
		return "", false
	}
	switch v := value.(type) {
	case []byte:
		return string(v), len(v) > 0
	case string:
		return v, v != ""
	default:
		return "", false
	}
}

// slugify 把标题文本转成 id：保留字母、数字与中日韩文字，其余字符折叠成单个 "-"。
// 行内标记符号（`、*、_）直接丢弃，避免 “`code`” 变成 “-code-”。
func slugify(title string) string {
	var b strings.Builder
	pendingSep := false
	for _, r := range title {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if pendingSep && b.Len() > 0 {
				b.WriteByte('-')
			}
			pendingSep = false
			b.WriteRune(unicode.ToLower(r))
		case r == '`' || r == '*' || r == '_':
			// 行内标记，丢弃
		default:
			pendingSep = true
		}
	}
	return b.String()
}

// uniqueHeadingID 返回未被占用的 id：重名时依次追加 -1、-2…
func uniqueHeadingID(base string, used map[string]bool) string {
	if base == "" {
		base = "section"
	}
	if !used[base] {
		used[base] = true
		return base
	}
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if !used[candidate] {
			used[candidate] = true
			return candidate
		}
	}
}
