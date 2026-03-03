// Package source provides data source interfaces and implementations.
package source

import (
	"context"
	"time"
)

// Source is the base interface for all sources.
type Source interface {
	// Name returns the unique name of this source.
	Name() string

	// Type returns the type of this source (rss, email, web).
	Type() string

	// Start begins the source operation.
	Start(ctx context.Context) error

	// Stop gracefully stops the source.
	Stop() error
}

// ActiveSource is for sources that actively fetch data (RSS, Web).
type ActiveSource interface {
	Source

	// Fetch retrieves new entries from the source.
	Fetch(ctx context.Context) ([]*Entry, error)

	// Interval returns the fetch interval.
	Interval() time.Duration
}

// PassiveSource is for sources that passively receive data (Email).
type PassiveSource interface {
	Source

	// SetHandler sets the callback for received entries.
	SetHandler(handler EntryHandler)
}

// EntryHandler is called when new entries are received.
type EntryHandler func(entries []*Entry)
