# AGENTS.md

This file provides guidance to Qoder (qoder.com) when working with code in this repository.

## Build & Test Commands

```bash
make build      # Build binary to ./build/anyfeed
make test       # Run all tests with race detector
make run-dev    # Run with example config (hot reload via go run)
make fmt        # Format code
make tidy       # Clean up go.mod

# Run single test
go test -v -run TestName ./internal/source/rss/

# Run with debug logging
./build/anyfeed --config configs/example.yaml --debug
```

## Architecture

### Source System (internal/source/)

Two interfaces define data acquisition:

- **ActiveSource**: Pulls data on interval (RSS feeds, web pages)
  - `Fetch(ctx) ([]*Entry, error)` - retrieves new entries
  - `Interval() time.Duration` - fetch frequency
  
- **PassiveSource**: Receives data via callback (email SMTP)
  - `SetHandler(EntryHandler)` - callback for incoming entries

All sources produce `*Entry` structs that are stored uniformly. The `Manager` coordinates source lifecycle and triggers fetches.

### Storage (internal/store/)

`Store` interface with SQLite implementation (`sqlite.go`). Key operations:
- `SaveEntries()` - upserts with deduplication by entry ID
- `GetEntries()` - query with filtering by source/tags
- `SaveWebState()`/`GetWebState()` - tracks web page content hash for change detection

### Server (internal/server/)

HTTP endpoints via gorilla/mux:
- `/health` - liveness check
- `/stats` - entry counts per source
- `/feed/*` - RSS output (configured in `output:` section)

Optional API key auth via `X-API-Key` header or `api_key` query param.

### RSS Output (internal/rss/)

Generates RSS 2.0 XML from stored entries. Filters by source names or tags based on output config.

## Git Workflow

- Branch protection on `main` - all changes via PR
- CI runs on all branches, deploy only on `main` push or `v*` tags
- Commit format: `<type>(<scope>): <description>` (feat, fix, refactor, docs, test, chore, ci)

## Deployment

Docker images pushed to:
- Docker Hub: `huipengcheng/anyfeed`
- ACR: configured via GitHub secrets

Image tags:
- `:dev` on push to main
- `:vX.Y.Z` + `:latest` on version tags
