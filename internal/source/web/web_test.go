package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/huipeng/anyfeed/internal/config"
)

// mockStateStore implements StateStore for testing.
type mockStateStore struct {
	mu      sync.RWMutex
	states  map[string]struct{ hash, content string }
}

func newMockStateStore() *mockStateStore {
	return &mockStateStore{
		states: make(map[string]struct{ hash, content string }),
	}
}

func (m *mockStateStore) SaveWebState(ctx context.Context, sourceName, hash, content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.states[sourceName] = struct{ hash, content string }{hash, content}
	return nil
}

func (m *mockStateStore) GetWebState(ctx context.Context, sourceName string) (string, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	state, ok := m.states[sourceName]
	if !ok {
		return "", "", nil
	}
	return state.hash, state.content, nil
}

const testHTMLPage = `<!DOCTYPE html>
<html>
<head><title>Test Page</title></head>
<body>
	<div class="content">
		<h1>Hello World</h1>
		<p>This is test content.</p>
	</div>
	<div class="sidebar">
		<p>Sidebar content</p>
	</div>
</body>
</html>`

const testHTMLPageChanged = `<!DOCTYPE html>
<html>
<head><title>Test Page</title></head>
<body>
	<div class="content">
		<h1>Hello World</h1>
		<p>This is UPDATED content.</p>
	</div>
	<div class="sidebar">
		<p>Sidebar content</p>
	</div>
</body>
</html>`

func TestWebSource_Fetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(testHTMLPage))
	}))
	defer server.Close()

	cfg := config.FeedConfig{
		Name:     "test-web",
		Type:     config.FeedTypeWeb,
		URL:      server.URL,
		Selector: ".content",
		Interval: time.Minute,
		Tags:     []string{"test"},
		Enabled:  true,
	}

	store := newMockStateStore()
	src := New(cfg, store)
	
	if err := src.Start(context.Background()); err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	// First fetch - should record baseline, no entry returned
	entries, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatalf("first fetch failed: %v", err)
	}
	if entries != nil {
		t.Error("expected nil entries on first fetch (baseline)")
	}

	// Verify state was saved
	hash, content, err := store.GetWebState(context.Background(), "test-web")
	if err != nil {
		t.Fatalf("failed to get state: %v", err)
	}
	if hash == "" {
		t.Error("expected hash to be saved")
	}
	if content == "" {
		t.Error("expected content to be saved")
	}
}

func TestWebSource_ChangeDetection(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "text/html")
		if callCount == 1 {
			w.Write([]byte(testHTMLPage))
		} else {
			w.Write([]byte(testHTMLPageChanged))
		}
	}))
	defer server.Close()

	cfg := config.FeedConfig{
		Name:     "test-web",
		Type:     config.FeedTypeWeb,
		URL:      server.URL,
		Selector: ".content",
		Interval: time.Minute,
		Tags:     []string{"test"},
		Enabled:  true,
	}

	store := newMockStateStore()
	src := New(cfg, store)
	
	if err := src.Start(context.Background()); err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	// First fetch - baseline
	entries, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatalf("first fetch failed: %v", err)
	}
	if entries != nil {
		t.Error("expected nil entries on first fetch")
	}

	// Second fetch - should detect change
	entries, err = src.Fetch(context.Background())
	if err != nil {
		t.Fatalf("second fetch failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry on change, got %d", len(entries))
	}

	entry := entries[0]
	if entry.SourceName != "test-web" {
		t.Errorf("expected source name 'test-web', got '%s'", entry.SourceName)
	}
	if entry.SourceType != "web" {
		t.Errorf("expected source type 'web', got '%s'", entry.SourceType)
	}
	if len(entry.Tags) != 1 || entry.Tags[0] != "test" {
		t.Errorf("expected tags ['test'], got %v", entry.Tags)
	}
}

func TestWebSource_NoChange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(testHTMLPage))
	}))
	defer server.Close()

	cfg := config.FeedConfig{
		Name:     "test-web",
		Type:     config.FeedTypeWeb,
		URL:      server.URL,
		Selector: ".content",
		Interval: time.Minute,
		Enabled:  true,
	}

	store := newMockStateStore()
	src := New(cfg, store)
	src.Start(context.Background())

	// First fetch - baseline
	src.Fetch(context.Background())

	// Second fetch - no change
	entries, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	if entries != nil {
		t.Error("expected nil entries when no change")
	}
}

func TestWebSource_SelectorNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(testHTMLPage))
	}))
	defer server.Close()

	cfg := config.FeedConfig{
		Name:     "test-web",
		Type:     config.FeedTypeWeb,
		URL:      server.URL,
		Selector: ".nonexistent",
		Interval: time.Minute,
		Enabled:  true,
	}

	src := New(cfg, nil)
	src.Start(context.Background())

	_, err := src.Fetch(context.Background())
	if err == nil {
		t.Error("expected error for non-existent selector")
	}
}

func TestWebSource_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := config.FeedConfig{
		Name:     "test-web",
		Type:     config.FeedTypeWeb,
		URL:      server.URL,
		Selector: ".content",
		Interval: time.Minute,
		Enabled:  true,
	}

	src := New(cfg, nil)
	_, err := src.Fetch(context.Background())
	if err == nil {
		t.Error("expected error for 404 response")
	}
}

func TestWebSource_Name(t *testing.T) {
	cfg := config.FeedConfig{
		Name:     "my-web-source",
		Type:     config.FeedTypeWeb,
		URL:      "https://example.com",
		Selector: ".content",
	}

	src := New(cfg, nil)
	if src.Name() != "my-web-source" {
		t.Errorf("expected name 'my-web-source', got '%s'", src.Name())
	}
	if src.Type() != "web" {
		t.Errorf("expected type 'web', got '%s'", src.Type())
	}
}

func TestWebSource_Interval(t *testing.T) {
	cfg := config.FeedConfig{
		Name:     "my-web-source",
		Type:     config.FeedTypeWeb,
		URL:      "https://example.com",
		Selector: ".content",
		Interval: 2 * time.Hour,
	}

	src := New(cfg, nil)
	if src.Interval() != 2*time.Hour {
		t.Errorf("expected interval 2h, got %v", src.Interval())
	}
}

func TestWebSource_PersistState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(testHTMLPage))
	}))
	defer server.Close()

	cfg := config.FeedConfig{
		Name:     "test-web",
		Type:     config.FeedTypeWeb,
		URL:      server.URL,
		Selector: ".content",
		Interval: time.Minute,
		Enabled:  true,
	}

	store := newMockStateStore()

	// First instance - establish baseline
	src1 := New(cfg, store)
	src1.Start(context.Background())
	src1.Fetch(context.Background())

	// Get saved hash
	savedHash, _, _ := store.GetWebState(context.Background(), "test-web")

	// Second instance - should load previous state
	src2 := New(cfg, store)
	src2.Start(context.Background())

	// Fetch should detect no change
	entries, err := src2.Fetch(context.Background())
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	if entries != nil {
		t.Error("expected no change after loading previous state")
	}

	// Verify state was loaded
	if src2.lastHash != savedHash {
		t.Errorf("expected loaded hash '%s', got '%s'", savedHash, src2.lastHash)
	}
}
