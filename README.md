# Heimdex Agent

Heimdex Local Agent is the on-device runtime that indexes local video sources, enables playback via a localhost streaming proxy, and syncs metadata to the Heimdex cloud.

## Features (v0)

- **Always-on Agent**: Runs as a background process with system tray UI
- **Local Catalog**: SQLite database for sources, files, and jobs
- **Manual Scan**: Scan folders to discover video files (.mp4, .mov, .mkv)
- **Playback Proxy**: HTTP Range-supporting proxy for video playback
- **Cloud Stubs**: Prepared interfaces for future cloud sync

## Requirements

- Go 1.21 or later
- macOS 10.15+ or Windows 10+

## Quick Start

### Development

```bash
# Install dependencies
make deps

# Run in development mode
make dev
```

The agent will start and display:
- Auth token for API access
- API URL (http://127.0.0.1:8787)
- System tray icon

### Build

```bash
# Build for current platform
make build

# Build for all platforms
make build-all

# Run tests
make test

# Run linter
make lint
```

## API Endpoints

All endpoints (except `/health`) require Bearer token authentication.

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check (no auth required) |
| `/status` | GET | Agent status and stats |
| `/sources` | GET | List all sources |
| `/sources/folders` | POST | Add a folder source |
| `/scan` | POST | Start a scan job |
| `/jobs` | GET | List recent jobs |
| `/playback/file?file_id=...` | GET | Stream video file (Range support) |

### Example Usage

```bash
# Get the auth token from agent startup output
TOKEN="your-auth-token"

# Check health
curl http://127.0.0.1:8787/health

# Add a folder
curl -X POST http://127.0.0.1:8787/sources/folders \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"path": "/path/to/videos", "display_name": "My Videos"}'

# Scan the folder
curl -X POST http://127.0.0.1:8787/scan \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"source_id": "source-id-here"}'

# List jobs
curl http://127.0.0.1:8787/jobs \
  -H "Authorization: Bearer $TOKEN"

# Playback with Range support
curl -H "Range: bytes=0-1023" \
  -H "Authorization: Bearer $TOKEN" \
  "http://127.0.0.1:8787/playback/file?file_id=file-id-here"
```

## Configuration

Configuration via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `HEIMDEX_PORT` | 8787 | HTTP server port |
| `HEIMDEX_LOG_LEVEL` | info | Log level (debug, info, warn, error) |
| `HEIMDEX_DATA_DIR` | ~/.heimdex | Data directory for database |

## Project Structure

```
heimdex-agent/
├── cmd/
│   └── agent/
│       └── main.go           # Entry point
├── internal/
│   ├── api/                  # HTTP API server
│   ├── catalog/              # Catalog service (sources, files, jobs)
│   ├── cloud/                # Cloud communication (stubs)
│   ├── config/               # Configuration
│   ├── db/                   # SQLite database
│   ├── logging/              # Structured logging
│   ├── pipeline/             # Processing pipeline (stub)
│   ├── playback/             # Video playback proxy
│   ├── ui/                   # System tray UI
│   └── watcher/              # File watcher (stub)
├── packaging/
│   ├── macos/                # macOS installer files
│   └── windows/              # Windows installer files
├── docs/
│   ├── architecture.md       # Architecture documentation
│   ├── threat_model.md       # Security considerations
│   └── api.md                # API documentation
├── Makefile
├── go.mod
└── README.md
```

## Security

- **Localhost Only**: API binds to 127.0.0.1 only
- **Bearer Token**: All protected endpoints require authentication
- **No Secrets in Logs**: Tokens are sanitized in log output

## Limitations (v0)

- No real-time file watching (manual scan only)
- No cloud upload (stubs only)
- No GPU-accelerated processing
- No scene detection or transcription
- Single job runner (max 1 concurrent job)

## Next Steps (v1)

- [ ] Real-time file watching with fsnotify
- [ ] Cloud authentication and upload
- [ ] Web app handshake for browser access
- [ ] Windows service mode
- [ ] macOS notarization

## License

Proprietary - Heimdex Inc.
