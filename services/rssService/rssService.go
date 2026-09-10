package rssService

import (
	"encoding/xml"
	"fmt"
	"strings"
	"time"
)

// FeedMeta 描述 RSS 频道级别的元信息
type FeedMeta struct {
	Title       string
	Link        string
	Description string
	Author      string
	Email       string
	Created     time.Time
	SelfURL     string
	Language    string
}

// Item 描述 RSS 2.0 的文章条目。
type Item struct {
	Title       string
	Link        string
	Description string
	Content     string
	ID          string
	Author      string
	Email       string
	Created     time.Time
	Updated     time.Time
	Categories  []string
	Enclosure   *Enclosure
}

// Enclosure 描述 RSS 2.0 附件，常用于封面图/播客等媒体资源。
type Enclosure struct {
	URL    string
	Type   string
	Length string
}

type rssXML struct {
	XMLName      xml.Name   `xml:"rss"`
	Version      string     `xml:"version,attr"`
	XMLNSAtom    string     `xml:"xmlns:atom,attr"`
	XMLNSContent string     `xml:"xmlns:content,attr"`
	XMLNSDC      string     `xml:"xmlns:dc,attr"`
	XMLNSMedia   string     `xml:"xmlns:media,attr"`
	Channel      channelXML `xml:"channel"`
}

type channelXML struct {
	Title          string       `xml:"title"`
	Link           string       `xml:"link"`
	Description    cdata        `xml:"description"`
	Language       string       `xml:"language,omitempty"`
	LastBuildDate  string       `xml:"lastBuildDate,omitempty"`
	PubDate        string       `xml:"pubDate,omitempty"`
	Generator      string       `xml:"generator,omitempty"`
	TTL            int          `xml:"ttl,omitempty"`
	ManagingEditor string       `xml:"managingEditor,omitempty"`
	WebMaster      string       `xml:"webMaster,omitempty"`
	AtomLink       *atomLinkXML `xml:"atom:link,omitempty"`
	Items          []itemXML    `xml:"item"`
}

type atomLinkXML struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
}

type itemXML struct {
	Title       string        `xml:"title,omitempty"`
	Link        string        `xml:"link,omitempty"`
	GUID        *guidXML      `xml:"guid,omitempty"`
	Description *cdata        `xml:"description,omitempty"`
	Content     *cdata        `xml:"content:encoded,omitempty"`
	PubDate     string        `xml:"pubDate,omitempty"`
	Updated     string        `xml:"atom:updated,omitempty"`
	Author      string        `xml:"author,omitempty"`
	Creator     string        `xml:"dc:creator,omitempty"`
	Categories  []string      `xml:"category,omitempty"`
	Enclosure   *enclosureXML `xml:"enclosure,omitempty"`
	Thumbnail   *thumbnailXML `xml:"media:thumbnail,omitempty"`
}

type guidXML struct {
	IsPermaLink bool   `xml:"isPermaLink,attr"`
	Value       string `xml:",chardata"`
}

type enclosureXML struct {
	URL    string `xml:"url,attr"`
	Type   string `xml:"type,attr,omitempty"`
	Length string `xml:"length,attr"`
}

type thumbnailXML struct {
	URL string `xml:"url,attr"`
}

type cdata string

func (c cdata) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	for i, part := range strings.Split(string(c), "]]>") {
		if err := e.EncodeToken(xml.Directive([]byte("[CDATA[" + part + "]]"))); err != nil {
			return err
		}
		if i < len(strings.Split(string(c), "]]>"))-1 {
			if err := e.EncodeToken(xml.CharData("]]>")); err != nil {
				return err
			}
		}
	}
	return e.EncodeToken(start.End())
}

// GenerateRSS 根据频道元信息和文章条目生成标准 RSS 2.0 XML。
func GenerateRSS(meta FeedMeta, items []*Item) (string, error) {
	buildDate := formatRSSDate(meta.Created)
	language := strings.TrimSpace(meta.Language)
	if language == "" {
		language = "zh-CN"
	}

	channel := channelXML{
		Title:          meta.Title,
		Link:           meta.Link,
		Description:    cdata(meta.Description),
		Language:       language,
		LastBuildDate:  buildDate,
		PubDate:        buildDate,
		Generator:      "CoBlog RSS Generator",
		TTL:            60,
		ManagingEditor: rssPerson(meta.Email, meta.Author),
		WebMaster:      rssPerson(meta.Email, meta.Author),
	}
	if meta.SelfURL != "" {
		channel.AtomLink = &atomLinkXML{
			Href: meta.SelfURL,
			Rel:  "self",
			Type: "application/rss+xml",
		}
	}

	for _, item := range items {
		if item == nil {
			continue
		}
		// 缺少链接的条目既没有订阅价值，也会产出空的 <link>/<guid>，直接跳过
		if strings.TrimSpace(item.Link) == "" {
			continue
		}
		channel.Items = append(channel.Items, toItemXML(meta, item))
	}

	doc := rssXML{
		Version:      "2.0",
		XMLNSAtom:    "http://www.w3.org/2005/Atom",
		XMLNSContent: "http://purl.org/rss/1.0/modules/content/",
		XMLNSDC:      "http://purl.org/dc/elements/1.1/",
		XMLNSMedia:   "http://search.yahoo.com/mrss/",
		Channel:      channel,
	}

	body, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", err
	}
	return xml.Header + string(body) + "\n", nil
}

func toItemXML(meta FeedMeta, item *Item) itemXML {
	// pubDate 固定用创建时间：若改用更新时间，编辑旧文章会让其冒泡到订阅列表顶部
	result := itemXML{
		Title:      item.Title,
		Link:       item.Link,
		PubDate:    formatRSSDate(item.Created),
		Author:     rssPerson(firstNonEmpty(item.Email, meta.Email), firstNonEmpty(item.Author, meta.Author)),
		Creator:    firstNonEmpty(item.Author, meta.Author),
		Categories: uniqueNonEmpty(item.Categories),
	}

	// guid 为空会违反 RSS 规范，仅在确有唯一标识时才输出
	if id := firstNonEmpty(item.ID, item.Link); id != "" {
		result.GUID = &guidXML{IsPermaLink: true, Value: id}
	}
	// 仅在文章确实被编辑过时补充更新时间，避免读者误判为重新发布
	if !item.Created.IsZero() && item.Updated.After(item.Created) {
		result.Updated = formatRSSDate(item.Updated)
	}

	if description := strings.TrimSpace(item.Description); description != "" {
		value := cdata(description)
		result.Description = &value
	}
	if content := strings.TrimSpace(item.Content); content != "" {
		value := cdata(content)
		result.Content = &value
	}
	if item.Enclosure != nil && item.Enclosure.URL != "" {
		length := item.Enclosure.Length
		if length == "" {
			length = "0"
		}
		result.Enclosure = &enclosureXML{
			URL:    item.Enclosure.URL,
			Type:   item.Enclosure.Type,
			Length: length,
		}
		result.Thumbnail = &thumbnailXML{URL: item.Enclosure.URL}
	}

	return result
}

func formatRSSDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC1123Z)
}

func rssPerson(email, name string) string {
	email = strings.TrimSpace(email)
	name = strings.TrimSpace(name)
	if email != "" && name != "" {
		return fmt.Sprintf("%s (%s)", email, name)
	}
	return email
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func uniqueNonEmpty(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
