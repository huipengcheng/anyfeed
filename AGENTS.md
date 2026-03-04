# AGENTS.md

This file provides guidance to Qoder (qoder.com) when working with code in this repository.

## Build & Test Commands

```bash
make build      # Build binary to ./build/anyfeed
make test       # Run all tests with race detector (-v -race ./...)
make run-dev    # Run with example config via go run
make fmt        # Format code (go fmt ./...)
make tidy       # Clean up go.mod
make build-all  # Cross-compile for linux/darwin/windows amd64 + darwin arm64

# Run single test
go test -v -run TestName ./internal/source/rss/

# Run with debug logging
./build/anyfeed --config configs/example.yaml --debug
```

No linter is configured. Use `make fmt` before committing.

## Architecture

Single-binary Go application. Entry point is `cmd/anyfeed/main.go`, which wires together config, store, sources, and the HTTP server. Graceful shutdown via SIGINT/SIGTERM with a 30s timeout.

### Data Flow

```
Sources (RSS/Web/Email) --> *source.Entry --> Store (SQLite) --> Server (HTTP) --> RSS XML output
```

All source types produce `*source.Entry` structs (defined in `internal/source/entry.go`). The entry `ID` is a SHA256 hash of `sourceName|title|url|publishedAt`, which provides content-based deduplication at the storage layer (`INSERT OR IGNORE`).

### Source System (internal/source/)

Three source types share a common interface hierarchy in `source.go`:

- **`Source`** (base): `Name()`, `Type()`, `Start(ctx)`, `Stop()`
- **`ActiveSource`** extends Source: `Fetch(ctx) ([]*Entry, error)`, `Interval() time.Duration`
  - `rss.Source` - fetches RSS/Atom feeds via `gofeed`, uses ETag/If-Modified-Since for conditional requests
  - `web.Source` - fetches web pages, extracts content via CSS selector (`goquery`), detects changes via SHA256 content hashing. Persists state via `StateStore` interface (satisfied by `store.SQLiteStore`)
- **`PassiveSource`** extends Source: `SetHandler(EntryHandler)`
  - `email.Source` - receives emails via built-in SMTP server. Filters by match rules (from/to/subject with wildcards). Each email source creates its own SMTP server instance on the configured port.

The `Manager` (`manager.go`) orchestrates all sources: runs ticker-based fetch loops for ActiveSources, sets handlers on PassiveSources. The handler callback (set in `main.go`) saves entries to the store and runs retention cleanup.

### Storage (internal/store/)

`Store` interface in `store.go` with SQLite implementation in `sqlite.go` (using pure-Go `modernc.org/sqlite`, no CGO). Migrations are append-only DDL statements in `migrations.go` (all `CREATE TABLE/INDEX IF NOT EXISTS`).

SQLite is configured with WAL mode and foreign keys. Tables: `entries`, `entry_tags`, `source_tags`, `web_state`, `schema_version`.

### Server (internal/server/)

HTTP server using `gorilla/mux`. Routes are set up dynamically from `output:` config entries. Auth middleware (`middleware.go`) supports three methods: `X-API-Key` header, `api_key` query param, or `Authorization: Bearer` header. The `/health` endpoint is always unauthenticated.

Feed handlers query the store with `QueryOptions` (source names, tags, limit) and delegate XML generation to `internal/rss/`.

### Configuration (internal/config/)

YAML config loaded at startup. `Config.Validate()` checks port ranges, feed name uniqueness, type-specific required fields (url for rss/web, selector for web, match for email), and validates output source references exist.

## Git Workflow

- Branch protection on `main` - all changes via PR
- CI (`.github/workflows/ci.yml`) runs `go test -v -race ./...` + build check on all branches
- Commit format: `<type>(<scope>): <description>` (feat, fix, refactor, docs, test, chore, ci)

## Docker

Dockerfile uses multi-stage build (golang:1.22-alpine builder, alpine:3.19 runtime). Runs as non-root `anyfeed` user. Exposes ports 8080 (HTTP) and 2525 (SMTP). Config and data are mounted as volumes.
