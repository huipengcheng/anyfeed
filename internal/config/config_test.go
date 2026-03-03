package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	configContent := `
server:
  port: 8080
  smtp_port: 2525
  api_key: "test-api-key"

storage:
  path: "./test.db"
  max_items_per_feed: 500

feeds:
  - name: "test-rss"
    type: rss
    url: "https://example.com/feed.xml"
    interval: 15m
    tags: ["test"]
    enabled: true

  - name: "test-web"
    type: web
    url: "https://example.com/page"
    selector: ".content"
    interval: 1h
    enabled: true

  - name: "test-email"
    type: email
    match: "from:*@example.com"
    enabled: true

output:
  - path: "/feed/all"
    limit: 50
  - path: "/feed/test"
    sources: ["test-rss"]
    tags: ["test"]
    limit: 25
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify server config
	if cfg.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Server.SMTPPort != 2525 {
		t.Errorf("expected smtp_port 2525, got %d", cfg.Server.SMTPPort)
	}
	if cfg.Server.APIKey != "test-api-key" {
		t.Errorf("expected api_key 'test-api-key', got '%s'", cfg.Server.APIKey)
	}

	// Verify storage config
	if cfg.Storage.Path != "./test.db" {
		t.Errorf("expected storage path './test.db', got '%s'", cfg.Storage.Path)
	}
	if cfg.Storage.MaxItemsPerFeed != 500 {
		t.Errorf("expected max_items_per_feed 500, got %d", cfg.Storage.MaxItemsPerFeed)
	}

	// Verify feeds
	if len(cfg.Feeds) != 3 {
		t.Errorf("expected 3 feeds, got %d", len(cfg.Feeds))
	}

	rssFeed := cfg.GetFeedByName("test-rss")
	if rssFeed == nil {
		t.Fatal("expected to find feed 'test-rss'")
	}
	if rssFeed.Type != FeedTypeRSS {
		t.Errorf("expected feed type 'rss', got '%s'", rssFeed.Type)
	}
	if rssFeed.Interval != 15*time.Minute {
		t.Errorf("expected interval 15m, got %v", rssFeed.Interval)
	}

	// Verify output
	if len(cfg.Output) != 2 {
		t.Errorf("expected 2 outputs, got %d", len(cfg.Output))
	}
}

func TestLoadDefaults(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// Minimal config without optional fields
	configContent := `
feeds:
  - name: "test-rss"
    type: rss
    url: "https://example.com/feed.xml"
    enabled: true

output:
  - path: "/feed/all"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Check defaults
	if cfg.Server.Port != DefaultHTTPPort {
		t.Errorf("expected default port %d, got %d", DefaultHTTPPort, cfg.Server.Port)
	}
	if cfg.Server.SMTPPort != DefaultSMTPPort {
		t.Errorf("expected default smtp_port %d, got %d", DefaultSMTPPort, cfg.Server.SMTPPort)
	}
	if cfg.Storage.Path != DefaultDBPath {
		t.Errorf("expected default db path '%s', got '%s'", DefaultDBPath, cfg.Storage.Path)
	}
	if cfg.Storage.MaxItemsPerFeed != DefaultMaxItemsPerFeed {
		t.Errorf("expected default max_items_per_feed %d, got %d", DefaultMaxItemsPerFeed, cfg.Storage.MaxItemsPerFeed)
	}
	if cfg.Feeds[0].Interval != DefaultFetchInterval {
		t.Errorf("expected default interval %v, got %v", DefaultFetchInterval, cfg.Feeds[0].Interval)
	}
	if cfg.Output[0].Limit != DefaultOutputLimit {
		t.Errorf("expected default limit %d, got %d", DefaultOutputLimit, cfg.Output[0].Limit)
	}
}

func TestValidateInvalidConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "missing feed name",
			config: Config{
				Server: ServerConfig{Port: 8080, SMTPPort: 2525},
				Feeds: []FeedConfig{
					{Type: FeedTypeRSS, URL: "https://example.com/feed.xml", Enabled: true},
				},
				Output: []OutputConfig{
					{Path: "/feed/all", Limit: 10},
				},
			},
			wantErr: true,
		},
		{
			name: "missing feed type",
			config: Config{
				Server: ServerConfig{Port: 8080, SMTPPort: 2525},
				Feeds: []FeedConfig{
					{Name: "test", URL: "https://example.com/feed.xml", Enabled: true},
				},
				Output: []OutputConfig{
					{Path: "/feed/all", Limit: 10},
				},
			},
			wantErr: true,
		},
		{
			name: "missing RSS URL",
			config: Config{
				Server: ServerConfig{Port: 8080, SMTPPort: 2525},
				Feeds: []FeedConfig{
					{Name: "test", Type: FeedTypeRSS, Enabled: true},
				},
				Output: []OutputConfig{
					{Path: "/feed/all", Limit: 10},
				},
			},
			wantErr: true,
		},
		{
			name: "missing web selector",
			config: Config{
				Server: ServerConfig{Port: 8080, SMTPPort: 2525},
				Feeds: []FeedConfig{
					{Name: "test", Type: FeedTypeWeb, URL: "https://example.com", Enabled: true},
				},
				Output: []OutputConfig{
					{Path: "/feed/all", Limit: 10},
				},
			},
			wantErr: true,
		},
		{
			name: "missing email match",
			config: Config{
				Server: ServerConfig{Port: 8080, SMTPPort: 2525},
				Feeds: []FeedConfig{
					{Name: "test", Type: FeedTypeEmail, Enabled: true},
				},
				Output: []OutputConfig{
					{Path: "/feed/all", Limit: 10},
				},
			},
			wantErr: true,
		},
		{
			name: "duplicate feed names",
			config: Config{
				Server: ServerConfig{Port: 8080, SMTPPort: 2525},
				Feeds: []FeedConfig{
					{Name: "test", Type: FeedTypeRSS, URL: "https://example.com/feed1.xml", Enabled: true},
					{Name: "test", Type: FeedTypeRSS, URL: "https://example.com/feed2.xml", Enabled: true},
				},
				Output: []OutputConfig{
					{Path: "/feed/all", Limit: 10},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid output path",
			config: Config{
				Server: ServerConfig{Port: 8080, SMTPPort: 2525},
				Feeds: []FeedConfig{
					{Name: "test", Type: FeedTypeRSS, URL: "https://example.com/feed.xml", Enabled: true},
				},
				Output: []OutputConfig{
					{Path: "feed/all", Limit: 10}, // Missing leading /
				},
			},
			wantErr: true,
		},
		{
			name: "output references unknown source",
			config: Config{
				Server: ServerConfig{Port: 8080, SMTPPort: 2525},
				Feeds: []FeedConfig{
					{Name: "test", Type: FeedTypeRSS, URL: "https://example.com/feed.xml", Enabled: true},
				},
				Output: []OutputConfig{
					{Path: "/feed/all", Sources: []string{"unknown"}, Limit: 10},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid port",
			config: Config{
				Server: ServerConfig{Port: 0, SMTPPort: 2525},
				Feeds: []FeedConfig{
					{Name: "test", Type: FeedTypeRSS, URL: "https://example.com/feed.xml", Enabled: true},
				},
				Output: []OutputConfig{
					{Path: "/feed/all", Limit: 10},
				},
			},
			wantErr: true,
		},
		{
			name: "valid config",
			config: Config{
				Server: ServerConfig{Port: 8080, SMTPPort: 2525},
				Feeds: []FeedConfig{
					{Name: "test", Type: FeedTypeRSS, URL: "https://example.com/feed.xml", Enabled: true},
				},
				Output: []OutputConfig{
					{Path: "/feed/all", Limit: 10},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetFeedsByType(t *testing.T) {
	cfg := Config{
		Feeds: []FeedConfig{
			{Name: "rss1", Type: FeedTypeRSS, Enabled: true},
			{Name: "rss2", Type: FeedTypeRSS, Enabled: false},
			{Name: "web1", Type: FeedTypeWeb, Enabled: true},
			{Name: "email1", Type: FeedTypeEmail, Enabled: true},
		},
	}

	rssFeeds := cfg.GetFeedsByType(FeedTypeRSS)
	if len(rssFeeds) != 1 {
		t.Errorf("expected 1 enabled RSS feed, got %d", len(rssFeeds))
	}
	if rssFeeds[0].Name != "rss1" {
		t.Errorf("expected feed name 'rss1', got '%s'", rssFeeds[0].Name)
	}

	webFeeds := cfg.GetFeedsByType(FeedTypeWeb)
	if len(webFeeds) != 1 {
		t.Errorf("expected 1 enabled Web feed, got %d", len(webFeeds))
	}
}

func TestHasEmailSources(t *testing.T) {
	cfg1 := Config{
		Feeds: []FeedConfig{
			{Name: "rss1", Type: FeedTypeRSS, Enabled: true},
		},
	}
	if cfg1.HasEmailSources() {
		t.Error("expected no email sources")
	}

	cfg2 := Config{
		Feeds: []FeedConfig{
			{Name: "email1", Type: FeedTypeEmail, Enabled: true},
		},
	}
	if !cfg2.HasEmailSources() {
		t.Error("expected email sources")
	}

	cfg3 := Config{
		Feeds: []FeedConfig{
			{Name: "email1", Type: FeedTypeEmail, Enabled: false},
		},
	}
	if cfg3.HasEmailSources() {
		t.Error("expected no enabled email sources")
	}
}

func TestLoadNonExistentFile(t *testing.T) {
	_, err := Load("/non/existent/path.yaml")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "invalid.yaml")

	if err := os.WriteFile(configPath, []byte("invalid: yaml: content:"), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}
