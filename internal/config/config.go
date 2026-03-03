// Package config handles loading and validation of application configuration.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// FeedType represents the type of feed source.
type FeedType string

const (
	FeedTypeRSS   FeedType = "rss"
	FeedTypeEmail FeedType = "email"
	FeedTypeWeb   FeedType = "web"
)

// Default values for configuration.
const (
	DefaultHTTPPort        = 8080
	DefaultSMTPPort        = 2525
	DefaultDBPath          = "./anyfeed.db"
	DefaultMaxItemsPerFeed = 1000
	DefaultFetchInterval   = 30 * time.Minute
	DefaultOutputLimit     = 100
)

// Config represents the root configuration.
type Config struct {
	Server  ServerConfig   `yaml:"server"`
	Feeds   []FeedConfig   `yaml:"feeds"`
	Output  []OutputConfig `yaml:"output"`
	Storage StorageConfig  `yaml:"storage"`
}

// ServerConfig holds HTTP and SMTP server settings.
type ServerConfig struct {
	Port     int    `yaml:"port"`      // HTTP server port, default 8080
	SMTPPort int    `yaml:"smtp_port"` // SMTP server port, default 2525
	APIKey   string `yaml:"api_key"`   // Optional API key for authentication
}

// FeedConfig represents a single feed source configuration.
type FeedConfig struct {
	Name     string        `yaml:"name"`              // Unique identifier for the feed
	Type     FeedType      `yaml:"type"`              // rss, email, or web
	Tags     []string      `yaml:"tags,omitempty"`    // Tags for filtering
	Enabled  bool          `yaml:"enabled"`           // Whether this feed is active
	URL      string        `yaml:"url,omitempty"`     // RSS/Web URL (Active fetching)
	Interval time.Duration `yaml:"interval,omitempty"` // Fetch interval (supports: s, m, h)
	Match    string        `yaml:"match,omitempty"`   // Email filter expression (Passive receiving)
	Selector string        `yaml:"selector,omitempty"` // CSS selector for web scraping
}

// OutputConfig represents an RSS output endpoint.
type OutputConfig struct {
	Path    string   `yaml:"path"`              // URL path for the feed
	Sources []string `yaml:"sources,omitempty"` // Filter by source names
	Tags    []string `yaml:"tags,omitempty"`    // Filter by tags
	Limit   int      `yaml:"limit,omitempty"`   // Max items to return
	Title   string   `yaml:"title,omitempty"`   // Feed title
	Desc    string   `yaml:"description,omitempty"` // Feed description
}

// StorageConfig holds database settings.
type StorageConfig struct {
	Path            string `yaml:"path"`              // SQLite database file path
	MaxItemsPerFeed int    `yaml:"max_items_per_feed"` // Max items to keep per feed
}

// Load loads configuration from a YAML file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	cfg.setDefaults()

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// setDefaults sets default values for missing configuration.
func (c *Config) setDefaults() {
	// Server defaults
	if c.Server.Port == 0 {
		c.Server.Port = DefaultHTTPPort
	}
	if c.Server.SMTPPort == 0 {
		c.Server.SMTPPort = DefaultSMTPPort
	}

	// Storage defaults
	if c.Storage.Path == "" {
		c.Storage.Path = DefaultDBPath
	}
	if c.Storage.MaxItemsPerFeed == 0 {
		c.Storage.MaxItemsPerFeed = DefaultMaxItemsPerFeed
	}

	// Feed defaults
	for i := range c.Feeds {
		if c.Feeds[i].Interval == 0 {
			c.Feeds[i].Interval = DefaultFetchInterval
		}
	}

	// Output defaults
	for i := range c.Output {
		if c.Output[i].Limit == 0 {
			c.Output[i].Limit = DefaultOutputLimit
		}
	}
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	var errs []string

	// Validate server config
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		errs = append(errs, "server.port must be between 1 and 65535")
	}
	if c.Server.SMTPPort < 1 || c.Server.SMTPPort > 65535 {
		errs = append(errs, "server.smtp_port must be between 1 and 65535")
	}

	// Validate feeds
	feedNames := make(map[string]bool)
	for i, feed := range c.Feeds {
		if feed.Name == "" {
			errs = append(errs, fmt.Sprintf("feeds[%d].name is required", i))
		} else if feedNames[feed.Name] {
			errs = append(errs, fmt.Sprintf("feeds[%d].name '%s' is duplicated", i, feed.Name))
		} else {
			feedNames[feed.Name] = true
		}

		if err := validateFeedType(feed, i); err != nil {
			errs = append(errs, err.Error())
		}
	}

	// Validate output
	outputPaths := make(map[string]bool)
	for i, output := range c.Output {
		if output.Path == "" {
			errs = append(errs, fmt.Sprintf("output[%d].path is required", i))
		} else {
			if !strings.HasPrefix(output.Path, "/") {
				errs = append(errs, fmt.Sprintf("output[%d].path must start with '/'", i))
			}
			if outputPaths[output.Path] {
				errs = append(errs, fmt.Sprintf("output[%d].path '%s' is duplicated", i, output.Path))
			}
			outputPaths[output.Path] = true
		}

		if output.Limit < 0 {
			errs = append(errs, fmt.Sprintf("output[%d].limit must be non-negative", i))
		}

		// Validate source references
		for _, srcName := range output.Sources {
			if !feedNames[srcName] {
				errs = append(errs, fmt.Sprintf("output[%d] references unknown source '%s'", i, srcName))
			}
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

// validateFeedType validates feed-type-specific fields.
func validateFeedType(feed FeedConfig, index int) error {
	switch feed.Type {
	case FeedTypeRSS:
		if feed.URL == "" {
			return fmt.Errorf("feeds[%d].url is required for RSS type", index)
		}
	case FeedTypeWeb:
		if feed.URL == "" {
			return fmt.Errorf("feeds[%d].url is required for Web type", index)
		}
		if feed.Selector == "" {
			return fmt.Errorf("feeds[%d].selector is required for Web type", index)
		}
	case FeedTypeEmail:
		if feed.Match == "" {
			return fmt.Errorf("feeds[%d].match is required for Email type", index)
		}
	case "":
		return fmt.Errorf("feeds[%d].type is required", index)
	default:
		return fmt.Errorf("feeds[%d].type '%s' is invalid (must be rss, email, or web)", index, feed.Type)
	}
	return nil
}

// GetFeedsByType returns feeds of a specific type.
func (c *Config) GetFeedsByType(feedType FeedType) []FeedConfig {
	var feeds []FeedConfig
	for _, feed := range c.Feeds {
		if feed.Type == feedType && feed.Enabled {
			feeds = append(feeds, feed)
		}
	}
	return feeds
}

// GetFeedByName returns a feed by name.
func (c *Config) GetFeedByName(name string) *FeedConfig {
	for i := range c.Feeds {
		if c.Feeds[i].Name == name {
			return &c.Feeds[i]
		}
	}
	return nil
}

// HasEmailSources returns true if there are any enabled email sources.
func (c *Config) HasEmailSources() bool {
	for _, feed := range c.Feeds {
		if feed.Type == FeedTypeEmail && feed.Enabled {
			return true
		}
	}
	return false
}
