// Package email provides email receiving source via SMTP.
package email

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/huipeng/anyfeed/internal/config"
	"github.com/huipeng/anyfeed/internal/source"
)

// Source implements email receiving via SMTP (Passive).
type Source struct {
	config   config.FeedConfig
	smtpPort int
	rules    []MatchRule
	handler  source.EntryHandler
	server   *SMTPServer
	mu       sync.RWMutex
}

// New creates a new Email source.
func New(cfg config.FeedConfig, smtpPort int) (*Source, error) {
	rules, err := ParseMatch(cfg.Match)
	if err != nil {
		return nil, err
	}

	return &Source{
		config:   cfg,
		smtpPort: smtpPort,
		rules:    rules,
	}, nil
}

// Name returns the source name.
func (s *Source) Name() string {
	return s.config.Name
}

// Type returns the source type.
func (s *Source) Type() string {
	return string(config.FeedTypeEmail)
}

// SetHandler sets the entry handler.
func (s *Source) SetHandler(handler source.EntryHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handler = handler
}

// Start starts the SMTP server to receive emails.
func (s *Source) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create SMTP server with email handler
	s.server = NewSMTPServer(s.smtpPort, func(email *Email) {
		s.onEmailReceived(email)
	})

	if err := s.server.Start(ctx); err != nil {
		return err
	}

	slog.Info("Email source started",
		"name", s.config.Name,
		"smtp_port", s.smtpPort,
		"match", s.config.Match,
	)
	return nil
}

// Stop stops the SMTP server.
func (s *Source) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.server != nil {
		return s.server.Stop()
	}
	return nil
}

// onEmailReceived is called when an email is received.
func (s *Source) onEmailReceived(email *Email) {
	s.mu.RLock()
	handler := s.handler
	s.mu.RUnlock()

	if handler == nil {
		slog.Debug("no handler set for email source", "name", s.config.Name)
		return
	}

	// Check if email matches the rules
	to := ""
	if len(email.To) > 0 {
		to = email.To[0]
	}

	if !MatchEmail(s.rules, email.From, to, email.Subject) {
		slog.Debug("email does not match rules",
			"source", s.config.Name,
			"from", email.From,
			"subject", email.Subject,
		)
		return
	}

	slog.Info("email matched",
		"source", s.config.Name,
		"from", email.From,
		"subject", email.Subject,
	)

	// Convert email to entry
	entry := s.emailToEntry(email)
	handler([]*source.Entry{entry})
}

// emailToEntry converts an Email to source.Entry.
func (s *Source) emailToEntry(email *Email) *source.Entry {
	// Prefer HTML content, fallback to plain text
	content := email.HTML
	if content == "" {
		content = email.Body
	}

	// Clean up content
	content = strings.TrimSpace(content)

	// If we have plain text and it's not HTML, wrap in pre tags
	if email.HTML == "" && content != "" && !strings.HasPrefix(content, "<") {
		content = "<pre>" + content + "</pre>"
	}

	publishedAt := email.Date
	if publishedAt.IsZero() {
		publishedAt = time.Now()
	}

	entry := source.NewEntry(
		s.config.Name,
		string(config.FeedTypeEmail),
		email.Subject,
		content,
		"", // No URL for emails
		email.From,
		publishedAt,
		s.config.Tags,
	)

	// Add metadata
	entry.Metadata["from"] = email.From
	if len(email.To) > 0 {
		entry.Metadata["to"] = strings.Join(email.To, ", ")
	}

	return entry
}
