package rssController

import (
	"net/http"
	"coblog-backend/services/rssService"

	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/feeds"
)

// GenerateRSSHandler handles the generation of RSS feeds
func GenerateRSSHandler(c *gin.Context) {
	// Example articles (replace with database query results)
	articles := []feeds.Item{
		{
			Title:       "First Article",
			Link:        &feeds.Link{Href: "https://example.com/articles/1"},
			Description: "This is the first article.",
			Author:      &feeds.Author{Name: "Author 1"},
			Created:     time.Now(),
		},
		{
			Title:       "Second Article",
			Link:        &feeds.Link{Href: "https://example.com/articles/2"},
			Description: "This is the second article.",
			Author:      &feeds.Author{Name: "Author 2"},
			Created:     time.Now(),
		},
	}

	rss, err := rssService.GenerateRSS("CoBlog RSS Feed", "https://example.com/rss", "Latest articles from CoBlog", articles)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate RSS"})
		return
	}

	c.Data(http.StatusOK, "application/rss+xml", []byte(rss))
}
