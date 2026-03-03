// Package store provides data storage interfaces and implementations.
package store

import (
	"context"
	"time"

	"github.com/huipeng/anyfeed/internal/source"
)

// QueryOptions defines filtering options for queries.
type QueryOptions struct {
	SourceNames []string  // Filter by source names
	Tags        []string  // Filter by tags
	Since       time.Time // Only entries after this time
	Limit       int       // Maximum number of entries
	Offset      int       // Pagination offset
}

// Store defines the interface for entry storage.
type Store interface {
	// SaveEntries saves entries to storage (deduplicates by ID).
	SaveEntries(ctx context.Context, entries []*source.Entry) error

	// GetEntries retrieves entries with optional filters.
	GetEntries(ctx context.Context, opts QueryOptions) ([]*source.Entry, error)

	// GetEntryByID retrieves a single entry by ID.
	GetEntryByID(ctx context.Context, id string) (*source.Entry, error)

	// DeleteOldEntries removes entries beyond retention limit.
	DeleteOldEntries(ctx context.Context, sourceName string, keepCount int) error

	// GetEntriesCount returns the total count of entries for a source.
	GetEntriesCount(ctx context.Context, sourceName string) (int, error)

	// GetAllSourcesCount returns the count of entries for all sources.
	GetAllSourcesCount(ctx context.Context) (map[string]int, error)

	// SaveWebState saves the web source state (last hash, content).
	SaveWebState(ctx context.Context, sourceName, hash, content string) error

	// GetWebState retrieves the web source state.
	GetWebState(ctx context.Context, sourceName string) (hash string, content string, err error)

	// Close closes the store.
	Close() error
}
