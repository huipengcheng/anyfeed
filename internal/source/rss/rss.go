// Package rss provides RSS/Atom feed fetching source.
package rss

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/huipeng/anyfeed/internal/config"
	"github.com/huipeng/anyfeed/internal/source"
	"github.com/mmcdole/gofeed"
)

// Source implements RSS/Atom feed fetching (Active).
type Source struct {
	config   config.FeedConfig
	parser   *gofeed.Parser
	client   *http.Client
	handler  source.EntryHandler
	lastETag string
	lastMod  string
	stopCh   chan struct{}
	mu       sync.RWMutex
}

// New creates a new RSS source.
func New(cfg config.FeedConfig) *Source {
	return &Source{
		config: cfg,
		parser: gofeed.NewParser(),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		stopCh: make(chan struct{}),
	}
}

// Name returns the source name.
func (s *Source) Name() string {
	return s.config.Name
}

// Type returns the source type.
func (s *Source) Type() string {
	return string(config.FeedTypeRSS)
}

// Interval returns the fetch interval.
func (s *Source) Interval() time.Duration {
	return s.config.Interval
}

// SetHandler sets the entry handler.
func (s *Source) SetHandler(handler source.EntryHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handler = handler
}

// Fetch retrieves new entries from the RSS feed.
func (s *Source) Fetch(ctx context.Context) ([]*source.Entry, error) {
	s.mu.RLock()
	lastETag := s.lastETag
	lastMod := s.lastMod
	s.mu.RUnlock()

	// Create request with conditional headers
	req, err := http.NewRequestWithContext(ctx, "GET", s.config.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set conditional headers to reduce bandwidth
	if lastETag != "" {
		req.Header.Set("If-None-Match", lastETag)
	}
	if lastMod != "" {
		req.Header.Set("If-Modified-Since", lastMod)
	}
	req.Header.Set("User-Agent", "Anyfeed/1.0")

	// Make request
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch feed: %w", err)
	}
	defer resp.Body.Close()

	// Handle 304 Not Modified
	if resp.StatusCode == http.StatusNotModified {
		slog.Debug("feed not modified", "source", s.config.Name)
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Update cache headers
	s.mu.Lock()
	s.lastETag = resp.Header.Get("ETag")
	s.lastMod = resp.Header.Get("Last-Modified")
	s.mu.Unlock()

	// Parse feed
	feed, err := s.parser.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse feed: %w", err)
	}

	// Convert feed items to entries
	entries := make([]*source.Entry, 0, len(feed.Items))
	for _, item := range feed.Items {
		entry := s.itemToEntry(item)
		entries = append(entries, entry)
	}

	slog.Debug("fetched feed", "source", s.config.Name, "items", len(entries))
	return entries, nil
}

// itemToEntry converts a gofeed.Item to source.Entry.
func (s *Source) itemToEntry(item *gofeed.Item) *source.Entry {
	var publishedAt time.Time
	if item.PublishedParsed != nil {
		publishedAt = *item.PublishedParsed
	} else if item.UpdatedParsed != nil {
		publishedAt = *item.UpdatedParsed
	} else {
		publishedAt = time.Now()
	}

	var author string
	if item.Author != nil {
		author = item.Author.Name
	}
	if author == "" && len(item.Authors) > 0 {
		author = item.Authors[0].Name
	}

	// Get content
	content := item.Content
	if content == "" {
		content = item.Description
	}

	// Get URL - prefer Link, fallback to GUID if it's a URL
	url := item.Link
	if url == "" && item.GUID != "" && (len(item.GUID) > 4 && item.GUID[:4] == "http") {
		url = item.GUID
	}

	entry := source.NewEntry(
		s.config.Name,
		string(config.FeedTypeRSS),
		item.Title,
		content,
		url,
		author,
		publishedAt,
		s.config.Tags,
	)

	// Add metadata
	if item.GUID != "" {
		entry.Metadata["guid"] = item.GUID
	}
	if len(item.Categories) > 0 {
		entry.Metadata["categories"] = item.Categories[0]
	}

	return entry
}

// Start starts the source (no-op for active sources, handled by manager).
func (s *Source) Start(ctx context.Context) error {
	slog.Info("RSS source started", "name", s.config.Name, "url", s.config.URL)
	return nil
}

// Stop stops the source.
func (s *Source) Stop() error {
	slog.Info("RSS source stopped", "name", s.config.Name)
	return nil
}
