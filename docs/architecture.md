# Heimdex Agent Architecture

## Overview

Heimdex Agent is a desktop application that runs as an always-on background service, providing local video indexing and playback capabilities with cloud synchronization.

## System Components

```
┌─────────────────────────────────────────────────────────────────┐
│                        System Tray UI                           │
│  (Status, Sources, Pause/Resume, Add Folder, Quit)             │
└─────────────────────────────────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Main Application                            │
│  - Configuration loading                                         │
│  - Dependency injection                                          │
│  - Graceful shutdown                                             │
└─────────────────────────────────────────────────────────────────┘
                               │
          ┌────────────────────┼────────────────────┐
          ▼                    ▼                    ▼
┌──────────────┐      ┌──────────────┐      ┌──────────────┐
│  HTTP API    │      │  Job Runner  │      │ Cloud Client │
│  Server      │      │              │      │   (Stub)     │
│  :8787       │      │  Background  │      │              │
│  127.0.0.1   │      │  Processing  │      │  Upload &    │
│  only        │      │              │      │  Sync        │
└──────────────┘      └──────────────┘      └──────────────┘
          │                    │
          └────────────────────┼────────────────────┐
                               ▼                    │
┌─────────────────────────────────────────────────────────────────┐
│                      Catalog Service                             │
│  - Source management                                             │
│  - File scanning                                                 │
│  - Fingerprint computation                                       │
└─────────────────────────────────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Repository Layer                            │
│  - CRUD operations for Sources, Files, Jobs, Config             │
└─────────────────────────────────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│                      SQLite Database                             │
│  - WAL mode enabled                                              │
│  - Embedded migrations                                           │
│  - Persistent storage                                            │
└─────────────────────────────────────────────────────────────────┘
```

## Key Design Decisions

### 1. Localhost-Only Binding
The HTTP API server binds exclusively to `127.0.0.1` to ensure no external access. This is a security requirement - the agent should never be accessible from the network.

### 2. Bearer Token Authentication
A random 64-character token is generated on first run and stored in the database. All API endpoints (except `/health`) require this token in the `Authorization: Bearer <token>` header.

### 3. Pure Go SQLite
We use `modernc.org/sqlite` instead of `mattn/go-sqlite3` because:
- No CGO dependency (simpler cross-compilation)
- Single binary distribution
- WAL mode for better concurrent read performance

### 4. Dependency Injection
All services receive their dependencies via constructor injection:
- API routes depend on interfaces, not concrete types
- Repository layer abstracts database access
- Logger is injected throughout

### 5. Job Runner
Single-threaded job execution (v0):
- Polls for pending jobs every 5 seconds
- Executes one job at a time
- Marks interrupted jobs as failed on restart
- Supports pause/resume

### 6. Video File Fingerprinting
SHA-256 hash of the first 64KB of each file:
- Fast to compute (only reads 64KB)
- Sufficient for deduplication detection
- Consistent across machines

## Data Flow

### Adding a Folder
1. User triggers "Add Folder" from UI or API
2. CatalogService validates path exists and is a directory
3. Source record created in database
4. Scan job created (if auto-scan enabled)

### Scanning a Source
1. Job runner picks up pending scan job
2. Walks directory tree, skipping hidden folders
3. For each video file (.mp4, .mov, .mkv):
   - Reads file metadata (size, mtime)
   - Computes fingerprint (SHA-256 of first 64KB)
   - Upserts file record
4. Updates job progress and status

### Video Playback
1. Client requests `/playback/file?file_id=...`
2. API looks up file record
3. Checks if source is present (drive connected)
4. PlaybackServer opens file and streams with Range support
5. Returns 206 Partial Content for range requests

## Configuration

Environment variables:
- `HEIMDEX_PORT`: HTTP server port (default: 8787)
- `HEIMDEX_LOG_LEVEL`: Logging level (default: info)
- `HEIMDEX_DATA_DIR`: Data directory (default: ~/.heimdex)

Database config table stores:
- `device_id`: Unique device identifier
- `auth_token`: API authentication token

## Future Considerations (v1+)

1. **Real-time File Watching**: Replace manual scan with fsnotify
2. **Cloud Sync**: Implement actual upload to Heimdex cloud
3. **GPU Processing**: Scene detection, embedding generation
4. **Windows Service**: Proper Windows service mode
5. **Notarization**: macOS app notarization for distribution
