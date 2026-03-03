// Package source provides data source interfaces and types.
package source

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// Entry represents a normalized content entry from any source.
type Entry struct {
	ID          string            `json:"id"`           // Unique identifier (hash of content)
	SourceName  string            `json:"source_name"`  // Source feed name
	SourceType  string            `json:"source_type"`  // Source type (rss, email, web)
	Title       string            `json:"title"`        // Entry title
	Content     string            `json:"content"`      // Entry content (HTML)
	URL         string            `json:"url"`          // Original URL if available
	Author      string            `json:"author"`       // Author name
	PublishedAt time.Time         `json:"published_at"` // Publication time
	ReceivedAt  time.Time         `json:"received_at"`  // When we received this
	Metadata    map[string]string `json:"metadata"`     // Additional metadata
	Tags        []string          `json:"tags"`         // Tags from source configuration
}

// GenerateID generates a unique ID for an entry based on its content.
// Uses SHA256 hash of source name, title, URL, and published time.
func GenerateID(sourceName, title, url string, publishedAt time.Time) string {
	data := sourceName + "|" + title + "|" + url + "|" + publishedAt.UTC().Format(time.RFC3339)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// NewEntry creates a new Entry with generated ID and current receive time.
func NewEntry(sourceName, sourceType, title, content, url, author string, publishedAt time.Time, tags []string) *Entry {
	if publishedAt.IsZero() {
		publishedAt = time.Now()
	}
	
	return &Entry{
		ID:          GenerateID(sourceName, title, url, publishedAt),
		SourceName:  sourceName,
		SourceType:  sourceType,
		Title:       title,
		Content:     content,
		URL:         url,
		Author:      author,
		PublishedAt: publishedAt,
		ReceivedAt:  time.Now(),
		Tags:        tags,
		Metadata:    make(map[string]string),
	}
}
