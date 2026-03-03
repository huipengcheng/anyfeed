package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/huipeng/anyfeed/internal/source"
)

func setupTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestSQLiteStore_SaveAndGetEntries(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	entries := []*source.Entry{
		{
			ID:          "entry1",
			SourceName:  "test-source",
			SourceType:  "rss",
			Title:       "Test Entry 1",
			Content:     "Content 1",
			URL:         "https://example.com/1",
			Author:      "Author 1",
			PublishedAt: time.Now().Add(-time.Hour),
			ReceivedAt:  time.Now(),
			Tags:        []string{"tech", "news"},
			Metadata:    map[string]string{"key": "value"},
		},
		{
			ID:          "entry2",
			SourceName:  "test-source",
			SourceType:  "rss",
			Title:       "Test Entry 2",
			Content:     "Content 2",
			URL:         "https://example.com/2",
			PublishedAt: time.Now(),
			ReceivedAt:  time.Now(),
			Tags:        []string{"tech"},
			Metadata:    make(map[string]string),
		},
	}

	// Save entries
	if err := store.SaveEntries(ctx, entries); err != nil {
		t.Fatalf("failed to save entries: %v", err)
	}

	// Get all entries
	retrieved, err := store.GetEntries(ctx, QueryOptions{})
	if err != nil {
		t.Fatalf("failed to get entries: %v", err)
	}
	if len(retrieved) != 2 {
		t.Errorf("expected 2 entries, got %d", len(retrieved))
	}

	// Entries should be ordered by published_at DESC
	if retrieved[0].Title != "Test Entry 2" {
		t.Errorf("expected first entry to be 'Test Entry 2', got '%s'", retrieved[0].Title)
	}
}

func TestSQLiteStore_GetEntriesWithFilters(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	now := time.Now()
	entries := []*source.Entry{
		{
			ID:          "entry1",
			SourceName:  "source-a",
			SourceType:  "rss",
			Title:       "Entry A1",
			PublishedAt: now.Add(-2 * time.Hour),
			ReceivedAt:  now,
			Tags:        []string{"tech"},
			Metadata:    make(map[string]string),
		},
		{
			ID:          "entry2",
			SourceName:  "source-a",
			SourceType:  "rss",
			Title:       "Entry A2",
			PublishedAt: now.Add(-1 * time.Hour),
			ReceivedAt:  now,
			Tags:        []string{"news"},
			Metadata:    make(map[string]string),
		},
		{
			ID:          "entry3",
			SourceName:  "source-b",
			SourceType:  "web",
			Title:       "Entry B1",
			PublishedAt: now,
			ReceivedAt:  now,
			Tags:        []string{"tech", "news"},
			Metadata:    make(map[string]string),
		},
	}

	if err := store.SaveEntries(ctx, entries); err != nil {
		t.Fatalf("failed to save entries: %v", err)
	}

	// Filter by source name
	t.Run("filter by source name", func(t *testing.T) {
		retrieved, err := store.GetEntries(ctx, QueryOptions{SourceNames: []string{"source-a"}})
		if err != nil {
			t.Fatalf("failed to get entries: %v", err)
		}
		if len(retrieved) != 2 {
			t.Errorf("expected 2 entries from source-a, got %d", len(retrieved))
		}
	})

	// Filter by tag
	t.Run("filter by tag", func(t *testing.T) {
		retrieved, err := store.GetEntries(ctx, QueryOptions{Tags: []string{"tech"}})
		if err != nil {
			t.Fatalf("failed to get entries: %v", err)
		}
		if len(retrieved) != 2 {
			t.Errorf("expected 2 entries with 'tech' tag, got %d", len(retrieved))
		}
	})

	// Filter by time
	t.Run("filter by time", func(t *testing.T) {
		retrieved, err := store.GetEntries(ctx, QueryOptions{Since: now.Add(-90 * time.Minute)})
		if err != nil {
			t.Fatalf("failed to get entries: %v", err)
		}
		if len(retrieved) != 2 {
			t.Errorf("expected 2 recent entries, got %d", len(retrieved))
		}
	})

	// Limit
	t.Run("limit", func(t *testing.T) {
		retrieved, err := store.GetEntries(ctx, QueryOptions{Limit: 2})
		if err != nil {
			t.Fatalf("failed to get entries: %v", err)
		}
		if len(retrieved) != 2 {
			t.Errorf("expected 2 entries with limit, got %d", len(retrieved))
		}
	})
}

func TestSQLiteStore_GetEntryByID(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	entry := &source.Entry{
		ID:          "unique-id",
		SourceName:  "test-source",
		SourceType:  "rss",
		Title:       "Test Entry",
		Content:     "Test Content",
		PublishedAt: time.Now(),
		ReceivedAt:  time.Now(),
		Tags:        []string{"test"},
		Metadata:    make(map[string]string),
	}

	if err := store.SaveEntries(ctx, []*source.Entry{entry}); err != nil {
		t.Fatalf("failed to save entry: %v", err)
	}

	retrieved, err := store.GetEntryByID(ctx, "unique-id")
	if err != nil {
		t.Fatalf("failed to get entry by ID: %v", err)
	}
	if retrieved == nil {
		t.Fatal("expected to find entry")
	}
	if retrieved.Title != "Test Entry" {
		t.Errorf("expected title 'Test Entry', got '%s'", retrieved.Title)
	}

	// Non-existent ID
	notFound, err := store.GetEntryByID(ctx, "non-existent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if notFound != nil {
		t.Error("expected nil for non-existent ID")
	}
}

func TestSQLiteStore_DeleteOldEntries(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	// Create 5 entries
	entries := make([]*source.Entry, 5)
	for i := 0; i < 5; i++ {
		entries[i] = &source.Entry{
			ID:          source.GenerateID("test-source", "Entry", "", time.Now().Add(time.Duration(i)*time.Minute)),
			SourceName:  "test-source",
			SourceType:  "rss",
			Title:       "Entry",
			PublishedAt: time.Now().Add(time.Duration(i) * time.Minute),
			ReceivedAt:  time.Now(),
			Metadata:    make(map[string]string),
		}
	}

	if err := store.SaveEntries(ctx, entries); err != nil {
		t.Fatalf("failed to save entries: %v", err)
	}

	// Keep only 3
	if err := store.DeleteOldEntries(ctx, "test-source", 3); err != nil {
		t.Fatalf("failed to delete old entries: %v", err)
	}

	count, err := store.GetEntriesCount(ctx, "test-source")
	if err != nil {
		t.Fatalf("failed to get count: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 entries after cleanup, got %d", count)
	}
}

func TestSQLiteStore_Deduplication(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	entry := &source.Entry{
		ID:          "duplicate-id",
		SourceName:  "test-source",
		SourceType:  "rss",
		Title:       "Original Title",
		Content:     "Original Content",
		PublishedAt: time.Now(),
		ReceivedAt:  time.Now(),
		Metadata:    make(map[string]string),
	}

	// Save same entry twice
	if err := store.SaveEntries(ctx, []*source.Entry{entry}); err != nil {
		t.Fatalf("failed to save entry: %v", err)
	}

	entry.Title = "Updated Title"
	if err := store.SaveEntries(ctx, []*source.Entry{entry}); err != nil {
		t.Fatalf("failed to save duplicate entry: %v", err)
	}

	// Should only have 1 entry with original title (INSERT OR IGNORE)
	retrieved, err := store.GetEntries(ctx, QueryOptions{})
	if err != nil {
		t.Fatalf("failed to get entries: %v", err)
	}
	if len(retrieved) != 1 {
		t.Errorf("expected 1 entry after dedup, got %d", len(retrieved))
	}
	if retrieved[0].Title != "Original Title" {
		t.Errorf("expected original title to be preserved")
	}
}

func TestSQLiteStore_WebState(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	// Get non-existent state
	hash, content, err := store.GetWebState(ctx, "test-source")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hash != "" || content != "" {
		t.Error("expected empty state for non-existent source")
	}

	// Save state
	if err := store.SaveWebState(ctx, "test-source", "abc123", "<html>content</html>"); err != nil {
		t.Fatalf("failed to save web state: %v", err)
	}

	// Get state
	hash, content, err = store.GetWebState(ctx, "test-source")
	if err != nil {
		t.Fatalf("failed to get web state: %v", err)
	}
	if hash != "abc123" {
		t.Errorf("expected hash 'abc123', got '%s'", hash)
	}
	if content != "<html>content</html>" {
		t.Errorf("expected content to match")
	}

	// Update state
	if err := store.SaveWebState(ctx, "test-source", "def456", "<html>new</html>"); err != nil {
		t.Fatalf("failed to update web state: %v", err)
	}

	hash, _, err = store.GetWebState(ctx, "test-source")
	if err != nil {
		t.Fatalf("failed to get updated web state: %v", err)
	}
	if hash != "def456" {
		t.Errorf("expected updated hash 'def456', got '%s'", hash)
	}
}

func TestSQLiteStore_GetAllSourcesCount(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	entries := []*source.Entry{
		{ID: "1", SourceName: "source-a", SourceType: "rss", Title: "A1", PublishedAt: time.Now(), ReceivedAt: time.Now(), Metadata: make(map[string]string)},
		{ID: "2", SourceName: "source-a", SourceType: "rss", Title: "A2", PublishedAt: time.Now(), ReceivedAt: time.Now(), Metadata: make(map[string]string)},
		{ID: "3", SourceName: "source-b", SourceType: "web", Title: "B1", PublishedAt: time.Now(), ReceivedAt: time.Now(), Metadata: make(map[string]string)},
	}

	if err := store.SaveEntries(ctx, entries); err != nil {
		t.Fatalf("failed to save entries: %v", err)
	}

	counts, err := store.GetAllSourcesCount(ctx)
	if err != nil {
		t.Fatalf("failed to get counts: %v", err)
	}

	if counts["source-a"] != 2 {
		t.Errorf("expected source-a count 2, got %d", counts["source-a"])
	}
	if counts["source-b"] != 1 {
		t.Errorf("expected source-b count 1, got %d", counts["source-b"])
	}
}
