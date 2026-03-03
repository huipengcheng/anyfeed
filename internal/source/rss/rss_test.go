package rss

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/huipeng/anyfeed/internal/config"
)

const testRSS2Feed = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Feed</title>
    <link>https://example.com</link>
    <description>A test RSS feed</description>
    <item>
      <title>Test Article 1</title>
      <link>https://example.com/article1</link>
      <description>Description of article 1</description>
      <pubDate>Mon, 01 Jan 2024 12:00:00 GMT</pubDate>
      <guid>https://example.com/article1</guid>
      <author>Author One</author>
    </item>
    <item>
      <title>Test Article 2</title>
      <link>https://example.com/article2</link>
      <description>Description of article 2</description>
      <pubDate>Tue, 02 Jan 2024 12:00:00 GMT</pubDate>
    </item>
  </channel>
</rss>`

const testAtomFeed = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Atom Test Feed</title>
  <link href="https://example.com"/>
  <entry>
    <title>Atom Article</title>
    <link href="https://example.com/atom-article"/>
    <id>urn:uuid:12345</id>
    <updated>2024-01-03T12:00:00Z</updated>
    <summary>Atom article summary</summary>
    <content type="html">&lt;p&gt;Atom content&lt;/p&gt;</content>
    <author>
      <name>Atom Author</name>
    </author>
  </entry>
</feed>`

func TestRSSSource_FetchRSS2(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write([]byte(testRSS2Feed))
	}))
	defer server.Close()

	cfg := config.FeedConfig{
		Name:     "test-feed",
		Type:     config.FeedTypeRSS,
		URL:      server.URL,
		Interval: time.Minute,
		Tags:     []string{"test"},
		Enabled:  true,
	}

	source := New(cfg)
	entries, err := source.Fetch(context.Background())
	if err != nil {
		t.Fatalf("failed to fetch: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}

	// Check first entry
	entry := entries[0]
	if entry.Title != "Test Article 1" {
		t.Errorf("expected title 'Test Article 1', got '%s'", entry.Title)
	}
	if entry.URL != "https://example.com/article1" {
		t.Errorf("expected URL 'https://example.com/article1', got '%s'", entry.URL)
	}
	if entry.SourceName != "test-feed" {
		t.Errorf("expected source name 'test-feed', got '%s'", entry.SourceName)
	}
	if entry.SourceType != "rss" {
		t.Errorf("expected source type 'rss', got '%s'", entry.SourceType)
	}
	if len(entry.Tags) != 1 || entry.Tags[0] != "test" {
		t.Errorf("expected tags ['test'], got %v", entry.Tags)
	}
}

func TestRSSSource_FetchAtom(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		w.Write([]byte(testAtomFeed))
	}))
	defer server.Close()

	cfg := config.FeedConfig{
		Name:     "atom-feed",
		Type:     config.FeedTypeRSS,
		URL:      server.URL,
		Interval: time.Minute,
		Enabled:  true,
	}

	source := New(cfg)
	entries, err := source.Fetch(context.Background())
	if err != nil {
		t.Fatalf("failed to fetch: %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.Title != "Atom Article" {
		t.Errorf("expected title 'Atom Article', got '%s'", entry.Title)
	}
	if entry.Author != "Atom Author" {
		t.Errorf("expected author 'Atom Author', got '%s'", entry.Author)
	}
	// Atom should prefer content over summary
	if entry.Content != "<p>Atom content</p>" {
		t.Errorf("expected HTML content, got '%s'", entry.Content)
	}
}

func TestRSSSource_Conditional(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		// First request - return full content
		if r.Header.Get("If-None-Match") == "" {
			w.Header().Set("ETag", `"abc123"`)
			w.Header().Set("Content-Type", "application/rss+xml")
			w.Write([]byte(testRSS2Feed))
			return
		}

		// Subsequent requests with matching ETag - return 304
		if r.Header.Get("If-None-Match") == `"abc123"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		w.Write([]byte(testRSS2Feed))
	}))
	defer server.Close()

	cfg := config.FeedConfig{
		Name:     "conditional-feed",
		Type:     config.FeedTypeRSS,
		URL:      server.URL,
		Interval: time.Minute,
		Enabled:  true,
	}

	source := New(cfg)

	// First fetch - should get entries
	entries, err := source.Fetch(context.Background())
	if err != nil {
		t.Fatalf("first fetch failed: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries on first fetch, got %d", len(entries))
	}

	// Second fetch - should get nil (not modified)
	entries, err = source.Fetch(context.Background())
	if err != nil {
		t.Fatalf("second fetch failed: %v", err)
	}
	if entries != nil {
		t.Errorf("expected nil entries on not modified, got %d", len(entries))
	}

	if callCount != 2 {
		t.Errorf("expected 2 server calls, got %d", callCount)
	}
}

func TestRSSSource_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := config.FeedConfig{
		Name:     "error-feed",
		Type:     config.FeedTypeRSS,
		URL:      server.URL,
		Interval: time.Minute,
		Enabled:  true,
	}

	source := New(cfg)
	_, err := source.Fetch(context.Background())
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestRSSSource_InvalidFeed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not a valid feed"))
	}))
	defer server.Close()

	cfg := config.FeedConfig{
		Name:     "invalid-feed",
		Type:     config.FeedTypeRSS,
		URL:      server.URL,
		Interval: time.Minute,
		Enabled:  true,
	}

	source := New(cfg)
	_, err := source.Fetch(context.Background())
	if err == nil {
		t.Error("expected error for invalid feed")
	}
}

func TestRSSSource_Name(t *testing.T) {
	cfg := config.FeedConfig{
		Name: "my-feed",
		Type: config.FeedTypeRSS,
		URL:  "https://example.com/feed.xml",
	}

	source := New(cfg)
	if source.Name() != "my-feed" {
		t.Errorf("expected name 'my-feed', got '%s'", source.Name())
	}
	if source.Type() != "rss" {
		t.Errorf("expected type 'rss', got '%s'", source.Type())
	}
}

func TestRSSSource_Interval(t *testing.T) {
	cfg := config.FeedConfig{
		Name:     "my-feed",
		Type:     config.FeedTypeRSS,
		URL:      "https://example.com/feed.xml",
		Interval: 15 * time.Minute,
	}

	source := New(cfg)
	if source.Interval() != 15*time.Minute {
		t.Errorf("expected interval 15m, got %v", source.Interval())
	}
}
