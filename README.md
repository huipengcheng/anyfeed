<div align="center">

# Anyfeed

**Subscribe to anything.**

Turn anything into an RSS feed - in seconds.

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Self-hostable](https://img.shields.io/badge/self--hostable-yes-green)]()
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)]()
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)]()

</div>

## Features

- **Multiple Data Sources**: Subscribe to RSS/Atom feeds, monitor web pages for changes, and receive emails
- **Unified RSS Output**: All sources are normalized and served as standard RSS 2.0 feeds
- **Flexible Filtering**: Filter feeds by source names or tags
- **Simple Deployment**: Single binary with SQLite storage, no external dependencies
- **Low Resource Usage**: Efficient Go implementation with minimal memory footprint
- **API Key Authentication**: Optional authentication for feed endpoints

## Quick Start

### Installation

#### From Source

```bash
# Clone the repository
git clone https://github.com/huipeng/anyfeed.git
cd anyfeed

# Build
make build

# Run
./build/anyfeed --config configs/example.yaml
```

#### Using Go Install

```bash
go install github.com/huipeng/anyfeed/cmd/anyfeed@latest
```

### Configuration

Create a configuration file (e.g., `config.yaml`):

```yaml
server:
  port: 8080
  smtp_port: 2525
  api_key: "your-secret-key"  # Optional

storage:
  path: "./data/anyfeed.db"
  max_items_per_feed: 1000

feeds:
  # RSS Feed
  - name: "tech-news"
    type: rss
    url: "https://hnrss.org/frontpage"
    interval: 30m
    tags: ["tech", "news"]
    enabled: true

  # Web Page Monitoring
  - name: "changelog"
    type: web
    url: "https://example.com/changelog"
    selector: ".changelog-list"
    interval: 1h
    tags: ["updates"]
    enabled: true

  # Email Subscription
  - name: "newsletter"
    type: email
    match: "from:*@newsletter.com"
    tags: ["newsletter"]
    enabled: true

output:
  - path: "/feed/all"
    title: "All Feeds"
    limit: 100

  - path: "/feed/tech"
    sources: ["tech-news"]
    tags: ["tech"]
    limit: 50
```

### Running

```bash
# Run with configuration file
./anyfeed --config config.yaml

# Enable debug logging
./anyfeed --config config.yaml --debug
```

### Accessing Feeds

Once running, access your feeds at:

- All feeds: `http://localhost:8080/feed/all`
- Health check: `http://localhost:8080/health`
- Statistics: `http://localhost:8080/stats`

If API key is configured, include it in requests:

```bash
# Using header
curl -H "X-API-Key: your-secret-key" http://localhost:8080/feed/all

# Using query parameter
curl "http://localhost:8080/feed/all?api_key=your-secret-key"
```

## Source Types

### RSS/Atom Feeds

Subscribe to any RSS or Atom feed:

```yaml
- name: "example-rss"
  type: rss
  url: "https://example.com/feed.xml"
  interval: 30m  # Fetch interval
  enabled: true
```

### Web Page Monitoring

Monitor web pages for changes using CSS selectors:

```yaml
- name: "product-updates"
  type: web
  url: "https://example.com/changelog"
  selector: ".changelog-list"  # CSS selector
  interval: 1h
  enabled: true
```

### Email Subscription

Receive emails via built-in SMTP server:

```yaml
- name: "newsletter"
  type: email
  match: "from:*@newsletter.com"  # Filter expression
  enabled: true
```

#### Email Match Expressions

```
from:user@example.com       # Exact match
from:*@example.com          # Wildcard
to:rss@yourdomain.com       # Match recipient
subject:*Newsletter*        # Match subject
from:*@example.com,subject:*weekly*  # Multiple conditions (AND)
```

## API Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /health` | Health check (always accessible) |
| `GET /stats` | Source and entry statistics |
| `GET /feed/*` | RSS feed endpoints (configured in output) |

## Deployment

### Docker Compose

```yaml
services:
  anyfeed:
    image: huipengcheng/anyfeed:latest
    container_name: anyfeed
    restart: unless-stopped
    ports:
      - "8080:8080"
      - "2525:2525"
    volumes:
      - ./data:/app/data
      - ./config.yaml:/app/configs/config.yaml:ro
    environment:
      - TZ=Asia/Shanghai
```

### systemd

1. Copy the binary and service file:
```bash
cp build/anyfeed /usr/local/bin/
cp configs/anyfeed.service /etc/systemd/system/
```

2. Create user and directories:
```bash
useradd -r -s /bin/false anyfeed
mkdir -p /etc/anyfeed /var/lib/anyfeed
chown -R anyfeed:anyfeed /var/lib/anyfeed
```

3. Create config at `/etc/anyfeed/config.yaml` and start:
```bash
systemctl enable --now anyfeed
```

### Binary

Download pre-built binary from [Releases](https://github.com/huipeng/anyfeed/releases) or build from source:

```bash
make build
./build/anyfeed --config config.yaml
```

## Development

### Building

```bash
make build      # Build for current platform
make test       # Run tests
make build-all  # Build for all platforms
```

### Project Structure

```
anyfeed/
├── cmd/anyfeed/          # Application entry point
├── internal/
│   ├── config/           # Configuration handling
│   ├── source/           # Data source implementations
│   │   ├── rss/          # RSS/Atom fetcher
│   │   ├── web/          # Web page monitor
│   │   └── email/        # Email receiver
│   ├── store/            # SQLite storage
│   ├── server/           # HTTP server
│   └── rss/              # RSS XML generator
├── configs/              # Configuration examples
└── scripts/              # Build scripts
```

## License

MIT License - see [LICENSE](LICENSE) for details.
