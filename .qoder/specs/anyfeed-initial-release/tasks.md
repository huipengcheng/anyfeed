# Anyfeed - Implementation Tasks

## Overview

基于 MVP 优先原则，实现任务按以下优先级排列：
1. **Phase 1**: 核心基础设施（配置、存储、HTTP 服务器）
2. **Phase 2**: RSS 源支持（最基础的数据源）
3. **Phase 3**: RSS 输出（完成最小可用版本）
4. **Phase 4**: 网页监控支持
5. **Phase 5**: 邮件订阅支持
6. **Phase 6**: 增强功能和优化

---

## Phase 1: Core Infrastructure

### Task 1.1: Project Setup
- [ ] 1.1.1 **Initialize Go module**
  - Create `go.mod` with module name `github.com/huipeng/anyfeed`
  - Set Go version to 1.22+
  - File: `go.mod`

- [ ] 1.1.2 **Create project directory structure**
  - Create all directories as per design document
  - Directories: `cmd/anyfeed/`, `internal/{config,source,store,server,rss}/`

- [ ] 1.1.3 **Create Makefile**
  - Build target: `make build`
  - Test target: `make test`
  - Run target: `make run`
  - File: `Makefile`

- [ ] 1.1.4 **Create .gitignore**
  - Ignore build artifacts, database files, config files with secrets
  - File: `.gitignore`

### Task 1.2: Configuration Module
- [ ] 1.2.1 **Define configuration types**
  - Implement `Config`, `ServerConfig`, `FeedConfig`, `OutputConfig`, `StorageConfig`
  - File: `internal/config/config.go`

- [ ] 1.2.2 **Implement config loading**
  - Load YAML file using `gopkg.in/yaml.v3`
  - Support default values
  - File: `internal/config/config.go`

- [ ] 1.2.3 **Implement config validation**
  - Validate required fields
  - Validate feed type specific fields
  - Validate output paths
  - File: `internal/config/config.go`

- [ ] 1.2.4 **Write config tests**
  - Test YAML parsing
  - Test validation logic
  - Test default values
  - File: `internal/config/config_test.go`

- [ ] 1.2.5 **Create example configuration**
  - Complete example with all options documented
  - File: `configs/example.yaml`

### Task 1.3: Storage Module
- [ ] 1.3.1 **Define Store interface**
  - Define `Store` interface with CRUD methods
  - Define `QueryOptions` struct
  - File: `internal/store/store.go`

- [ ] 1.3.2 **Define Entry type**
  - Create shared `Entry` struct
  - File: `internal/source/entry.go`

- [ ] 1.3.3 **Implement SQLite store**
  - Use `modernc.org/sqlite` (CGO-free)
  - Implement connection management
  - File: `internal/store/sqlite.go`

- [ ] 1.3.4 **Implement database migrations**
  - Create tables: sources, entries, source_tags, web_state
  - Create indexes
  - File: `internal/store/migrations.go`

- [ ] 1.3.5 **Implement SaveEntries**
  - Handle duplicates (upsert)
  - Batch insert for performance
  - File: `internal/store/sqlite.go`

- [ ] 1.3.6 **Implement GetEntries**
  - Filter by source names, tags, time range
  - Support pagination (limit, offset)
  - File: `internal/store/sqlite.go`

- [ ] 1.3.7 **Implement DeleteOldEntries**
  - Delete entries beyond retention limit
  - Keep most recent entries per source
  - File: `internal/store/sqlite.go`

- [ ] 1.3.8 **Write store tests**
  - Test all CRUD operations
  - Test query filtering
  - Use in-memory SQLite for tests
  - File: `internal/store/store_test.go`

### Task 1.4: HTTP Server Foundation
- [ ] 1.4.1 **Implement basic HTTP server**
  - Use `net/http` with `gorilla/mux`
  - Support graceful shutdown
  - File: `internal/server/server.go`

- [ ] 1.4.2 **Implement health check endpoint**
  - `GET /health` - returns 200 OK
  - File: `internal/server/handlers.go`

- [ ] 1.4.3 **Implement API key authentication middleware**
  - Check `X-API-Key` header or `api_key` query param
  - Skip if no API key configured
  - File: `internal/server/middleware.go`

---

## Phase 2: RSS Source Support

### Task 2.1: Source Module Foundation
- [ ] 2.1.1 **Define Source interfaces**
  - Define `Source`, `ActiveSource`, `PassiveSource` interfaces
  - File: `internal/source/source.go`

- [ ] 2.1.2 **Implement Source Manager**
  - Register and manage multiple sources
  - Start/Stop all sources
  - File: `internal/source/manager.go`

### Task 2.2: RSS Source Implementation
- [ ] 2.2.1 **Implement RSS source**
  - Use `gofeed` library for parsing
  - Support RSS 1.0, 2.0, and Atom
  - File: `internal/source/rss/rss.go`

- [ ] 2.2.2 **Implement interval-based fetching**
  - Use `time.Ticker` for scheduling
  - Support context cancellation
  - File: `internal/source/rss/rss.go`

- [ ] 2.2.3 **Implement ETag/Last-Modified support**
  - Cache ETag and Last-Modified headers
  - Send conditional requests to reduce bandwidth
  - File: `internal/source/rss/rss.go`

- [ ] 2.2.4 **Convert RSS items to Entry**
  - Map gofeed.Item to source.Entry
  - Generate unique ID (hash of guid + feed name)
  - File: `internal/source/rss/rss.go`

- [ ] 2.2.5 **Write RSS source tests**
  - Test parsing various feed formats
  - Mock HTTP responses
  - File: `internal/source/rss/rss_test.go`

---

## Phase 3: RSS Output (MVP Complete)

### Task 3.1: RSS Generator
- [ ] 3.1.1 **Implement RSS 2.0 XML generator**
  - Define Feed, Channel, Item structs with XML tags
  - Generate valid RSS 2.0 XML
  - File: `internal/rss/rss.go`

- [ ] 3.1.2 **Implement date formatting**
  - Format dates in RFC822 format
  - Handle timezone correctly
  - File: `internal/rss/rss.go`

- [ ] 3.1.3 **Write RSS generator tests**
  - Validate generated XML
  - File: `internal/rss/rss_test.go`

### Task 3.2: Feed Output Endpoints
- [ ] 3.2.1 **Implement dynamic feed endpoint**
  - Parse output config to create routes
  - Support source name filtering
  - Support tag filtering
  - File: `internal/server/handlers.go`

- [ ] 3.2.2 **Set correct Content-Type**
  - Return `application/rss+xml; charset=utf-8`
  - File: `internal/server/handlers.go`

### Task 3.3: Application Entry Point
- [ ] 3.3.1 **Implement main.go**
  - Parse command line flags (`--config`)
  - Load configuration
  - Initialize store
  - Start source manager
  - Start HTTP server
  - Handle SIGINT/SIGTERM for graceful shutdown
  - File: `cmd/anyfeed/main.go`

- [ ] 3.3.2 **Add logging**
  - Use `log/slog` for structured logging
  - Log startup, shutdown, errors, fetch events
  - File: `cmd/anyfeed/main.go`

### Task 3.4: MVP Testing
- [ ] 3.4.1 **Write integration test**
  - Test full flow: config -> fetch -> store -> output
  - File: `internal/server/server_test.go`

- [ ] 3.4.2 **Manual testing**
  - Test with real RSS feeds
  - Verify output in RSS reader

---

## Phase 4: Web Monitoring Support

### Task 4.1: Web Source Implementation
- [ ] 4.1.1 **Implement web source**
  - Use `colly` for HTTP requests
  - Use `goquery` for CSS selector extraction
  - File: `internal/source/web/web.go`

- [ ] 4.1.2 **Implement content extraction**
  - Extract content using CSS selector
  - Handle multiple matches
  - File: `internal/source/web/web.go`

- [ ] 4.1.3 **Implement change detection**
  - Hash content with SHA256
  - Compare with stored hash
  - File: `internal/source/web/web.go`

- [ ] 4.1.4 **Store web state**
  - Save last hash to database
  - Persist across restarts
  - File: `internal/source/web/web.go`

- [ ] 4.1.5 **Generate change entries**
  - Create Entry with change description
  - Include diff or new content
  - File: `internal/source/web/web.go`

- [ ] 4.1.6 **Write web source tests**
  - Mock HTTP responses
  - Test CSS selector extraction
  - Test change detection
  - File: `internal/source/web/web_test.go`

---

## Phase 5: Email Subscription Support

### Task 5.1: Email Match Parser
- [ ] 5.1.1 **Implement match expression parser**
  - Parse `from:`, `to:`, `subject:` prefixes
  - Support wildcard `*` matching
  - Support multiple conditions with `,`
  - File: `internal/source/email/match.go`

- [ ] 5.1.2 **Write match parser tests**
  - Test various match expressions
  - File: `internal/source/email/match_test.go`

### Task 5.2: SMTP Server
- [ ] 5.2.1 **Implement SMTP server wrapper**
  - Use `go-guerrilla` library
  - Configure listening port
  - File: `internal/source/email/smtp.go`

- [ ] 5.2.2 **Implement email handler**
  - Parse received emails
  - Extract subject, from, to, body
  - File: `internal/source/email/smtp.go`

### Task 5.3: Email Source
- [ ] 5.3.1 **Implement email source**
  - Match received emails against rules
  - Convert matched emails to Entry
  - File: `internal/source/email/email.go`

- [ ] 5.3.2 **Handle HTML emails**
  - Extract text from HTML body
  - Support multipart emails
  - File: `internal/source/email/email.go`

- [ ] 5.3.3 **Write email source tests**
  - Mock SMTP connections
  - Test email matching
  - File: `internal/source/email/email_test.go`

---

## Phase 6: Enhancements

### Task 6.1: Data Retention
- [ ] 6.1.1 **Implement automatic cleanup**
  - Run cleanup on schedule
  - Respect `max_items_per_feed` config
  - File: `internal/store/sqlite.go`

### Task 6.2: Monitoring
- [ ] 6.2.1 **Implement stats endpoint**
  - `GET /stats` - return source and entry counts
  - File: `internal/server/handlers.go`

### Task 6.3: Documentation
- [ ] 6.3.1 **Update README**
  - Add installation instructions
  - Add configuration guide
  - Add usage examples
  - File: `README.md`

### Task 6.4: Build and Release
- [ ] 6.4.1 **Create build script**
  - Cross-compile for Linux, macOS, Windows
  - File: `scripts/build.sh`

- [ ] 6.4.2 **Create systemd service file**
  - For Linux deployment
  - File: `configs/anyfeed.service`

---

## Files to Create/Modify

### New Files
| Path | Description |
|------|-------------|
| `go.mod` | Go module definition |
| `Makefile` | Build automation |
| `.gitignore` | Git ignore rules |
| `cmd/anyfeed/main.go` | Application entry point |
| `internal/config/config.go` | Configuration types and loading |
| `internal/config/config_test.go` | Configuration tests |
| `internal/source/entry.go` | Entry type definition |
| `internal/source/source.go` | Source interfaces |
| `internal/source/manager.go` | Source manager |
| `internal/source/rss/rss.go` | RSS source implementation |
| `internal/source/rss/rss_test.go` | RSS source tests |
| `internal/source/web/web.go` | Web source implementation |
| `internal/source/web/web_test.go` | Web source tests |
| `internal/source/email/email.go` | Email source implementation |
| `internal/source/email/smtp.go` | SMTP server wrapper |
| `internal/source/email/match.go` | Email match expression parser |
| `internal/source/email/email_test.go` | Email source tests |
| `internal/store/store.go` | Store interface |
| `internal/store/sqlite.go` | SQLite implementation |
| `internal/store/migrations.go` | Database migrations |
| `internal/store/store_test.go` | Store tests |
| `internal/server/server.go` | HTTP server |
| `internal/server/handlers.go` | HTTP handlers |
| `internal/server/middleware.go` | Auth middleware |
| `internal/server/server_test.go` | Server tests |
| `internal/rss/rss.go` | RSS XML generator |
| `internal/rss/rss_test.go` | RSS generator tests |
| `configs/example.yaml` | Example configuration |
| `scripts/build.sh` | Build script |
| `configs/anyfeed.service` | Systemd service file |

### Files to Modify
| Path | Description |
|------|-------------|
| `README.md` | Update with documentation |

---

## Success Criteria

### MVP (Phase 1-3)
- [ ] Application starts and loads configuration
- [ ] RSS feeds are fetched on schedule
- [ ] Entries are stored in SQLite
- [ ] RSS output is accessible via HTTP
- [ ] API key authentication works
- [ ] Graceful shutdown on SIGINT/SIGTERM

### Full Release (Phase 1-6)
- [ ] All source types work (RSS, Web, Email)
- [ ] All tests pass
- [ ] Documentation is complete
- [ ] Cross-platform builds available

---

## Dependencies

```go
// go.mod dependencies
require (
    github.com/gorilla/mux v1.8.1
    github.com/mmcdole/gofeed v1.3.0
    github.com/gocolly/colly/v2 v2.1.0
    github.com/PuerkitoBio/goquery v1.9.1
    github.com/flashmob/go-guerrilla v1.6.1
    gopkg.in/yaml.v3 v3.0.1
    modernc.org/sqlite v1.29.5
)
```

---

## Estimated Timeline

| Phase | Tasks | Est. Time |
|-------|-------|-----------|
| Phase 1 | Core Infrastructure | 2-3 days |
| Phase 2 | RSS Source | 1-2 days |
| Phase 3 | RSS Output (MVP) | 1 day |
| Phase 4 | Web Monitoring | 1-2 days |
| Phase 5 | Email Subscription | 2-3 days |
| Phase 6 | Enhancements | 1-2 days |
| **Total** | | **8-13 days** |
