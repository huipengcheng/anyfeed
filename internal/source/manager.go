package source

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Manager manages multiple sources.
type Manager struct {
	sources []Source
	handler EntryHandler
	wg      sync.WaitGroup
	stopCh  chan struct{}
	mu      sync.RWMutex
}

// NewManager creates a new source manager.
func NewManager(handler EntryHandler) *Manager {
	return &Manager{
		handler: handler,
		stopCh:  make(chan struct{}),
	}
}

// Register adds a source to the manager.
func (m *Manager) Register(s Source) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Set handler for passive sources
	if ps, ok := s.(PassiveSource); ok {
		ps.SetHandler(m.handler)
	}

	m.sources = append(m.sources, s)
	slog.Info("registered source", "name", s.Name(), "type", s.Type())
	return nil
}

// Start starts all registered sources.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, s := range m.sources {
		// For active sources, run fetch loop
		if as, ok := s.(ActiveSource); ok {
			m.wg.Add(1)
			go m.runActiveSource(ctx, as)
		}

		// Start the source (for passive sources, this starts their server)
		if err := s.Start(ctx); err != nil {
			slog.Error("failed to start source", "name", s.Name(), "error", err)
			// Continue with other sources
		}
	}

	slog.Info("source manager started", "sources", len(m.sources))
	return nil
}

// runActiveSource runs the fetch loop for an active source.
func (m *Manager) runActiveSource(ctx context.Context, as ActiveSource) {
	defer m.wg.Done()

	name := as.Name()
	interval := as.Interval()

	slog.Info("starting active source fetch loop", "name", name, "interval", interval)

	// Initial fetch
	m.fetchAndHandle(ctx, as)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("stopping active source", "name", name, "reason", "context cancelled")
			return
		case <-m.stopCh:
			slog.Info("stopping active source", "name", name, "reason", "manager stopped")
			return
		case <-ticker.C:
			m.fetchAndHandle(ctx, as)
		}
	}
}

// fetchAndHandle fetches entries and calls the handler.
func (m *Manager) fetchAndHandle(ctx context.Context, as ActiveSource) {
	name := as.Name()
	slog.Debug("fetching from source", "name", name)

	entries, err := as.Fetch(ctx)
	if err != nil {
		slog.Error("fetch failed", "name", name, "error", err)
		return
	}

	if len(entries) > 0 && m.handler != nil {
		slog.Info("received entries", "name", name, "count", len(entries))
		m.handler(entries)
	} else {
		slog.Debug("no new entries", "name", name)
	}
}

// Stop stops all sources.
func (m *Manager) Stop() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	slog.Info("stopping source manager")

	// Signal all goroutines to stop
	close(m.stopCh)

	// Stop all sources
	for _, s := range m.sources {
		if err := s.Stop(); err != nil {
			slog.Error("failed to stop source", "name", s.Name(), "error", err)
		}
	}

	// Wait for all fetch loops to finish
	m.wg.Wait()

	slog.Info("source manager stopped")
	return nil
}

// Sources returns the list of registered sources.
func (m *Manager) Sources() []Source {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sources
}
