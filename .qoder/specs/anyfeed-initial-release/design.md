# Anyfeed - Design Document

## Overview

Anyfeed 是一个通用 RSS 聚合系统，能够订阅多种来源（RSS/Atom 源、邮件、网页变更）并统一输出为 RSS 格式。系统采用单体架构，以单一 binary 形式部署，使用 SQLite 作为数据存储。

### Goals
- 支持多种数据源订阅（RSS、Email、Web）
- 统一输出标准 RSS 格式
- 简单部署，低资源占用
- 灵活的配置和过滤机制

### Non-Goals
- 分布式部署
- 用户多租户管理
- 复杂的权限控制系统

## Technical Architecture

### Technology Stack

| Component | Choice | Rationale |
|-----------|--------|-----------|
| Language | Go 1.22+ | 性能好、单 binary 部署、并发友好 |
| HTTP Framework | net/http + gorilla/mux | 轻量、无额外依赖、社区成熟 |
| RSS Parser | gofeed | 功能全面，支持 RSS 1.0/2.0/Atom |
| SMTP Server | go-guerrilla | 成熟稳定，支持 TLS |
| Web Scraper | colly + goquery | 强大灵活，CSS 选择器支持好 |
| Database | SQLite + modernc.org/sqlite | 纯 Go 实现，CGO-free |
| Config | YAML (gopkg.in/yaml.v3) | 人类可读，Go 支持好 |
| Logging | log/slog | Go 1.21+ 标准库，零依赖 |

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Anyfeed Server                                  │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                         │
│  │ RSS Fetcher │  │Email Receiver│  │ Web Watcher │      Data Sources       │
│  │  (Active)   │  │  (Passive)  │  │  (Active)   │                         │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘                         │
│         │                │                │                                  │
│         │    ┌───────────┘                │                                  │
│         │    │  SMTP Server               │                                  │
│         │    │  (Receive emails)          │                                  │
│         │    │                            │                                  │
│         └────┼────────────────────────────┘                                  │
│              ▼                                                               │
│       ┌──────────────┐                                                       │
│       │  Processor   │                   Core Processing                     │
│       │  (Normalize) │                                                       │
│       └──────┬───────┘                                                       │
│              │                                                               │
│              ▼                                                               │
│       ┌──────────────┐                                                       │
│       │    Store     │                   Storage Layer                       │
│       │   (SQLite)   │                                                       │
│       └──────┬───────┘                                                       │
│              │                                                               │
│              ▼                                                               │
│       ┌──────────────┐                                                       │
│       │  RSS Output  │                   Output Layer                        │
│       │   (HTTP)     │                                                       │
│       └──────────────┘                                                       │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Data Flow

| Source Type | Mode | Description |
|-------------|------|-------------|
| RSS/Atom | **主动拉取** | 定时从远程 URL 获取 Feed |
| Email | **被动接收** | SMTP 服务器监听，邮件到达时触发处理 |
| Web | **主动拉取** | 定时抓取网页，检测内容变更 |

### Module Dependency Graph

```
cmd/anyfeed
    │
    └── internal/
            ├── config     ← YAML 配置解析
            ├── source     ← 数据源（依赖 config）
            │   ├── rss    ← RSS/Atom 抓取（主动）
            │   ├── email  ← 邮件接收（被动）
            │   └── web    ← 网页监控（主动）
            ├── store      ← 数据存储
            └── server     ← HTTP 服务（依赖 store, config）
```

## Component Design

### 1. Config Module (`internal/config`)

负责加载和验证 YAML 配置文件。

```go
// config.go
package config

import (
    "time"
)

// Config represents the root configuration
type Config struct {
    Server  ServerConfig   `yaml:"server"`
    Feeds   []FeedConfig   `yaml:"feeds"`
    Output  []OutputConfig `yaml:"output"`
    Storage StorageConfig  `yaml:"storage"`
}

// ServerConfig holds HTTP and SMTP server settings
type ServerConfig struct {
    Port     int    `yaml:"port"`      // HTTP server port, default 8080
    SMTPPort int    `yaml:"smtp_port"` // SMTP server port, default 2525
    APIKey   string `yaml:"api_key"`   // Optional API key for authentication
}

// FeedConfig represents a single feed source configuration
type FeedConfig struct {
    Name     string        `yaml:"name"`     // Unique identifier for the feed
    Type     FeedType      `yaml:"type"`     // rss, email, or web
    Tags     []string      `yaml:"tags"`     // Tags for filtering
    Enabled  bool          `yaml:"enabled"`  // Whether this feed is active
    
    // RSS specific (Active fetching)
    URL      string        `yaml:"url,omitempty"`
    Interval time.Duration `yaml:"interval,omitempty"`
    
    // Email specific (Passive receiving)
    Match    string        `yaml:"match,omitempty"` // Filter expression
    
    // Web specific (Active fetching)
    Selector string        `yaml:"selector,omitempty"` // CSS selector
}

// FeedType represents the type of feed source
type FeedType string

const (
    FeedTypeRSS   FeedType = "rss"
    FeedTypeEmail FeedType = "email"
    FeedTypeWeb   FeedType = "web"
)

// OutputConfig represents an RSS output endpoint
type OutputConfig struct {
    Path    string   `yaml:"path"`              // URL path for the feed
    Sources []string `yaml:"sources,omitempty"` // Filter by source names
    Tags    []string `yaml:"tags,omitempty"`    // Filter by tags
    Limit   int      `yaml:"limit,omitempty"`   // Max items to return
}

// StorageConfig holds database settings
type StorageConfig struct {
    Path            string `yaml:"path"`              // SQLite database file path
    MaxItemsPerFeed int    `yaml:"max_items_per_feed"` // Max items to keep per feed
}

// Load loads configuration from a YAML file
func Load(path string) (*Config, error)

// Validate validates the configuration
func (c *Config) Validate() error
```

### 2. Source Module (`internal/source`)

定义统一的数据源接口。数据源分为两类：
- **ActiveSource**: 主动拉取（RSS、Web），需要定时调度
- **PassiveSource**: 被动接收（Email），持续监听

```go
// source.go
package source

import (
    "context"
    "time"
)

// Entry represents a normalized content entry from any source
type Entry struct {
    ID          string            // Unique identifier (hash of content)
    SourceName  string            // Source feed name
    Title       string            // Entry title
    Content     string            // Entry content (HTML)
    URL         string            // Original URL if available
    Author      string            // Author name
    PublishedAt time.Time         // Publication time
    ReceivedAt  time.Time         // When we received this
    Metadata    map[string]string // Additional metadata
}

// Source is the base interface for all sources
type Source interface {
    // Name returns the unique name of this source
    Name() string
    
    // Type returns the type of this source (rss, email, web)
    Type() string
    
    // Start begins the source operation
    Start(ctx context.Context) error
    
    // Stop gracefully stops the source
    Stop() error
}

// ActiveSource is for sources that actively fetch data (RSS, Web)
type ActiveSource interface {
    Source
    
    // Fetch retrieves new entries from the source
    Fetch(ctx context.Context) ([]Entry, error)
    
    // Interval returns the fetch interval
    Interval() time.Duration
}

// PassiveSource is for sources that passively receive data (Email)
type PassiveSource interface {
    Source
    
    // SetHandler sets the callback for received entries
    SetHandler(handler EntryHandler)
}

// EntryHandler is called when new entries are received
type EntryHandler func(entries []Entry)

// Manager manages multiple sources
type Manager struct {
    sources []Source
    handler EntryHandler
}

func NewManager(handler EntryHandler) *Manager
func (m *Manager) Register(s Source) error
func (m *Manager) Start(ctx context.Context) error
func (m *Manager) Stop() error
```

#### 2.1 RSS Source (`internal/source/rss`)

**模式：主动拉取**

```go
// rss.go
package rss

import (
    "context"
    "time"
    
    "github.com/mmcdole/gofeed"
    "anyfeed/internal/config"
    "anyfeed/internal/source"
)

// Source implements RSS/Atom feed fetching (Active)
type Source struct {
    config   config.FeedConfig
    parser   *gofeed.Parser
    handler  source.EntryHandler
    lastETag string
    stopCh   chan struct{}
}

// New creates a new RSS source
func New(cfg config.FeedConfig, handler source.EntryHandler) *Source

func (s *Source) Name() string
func (s *Source) Type() string
func (s *Source) Interval() time.Duration
func (s *Source) Fetch(ctx context.Context) ([]source.Entry, error)
func (s *Source) Start(ctx context.Context) error  // Starts scheduler
func (s *Source) Stop() error
```

#### 2.2 Email Source (`internal/source/email`)

**模式：被动接收** - 启动 SMTP 服务器，等待邮件到达

```go
// email.go
package email

import (
    "context"
    "regexp"
    
    "anyfeed/internal/config"
    "anyfeed/internal/source"
)

// MatchRule represents an email matching rule
type MatchRule struct {
    Field   string         // from, to, subject
    Pattern *regexp.Regexp // Compiled pattern
}

// Source implements email receiving via SMTP (Passive)
type Source struct {
    config  config.FeedConfig
    rules   []MatchRule
    handler source.EntryHandler
    server  *smtpServer  // internal SMTP server
}

// New creates a new Email source
func New(cfg config.FeedConfig, smtpPort int) (*Source, error)

func (s *Source) Name() string
func (s *Source) Type() string
func (s *Source) SetHandler(handler source.EntryHandler)

// Start starts the SMTP server to receive emails
// This is a blocking operation - emails are processed as they arrive
func (s *Source) Start(ctx context.Context) error
func (s *Source) Stop() error

// onEmailReceived is called internally when an email arrives
func (s *Source) onEmailReceived(email *Email)

// matchEmail checks if an email matches the configured rules
func (s *Source) matchEmail(email *Email) bool

// ParseMatch parses match expression like "from:*@example.com"
func ParseMatch(expr string) ([]MatchRule, error)
```

#### 2.3 Web Source (`internal/source/web`)

**模式：主动拉取**

```go
// web.go
package web

import (
    "context"
    "crypto/sha256"
    
    "github.com/gocolly/colly/v2"
    "anyfeed/internal/config"
    "anyfeed/internal/source"
)

// Source implements web page change detection (Active)
type Source struct {
    config       config.FeedConfig
    collector    *colly.Collector
    handler      source.EntryHandler
    lastHash     string // Hash of last content
    stopCh       chan struct{}
}

// New creates a new Web source
func New(cfg config.FeedConfig, handler source.EntryHandler) *Source

func (s *Source) Name() string
func (s *Source) Type() string
func (s *Source) Interval() time.Duration
func (s *Source) Fetch(ctx context.Context) ([]source.Entry, error)
func (s *Source) Start(ctx context.Context) error  // Starts scheduler
func (s *Source) Stop() error

// extractContent extracts content using CSS selector
func (s *Source) extractContent(html string) (string, error)

// detectChanges compares content and returns changes if any
func (s *Source) detectChanges(content string) (bool, string)
```

### 3. Store Module (`internal/store`)

负责数据持久化和查询。

```go
// store.go
package store

import (
    "context"
    "time"
    
    "anyfeed/internal/source"
)

// Store defines the interface for entry storage
type Store interface {
    // SaveEntries saves entries to storage (deduplicates by ID)
    SaveEntries(ctx context.Context, entries []source.Entry) error
    
    // GetEntries retrieves entries with optional filters
    GetEntries(ctx context.Context, opts QueryOptions) ([]source.Entry, error)
    
    // GetEntryByID retrieves a single entry by ID
    GetEntryByID(ctx context.Context, id string) (*source.Entry, error)
    
    // DeleteOldEntries removes entries beyond retention limit
    DeleteOldEntries(ctx context.Context, sourceName string, keepCount int) error
    
    // Close closes the store
    Close() error
}

// QueryOptions defines filtering options for queries
type QueryOptions struct {
    SourceNames []string  // Filter by source names
    Tags        []string  // Filter by tags
    Since       time.Time // Only entries after this time
    Limit       int       // Maximum number of entries
    Offset      int       // Pagination offset
}

// SQLiteStore implements Store using SQLite
type SQLiteStore struct {
    db *sql.DB
}

func NewSQLiteStore(dbPath string) (*SQLiteStore, error)
func (s *SQLiteStore) SaveEntries(ctx context.Context, entries []source.Entry) error
func (s *SQLiteStore) GetEntries(ctx context.Context, opts QueryOptions) ([]source.Entry, error)
func (s *SQLiteStore) GetEntryByID(ctx context.Context, id string) (*source.Entry, error)
func (s *SQLiteStore) DeleteOldEntries(ctx context.Context, sourceName string, keepCount int) error
func (s *SQLiteStore) Close() error
func (s *SQLiteStore) migrate() error // Run database migrations
```

### 4. Server Module (`internal/server`)

HTTP 服务器，提供 RSS 输出和管理接口。

```go
// server.go
package server

import (
    "context"
    "net/http"
    
    "github.com/gorilla/mux"
    "anyfeed/internal/config"
    "anyfeed/internal/store"
)

// Server is the HTTP server for RSS output
type Server struct {
    config     config.Config
    store      store.Store
    router     *mux.Router
    httpServer *http.Server
}

// New creates a new server
func New(cfg config.Config, store store.Store) *Server

// Start starts the HTTP server
func (s *Server) Start(ctx context.Context) error

// Stop gracefully stops the server
func (s *Server) Stop(ctx context.Context) error

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes()

// Handlers
func (s *Server) handleFeed(w http.ResponseWriter, r *http.Request)
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request)
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request)

// Middleware
func (s *Server) authMiddleware(next http.Handler) http.Handler
```

### 5. RSS Generator (`internal/rss`)

生成标准 RSS XML 输出。

```go
// rss.go
package rss

import (
    "encoding/xml"
    "time"
    
    "anyfeed/internal/source"
)

// Feed represents an RSS 2.0 feed
type Feed struct {
    XMLName     xml.Name `xml:"rss"`
    Version     string   `xml:"version,attr"`
    Channel     Channel  `xml:"channel"`
}

// Channel represents an RSS channel
type Channel struct {
    Title       string    `xml:"title"`
    Link        string    `xml:"link"`
    Description string    `xml:"description"`
    PubDate     string    `xml:"pubDate,omitempty"`
    LastBuild   string    `xml:"lastBuildDate"`
    Items       []Item    `xml:"item"`
}

// Item represents an RSS item
type Item struct {
    Title       string `xml:"title"`
    Link        string `xml:"link,omitempty"`
    Description string `xml:"description"`
    Author      string `xml:"author,omitempty"`
    GUID        string `xml:"guid"`
    PubDate     string `xml:"pubDate"`
}

// Generate creates RSS XML from entries
func Generate(title, link, description string, entries []source.Entry) ([]byte, error)

// FormatRFC822 formats time in RFC822 format for RSS
func FormatRFC822(t time.Time) string
```

## Data Models

### SQLite Schema

```sql
-- Database schema for Anyfeed

-- Sources table stores source configurations and state
CREATE TABLE IF NOT EXISTS sources (
    name        TEXT PRIMARY KEY,
    type        TEXT NOT NULL,      -- 'rss', 'email', 'web'
    config_json TEXT NOT NULL,      -- Original config as JSON
    enabled     INTEGER DEFAULT 1,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Entries table stores all received content
CREATE TABLE IF NOT EXISTS entries (
    id          TEXT PRIMARY KEY,   -- SHA256 hash of content
    source_name TEXT NOT NULL,
    title       TEXT NOT NULL,
    content     TEXT,
    url         TEXT,
    author      TEXT,
    published_at DATETIME,
    received_at DATETIME NOT NULL,
    metadata    TEXT,               -- JSON for additional data
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (source_name) REFERENCES sources(name) ON DELETE CASCADE
);

-- Tags table for source tagging
CREATE TABLE IF NOT EXISTS source_tags (
    source_name TEXT NOT NULL,
    tag         TEXT NOT NULL,
    PRIMARY KEY (source_name, tag),
    FOREIGN KEY (source_name) REFERENCES sources(name) ON DELETE CASCADE
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_entries_source_name ON entries(source_name);
CREATE INDEX IF NOT EXISTS idx_entries_published_at ON entries(published_at DESC);
CREATE INDEX IF NOT EXISTS idx_entries_received_at ON entries(received_at DESC);
CREATE INDEX IF NOT EXISTS idx_source_tags_tag ON source_tags(tag);

-- Web source state for change detection
CREATE TABLE IF NOT EXISTS web_state (
    source_name TEXT PRIMARY KEY,
    last_hash   TEXT,
    last_content TEXT,
    checked_at  DATETIME,
    FOREIGN KEY (source_name) REFERENCES sources(name) ON DELETE CASCADE
);
```

## Configuration File Schema

### Complete YAML Configuration

```yaml
# Anyfeed Configuration File

# Server settings
server:
  # HTTP server port (default: 8080)
  port: 8080
  
  # SMTP server port for email receiving (default: 2525)
  smtp_port: 2525
  
  # API key for feed access authentication (optional)
  # If not set, feeds are publicly accessible
  api_key: "your-secret-api-key"

# Storage settings
storage:
  # SQLite database file path (default: ./anyfeed.db)
  path: "./data/anyfeed.db"
  
  # Maximum items to keep per feed (default: 1000)
  max_items_per_feed: 1000

# Feed sources
feeds:
  # RSS/Atom feed example (Active - scheduled fetching)
  - name: "tech-news"
    type: rss
    url: "https://example.com/feed.xml"
    interval: 30m  # Fetch interval (supports: s, m, h)
    tags: ["tech", "news"]
    enabled: true
  
  # Email subscription example (Passive - SMTP receives)
  - name: "newsletter"
    type: email
    match: "from:*@newsletter.com"  # Email filter expression
    tags: ["newsletter"]
    enabled: true
  
  # Web change monitoring example (Active - scheduled fetching)
  - name: "product-updates"
    type: web
    url: "https://example.com/changelog"
    selector: ".changelog-list"  # CSS selector for content
    interval: 1h
    tags: ["product"]
    enabled: true

# Output endpoints
output:
  # Aggregate all feeds
  - path: "/feed/all"
    limit: 100
  
  # Filter by source names
  - path: "/feed/tech"
    sources: ["tech-news"]
    limit: 50
  
  # Filter by tags
  - path: "/feed/newsletters"
    tags: ["newsletter"]
    limit: 50
```

### Email Match Expression Syntax

```
# Supported match expressions:

# Match by sender
from:user@example.com
from:*@example.com           # Wildcard matching

# Match by recipient
to:rss@yourdomain.com

# Match by subject
subject:*Newsletter*         # Contains "Newsletter"
subject:Weekly Update        # Exact match

# Combine with AND (comma)
from:*@newsletter.com,subject:*weekly*
```

## Project Directory Structure

```
anyfeed/
├── cmd/
│   └── anyfeed/
│       └── main.go              # Application entry point
├── internal/
│   ├── config/
│   │   ├── config.go            # Configuration types and loading
│   │   └── config_test.go
│   ├── source/
│   │   ├── source.go            # Source interfaces (Active/Passive)
│   │   ├── entry.go             # Entry type definition
│   │   ├── manager.go           # Source manager
│   │   ├── rss/
│   │   │   ├── rss.go           # RSS source (Active)
│   │   │   └── rss_test.go
│   │   ├── email/
│   │   │   ├── email.go         # Email source (Passive)
│   │   │   ├── smtp.go          # SMTP server wrapper
│   │   │   ├── match.go         # Match expression parser
│   │   │   └── email_test.go
│   │   └── web/
│   │       ├── web.go           # Web source (Active)
│   │       └── web_test.go
│   ├── store/
│   │   ├── store.go             # Store interface
│   │   ├── sqlite.go            # SQLite implementation
│   │   ├── migrations.go        # Database migrations
│   │   └── store_test.go
│   ├── server/
│   │   ├── server.go            # HTTP server
│   │   ├── handlers.go          # HTTP handlers
│   │   ├── middleware.go        # Auth middleware
│   │   └── server_test.go
│   └── rss/
│       ├── rss.go               # RSS XML generator
│       └── rss_test.go
├── configs/
│   └── example.yaml             # Example configuration file
├── scripts/
│   └── build.sh                 # Build script
├── .gitignore
├── go.mod
├── go.sum
├── LICENSE
├── README.md
└── Makefile
```

## Error Handling Strategy

### Error Types

```go
// internal/errors/errors.go
package errors

import "errors"

var (
    // Configuration errors
    ErrInvalidConfig     = errors.New("invalid configuration")
    ErrMissingRequired   = errors.New("missing required field")
    
    // Source errors
    ErrFetchFailed       = errors.New("fetch failed")
    ErrParseError        = errors.New("parse error")
    ErrTimeout           = errors.New("operation timeout")
    
    // Store errors
    ErrNotFound          = errors.New("entry not found")
    ErrDuplicateEntry    = errors.New("duplicate entry")
    ErrDatabaseError     = errors.New("database error")
    
    // Server errors
    ErrUnauthorized      = errors.New("unauthorized")
    ErrBadRequest        = errors.New("bad request")
)
```

### Error Handling Principles

1. **Source Errors**: Log and continue - one failing source shouldn't stop others
2. **Store Errors**: Retry with exponential backoff for transient errors
3. **Server Errors**: Return appropriate HTTP status codes
4. **Config Errors**: Fail fast on startup with clear error messages

## Testing Strategy

### Unit Tests
- Config parsing and validation
- Match expression parsing for emails
- RSS XML generation
- Individual source logic (with mocked HTTP)

### Integration Tests
- Store operations with in-memory SQLite
- HTTP server endpoints
- Full source-store-output pipeline

### Test Data
- Sample RSS feeds in `testdata/`
- Sample HTML pages for web scraping tests
- Sample emails for email matching tests

## Implementation Notes

### Concurrency Model
- RSS/Web sources: Each runs timer-based fetch in its goroutine
- Email source: SMTP server runs in dedicated goroutine, callbacks on receive
- Shared store access protected with database transactions
- Context-based cancellation for graceful shutdown

### Performance Considerations
- HTTP client connection pooling
- Batch database inserts for multiple entries
- ETag/Last-Modified support for RSS to reduce bandwidth

### Security Considerations
- Sanitize HTML content before storage
- Rate limiting on SMTP server
- Configurable API key authentication
- No shell command execution from fetched content

### Deployment
- Single binary with embedded migrations
- Supports `--config` flag for config file path
- Environment variable overrides for sensitive settings
- Systemd service file provided
