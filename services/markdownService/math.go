package markdownService

import (
	"bytes"
	"html"
	"strings"

	"github.com/yuin/goldmark"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// LaTeX 公式支持。
//
// 职责划分：这里只做「原样保护 + 占位」，把 $…$ / $$…$$ 里的内容当成不可再解析的
// 原始文本包进占位元素；真正的排版交给前端 KaTeX。
// 这样做的原因：goldmark（v1.7.13）没有官方 math 扩展，而公式里出现 `_`、`*`、`\`
// 时如果不保护，会被 Markdown 的行内语法吃掉（例如 a_1 * b_2 会被当成强调）。
//
// 输出形态：
//
//	<span class="math-inline">E = mc^2</span>
//	<div class="math-block">\int_0^1 x^2 dx</div>
//
// TeX 以转义后的纯文本放在元素里，前端读 textContent 交给 KaTeX，
// 不需要额外的 data 属性；未启用 JS 时也能看到（可读的）原始公式。
var (
	kindMathInline = gast.NewNodeKind("MathInline")
	kindMathBlock  = gast.NewNodeKind("MathBlock")
)

// mathInlineNode 行内公式 $…$，以及写在同一行里的 $$…$$（display 为 true）
type mathInlineNode struct {
	gast.BaseInline
	tex     []byte
	display bool
}

func (n *mathInlineNode) Kind() gast.NodeKind { return kindMathInline }

func (n *mathInlineNode) Dump(source []byte, level int) {
	gast.DumpHelper(n, source, level, nil, nil)
}

// mathBlockNode 独占若干行的块级公式（$$ 单独占一行）
type mathBlockNode struct {
	gast.BaseBlock
	tex bytes.Buffer
}

func (n *mathBlockNode) Kind() gast.NodeKind { return kindMathBlock }

func (n *mathBlockNode) Dump(source []byte, level int) {
	gast.DumpHelper(n, source, level, nil, nil)
}

// mathInlineParser 解析行内公式。
// 定界符规则沿用通行约定：开定界符后不能紧跟空白、闭定界符前不能是空白，
// 否则「打折是 $5 到 $10」这类正文会被误判成公式。
type mathInlineParser struct{}

func (p *mathInlineParser) Trigger() []byte { return []byte{'$'} }

func (p *mathInlineParser) Parse(_ gast.Node, block text.Reader, _ parser.Context) gast.Node {
	line, _ := block.PeekLine()
	if len(line) == 0 || line[0] != '$' {
		return nil
	}

	display := len(line) > 1 && line[1] == '$'
	openLen := 1
	if display {
		openLen = 2
	}
	if openLen >= len(line) {
		return nil
	}
	// 单个 $ 的开定界符后不能紧跟空白，否则「打折是 $5 到 $10」会被误判成公式；
	// $$ 没有这类歧义，允许写成 $$ x $$（空格夹着写很常见）
	if !display && util.IsSpace(line[openLen]) {
		return nil
	}

	closeIdx := mathCloseIndex(line, openLen, display)
	if closeIdx < 0 {
		return nil
	}
	tex := line[openLen:closeIdx]
	if display {
		tex = util.TrimRightSpace(util.TrimLeftSpace(tex))
	}
	if len(tex) == 0 || (!display && util.IsSpace(tex[len(tex)-1])) {
		return nil
	}

	block.Advance(closeIdx + openLen)
	return &mathInlineNode{tex: append([]byte(nil), tex...), display: display}
}

// mathCloseIndex 返回闭合定界符的下标，找不到返回 -1。
// \$ 是 LaTeX 的转义美元符，不当作定界符。
func mathCloseIndex(line []byte, from int, display bool) int {
	for i := from; i < len(line); i++ {
		if line[i] != '$' || (i > 0 && line[i-1] == '\\') {
			continue
		}
		if i+1 < len(line) && line[i+1] == '$' {
			if display {
				return i
			}
			i++ // 单个 $ 的闭合不能落在 $$ 中间
			continue
		}
		if !display {
			return i
		}
	}
	return -1
}

// mathBlockParser 解析块级公式，只接管「$$ 独占一行」的写法：
//
//	$$
//	\int_0^1 x^2 dx
//	$$
//
// 写在同一行里的 $$ … $$ 由 mathInlineParser 处理 —— 块级解析器在 Open 阶段结束不了自己
// （框架对 Open 的返回值只认 HasChildren / RequireParagraph，且随后会推进到下一行），
// 硬要在 Open 里吃掉整行会把后续内容一起卷进来。
type mathBlockParser struct{}

func (p *mathBlockParser) Trigger() []byte { return []byte{'$'} }

func (p *mathBlockParser) Open(_ gast.Node, reader text.Reader, _ parser.Context) (gast.Node, parser.State) {
	line, _ := reader.PeekLine()
	rest := util.TrimLeftSpace(line)
	if !bytes.HasPrefix(rest, []byte("$$")) {
		return nil, parser.NoChildren
	}
	body := util.TrimRightSpace(rest[2:])
	if bytes.Index(body, []byte("$$")) >= 0 {
		return nil, parser.NoChildren // 单行写法，交给行内解析器
	}

	// 不在这里推进 reader：Open 返回后框架会自己换到下一行
	node := &mathBlockNode{}
	if len(body) > 0 {
		node.tex.Write(body)
		node.tex.WriteByte('\n')
	}
	return node, parser.NoChildren
}

func (p *mathBlockParser) Continue(node gast.Node, reader text.Reader, _ parser.Context) parser.State {
	line, _ := reader.PeekLine()
	math := node.(*mathBlockNode)
	if idx := bytes.Index(line, []byte("$$")); idx >= 0 {
		_, _ = math.tex.Write(util.TrimRightSpace(line[:idx]))
		reader.Advance(idx + 2)
		return parser.Close | parser.NoChildren
	}
	_, _ = math.tex.Write(line)
	// 只推进到行尾换行符之前：跨行的推进由解析框架自己负责，
	// 这里若用 AdvanceLine() 会与框架的推进叠加，导致隔行丢内容
	reader.Advance(lineContentLength(line))
	return parser.Continue | parser.NoChildren
}

// lineContentLength 返回一行去掉行尾换行符后的长度。
func lineContentLength(line []byte) int {
	if len(line) > 0 && line[len(line)-1] == '\n' {
		return len(line) - 1
	}
	return len(line)
}

func (p *mathBlockParser) Close(_ gast.Node, _ text.Reader, _ parser.Context) {}

func (p *mathBlockParser) CanInterruptParagraph() bool { return true }

func (p *mathBlockParser) CanAcceptIndentedLine() bool { return false }

// mathRenderer 把公式节点渲染成占位元素。
type mathRenderer struct{}

func (r *mathRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindMathInline, r.renderInline)
	reg.Register(kindMathBlock, r.renderBlock)
}

func (r *mathRenderer) renderInline(w util.BufWriter, _ []byte, node gast.Node, entering bool) (gast.WalkStatus, error) {
	if !entering {
		return gast.WalkContinue, nil
	}
	n := node.(*mathInlineNode)
	class := "math-inline"
	if n.display {
		// 同一行里的 $$ … $$：元素仍是 span（它身处 <p> 之内），
		// 块级观感由 CSS 的 display:block 提供
		class = "math-block"
	}
	writeMathPlaceholder(w, "span", class, n.tex)
	return gast.WalkSkipChildren, nil
}

func (r *mathRenderer) renderBlock(w util.BufWriter, _ []byte, node gast.Node, entering bool) (gast.WalkStatus, error) {
	if !entering {
		return gast.WalkContinue, nil
	}
	tex := strings.TrimSpace(node.(*mathBlockNode).tex.String())
	writeMathPlaceholder(w, "div", "math-block", []byte(tex))
	return gast.WalkSkipChildren, nil
}

func writeMathPlaceholder(w util.BufWriter, tag, class string, tex []byte) {
	_, _ = w.WriteString("<" + tag + ` class="` + class + `">`)
	_, _ = w.WriteString(html.EscapeString(string(tex)))
	_, _ = w.WriteString("</" + tag + ">")
}

// mathExtender 注册公式解析器与渲染器。
//
// 优先级 510：排在缩进代码块（500）之后 —— 4 空格缩进的 $$ 行应继续被当成代码块；
// 排在段落（1000）之前，才能抢在它把 $$ 行当成普通段落之前接管。
type mathExtender struct{}

func (e *mathExtender) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithBlockParsers(util.Prioritized(&mathBlockParser{}, 510)),
		parser.WithInlineParsers(util.Prioritized(&mathInlineParser{}, 500)),
	)
	m.Renderer().AddOptions(renderer.WithNodeRenderers(util.Prioritized(&mathRenderer{}, 1000)))
}
