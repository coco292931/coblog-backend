package rssService

import (
	"strings"
	"testing"
	"time"
)

var testTime = time.Date(2026, 7, 12, 5, 18, 3, 0, time.UTC)

func newTestMeta() FeedMeta {
	return FeedMeta{
		Title:       "coco的避风港",
		Link:        "https://blog.coco-29.wang",
		Description: "描述 & 更多",
		Author:      "coco",
		Email:       "coco@coco-29.wang",
		Created:     testTime,
		SelfURL:     "https://api.coco-29.wang/api/rss",
		Language:    "zh-CN",
	}
}

func TestGenerateRSSBasicStructure(t *testing.T) {
	feed, err := GenerateRSS(newTestMeta(), []*Item{{
		Title:       "标题",
		Link:        "https://blog.coco-29.wang/articles/1",
		Description: "摘要",
		Content:     "<p>正文</p>",
		ID:          "https://blog.coco-29.wang/articles/1",
		Created:     testTime,
	}})
	if err != nil {
		t.Fatalf("生成 RSS 失败: %v", err)
	}

	for _, want := range []string{
		`<?xml version="1.0" encoding="UTF-8"?>`,
		`<rss version="2.0"`,
		`<language>zh-CN</language>`,
		`<atom:link href="https://api.coco-29.wang/api/rss" rel="self" type="application/rss+xml">`,
		`<lastBuildDate>Sun, 12 Jul 2026 05:18:03 +0000</lastBuildDate>`,
		`<guid isPermaLink="true">https://blog.coco-29.wang/articles/1</guid>`,
		`<dc:creator>coco</dc:creator>`,
	} {
		if !strings.Contains(feed, want) {
			t.Errorf("输出缺少 %s\n---\n%s", want, feed)
		}
	}
}

func TestGenerateRSSSkipsItemWithoutLink(t *testing.T) {
	feed, err := GenerateRSS(newTestMeta(), []*Item{
		{Title: "无链接文章", Created: testTime},
		nil,
		{
			Title:   "正常文章",
			Link:    "https://blog.coco-29.wang/articles/2",
			Created: testTime,
		},
	})
	if err != nil {
		t.Fatalf("生成 RSS 失败: %v", err)
	}

	if n := strings.Count(feed, "<item>"); n != 1 {
		t.Fatalf("应当只输出 1 个条目，实际 %d 个\n%s", n, feed)
	}
	if strings.Contains(feed, "无链接文章") {
		t.Errorf("无链接条目不应出现在输出中\n%s", feed)
	}
}

func TestGenerateRSSOmitsEmptyFields(t *testing.T) {
	feed, err := GenerateRSS(newTestMeta(), []*Item{{
		Title:   "只有标题",
		Link:    "https://blog.coco-29.wang/articles/3",
		Created: testTime,
	}})
	if err != nil {
		t.Fatalf("生成 RSS 失败: %v", err)
	}

	for _, unwanted := range []string{"<description></description>", "<author></author>", "<dc:creator></dc:creator>", "<guid isPermaLink=\"true\"></guid>"} {
		if strings.Contains(feed, unwanted) {
			t.Errorf("不应输出空标签 %s\n%s", unwanted, feed)
		}
	}
}

func TestGenerateRSSEmptyItemList(t *testing.T) {
	feed, err := GenerateRSS(newTestMeta(), nil)
	if err != nil {
		t.Fatalf("生成 RSS 失败: %v", err)
	}
	if strings.Contains(feed, "<item>") {
		t.Errorf("无文章时不应输出条目\n%s", feed)
	}
	// 频道必填字段仍需存在
	if !strings.Contains(feed, "<title>coco的避风港</title>") {
		t.Errorf("频道标题缺失\n%s", feed)
	}
}

func TestGenerateRSSCdataEscaping(t *testing.T) {
	feed, err := GenerateRSS(newTestMeta(), []*Item{{
		Title:       "转义测试",
		Link:        "https://blog.coco-29.wang/articles/4",
		Description: `含 <b>html</b> & 与 ]]> 结尾`,
		Content:     "a ]]> b",
		Created:     testTime,
	}})
	if err != nil {
		t.Fatalf("生成 RSS 失败: %v", err)
	}

	// ]]> 必须被切断，否则会提前结束 CDATA 段造成非法 XML
	if strings.Contains(feed, "a ]]> b") {
		t.Errorf("]]> 未被正确切断\n%s", feed)
	}
	if !strings.Contains(feed, "]]&gt;") {
		t.Errorf("]]> 应当被转义为 ]]> 的等价形式\n%s", feed)
	}
	// CDATA 内的 HTML 不需要转义，保留原文
	if !strings.Contains(feed, `<b>html</b> & 与`) {
		t.Errorf("CDATA 内的 HTML 应保持原样\n%s", feed)
	}
}

func TestGenerateRSSAtomUpdatedOnlyWhenEdited(t *testing.T) {
	edited := testTime.Add(48 * time.Hour)

	feed, err := GenerateRSS(newTestMeta(), []*Item{{
		Title:   "已编辑",
		Link:    "https://blog.coco-29.wang/articles/5",
		Created: testTime,
		Updated: edited,
	}})
	if err != nil {
		t.Fatalf("生成 RSS 失败: %v", err)
	}
	if !strings.Contains(feed, "<atom:updated>Tue, 14 Jul 2026 05:18:03 +0000</atom:updated>") {
		t.Errorf("被编辑的文章应当输出 atom:updated\n%s", feed)
	}
	// pubDate 保持创建时间，避免编辑旧文章导致其在订阅列表中冒泡
	if !strings.Contains(feed, "<pubDate>Sun, 12 Jul 2026 05:18:03 +0000</pubDate>") {
		t.Errorf("pubDate 应为创建时间\n%s", feed)
	}

	// 未编辑（Updated == Created）时不输出 atom:updated
	feed, err = GenerateRSS(newTestMeta(), []*Item{{
		Title:   "未编辑",
		Link:    "https://blog.coco-29.wang/articles/6",
		Created: testTime,
		Updated: testTime,
	}})
	if err != nil {
		t.Fatalf("生成 RSS 失败: %v", err)
	}
	if strings.Contains(feed, "atom:updated") {
		t.Errorf("未编辑的文章不应输出 atom:updated\n%s", feed)
	}
}

func TestGenerateRSSEnclosureAndCategories(t *testing.T) {
	feed, err := GenerateRSS(newTestMeta(), []*Item{{
		Title:      "带封面",
		Link:       "https://blog.coco-29.wang/articles/7",
		Created:    testTime,
		Categories: []string{"tech", " tech ", "", "music"},
		Enclosure: &Enclosure{
			URL:    "https://api.coco-29.wang/images/cover.png",
			Type:   "image/png",
			Length: "0",
		},
	}})
	if err != nil {
		t.Fatalf("生成 RSS 失败: %v", err)
	}

	if !strings.Contains(feed, `<enclosure url="https://api.coco-29.wang/images/cover.png" type="image/png" length="0">`) {
		t.Errorf("应输出 enclosure\n%s", feed)
	}
	if !strings.Contains(feed, `<media:thumbnail url="https://api.coco-29.wang/images/cover.png">`) {
		t.Errorf("应输出 media:thumbnail\n%s", feed)
	}
	// 分类去重去空，保留两个
	if n := strings.Count(feed, "<category>"); n != 2 {
		t.Errorf("应当输出 2 个 category，实际 %d 个\n%s", n, feed)
	}
}

func TestGenerateRSSSelfURLOmittedWhenEmpty(t *testing.T) {
	meta := newTestMeta()
	meta.SelfURL = ""

	feed, err := GenerateRSS(meta, nil)
	if err != nil {
		t.Fatalf("生成 RSS 失败: %v", err)
	}
	if strings.Contains(feed, "atom:link") {
		t.Errorf("未提供 SelfURL 时不应输出 atom:link\n%s", feed)
	}
	// 命名空间声明保留，不影响合法性
	if !strings.Contains(feed, `xmlns:atom="http://www.w3.org/2005/Atom"`) {
		t.Errorf("命名空间声明缺失\n%s", feed)
	}
}
