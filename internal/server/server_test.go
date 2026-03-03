package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/huipeng/anyfeed/internal/config"
	"github.com/huipeng/anyfeed/internal/source"
	"github.com/huipeng/anyfeed/internal/store"
)

func setupTestServer(t *testing.T) (*Server, *store.SQLiteStore) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:     8080,
			SMTPPort: 2525,
		},
		Storage: config.StorageConfig{
			Path:            dbPath,
			MaxItemsPerFeed: 1000,
		},
		Feeds: []config.FeedConfig{
			{
				Name:     "test-feed",
				Type:     config.FeedTypeRSS,
				URL:      "https://example.com/feed.xml",
				Interval: time.Minute,
				Tags:     []string{"test"},
				Enabled:  true,
			},
		},
		Output: []config.OutputConfig{
			{
				Path:  "/feed/all",
				Limit: 100,
				Title: "All Feeds",
			},
			{
				Path:    "/feed/test",
				Sources: []string{"test-feed"},
				Limit:   50,
			},
		},
	}

	srv := New(cfg, st)
	return srv, st
}

func TestServer_HealthEndpoint(t *testing.T) {
	srv, st := setupTestServer(t)
	defer st.Close()

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var response map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response["status"] != "ok" {
		t.Errorf("expected status 'ok', got '%s'", response["status"])
	}
}

func TestServer_StatsEndpoint(t *testing.T) {
	srv, st := setupTestServer(t)
	defer st.Close()

	// Add some test entries
	entries := []*source.Entry{
		{
			ID:          "1",
			SourceName:  "test-feed",
			SourceType:  "rss",
			Title:       "Test Entry",
			PublishedAt: time.Now(),
			ReceivedAt:  time.Now(),
			Metadata:    make(map[string]string),
		},
	}
	if err := st.SaveEntries(context.Background(), entries); err != nil {
		t.Fatalf("failed to save entries: %v", err)
	}

	req := httptest.NewRequest("GET", "/stats", nil)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response["total_entries"].(float64) != 1 {
		t.Errorf("expected total_entries 1, got %v", response["total_entries"])
	}
}

func TestServer_FeedEndpoint(t *testing.T) {
	srv, st := setupTestServer(t)
	defer st.Close()

	// Add some test entries
	entries := []*source.Entry{
		{
			ID:          "entry1",
			SourceName:  "test-feed",
			SourceType:  "rss",
			Title:       "Test Entry 1",
			Content:     "Content 1",
			URL:         "https://example.com/1",
			PublishedAt: time.Now(),
			ReceivedAt:  time.Now(),
			Tags:        []string{"test"},
			Metadata:    make(map[string]string),
		},
	}
	if err := st.SaveEntries(context.Background(), entries); err != nil {
		t.Fatalf("failed to save entries: %v", err)
	}

	req := httptest.NewRequest("GET", "/feed/all", nil)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/rss+xml; charset=utf-8" {
		t.Errorf("expected Content-Type 'application/rss+xml; charset=utf-8', got '%s'", contentType)
	}

	// Verify RSS content
	body := rec.Body.String()
	if !contains(body, "<rss") {
		t.Error("expected RSS XML in response")
	}
	if !contains(body, "Test Entry 1") {
		t.Error("expected entry title in response")
	}
}

func TestServer_FeedEndpointWithSourceFilter(t *testing.T) {
	srv, st := setupTestServer(t)
	defer st.Close()

	// Add entries from different sources
	entries := []*source.Entry{
		{
			ID:          "entry1",
			SourceName:  "test-feed",
			SourceType:  "rss",
			Title:       "Test Entry",
			PublishedAt: time.Now(),
			ReceivedAt:  time.Now(),
			Tags:        []string{"test"},
			Metadata:    make(map[string]string),
		},
		{
			ID:          "entry2",
			SourceName:  "other-feed",
			SourceType:  "rss",
			Title:       "Other Entry",
			PublishedAt: time.Now(),
			ReceivedAt:  time.Now(),
			Metadata:    make(map[string]string),
		},
	}
	if err := st.SaveEntries(context.Background(), entries); err != nil {
		t.Fatalf("failed to save entries: %v", err)
	}

	req := httptest.NewRequest("GET", "/feed/test", nil)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !contains(body, "Test Entry") {
		t.Error("expected 'Test Entry' in response")
	}
	if contains(body, "Other Entry") {
		t.Error("should not contain 'Other Entry' from different source")
	}
}

func TestServer_APIKeyAuth(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer st.Close()

	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:   8080,
			APIKey: "secret-key",
		},
		Output: []config.OutputConfig{
			{Path: "/feed/all", Limit: 100},
		},
	}

	srv := New(cfg, st)

	// Without API key
	t.Run("without API key", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/feed/all", nil)
		rec := httptest.NewRecorder()
		srv.Router().ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rec.Code)
		}
	})

	// With wrong API key
	t.Run("with wrong API key", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/feed/all", nil)
		req.Header.Set("X-API-Key", "wrong-key")
		rec := httptest.NewRecorder()
		srv.Router().ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rec.Code)
		}
	})

	// With correct API key in header
	t.Run("with correct API key in header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/feed/all", nil)
		req.Header.Set("X-API-Key", "secret-key")
		rec := httptest.NewRecorder()
		srv.Router().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})

	// With correct API key in query param
	t.Run("with correct API key in query param", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/feed/all?api_key=secret-key", nil)
		rec := httptest.NewRecorder()
		srv.Router().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})

	// With Bearer token
	t.Run("with Bearer token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/feed/all", nil)
		req.Header.Set("Authorization", "Bearer secret-key")
		rec := httptest.NewRecorder()
		srv.Router().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})

	// Health endpoint should be accessible without auth
	t.Run("health without auth", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		rec := httptest.NewRecorder()
		srv.Router().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
