// Package rss provides RSS XML generation.
package rss

import (
	"encoding/xml"
	"time"

	"github.com/huipeng/anyfeed/internal/source"
)

// Feed represents an RSS 2.0 feed.
type Feed struct {
	XMLName xml.Name `xml:"rss"`
	Version string   `xml:"version,attr"`
	Channel Channel  `xml:"channel"`
}

// Channel represents an RSS channel.
type Channel struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate,omitempty"`
	LastBuild   string `xml:"lastBuildDate"`
	Generator   string `xml:"generator"`
	Items       []Item `xml:"item"`
}

// Item represents an RSS item.
type Item struct {
	Title       string `xml:"title"`
	Link        string `xml:"link,omitempty"`
	Description string `xml:"description"`
	Author      string `xml:"author,omitempty"`
	GUID        GUID   `xml:"guid"`
	PubDate     string `xml:"pubDate"`
	Source      string `xml:"source,omitempty"`
}

// GUID represents an RSS guid element.
type GUID struct {
	IsPermaLink bool   `xml:"isPermaLink,attr"`
	Value       string `xml:",chardata"`
}

// Generate creates RSS XML from entries.
func Generate(title, link, description string, entries []*source.Entry) ([]byte, error) {
	if title == "" {
		title = "Anyfeed"
	}
	if description == "" {
		description = "RSS feed aggregated by Anyfeed"
	}

	items := make([]Item, 0, len(entries))
	var latestPubDate time.Time

	for _, entry := range entries {
		if entry.PublishedAt.After(latestPubDate) {
			latestPubDate = entry.PublishedAt
		}

		item := Item{
			Title:       entry.Title,
			Link:        entry.URL,
			Description: entry.Content,
			Author:      entry.Author,
			GUID: GUID{
				IsPermaLink: entry.URL != "",
				Value:       entry.ID,
			},
			PubDate: FormatRFC822(entry.PublishedAt),
			Source:  entry.SourceName,
		}
		items = append(items, item)
	}

	// Set pubDate to latest entry's publish time
	pubDate := ""
	if !latestPubDate.IsZero() {
		pubDate = FormatRFC822(latestPubDate)
	}

	feed := Feed{
		Version: "2.0",
		Channel: Channel{
			Title:       title,
			Link:        link,
			Description: description,
			PubDate:     pubDate,
			LastBuild:   FormatRFC822(time.Now()),
			Generator:   "Anyfeed/1.0",
			Items:       items,
		},
	}

	output, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		return nil, err
	}

	// Add XML declaration
	return append([]byte(xml.Header), output...), nil
}

// FormatRFC822 formats time in RFC822 format for RSS.
// RFC822 format: "Mon, 02 Jan 2006 15:04:05 MST"
func FormatRFC822(t time.Time) string {
	return t.UTC().Format(time.RFC1123)
}
