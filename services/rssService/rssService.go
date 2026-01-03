package rssService

import (
	"time"

	"github.com/gorilla/feeds"
)

// GenerateRSS generates an RSS feed based on the provided articles
func GenerateRSS(title, link, description string, articles []feeds.Item) (string, error) {
	feed := &feeds.Feed{
		Title:       title,
		Link:        &feeds.Link{Href: link},
		Description: description,
		Author:      &feeds.Author{Name: "CoBlog", Email: "support@coblog.com"},
		Created:     time.Now(),
	}

	for _, article := range articles {
		feed.Items = append(feed.Items, &article)
	}

	rss, err := feed.ToRss()
	if err != nil {
		return "", err
	}

	return rss, nil
}
