package server

import (
	"net/http"
	"strings"
)

// authMiddleware checks for API key authentication.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If no API key configured, skip auth
		if s.config.Server.APIKey == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Check header first
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			// Check query parameter
			apiKey = r.URL.Query().Get("api_key")
		}

		// Also support Authorization: Bearer <token>
		if apiKey == "" {
			auth := r.Header.Get("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				apiKey = strings.TrimPrefix(auth, "Bearer ")
			}
		}

		if apiKey != s.config.Server.APIKey {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error": "unauthorized", "message": "invalid or missing API key"}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}
