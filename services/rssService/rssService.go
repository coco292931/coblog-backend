package rssService

import (
	"time"

	"github.com/gorilla/feeds"
)

// FeedMeta 描述 RSS 频道级别的元信息
type FeedMeta struct {
	Title       string
	Link        string
	Description string
	Author      string
	Email       string
	Created     time.Time
}

// GenerateRSS 根据频道元信息和文章条目生成 RSS XML
func GenerateRSS(meta FeedMeta, items []*feeds.Item) (string, error) {
	feed := &feeds.Feed{
		Title:       meta.Title,
		Link:        &feeds.Link{Href: meta.Link},
		Description: meta.Description,
		Author:      &feeds.Author{Name: meta.Author, Email: meta.Email},
		Created:     meta.Created,
	}

	feed.Items = items

	return feed.ToRss()
}
