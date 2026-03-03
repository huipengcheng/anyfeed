// Package web provides web page change detection source.
package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/huipeng/anyfeed/internal/config"
	"github.com/huipeng/anyfeed/internal/source"
)

// StateStore defines the interface for persisting web state.
type StateStore interface {
	SaveWebState(ctx context.Context, sourceName, hash, content string) error
	GetWebState(ctx context.Context, sourceName string) (hash string, content string, err error)
}

// Source implements web page change detection (Active).
type Source struct {
	config   config.FeedConfig
	client   *http.Client
	handler  source.EntryHandler
	store    StateStore
	lastHash string
	stopCh   chan struct{}
	mu       sync.RWMutex
}

// New creates a new Web source.
func New(cfg config.FeedConfig, store StateStore) *Source {
	return &Source{
		config: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		store:  store,
		stopCh: make(chan struct{}),
	}
}

// Name returns the source name.
func (s *Source) Name() string {
	return s.config.Name
}

// Type returns the source type.
func (s *Source) Type() string {
	return string(config.FeedTypeWeb)
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

// Start initializes the source by loading previous state.
func (s *Source) Start(ctx context.Context) error {
	// Load previous state from store
	if s.store != nil {
		hash, _, err := s.store.GetWebState(ctx, s.config.Name)
		if err != nil {
			slog.Warn("failed to load web state", "source", s.config.Name, "error", err)
		} else {
			s.mu.Lock()
			s.lastHash = hash
			s.mu.Unlock()
			slog.Debug("loaded web state", "source", s.config.Name, "hash", hash)
		}
	}

	slog.Info("Web source started", "name", s.config.Name, "url", s.config.URL, "selector", s.config.Selector)
	return nil
}

// Stop stops the source.
func (s *Source) Stop() error {
	slog.Info("Web source stopped", "name", s.config.Name)
	return nil
}

// Fetch retrieves new entries by checking for page changes.
func (s *Source) Fetch(ctx context.Context) ([]*source.Entry, error) {
	// Fetch the page
	req, err := http.NewRequestWithContext(ctx, "GET", s.config.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Anyfeed/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse HTML and extract content
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Extract content using CSS selector
	content, err := s.extractContent(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to extract content: %w", err)
	}

	// Check for changes
	changed, isBaseline, newHash := s.detectChanges(content)

	// Always update state (including baseline)
	s.mu.Lock()
	s.lastHash = newHash
	s.mu.Unlock()

	// Persist state
	if s.store != nil {
		if err := s.store.SaveWebState(ctx, s.config.Name, newHash, content); err != nil {
			slog.Warn("failed to save web state", "source", s.config.Name, "error", err)
		}
	}

	// Don't return entry for baseline (first fetch) or if no change
	if isBaseline || !changed {
		slog.Debug("no changes detected", "source", s.config.Name, "baseline", isBaseline)
		return nil, nil
	}

	// Create entry for the change
	entry := source.NewEntry(
		s.config.Name,
		string(config.FeedTypeWeb),
		fmt.Sprintf("Update: %s", s.config.Name),
		content,
		s.config.URL,
		"",
		time.Now(),
		s.config.Tags,
	)
	entry.Metadata["hash"] = newHash
	entry.Metadata["selector"] = s.config.Selector

	slog.Info("web page changed", "source", s.config.Name, "hash", newHash)
	return []*source.Entry{entry}, nil
}

// extractContent extracts content using the configured CSS selector.
func (s *Source) extractContent(doc *goquery.Document) (string, error) {
	selector := s.config.Selector
	if selector == "" {
		selector = "body"
	}

	var contents []string
	doc.Find(selector).Each(func(i int, sel *goquery.Selection) {
		html, err := sel.Html()
		if err == nil {
			contents = append(contents, strings.TrimSpace(html))
		}
	})

	if len(contents) == 0 {
		// Try getting text instead of HTML
		doc.Find(selector).Each(func(i int, sel *goquery.Selection) {
			text := strings.TrimSpace(sel.Text())
			if text != "" {
				contents = append(contents, text)
			}
		})
	}

	if len(contents) == 0 {
		return "", fmt.Errorf("no content found with selector '%s'", selector)
	}

	return strings.Join(contents, "\n"), nil
}

// detectChanges compares content hash and returns (changed, isBaseline, newHash).
func (s *Source) detectChanges(content string) (bool, bool, string) {
	hash := sha256.Sum256([]byte(content))
	newHash := hex.EncodeToString(hash[:])

	s.mu.RLock()
	lastHash := s.lastHash
	s.mu.RUnlock()

	// First time (no previous hash) - this is baseline
	if lastHash == "" {
		slog.Debug("first fetch, recording baseline", "source", s.config.Name)
		return false, true, newHash
	}

	return newHash != lastHash, false, newHash
}
