package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/huipeng/anyfeed/internal/config"
	"github.com/huipeng/anyfeed/internal/rss"
	"github.com/huipeng/anyfeed/internal/store"
)

// handleHealth handles the health check endpoint.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

// handleStats handles the stats endpoint.
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	counts, err := s.store.GetAllSourcesCount(ctx)
	if err != nil {
		http.Error(w, "failed to get stats", http.StatusInternalServerError)
		return
	}

	totalEntries := 0
	for _, count := range counts {
		totalEntries += count
	}

	stats := map[string]interface{}{
		"sources":       counts,
		"total_entries": totalEntries,
		"feeds":         len(s.config.Feeds),
		"outputs":       len(s.config.Output),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// createFeedHandler creates a handler for a specific feed output configuration.
func (s *Server) createFeedHandler(output config.OutputConfig) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		opts := store.QueryOptions{
			SourceNames: output.Sources,
			Tags:        output.Tags,
			Limit:       output.Limit,
		}

		entries, err := s.store.GetEntries(ctx, opts)
		if err != nil {
			http.Error(w, "failed to get entries", http.StatusInternalServerError)
			return
		}

		// Generate feed metadata
		title := output.Title
		if title == "" {
			title = "Anyfeed - " + output.Path
		}

		description := output.Desc
		if description == "" {
			description = "RSS feed aggregated by Anyfeed"
		}

		// Generate base URL from request
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		link := scheme + "://" + r.Host + output.Path

		// Generate RSS XML
		xmlData, err := rss.Generate(title, link, description, entries)
		if err != nil {
			http.Error(w, "failed to generate RSS", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=300")
		w.Write(xmlData)
	})
}

// handleFeed is a generic feed handler (kept for backwards compatibility).
func (s *Server) handleFeed(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get all entries with default limit
	entries, err := s.store.GetEntries(ctx, store.QueryOptions{Limit: 100})
	if err != nil {
		http.Error(w, "failed to get entries", http.StatusInternalServerError)
		return
	}

	xmlData, err := rss.Generate("Anyfeed", "", "RSS feed aggregated by Anyfeed", entries)
	if err != nil {
		http.Error(w, "failed to generate RSS", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.Write(xmlData)
}
