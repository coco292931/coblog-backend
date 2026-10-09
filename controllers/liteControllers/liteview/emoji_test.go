package liteview

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 这类设备普遍没有 emoji 字体，这些字符会渲染成一个空方框 —— 比不显示还难看。
//
// 扫描范围是整个 liteview 目录（模板 / 样式 / Go 源码），注释里也不放：
// 注释本身不渲染，但同一个文件里混着两套记号很容易看漏，
// 复制粘贴时也容易把注释里的 emoji 带进要渲染的那行。
//
// 黑名单用码点写，不用字面量 —— 否则测试文件自己就会被扫出来。
func TestNoEmojiAnywhere(t *testing.T) {
	banned := []int{
		0x26A0,  // warning sign
		0x2705,  // check mark button
		0x274C,  // cross mark
		0x2728,  // sparkles
		0x1F389, // party popper
		0x1F54A, // dove
		0x1F375, // teacup
		0x1F4CA, // bar chart
		0x1F441, // eye
		0x1F44D, // thumbs up
		0x1F4AC, // speech balloon
		0x1F517, // link
		0x1F4D6, // open book
		0x1F449, // pointing right
		0x1F31F, // glowing star
		0x1F4CC, // pushpin
		0x1F50D, // magnifying glass
		0x231B,  // hourglass
	}

	var files []string
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".html", ".css", ".go":
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("遍历目录失败: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("没有扫到任何文件，路径可能变了")
	}

	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("读 %s 失败: %v", f, err)
		}
		content := string(b)
		for _, code := range banned {
			if strings.Contains(content, string(rune(code))) {
				t.Errorf("%s 含 emoji U+%04X —— 老设备会把它渲染成方框", f, code)
			}
		}
	}
}
