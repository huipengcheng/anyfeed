// Package server provides the HTTP server for RSS output.
package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/huipeng/anyfeed/internal/config"
	"github.com/huipeng/anyfeed/internal/store"
)

// Server is the HTTP server for RSS output.
type Server struct {
	config     *config.Config
	store      store.Store
	router     *mux.Router
	httpServer *http.Server
}

// New creates a new server.
func New(cfg *config.Config, st store.Store) *Server {
	s := &Server{
		config: cfg,
		store:  st,
		router: mux.NewRouter(),
	}
	s.setupRoutes()
	return s
}

// setupRoutes configures all HTTP routes.
func (s *Server) setupRoutes() {
	// Health check - no auth required
	s.router.HandleFunc("/health", s.handleHealth).Methods("GET")

	// Stats endpoint - optionally protected
	statsHandler := http.HandlerFunc(s.handleStats)
	if s.config.Server.APIKey != "" {
		s.router.Handle("/stats", s.authMiddleware(statsHandler)).Methods("GET")
	} else {
		s.router.Handle("/stats", statsHandler).Methods("GET")
	}

	// Setup dynamic feed endpoints from config
	for _, output := range s.config.Output {
		handler := s.createFeedHandler(output)
		if s.config.Server.APIKey != "" {
			s.router.Handle(output.Path, s.authMiddleware(handler)).Methods("GET")
		} else {
			s.router.Handle(output.Path, handler).Methods("GET")
		}
		slog.Info("registered feed endpoint", "path", output.Path)
	}

	// Add logging middleware
	s.router.Use(loggingMiddleware)
}

// loggingMiddleware logs HTTP requests.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(wrapped, r)
		slog.Debug("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.status,
			"duration", time.Since(start),
			"remote", r.RemoteAddr,
		)
	})
}

// responseWriter wraps http.ResponseWriter to capture status code.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// Start starts the HTTP server.
func (s *Server) Start(ctx context.Context) error {
	addr := fmt.Sprintf(":%d", s.config.Server.Port)
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	slog.Info("starting HTTP server", "addr", addr)

	errCh := make(chan error, 1)
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("server failed to start: %w", err)
	case <-time.After(100 * time.Millisecond):
		// Server started successfully
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stop gracefully stops the server.
func (s *Server) Stop(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	slog.Info("stopping HTTP server")
	return s.httpServer.Shutdown(ctx)
}

// Router returns the underlying router for testing.
func (s *Server) Router() *mux.Router {
	return s.router
}
