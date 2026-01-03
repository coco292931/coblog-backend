package markdownController

import (
	"coblog-backend/services/markdownService"
	"net/http"

	"github.com/gin-gonic/gin"
)

// MarkdownToHTMLHandler handles the conversion of Markdown to HTML
func MarkdownToHTMLHandler(c *gin.Context) {
	// Get Markdown content from request body
	var requestBody struct {
		Markdown string `json:"markdown"`
	}
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Convert Markdown to HTML
	html, err := markdownService.ParseMarkdownToHTML(requestBody.Markdown)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse Markdown"})
		return
	}

	// Return the HTML
	c.JSON(http.StatusOK, gin.H{"html": html})
}
