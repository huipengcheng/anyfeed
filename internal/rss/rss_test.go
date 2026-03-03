package rss

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/huipeng/anyfeed/internal/source"
)

func TestGenerate(t *testing.T) {
	entries := []*source.Entry{
		{
			ID:          "entry1",
			SourceName:  "test-source",
			Title:       "Test Entry 1",
			Content:     "Content of entry 1",
			URL:         "https://example.com/1",
			Author:      "Author One",
			PublishedAt: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			ReceivedAt:  time.Now(),
		},
		{
			ID:          "entry2",
			SourceName:  "test-source",
			Title:       "Test Entry 2",
			Content:     "<p>HTML content</p>",
			URL:         "https://example.com/2",
			PublishedAt: time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
			ReceivedAt:  time.Now(),
		},
	}

	xmlData, err := Generate("Test Feed", "https://example.com", "Test description", entries)
	if err != nil {
		t.Fatalf("failed to generate RSS: %v", err)
	}

	// Check XML declaration
	if !strings.HasPrefix(string(xmlData), "<?xml") {
		t.Error("expected XML declaration")
	}

	// Parse to verify valid XML
	var feed Feed
	if err := xml.Unmarshal(xmlData, &feed); err != nil {
		t.Fatalf("generated invalid XML: %v", err)
	}

	// Verify feed structure
	if feed.Version != "2.0" {
		t.Errorf("expected version 2.0, got %s", feed.Version)
	}
	if feed.Channel.Title != "Test Feed" {
		t.Errorf("expected title 'Test Feed', got '%s'", feed.Channel.Title)
	}
	if feed.Channel.Link != "https://example.com" {
		t.Errorf("expected link 'https://example.com', got '%s'", feed.Channel.Link)
	}
	if feed.Channel.Description != "Test description" {
		t.Errorf("expected description 'Test description', got '%s'", feed.Channel.Description)
	}
	if len(feed.Channel.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(feed.Channel.Items))
	}

	// Verify first item
	item := feed.Channel.Items[0]
	if item.Title != "Test Entry 1" {
		t.Errorf("expected item title 'Test Entry 1', got '%s'", item.Title)
	}
	if item.Link != "https://example.com/1" {
		t.Errorf("expected item link 'https://example.com/1', got '%s'", item.Link)
	}
	if item.Author != "Author One" {
		t.Errorf("expected author 'Author One', got '%s'", item.Author)
	}
	if item.GUID.Value != "entry1" {
		t.Errorf("expected GUID 'entry1', got '%s'", item.GUID.Value)
	}
}

func TestGenerateDefaults(t *testing.T) {
	xmlData, err := Generate("", "", "", nil)
	if err != nil {
		t.Fatalf("failed to generate RSS: %v", err)
	}

	var feed Feed
	if err := xml.Unmarshal(xmlData, &feed); err != nil {
		t.Fatalf("generated invalid XML: %v", err)
	}

	if feed.Channel.Title != "Anyfeed" {
		t.Errorf("expected default title 'Anyfeed', got '%s'", feed.Channel.Title)
	}
	if feed.Channel.Description != "RSS feed aggregated by Anyfeed" {
		t.Errorf("expected default description, got '%s'", feed.Channel.Description)
	}
	if feed.Channel.Generator != "Anyfeed/1.0" {
		t.Errorf("expected generator 'Anyfeed/1.0', got '%s'", feed.Channel.Generator)
	}
}

func TestFormatRFC822(t *testing.T) {
	tm := time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC)
	formatted := FormatRFC822(tm)

	// RFC1123 format: "Mon, 02 Jan 2006 15:04:05 MST"
	expected := "Mon, 15 Jan 2024 14:30:00 UTC"
	if formatted != expected {
		t.Errorf("expected '%s', got '%s'", expected, formatted)
	}
}

func TestGenerateEmptyEntries(t *testing.T) {
	xmlData, err := Generate("Empty Feed", "https://example.com", "No entries", []*source.Entry{})
	if err != nil {
		t.Fatalf("failed to generate RSS: %v", err)
	}

	var feed Feed
	if err := xml.Unmarshal(xmlData, &feed); err != nil {
		t.Fatalf("generated invalid XML: %v", err)
	}

	if len(feed.Channel.Items) != 0 {
		t.Errorf("expected 0 items, got %d", len(feed.Channel.Items))
	}
}

func TestGenerateHTMLContent(t *testing.T) {
	entries := []*source.Entry{
		{
			ID:          "html-entry",
			Title:       "HTML Entry",
			Content:     "<p>This is <strong>HTML</strong> content & special chars</p>",
			PublishedAt: time.Now(),
		},
	}

	xmlData, err := Generate("Test", "", "", entries)
	if err != nil {
		t.Fatalf("failed to generate RSS: %v", err)
	}

	// Should be valid XML with properly escaped content
	var feed Feed
	if err := xml.Unmarshal(xmlData, &feed); err != nil {
		t.Fatalf("generated invalid XML: %v", err)
	}

	// Content should be preserved (XML escaping is handled by encoder)
	if !strings.Contains(feed.Channel.Items[0].Description, "HTML") {
		t.Error("expected HTML content to be preserved")
	}
}
