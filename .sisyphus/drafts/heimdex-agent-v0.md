# Draft: Heimdex Local Agent v0

## Requirements (confirmed)
- **Technology Stack**: Go with getlantern/systray, modernc.org/sqlite, go-chi/chi v5, log/slog
- **Platform**: Windows + macOS (v0 as regular app, v1 will add Windows service)
- **Security**: localhost only (127.0.0.1), Bearer token auth, no secret logging
- **Architecture**: Clean layering (API -> Service -> Repository -> DB), dependency injection via constructors

## Technical Decisions (from user)
- Systray: getlantern/systray
- SQLite: modernc.org/sqlite (pure Go, no CGO)
- HTTP Router: go-chi/chi v5
- Logging: log/slog (Go 1.21+)
- Service mode: LaunchAgent/startup app for v0

## Repo Structure
- Fully specified by user (see requirements)
- cmd/agent/main.go as entry point
- internal/ for all packages
- packaging/ for platform installers

## Database Schema
- sources: id, type, path, display_name, drive_nickname, present, created_at
- files: id, source_id, path, filename, size, mtime, fingerprint, created_at
- jobs: id, type, status, source_id, progress, error, created_at, updated_at
- config: key, value

## API Endpoints
- GET /health (no auth)
- GET /status (auth required)
- POST /sources/folders (auth required)
- GET /sources (auth required)
- POST /scan (auth required)
- GET /jobs (auth required)
- GET /playback/file?file_id=... (auth required, Range support)

## File Scanning
- Video extensions: .mp4, .mov, .mkv
- Fingerprint: hash of first 64KB

## Confirmed Decisions (from interview)
1. **Test Strategy**: Tests-after with critical path coverage
   - Range parsing logic (unit tests)
   - Scan logic on temp directory (integration tests)
   - DB migrations apply cleanly (migration tests)
2. **Configuration**: Environment variables for runtime (port, log level, data dir) + config table for app state (auth token, device_id)
3. **Video Formats**: Minimal - .mp4, .mov, .mkv only
4. **Fingerprint**: SHA-256 of first 64KB
5. **Job Runner**: 
   - Max 1 concurrent job for v0
   - Jobs persist in SQLite across restarts
   - On startup: mark "running" jobs as "failed" (interrupted)

## Research Findings
- systray: Launched background research
- sqlite: Launched background research  
- chi router: Launched background research
- HTTP Range: Launched background research

## Scope Boundaries
- INCLUDE: Full v0 functionality as specified
- EXCLUDE: Windows service mode (v1), cloud API integration (stubs only), ffmpeg pipeline (stub), installer scripts (plist template only)
