package markdownService

import (
	"bytes"

	"github.com/yuin/goldmark"
)

// ParseMarkdownToHTML parses a Markdown string and returns the HTML string
func ParseMarkdownToHTML(markdown string) (string, error) {
	var buf bytes.Buffer
	md := goldmark.New()
	if err := md.Convert([]byte(markdown), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}
