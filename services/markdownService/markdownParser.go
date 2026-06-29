package markdownService

import (
	"bytes"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

// ParseMarkdownToHTML parses a Markdown string and returns the HTML string
func ParseMarkdownToHTML(markdown string) (string, error) {
	var buf bytes.Buffer
	md := goldmark.New(goldmark.WithExtensions(extension.GFM))

	if err := md.Convert([]byte(markdown), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}
