# Heimdex Local Agent v0 - Implementation Plan

## TL;DR

> **Quick Summary**: Build a complete desktop agent in Go with system tray UI, SQLite catalog, localhost HTTP API with Range support, and cloud communication stubs. Greenfield implementation following clean architecture.
> 
> **Deliverables**:
> - Fully functional system tray application (Windows/macOS)
> - SQLite database with migrations for sources, files, jobs, config
> - HTTP API server (127.0.0.1 only) with Bearer auth
> - Video playback proxy with HTTP Range support (206 Partial Content)
> - Cloud communication stubs (logging intent only)
> - Makefile with dev, test, lint, build targets
> 
> **Estimated Effort**: Large (20+ tasks across 8 waves)
> **Parallel Execution**: YES - 8 waves with significant parallelism
> **Critical Path**: Foundation → DB → Repository → Service → API → Main → Tests

---

## Context

### Original Request
Build Heimdex Local Agent v0 - a desktop agent application in Go with:
- System tray UI (Windows/macOS)
- SQLite local database  
- HTTP API server (localhost only)
- Video file playback proxy with Range support
- Cloud communication stubs

### Interview Summary
**Key Discussions**:
- **Test Strategy**: Tests-after with critical path coverage (Range parsing, scan logic, migrations)
- **Configuration**: Env vars for runtime config + SQLite config table for app state
- **Video Formats**: Minimal set - .mp4, .mov, .mkv only
- **Fingerprint**: SHA-256 of first 64KB
- **Job Runner**: Max 1 concurrent, persists across restarts, marks interrupted as failed

**Research Findings**:
- **systray**: Use callback pattern with `systray.Run(onReady, onExit)`, handle clicks via `ClickedCh` channels in goroutines
- **modernc.org/sqlite**: Use separate read/write connections, WAL mode, `_busy_timeout=5000`, single-writer limitation
- **chi v5**: Use `r.Route()` for grouping, standard middleware stack, `middleware.Recoverer` for panics
- **HTTP Range**: Go stdlib `http.ServeContent` handles Range automatically, or implement custom for more control

### Gap Analysis (Self-Review)
**Addressed**:
- All technology choices specified
- Database schema fully defined
- API endpoints fully specified
- Test strategy confirmed

---

## Work Objectives

### Core Objective
Implement a production-ready v0 of Heimdex Local Agent with clean architecture, proper error handling, and comprehensive localhost security.

### Concrete Deliverables
- `cmd/agent/main.go` - Application entry point
- `internal/config/` - Configuration management
- `internal/logging/` - Structured JSON logging
- `internal/db/` - SQLite database layer with migrations
- `internal/catalog/` - Catalog service (models, repository, service)
- `internal/watcher/` - Directory watcher (stub)
- `internal/pipeline/` - Processing pipeline (stub)
- `internal/playback/` - Video playback proxy with Range support
- `internal/cloud/` - Cloud communication stubs
- `internal/api/` - HTTP API server
- `internal/ui/` - System tray UI
- `Makefile` - Build automation
- `docs/` - Architecture, threat model, API documentation

### Definition of Done
- [ ] `make build` produces working binaries for Windows/macOS
- [ ] `make test` passes all tests
- [ ] `make lint` passes with no errors
- [ ] Agent starts, shows tray icon, serves API on 127.0.0.1
- [ ] Can add folder, scan, and play back video via Range requests

### Must Have
- Clean layering: API → Service → Repository → DB
- Dependency injection via constructors
- Interfaces for all services
- Graceful shutdown handling
- Binding only to 127.0.0.1
- Structured JSON logging
- Bearer token authentication

### Must NOT Have (Guardrails)
- NO CGO dependencies (using pure Go SQLite)
- NO binding to 0.0.0.0 or external interfaces
- NO logging of secrets/tokens (mask in logs)
- NO hardcoded paths (use config)
- NO global state (use dependency injection)
- NO real cloud API calls in v0 (stubs only)
- NO Windows service mode in v0 (regular app only)
- NO over-engineered abstractions for simple operations

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: NO (greenfield)
- **Automated tests**: Tests-after with critical path coverage
- **Framework**: Go standard `testing` package

### Test Coverage Requirements
1. **Range parsing** - Unit tests for HTTP Range header parsing
2. **Scan logic** - Integration tests with temp directory
3. **DB migrations** - Migration tests ensuring clean apply

### Agent-Executed QA Scenarios

All tasks include QA scenarios using:
- **CLI/Build verification**: Bash commands
- **API testing**: curl requests to localhost
- **TUI verification**: interactive_bash for systray (limited - visual)

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Foundation - Start Immediately):
├── Task 1: Project initialization (go.mod, Makefile, README)
├── Task 2: Configuration package
└── Task 3: Logging package

Wave 2 (Data Layer - After Wave 1):
├── Task 4: Database package + migrations
└── Task 5: Catalog models

Wave 3 (Repository - After Wave 2):
├── Task 6: Catalog repository
└── Task 7: Playback Range parser (pure functions)

Wave 4 (Services - After Wave 3):
├── Task 8: Catalog service + scanner
├── Task 9: Playback server
└── Task 10: Job runner

Wave 5 (API Foundation - After Wave 4):
├── Task 11: API schemas
├── Task 12: API middleware (auth, logging)
└── Task 13: Cloud stubs

Wave 6 (API Routes - After Wave 5):
├── Task 14: API routes and handlers
└── Task 15: Watcher + Pipeline stubs

Wave 7 (UI + Integration - After Wave 6):
├── Task 16: System tray UI
└── Task 17: Main entry point

Wave 8 (Quality - After Wave 7):
├── Task 18: Unit tests (Range, migrations)
├── Task 19: Integration tests (scan)
├── Task 20: Documentation
└── Task 21: Packaging templates

Critical Path: Task 1 → 4 → 6 → 8 → 14 → 17 → 18
Estimated Parallel Speedup: ~50% faster than sequential
```

### Dependency Matrix

| Task | Depends On | Blocks | Can Parallelize With |
|------|------------|--------|---------------------|
| 1 | None | 2,3,4,5 | - |
| 2 | 1 | 4,11,17 | 3 |
| 3 | 1 | 4,12,17 | 2 |
| 4 | 1,2,3 | 5,6 | - |
| 5 | 4 | 6,8 | - |
| 6 | 4,5 | 8 | 7 |
| 7 | 1 | 9,18 | 6 |
| 8 | 6 | 14,17 | 9,10 |
| 9 | 7 | 14 | 8,10 |
| 10 | 6 | 17 | 8,9 |
| 11 | 2 | 14 | 12,13 |
| 12 | 2,3 | 14 | 11,13 |
| 13 | 2,3 | 17 | 11,12 |
| 14 | 8,9,11,12 | 17 | 15 |
| 15 | 1 | 17 | 14 |
| 16 | 2,3 | 17 | 14,15 |
| 17 | 8,10,14,16 | 18,19,20 | - |
| 18 | 7,17 | 21 | 19,20 |
| 19 | 8,17 | 21 | 18,20 |
| 20 | 17 | 21 | 18,19 |
| 21 | 17 | - | 18,19,20 |

---

## TODOs

### Wave 1: Foundation

---

- [ ] 1. Project Initialization

  **What to do**:
  - Create `go.mod` with module `github.com/heimdex/heimdex-agent`
  - Add dependencies: `github.com/getlantern/systray`, `modernc.org/sqlite`, `github.com/go-chi/chi/v5`
  - Create `Makefile` with targets: `dev`, `test`, `lint`, `build`, `clean`
  - Create `README.md` with basic project description and setup instructions
  - Create directory structure as specified

  **Must NOT do**:
  - Do NOT add CGO dependencies
  - Do NOT create actual implementation files (just directories)

  **Files to create**:
  ```
  go.mod
  Makefile
  README.md
  cmd/agent/.gitkeep
  internal/config/.gitkeep
  internal/logging/.gitkeep
  internal/db/.gitkeep
  internal/db/migrations/.gitkeep
  internal/catalog/.gitkeep
  internal/watcher/.gitkeep
  internal/pipeline/.gitkeep
  internal/playback/.gitkeep
  internal/cloud/.gitkeep
  internal/api/.gitkeep
  internal/ui/.gitkeep
  packaging/windows/installer/.gitkeep
  packaging/macos/installer/.gitkeep
  docs/.gitkeep
  ```

  **Key implementation details**:
  
  `go.mod`:
  ```go
  module github.com/heimdex/heimdex-agent

  go 1.21

  require (
      github.com/getlantern/systray v1.2.2
      github.com/go-chi/chi/v5 v5.0.11
      modernc.org/sqlite v1.28.0
  )
  ```

  `Makefile`:
  ```makefile
  .PHONY: dev test lint build clean

  VERSION ?= 0.1.0
  BINARY_NAME = heimdex-agent
  
  dev:
      go run ./cmd/agent
  
  test:
      go test -v -race ./...
  
  lint:
      go vet ./...
      golangci-lint run
  
  build:
      CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/$(BINARY_NAME) ./cmd/agent
  
  clean:
      rm -rf bin/
  ```

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: File scaffolding with no complex logic
  - **Skills**: `[]`
    - No special skills needed for file creation
  - **Skills Evaluated but Omitted**:
    - `git-master`: Not needed yet, no commits

  **Parallelization**:
  - **Can Run In Parallel**: NO (foundation task)
  - **Parallel Group**: Wave 1 (standalone)
  - **Blocks**: Tasks 2, 3, 4, 5
  - **Blocked By**: None

  **References**:
  - Pattern: Standard Go project layout: https://github.com/golang-standards/project-layout
  - Docs: Go modules: https://go.dev/ref/mod

  **Acceptance Criteria**:
  
  **Agent-Executed QA Scenarios:**

  ```
  Scenario: go.mod is valid and dependencies resolve
    Tool: Bash
    Preconditions: Go 1.21+ installed
    Steps:
      1. cd /Users/jangwonlee/Projects/heimdex/heimdex-agent
      2. go mod tidy
      3. go mod verify
    Expected Result: Exit code 0, no errors
    Evidence: Command output captured

  Scenario: Makefile targets exist
    Tool: Bash
    Preconditions: make installed
    Steps:
      1. make -n dev
      2. make -n test
      3. make -n lint
      4. make -n build
    Expected Result: Each shows the commands that would run (dry-run)
    Evidence: Command output captured

  Scenario: Directory structure created
    Tool: Bash
    Steps:
      1. ls -la cmd/agent/
      2. ls -la internal/
      3. ls -la internal/config/
      4. ls -la internal/db/migrations/
    Expected Result: All directories exist
    Evidence: Directory listing output
  ```

  **Commit**: YES
  - Message: `feat(init): initialize project structure and dependencies`
  - Files: `go.mod`, `Makefile`, `README.md`, all `.gitkeep` files

---

- [ ] 2. Configuration Package

  **What to do**:
  - Create `internal/config/config.go` with configuration struct and loader
  - Support environment variables: `HEIMDEX_PORT`, `HEIMDEX_LOG_LEVEL`, `HEIMDEX_DATA_DIR`
  - Provide sensible defaults (port 8787, log level "info", data dir `~/.heimdex`)
  - Create `Config` interface for dependency injection
  - Implement `EnvConfig` struct that reads from environment

  **Must NOT do**:
  - Do NOT read from files (env vars only for v0)
  - Do NOT use third-party config libraries
  - Do NOT hardcode any paths

  **Files to create**:
  ```
  internal/config/config.go
  ```

  **Key implementation details**:

  ```go
  // internal/config/config.go
  package config

  import (
      "os"
      "path/filepath"
      "strconv"
  )

  // Config defines the application configuration interface
  type Config interface {
      Port() int
      LogLevel() string
      DataDir() string
      DBPath() string
  }

  // EnvConfig reads configuration from environment variables
  type EnvConfig struct {
      port     int
      logLevel string
      dataDir  string
  }

  // New creates a new EnvConfig with defaults and env overrides
  func New() (*EnvConfig, error) {
      cfg := &EnvConfig{
          port:     8787,
          logLevel: "info",
          dataDir:  defaultDataDir(),
      }
      
      if p := os.Getenv("HEIMDEX_PORT"); p != "" {
          port, err := strconv.Atoi(p)
          if err != nil {
              return nil, fmt.Errorf("invalid HEIMDEX_PORT: %w", err)
          }
          cfg.port = port
      }
      
      if ll := os.Getenv("HEIMDEX_LOG_LEVEL"); ll != "" {
          cfg.logLevel = ll
      }
      
      if dd := os.Getenv("HEIMDEX_DATA_DIR"); dd != "" {
          cfg.dataDir = dd
      }
      
      return cfg, nil
  }

  func (c *EnvConfig) Port() int        { return c.port }
  func (c *EnvConfig) LogLevel() string { return c.logLevel }
  func (c *EnvConfig) DataDir() string  { return c.dataDir }
  func (c *EnvConfig) DBPath() string   { return filepath.Join(c.dataDir, "heimdex.db") }

  func defaultDataDir() string {
      home, _ := os.UserHomeDir()
      return filepath.Join(home, ".heimdex")
  }
  ```

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Small, focused package with clear requirements
  - **Skills**: `[]`
    - No special skills needed
  - **Skills Evaluated but Omitted**:
    - None relevant

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Task 3)
  - **Blocks**: Tasks 4, 11, 17
  - **Blocked By**: Task 1

  **References**:
  - Pattern: 12-factor app config: https://12factor.net/config
  - Docs: os.Getenv: https://pkg.go.dev/os#Getenv

  **Acceptance Criteria**:

  **Agent-Executed QA Scenarios:**

  ```
  Scenario: Config loads with defaults
    Tool: Bash
    Steps:
      1. Create temp test file that imports config and prints values
      2. go run temp_test.go
      3. Verify output contains port=8787, logLevel=info
    Expected Result: Default values printed
    Evidence: Command output

  Scenario: Config respects environment overrides
    Tool: Bash
    Steps:
      1. HEIMDEX_PORT=9999 HEIMDEX_LOG_LEVEL=debug go run temp_test.go
      2. Verify output contains port=9999, logLevel=debug
    Expected Result: Overridden values printed
    Evidence: Command output

  Scenario: Invalid port returns error
    Tool: Bash
    Steps:
      1. HEIMDEX_PORT=notanumber go run temp_test.go
      2. Check for error message
    Expected Result: Error about invalid port
    Evidence: Error output captured
  ```

  **Commit**: YES (groups with Task 3)
  - Message: `feat(config): add environment-based configuration`
  - Files: `internal/config/config.go`

---

- [ ] 3. Logging Package

  **What to do**:
  - Create `internal/logging/logging.go` with structured JSON logging using `log/slog`
  - Create logger factory function `NewLogger(level string) *slog.Logger`
  - Support log levels: debug, info, warn, error
  - Output JSON format to stdout
  - Add helper to create child loggers with context (e.g., request ID)
  - NEVER log secrets - add `SanitizeToken` helper

  **Must NOT do**:
  - Do NOT use third-party logging libraries
  - Do NOT log to files (stdout only for v0)
  - Do NOT log full tokens or secrets

  **Files to create**:
  ```
  internal/logging/logging.go
  ```

  **Key implementation details**:

  ```go
  // internal/logging/logging.go
  package logging

  import (
      "log/slog"
      "os"
      "strings"
  )

  // NewLogger creates a new structured JSON logger
  func NewLogger(level string) *slog.Logger {
      var lvl slog.Level
      switch strings.ToLower(level) {
      case "debug":
          lvl = slog.LevelDebug
      case "warn", "warning":
          lvl = slog.LevelWarn
      case "error":
          lvl = slog.LevelError
      default:
          lvl = slog.LevelInfo
      }

      opts := &slog.HandlerOptions{
          Level: lvl,
      }
      handler := slog.NewJSONHandler(os.Stdout, opts)
      return slog.New(handler)
  }

  // WithRequestID returns a logger with request_id attribute
  func WithRequestID(logger *slog.Logger, requestID string) *slog.Logger {
      return logger.With("request_id", requestID)
  }

  // WithComponent returns a logger with component attribute
  func WithComponent(logger *slog.Logger, component string) *slog.Logger {
      return logger.With("component", component)
  }

  // SanitizeToken masks a token for safe logging
  // Shows first 4 and last 4 characters only
  func SanitizeToken(token string) string {
      if len(token) <= 8 {
          return "****"
      }
      return token[:4] + "..." + token[len(token)-4:]
  }
  ```

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Small, focused package using stdlib
  - **Skills**: `[]`
    - No special skills needed
  - **Skills Evaluated but Omitted**:
    - None relevant

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Task 2)
  - **Blocks**: Tasks 4, 12, 17
  - **Blocked By**: Task 1

  **References**:
  - Docs: log/slog: https://pkg.go.dev/log/slog
  - Pattern: Structured logging best practices

  **Acceptance Criteria**:

  **Agent-Executed QA Scenarios:**

  ```
  Scenario: Logger outputs JSON format
    Tool: Bash
    Steps:
      1. Create temp test that logs a message
      2. go run temp_test.go 2>&1 | head -1
      3. Parse output as JSON
    Expected Result: Valid JSON with "level", "msg", "time" fields
    Evidence: JSON output captured

  Scenario: Log level filtering works
    Tool: Bash
    Steps:
      1. Create test with "warn" level logger
      2. Log debug, info, warn messages
      3. Verify only warn message appears
    Expected Result: Only warn-level output
    Evidence: Command output

  Scenario: SanitizeToken masks correctly
    Tool: Bash
    Steps:
      1. Test SanitizeToken("abcd1234efgh5678")
      2. Verify output is "abcd...5678"
    Expected Result: Masked token format
    Evidence: Test output
  ```

  **Commit**: YES (groups with Task 2)
  - Message: `feat(logging): add structured JSON logging with slog`
  - Files: `internal/logging/logging.go`

---

### Wave 2: Data Layer

---

- [ ] 4. Database Package with Migrations

  **What to do**:
  - Create `internal/db/db.go` with database connection management
  - Use `modernc.org/sqlite` (pure Go, no CGO)
  - Implement connection with WAL mode and proper pragmas
  - Create `internal/db/migrations/001_initial.sql` with schema
  - Implement migration runner that applies migrations in order
  - Store migration state in `_migrations` table
  - On startup, mark any "running" jobs as "failed"

  **Must NOT do**:
  - Do NOT use connection pooling (single connection for writes)
  - Do NOT use CGO-based sqlite3
  - Do NOT allow external file paths without validation

  **Files to create**:
  ```
  internal/db/db.go
  internal/db/migrations.go
  internal/db/migrations/001_initial.sql
  ```

  **Key implementation details**:

  `internal/db/db.go`:
  ```go
  package db

  import (
      "context"
      "database/sql"
      "fmt"
      
      _ "modernc.org/sqlite"
  )

  // DB wraps the database connection
  type DB struct {
      conn   *sql.DB
      logger *slog.Logger
  }

  // New creates a new database connection
  func New(dbPath string, logger *slog.Logger) (*DB, error) {
      dsn := fmt.Sprintf("%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON", dbPath)
      
      conn, err := sql.Open("sqlite", dsn)
      if err != nil {
          return nil, fmt.Errorf("failed to open database: %w", err)
      }
      
      // Single writer limitation
      conn.SetMaxOpenConns(1)
      conn.SetMaxIdleConns(1)
      
      // Verify connection
      if err := conn.Ping(); err != nil {
          return nil, fmt.Errorf("failed to ping database: %w", err)
      }
      
      db := &DB{conn: conn, logger: logger}
      
      // Run migrations
      if err := db.Migrate(); err != nil {
          return nil, fmt.Errorf("failed to run migrations: %w", err)
      }
      
      // Mark interrupted jobs as failed
      if err := db.markInterruptedJobs(); err != nil {
          logger.Warn("failed to mark interrupted jobs", "error", err)
      }
      
      return db, nil
  }

  func (d *DB) Close() error {
      return d.conn.Close()
  }

  func (d *DB) Conn() *sql.DB {
      return d.conn
  }

  func (d *DB) markInterruptedJobs() error {
      _, err := d.conn.Exec(`UPDATE jobs SET status = 'failed', error = 'interrupted' WHERE status = 'running'`)
      return err
  }
  ```

  `internal/db/migrations/001_initial.sql`:
  ```sql
  -- Sources table: folder locations to scan
  CREATE TABLE IF NOT EXISTS sources (
      id TEXT PRIMARY KEY,
      type TEXT NOT NULL DEFAULT 'folder',
      path TEXT NOT NULL UNIQUE,
      display_name TEXT NOT NULL,
      drive_nickname TEXT,
      present INTEGER NOT NULL DEFAULT 1,
      created_at TEXT NOT NULL DEFAULT (datetime('now'))
  );

  -- Files table: discovered video files
  CREATE TABLE IF NOT EXISTS files (
      id TEXT PRIMARY KEY,
      source_id TEXT NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
      path TEXT NOT NULL,
      filename TEXT NOT NULL,
      size INTEGER NOT NULL,
      mtime TEXT NOT NULL,
      fingerprint TEXT NOT NULL,
      created_at TEXT NOT NULL DEFAULT (datetime('now')),
      UNIQUE(source_id, path)
  );

  -- Jobs table: background processing jobs
  CREATE TABLE IF NOT EXISTS jobs (
      id TEXT PRIMARY KEY,
      type TEXT NOT NULL,
      status TEXT NOT NULL DEFAULT 'pending',
      source_id TEXT REFERENCES sources(id) ON DELETE SET NULL,
      progress INTEGER NOT NULL DEFAULT 0,
      error TEXT,
      created_at TEXT NOT NULL DEFAULT (datetime('now')),
      updated_at TEXT NOT NULL DEFAULT (datetime('now'))
  );

  -- Config table: app state storage
  CREATE TABLE IF NOT EXISTS config (
      key TEXT PRIMARY KEY,
      value TEXT NOT NULL
  );

  -- Indexes for common queries
  CREATE INDEX IF NOT EXISTS idx_files_source ON files(source_id);
  CREATE INDEX IF NOT EXISTS idx_files_fingerprint ON files(fingerprint);
  CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
  CREATE INDEX IF NOT EXISTS idx_jobs_source ON jobs(source_id);
  ```

  **Recommended Agent Profile**:
  - **Category**: `unspecified-low`
    - Reason: Database setup with specific patterns, moderate complexity
  - **Skills**: `[]`
    - No special skills needed
  - **Skills Evaluated but Omitted**:
    - None - this is core Go + SQL

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2 (sequential)
  - **Blocks**: Tasks 5, 6
  - **Blocked By**: Tasks 1, 2, 3

  **References**:
  - Docs: modernc.org/sqlite: https://pkg.go.dev/modernc.org/sqlite
  - Pattern: WAL mode + busy_timeout for concurrent access
  - Research: See librarian findings on connection setup

  **Acceptance Criteria**:

  **Agent-Executed QA Scenarios:**

  ```
  Scenario: Database creates successfully with migrations
    Tool: Bash
    Steps:
      1. Create temp test that calls db.New with temp file path
      2. Verify no error returned
      3. Query sqlite_master for tables
      4. Assert sources, files, jobs, config tables exist
    Expected Result: All 4 tables created
    Evidence: Table list output

  Scenario: WAL mode is enabled
    Tool: Bash
    Steps:
      1. After db.New, run PRAGMA journal_mode
      2. Assert returns "wal"
    Expected Result: journal_mode = wal
    Evidence: PRAGMA output

  Scenario: Interrupted jobs marked as failed
    Tool: Bash
    Steps:
      1. Create db, insert job with status='running'
      2. Close db
      3. Reopen db (simulates restart)
      4. Query job status
    Expected Result: Job status = 'failed', error = 'interrupted'
    Evidence: Query result
  ```

  **Commit**: YES
  - Message: `feat(db): add SQLite database layer with migrations`
  - Files: `internal/db/db.go`, `internal/db/migrations.go`, `internal/db/migrations/001_initial.sql`

---

- [ ] 5. Catalog Models

  **What to do**:
  - Create `internal/catalog/models.go` with domain types
  - Define `Source`, `File`, `Job`, `ConfigEntry` structs
  - Add constants for job types and statuses
  - Include validation methods where appropriate
  - Use UUIDs for IDs (use `crypto/rand` based generator)

  **Must NOT do**:
  - Do NOT include database logic (that's repository)
  - Do NOT include business logic (that's service)
  - Do NOT use external UUID libraries (stdlib is fine)

  **Files to create**:
  ```
  internal/catalog/models.go
  ```

  **Key implementation details**:

  ```go
  // internal/catalog/models.go
  package catalog

  import (
      "crypto/rand"
      "fmt"
      "time"
  )

  // Source represents a folder location to scan
  type Source struct {
      ID            string    `json:"id"`
      Type          string    `json:"type"`
      Path          string    `json:"path"`
      DisplayName   string    `json:"display_name"`
      DriveNickname string    `json:"drive_nickname,omitempty"`
      Present       bool      `json:"present"`
      CreatedAt     time.Time `json:"created_at"`
  }

  // File represents a discovered video file
  type File struct {
      ID          string    `json:"id"`
      SourceID    string    `json:"source_id"`
      Path        string    `json:"path"`
      Filename    string    `json:"filename"`
      Size        int64     `json:"size"`
      Mtime       time.Time `json:"mtime"`
      Fingerprint string    `json:"fingerprint"`
      CreatedAt   time.Time `json:"created_at"`
  }

  // JobType constants
  const (
      JobTypeScan = "scan"
  )

  // JobStatus constants
  const (
      JobStatusPending   = "pending"
      JobStatusRunning   = "running"
      JobStatusCompleted = "completed"
      JobStatusFailed    = "failed"
  )

  // Job represents a background processing job
  type Job struct {
      ID        string    `json:"id"`
      Type      string    `json:"type"`
      Status    string    `json:"status"`
      SourceID  string    `json:"source_id,omitempty"`
      Progress  int       `json:"progress"`
      Error     string    `json:"error,omitempty"`
      CreatedAt time.Time `json:"created_at"`
      UpdatedAt time.Time `json:"updated_at"`
  }

  // ConfigEntry represents a key-value config pair
  type ConfigEntry struct {
      Key   string `json:"key"`
      Value string `json:"value"`
  }

  // Video file extensions to scan
  var VideoExtensions = map[string]bool{
      ".mp4": true,
      ".mov": true,
      ".mkv": true,
  }

  // NewID generates a new UUID-like ID
  func NewID() string {
      b := make([]byte, 16)
      rand.Read(b)
      return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
  }
  ```

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Pure data structures, no complex logic
  - **Skills**: `[]`
    - No special skills needed
  - **Skills Evaluated but Omitted**:
    - None relevant

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2 (after Task 4)
  - **Blocks**: Tasks 6, 8
  - **Blocked By**: Task 4

  **References**:
  - Pattern: Domain models separate from persistence

  **Acceptance Criteria**:

  **Agent-Executed QA Scenarios:**

  ```
  Scenario: Models compile correctly
    Tool: Bash
    Steps:
      1. go build ./internal/catalog/...
    Expected Result: Exit code 0
    Evidence: Build output

  Scenario: NewID generates unique IDs
    Tool: Bash
    Steps:
      1. Create test that generates 100 IDs
      2. Verify all unique
    Expected Result: 100 unique IDs
    Evidence: Test output

  Scenario: VideoExtensions contains expected formats
    Tool: Bash
    Steps:
      1. Test VideoExtensions[".mp4"] == true
      2. Test VideoExtensions[".avi"] == false
    Expected Result: mp4/mov/mkv true, others false
    Evidence: Test output
  ```

  **Commit**: YES (groups with Task 4)
  - Message: `feat(catalog): add domain models`
  - Files: `internal/catalog/models.go`

---

### Wave 3: Repository & Pure Functions

---

- [ ] 6. Catalog Repository

  **What to do**:
  - Create `internal/catalog/repository.go` with data access layer
  - Define `Repository` interface with CRUD methods
  - Implement `SQLiteRepository` struct
  - Methods: CreateSource, GetSource, ListSources, DeleteSource
  - Methods: CreateFile, GetFile, GetFilesBySource, DeleteFilesBySource
  - Methods: CreateJob, GetJob, ListJobs, UpdateJobStatus, UpdateJobProgress
  - Methods: GetConfig, SetConfig
  - Use prepared statements for frequent queries

  **Must NOT do**:
  - Do NOT include business logic (that's service)
  - Do NOT expose raw SQL outside this package
  - Do NOT use ORM libraries

  **Files to create**:
  ```
  internal/catalog/repository.go
  ```

  **Key implementation details**:

  ```go
  // internal/catalog/repository.go
  package catalog

  import (
      "context"
      "database/sql"
  )

  // Repository defines the data access interface
  type Repository interface {
      // Sources
      CreateSource(ctx context.Context, source *Source) error
      GetSource(ctx context.Context, id string) (*Source, error)
      GetSourceByPath(ctx context.Context, path string) (*Source, error)
      ListSources(ctx context.Context) ([]*Source, error)
      DeleteSource(ctx context.Context, id string) error

      // Files
      CreateFile(ctx context.Context, file *File) error
      GetFile(ctx context.Context, id string) (*File, error)
      GetFilesBySource(ctx context.Context, sourceID string) ([]*File, error)
      DeleteFilesBySource(ctx context.Context, sourceID string) error
      UpsertFile(ctx context.Context, file *File) error

      // Jobs
      CreateJob(ctx context.Context, job *Job) error
      GetJob(ctx context.Context, id string) (*Job, error)
      ListJobs(ctx context.Context) ([]*Job, error)
      UpdateJobStatus(ctx context.Context, id, status, errorMsg string) error
      UpdateJobProgress(ctx context.Context, id string, progress int) error

      // Config
      GetConfig(ctx context.Context, key string) (string, error)
      SetConfig(ctx context.Context, key, value string) error
  }

  // SQLiteRepository implements Repository with SQLite
  type SQLiteRepository struct {
      db *sql.DB
  }

  // NewRepository creates a new SQLite repository
  func NewRepository(db *sql.DB) *SQLiteRepository {
      return &SQLiteRepository{db: db}
  }

  // Example implementation
  func (r *SQLiteRepository) CreateSource(ctx context.Context, s *Source) error {
      _, err := r.db.ExecContext(ctx, `
          INSERT INTO sources (id, type, path, display_name, drive_nickname, present, created_at)
          VALUES (?, ?, ?, ?, ?, ?, ?)
      `, s.ID, s.Type, s.Path, s.DisplayName, s.DriveNickname, s.Present, s.CreatedAt)
      return err
  }

  func (r *SQLiteRepository) GetSource(ctx context.Context, id string) (*Source, error) {
      row := r.db.QueryRowContext(ctx, `
          SELECT id, type, path, display_name, drive_nickname, present, created_at
          FROM sources WHERE id = ?
      `, id)
      
      var s Source
      var present int
      err := row.Scan(&s.ID, &s.Type, &s.Path, &s.DisplayName, &s.DriveNickname, &present, &s.CreatedAt)
      if err == sql.ErrNoRows {
          return nil, nil
      }
      if err != nil {
          return nil, err
      }
      s.Present = present == 1
      return &s, nil
  }

  // ... implement all other methods similarly
  ```

  **Recommended Agent Profile**:
  - **Category**: `unspecified-low`
    - Reason: Data access layer with many methods but standard patterns
  - **Skills**: `[]`
    - No special skills needed
  - **Skills Evaluated but Omitted**:
    - None - standard Go SQL patterns

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Task 7)
  - **Blocks**: Task 8
  - **Blocked By**: Tasks 4, 5

  **References**:
  - Pattern: Repository pattern in Go
  - Docs: database/sql: https://pkg.go.dev/database/sql

  **Acceptance Criteria**:

  **Agent-Executed QA Scenarios:**

  ```
  Scenario: CRUD operations work for sources
    Tool: Bash
    Steps:
      1. Create in-memory db
      2. CreateSource with test data
      3. GetSource returns same data
      4. ListSources includes the source
      5. DeleteSource removes it
      6. GetSource returns nil
    Expected Result: All operations succeed
    Evidence: Test output

  Scenario: UpsertFile updates existing files
    Tool: Bash
    Steps:
      1. Create file with path="/test.mp4"
      2. UpsertFile with same path but different size
      3. GetFile shows new size
    Expected Result: File updated, not duplicated
    Evidence: Query result

  Scenario: Config get/set works
    Tool: Bash
    Steps:
      1. SetConfig("test_key", "test_value")
      2. GetConfig("test_key")
    Expected Result: Returns "test_value"
    Evidence: Query result
  ```

  **Commit**: YES
  - Message: `feat(catalog): add repository layer`
  - Files: `internal/catalog/repository.go`

---

- [ ] 7. Playback Range Parser

  **What to do**:
  - Create `internal/playback/range.go` with HTTP Range header parsing
  - Parse `Range: bytes=start-end` header format
  - Support single range requests (multi-range optional)
  - Handle edge cases: `bytes=0-`, `bytes=-500`, `bytes=0-999`
  - Create `internal/playback/range_test.go` with comprehensive tests
  - Return parsed range struct with start, end, total calculations

  **Must NOT do**:
  - Do NOT handle file I/O here (pure parsing only)
  - Do NOT use external libraries

  **Files to create**:
  ```
  internal/playback/range.go
  internal/playback/range_test.go
  ```

  **Key implementation details**:

  ```go
  // internal/playback/range.go
  package playback

  import (
      "errors"
      "fmt"
      "strconv"
      "strings"
  )

  var (
      ErrInvalidRange     = errors.New("invalid range format")
      ErrUnsatisfiable    = errors.New("range not satisfiable")
  )

  // Range represents a parsed HTTP Range request
  type Range struct {
      Start int64
      End   int64
  }

  // ContentLength returns the length of the range
  func (r Range) ContentLength() int64 {
      return r.End - r.Start + 1
  }

  // ContentRange returns the Content-Range header value
  func (r Range) ContentRange(total int64) string {
      return fmt.Sprintf("bytes %d-%d/%d", r.Start, r.End, total)
  }

  // ParseRange parses a Range header value for a resource of the given size
  // Supports formats: "bytes=0-499", "bytes=500-", "bytes=-500"
  func ParseRange(header string, size int64) (*Range, error) {
      if header == "" {
          return nil, nil // No range requested
      }

      if !strings.HasPrefix(header, "bytes=") {
          return nil, ErrInvalidRange
      }

      rangeSpec := strings.TrimPrefix(header, "bytes=")
      
      // Handle multiple ranges - for v0, take first only
      if strings.Contains(rangeSpec, ",") {
          rangeSpec = strings.Split(rangeSpec, ",")[0]
          rangeSpec = strings.TrimSpace(rangeSpec)
      }

      parts := strings.Split(rangeSpec, "-")
      if len(parts) != 2 {
          return nil, ErrInvalidRange
      }

      var start, end int64
      var err error

      // bytes=-500 (last 500 bytes)
      if parts[0] == "" {
          suffixLen, err := strconv.ParseInt(parts[1], 10, 64)
          if err != nil || suffixLen <= 0 {
              return nil, ErrInvalidRange
          }
          start = size - suffixLen
          if start < 0 {
              start = 0
          }
          end = size - 1
      } else {
          start, err = strconv.ParseInt(parts[0], 10, 64)
          if err != nil || start < 0 {
              return nil, ErrInvalidRange
          }

          // bytes=500- (from 500 to end)
          if parts[1] == "" {
              end = size - 1
          } else {
              end, err = strconv.ParseInt(parts[1], 10, 64)
              if err != nil {
                  return nil, ErrInvalidRange
              }
          }
      }

      // Validate range
      if start > end || start >= size {
          return nil, ErrUnsatisfiable
      }

      // Clamp end to file size
      if end >= size {
          end = size - 1
      }

      return &Range{Start: start, End: end}, nil
  }
  ```

  `internal/playback/range_test.go`:
  ```go
  package playback

  import "testing"

  func TestParseRange(t *testing.T) {
      tests := []struct {
          name    string
          header  string
          size    int64
          want    *Range
          wantErr error
      }{
          {"empty header", "", 1000, nil, nil},
          {"full range", "bytes=0-999", 1000, &Range{0, 999}, nil},
          {"partial start", "bytes=500-", 1000, &Range{500, 999}, nil},
          {"suffix range", "bytes=-500", 1000, &Range{500, 999}, nil},
          {"beyond size", "bytes=0-2000", 1000, &Range{0, 999}, nil},
          {"unsatisfiable", "bytes=1000-", 1000, nil, ErrUnsatisfiable},
          {"invalid format", "invalid", 1000, nil, ErrInvalidRange},
      }

      for _, tt := range tests {
          t.Run(tt.name, func(t *testing.T) {
              got, err := ParseRange(tt.header, tt.size)
              // ... assertions
          })
      }
  }
  ```

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Pure parsing logic with clear spec
  - **Skills**: `[]`
    - No special skills needed
  - **Skills Evaluated but Omitted**:
    - None - pure string parsing

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Task 6)
  - **Blocks**: Task 9, 18
  - **Blocked By**: Task 1

  **References**:
  - Docs: RFC 7233 (HTTP Range Requests): https://tools.ietf.org/html/rfc7233
  - Pattern: Go stdlib net/http range handling

  **Acceptance Criteria**:

  **Agent-Executed QA Scenarios:**

  ```
  Scenario: Range parsing tests pass
    Tool: Bash
    Steps:
      1. go test -v ./internal/playback/...
    Expected Result: All tests pass
    Evidence: Test output with PASS

  Scenario: Handles all range formats
    Tool: Bash
    Steps:
      1. Run tests covering: bytes=0-499, bytes=500-, bytes=-500
    Expected Result: All formats parsed correctly
    Evidence: Test output

  Scenario: Edge cases handled
    Tool: Bash
    Steps:
      1. Test unsatisfiable range (start > size)
      2. Test range clamping (end > size)
      3. Test empty header
    Expected Result: Correct errors/behavior for each
    Evidence: Test output
  ```

  **Commit**: YES
  - Message: `feat(playback): add HTTP Range parser with tests`
  - Files: `internal/playback/range.go`, `internal/playback/range_test.go`

---

### Wave 4: Services

---

- [ ] 8. Catalog Service with Scanner

  **What to do**:
  - Create `internal/catalog/service.go` with business logic layer
  - Define `CatalogService` interface
  - Implement folder adding, source management, file scanning
  - Scanner: walk folder, find video files, compute SHA-256 fingerprint of first 64KB
  - Create scan job, update progress, handle errors
  - Methods: AddFolder, RemoveSource, GetSources, GetFiles, ScanSource

  **Must NOT do**:
  - Do NOT access database directly (use repository)
  - Do NOT block on long operations (use goroutines with job tracking)
  - Do NOT scan hidden folders (starting with .)

  **Files to create**:
  ```
  internal/catalog/service.go
  ```

  **Key implementation details**:

  ```go
  // internal/catalog/service.go
  package catalog

  import (
      "context"
      "crypto/sha256"
      "encoding/hex"
      "io"
      "log/slog"
      "os"
      "path/filepath"
      "strings"
      "time"
  )

  const fingerprintSize = 64 * 1024 // 64KB

  // CatalogService defines the business logic interface
  type CatalogService interface {
      AddFolder(ctx context.Context, path, displayName string) (*Source, error)
      RemoveSource(ctx context.Context, id string) error
      GetSources(ctx context.Context) ([]*Source, error)
      GetSource(ctx context.Context, id string) (*Source, error)
      GetFiles(ctx context.Context, sourceID string) ([]*File, error)
      GetFile(ctx context.Context, id string) (*File, error)
      ScanSource(ctx context.Context, sourceID string) (*Job, error)
  }

  // Service implements CatalogService
  type Service struct {
      repo   Repository
      logger *slog.Logger
  }

  // NewService creates a new catalog service
  func NewService(repo Repository, logger *slog.Logger) *Service {
      return &Service{repo: repo, logger: logger}
  }

  func (s *Service) AddFolder(ctx context.Context, path, displayName string) (*Source, error) {
      // Validate path exists and is directory
      info, err := os.Stat(path)
      if err != nil {
          return nil, fmt.Errorf("invalid path: %w", err)
      }
      if !info.IsDir() {
          return nil, fmt.Errorf("path is not a directory")
      }

      // Check if already exists
      existing, err := s.repo.GetSourceByPath(ctx, path)
      if err != nil {
          return nil, err
      }
      if existing != nil {
          return existing, nil
      }

      source := &Source{
          ID:          NewID(),
          Type:        "folder",
          Path:        filepath.Clean(path),
          DisplayName: displayName,
          Present:     true,
          CreatedAt:   time.Now(),
      }

      if err := s.repo.CreateSource(ctx, source); err != nil {
          return nil, err
      }

      return source, nil
  }

  func (s *Service) ScanSource(ctx context.Context, sourceID string) (*Job, error) {
      source, err := s.repo.GetSource(ctx, sourceID)
      if err != nil || source == nil {
          return nil, fmt.Errorf("source not found")
      }

      job := &Job{
          ID:        NewID(),
          Type:      JobTypeScan,
          Status:    JobStatusPending,
          SourceID:  sourceID,
          Progress:  0,
          CreatedAt: time.Now(),
          UpdatedAt: time.Now(),
      }

      if err := s.repo.CreateJob(ctx, job); err != nil {
          return nil, err
      }

      return job, nil
  }

  // ScanFolder performs the actual scan (called by job runner)
  func (s *Service) ScanFolder(ctx context.Context, jobID, sourceID, path string) error {
      s.repo.UpdateJobStatus(ctx, jobID, JobStatusRunning, "")

      var files []string
      err := filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
          if err != nil {
              return nil // Skip errors
          }
          if d.IsDir() && strings.HasPrefix(d.Name(), ".") {
              return filepath.SkipDir
          }
          if !d.IsDir() && isVideoFile(d.Name()) {
              files = append(files, p)
          }
          return nil
      })
      if err != nil {
          s.repo.UpdateJobStatus(ctx, jobID, JobStatusFailed, err.Error())
          return err
      }

      total := len(files)
      for i, filePath := range files {
          if err := s.processFile(ctx, sourceID, filePath); err != nil {
              s.logger.Warn("failed to process file", "path", filePath, "error", err)
          }
          progress := (i + 1) * 100 / total
          s.repo.UpdateJobProgress(ctx, jobID, progress)
      }

      s.repo.UpdateJobStatus(ctx, jobID, JobStatusCompleted, "")
      return nil
  }

  func (s *Service) processFile(ctx context.Context, sourceID, path string) error {
      info, err := os.Stat(path)
      if err != nil {
          return err
      }

      fingerprint, err := computeFingerprint(path)
      if err != nil {
          return err
      }

      file := &File{
          ID:          NewID(),
          SourceID:    sourceID,
          Path:        path,
          Filename:    filepath.Base(path),
          Size:        info.Size(),
          Mtime:       info.ModTime(),
          Fingerprint: fingerprint,
          CreatedAt:   time.Now(),
      }

      return s.repo.UpsertFile(ctx, file)
  }

  func computeFingerprint(path string) (string, error) {
      f, err := os.Open(path)
      if err != nil {
          return "", err
      }
      defer f.Close()

      h := sha256.New()
      if _, err := io.CopyN(h, f, fingerprintSize); err != nil && err != io.EOF {
          return "", err
      }

      return hex.EncodeToString(h.Sum(nil)), nil
  }

  func isVideoFile(name string) bool {
      ext := strings.ToLower(filepath.Ext(name))
      return VideoExtensions[ext]
  }
  ```

  **Recommended Agent Profile**:
  - **Category**: `unspecified-low`
    - Reason: Business logic layer with file I/O, moderate complexity
  - **Skills**: `[]`
    - No special skills needed
  - **Skills Evaluated but Omitted**:
    - None - standard Go patterns

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 9, 10)
  - **Blocks**: Tasks 14, 17
  - **Blocked By**: Task 6

  **References**:
  - Docs: filepath.WalkDir: https://pkg.go.dev/path/filepath#WalkDir
  - Docs: crypto/sha256: https://pkg.go.dev/crypto/sha256
  - Pattern: Service layer pattern

  **Acceptance Criteria**:

  **Agent-Executed QA Scenarios:**

  ```
  Scenario: AddFolder validates path
    Tool: Bash
    Steps:
      1. Call AddFolder with non-existent path
      2. Assert error returned
      3. Call AddFolder with file path (not dir)
      4. Assert error returned
    Expected Result: Both calls return appropriate errors
    Evidence: Test output

  Scenario: Scanner finds video files
    Tool: Bash
    Steps:
      1. Create temp dir with: test.mp4, test.txt, .hidden/test.mp4
      2. Run ScanFolder
      3. Query files from repo
    Expected Result: Only test.mp4 found (not .txt, not hidden)
    Evidence: File list

  Scenario: Fingerprint is computed correctly
    Tool: Bash
    Steps:
      1. Create test file with known content
      2. Compute fingerprint
      3. Compare to expected SHA-256 of first 64KB
    Expected Result: Fingerprints match
    Evidence: Hash comparison
  ```

  **Commit**: YES
  - Message: `feat(catalog): add service layer with scanner`
  - Files: `internal/catalog/service.go`

---

- [ ] 9. Playback Server

  **What to do**:
  - Create `internal/playback/server.go` with video file serving
  - Implement HTTP handler that serves files with Range support
  - Return 206 Partial Content for Range requests
  - Return 200 OK for full file requests
  - Set proper headers: Content-Type, Content-Length, Content-Range, Accept-Ranges
  - Use `io.Copy` with seeking for efficient streaming

  **Must NOT do**:
  - Do NOT buffer entire file in memory
  - Do NOT expose file paths to clients (use file IDs)

  **Files to create**:
  ```
  internal/playback/server.go
  ```

  **Key implementation details**:

  ```go
  // internal/playback/server.go
  package playback

  import (
      "fmt"
      "io"
      "log/slog"
      "mime"
      "net/http"
      "os"
      "path/filepath"
  )

  // PlaybackService handles video file serving
  type PlaybackService interface {
      ServeFile(w http.ResponseWriter, r *http.Request, filePath string) error
  }

  // Server implements PlaybackService
  type Server struct {
      logger *slog.Logger
  }

  // NewServer creates a new playback server
  func NewServer(logger *slog.Logger) *Server {
      return &Server{logger: logger}
  }

  // ServeFile serves a video file with Range support
  func (s *Server) ServeFile(w http.ResponseWriter, r *http.Request, filePath string) error {
      file, err := os.Open(filePath)
      if err != nil {
          return fmt.Errorf("failed to open file: %w", err)
      }
      defer file.Close()

      stat, err := file.Stat()
      if err != nil {
          return fmt.Errorf("failed to stat file: %w", err)
      }

      size := stat.Size()
      contentType := mime.TypeByExtension(filepath.Ext(filePath))
      if contentType == "" {
          contentType = "application/octet-stream"
      }

      // Always indicate we support ranges
      w.Header().Set("Accept-Ranges", "bytes")
      w.Header().Set("Content-Type", contentType)

      // Parse Range header
      rangeHeader := r.Header.Get("Range")
      parsedRange, err := ParseRange(rangeHeader, size)
      if err == ErrUnsatisfiable {
          w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", size))
          http.Error(w, "Range Not Satisfiable", http.StatusRequestedRangeNotSatisfiable)
          return nil
      }
      if err != nil && err != ErrInvalidRange {
          return err
      }

      // No range or invalid range - serve full file
      if parsedRange == nil {
          w.Header().Set("Content-Length", fmt.Sprintf("%d", size))
          w.WriteHeader(http.StatusOK)
          io.Copy(w, file)
          return nil
      }

      // Serve partial content
      w.Header().Set("Content-Length", fmt.Sprintf("%d", parsedRange.ContentLength()))
      w.Header().Set("Content-Range", parsedRange.ContentRange(size))
      w.WriteHeader(http.StatusPartialContent)

      // Seek to start of range
      if _, err := file.Seek(parsedRange.Start, io.SeekStart); err != nil {
          return fmt.Errorf("failed to seek: %w", err)
      }

      // Copy only the requested bytes
      io.CopyN(w, file, parsedRange.ContentLength())
      return nil
  }
  ```

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Uses the Range parser from Task 7, straightforward implementation
  - **Skills**: `[]`
    - No special skills needed
  - **Skills Evaluated but Omitted**:
    - None - standard HTTP patterns

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 8, 10)
  - **Blocks**: Task 14
  - **Blocked By**: Task 7

  **References**:
  - Docs: RFC 7233 (HTTP Range)
  - Docs: io.Copy, io.CopyN, io.Seeker

  **Acceptance Criteria**:

  **Agent-Executed QA Scenarios:**

  ```
  Scenario: Full file served with 200 OK
    Tool: Bash
    Steps:
      1. Start test server
      2. curl -I http://localhost:test/file (no Range header)
      3. Assert status 200
      4. Assert Accept-Ranges: bytes
    Expected Result: 200 OK with full content
    Evidence: curl output

  Scenario: Partial content served with 206
    Tool: Bash
    Steps:
      1. curl -H "Range: bytes=0-99" http://localhost:test/file
      2. Assert status 206
      3. Assert Content-Range: bytes 0-99/total
      4. Assert Content-Length: 100
    Expected Result: 206 Partial Content
    Evidence: curl output

  Scenario: Unsatisfiable range returns 416
    Tool: Bash
    Steps:
      1. curl -H "Range: bytes=9999999-" http://localhost:test/file
      2. Assert status 416
    Expected Result: 416 Range Not Satisfiable
    Evidence: curl output
  ```

  **Commit**: YES (groups with Task 8)
  - Message: `feat(playback): add video server with Range support`
  - Files: `internal/playback/server.go`

---

- [ ] 10. Job Runner

  **What to do**:
  - Create job runner component in `internal/catalog/runner.go`
  - Poll for pending jobs, execute one at a time
  - Support graceful shutdown via context cancellation
  - Execute scan jobs using CatalogService.ScanFolder
  - Update job status on completion/failure

  **Must NOT do**:
  - Do NOT run multiple jobs concurrently (v0 = max 1)
  - Do NOT block shutdown indefinitely

  **Files to create**:
  ```
  internal/catalog/runner.go
  ```

  **Key implementation details**:

  ```go
  // internal/catalog/runner.go
  package catalog

  import (
      "context"
      "log/slog"
      "time"
  )

  // Runner executes background jobs
  type Runner struct {
      service    *Service
      repo       Repository
      logger     *slog.Logger
      pollInterval time.Duration
  }

  // NewRunner creates a new job runner
  func NewRunner(service *Service, repo Repository, logger *slog.Logger) *Runner {
      return &Runner{
          service:      service,
          repo:         repo,
          logger:       logger,
          pollInterval: 5 * time.Second,
      }
  }

  // Start begins the job processing loop
  func (r *Runner) Start(ctx context.Context) {
      r.logger.Info("job runner started")
      
      ticker := time.NewTicker(r.pollInterval)
      defer ticker.Stop()

      for {
          select {
          case <-ctx.Done():
              r.logger.Info("job runner stopping")
              return
          case <-ticker.C:
              r.processNextJob(ctx)
          }
      }
  }

  func (r *Runner) processNextJob(ctx context.Context) {
      // Find pending job
      jobs, err := r.repo.ListJobs(ctx)
      if err != nil {
          r.logger.Error("failed to list jobs", "error", err)
          return
      }

      var pendingJob *Job
      for _, j := range jobs {
          if j.Status == JobStatusPending {
              pendingJob = j
              break
          }
      }

      if pendingJob == nil {
          return // No pending jobs
      }

      r.logger.Info("processing job", "job_id", pendingJob.ID, "type", pendingJob.Type)

      switch pendingJob.Type {
      case JobTypeScan:
          source, err := r.repo.GetSource(ctx, pendingJob.SourceID)
          if err != nil || source == nil {
              r.repo.UpdateJobStatus(ctx, pendingJob.ID, JobStatusFailed, "source not found")
              return
          }
          
          if err := r.service.ScanFolder(ctx, pendingJob.ID, source.ID, source.Path); err != nil {
              r.logger.Error("scan failed", "error", err)
          }
      default:
          r.logger.Warn("unknown job type", "type", pendingJob.Type)
          r.repo.UpdateJobStatus(ctx, pendingJob.ID, JobStatusFailed, "unknown job type")
      }
  }
  ```

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Simple polling loop with context handling
  - **Skills**: `[]`
    - No special skills needed
  - **Skills Evaluated but Omitted**:
    - None - standard Go concurrency

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 8, 9)
  - **Blocks**: Task 17
  - **Blocked By**: Task 6

  **References**:
  - Pattern: Worker pool pattern (simplified to single worker)
  - Docs: context.Context for cancellation

  **Acceptance Criteria**:

  **Agent-Executed QA Scenarios:**

  ```
  Scenario: Runner processes pending jobs
    Tool: Bash
    Steps:
      1. Insert pending scan job into DB
      2. Start runner
      3. Wait for poll interval
      4. Check job status
    Expected Result: Job status = completed
    Evidence: Job query result

  Scenario: Runner stops on context cancel
    Tool: Bash
    Steps:
      1. Start runner with context
      2. Cancel context
      3. Verify runner exits
    Expected Result: Runner logs "stopping" and exits
    Evidence: Log output

  Scenario: Runner handles missing source
    Tool: Bash
    Steps:
      1. Insert job with non-existent source_id
      2. Let runner process it
    Expected Result: Job marked as failed with "source not found"
    Evidence: Job status query
  ```

  **Commit**: YES
  - Message: `feat(catalog): add job runner`
  - Files: `internal/catalog/runner.go`

---

### Wave 5: API Foundation

---

- [ ] 11. API Schemas

  **What to do**:
  - Create `internal/api/schemas.go` with request/response types
  - Define JSON schemas for all endpoints
  - Include validation tags where appropriate
  - Error response format: `{"error": "message", "code": "ERROR_CODE"}`

  **Must NOT do**:
  - Do NOT include handler logic (that's routes)
  - Do NOT use external validation libraries

  **Files to create**:
  ```
  internal/api/schemas.go
  ```

  **Key implementation details**:

  ```go
  // internal/api/schemas.go
  package api

  import "github.com/heimdex/heimdex-agent/internal/catalog"

  // HealthResponse for GET /health
  type HealthResponse struct {
      Status  string `json:"status"`
      Version string `json:"version"`
  }

  // StatusResponse for GET /status
  type StatusResponse struct {
      Status         string `json:"status"` // idle, indexing, paused, error
      Sources        int    `json:"sources"`
      Files          int    `json:"files"`
      ActiveJob      *JobResponse `json:"active_job,omitempty"`
  }

  // AddFolderRequest for POST /sources/folders
  type AddFolderRequest struct {
      Path        string `json:"path"`
      DisplayName string `json:"display_name"`
  }

  // SourceResponse represents a source in API responses
  type SourceResponse struct {
      ID            string `json:"id"`
      Type          string `json:"type"`
      Path          string `json:"path"`
      DisplayName   string `json:"display_name"`
      DriveNickname string `json:"drive_nickname,omitempty"`
      Present       bool   `json:"present"`
      CreatedAt     string `json:"created_at"`
  }

  // SourcesResponse for GET /sources
  type SourcesResponse struct {
      Sources []SourceResponse `json:"sources"`
  }

  // ScanRequest for POST /scan
  type ScanRequest struct {
      SourceID string `json:"source_id"`
  }

  // JobResponse represents a job in API responses
  type JobResponse struct {
      ID        string `json:"id"`
      Type      string `json:"type"`
      Status    string `json:"status"`
      SourceID  string `json:"source_id,omitempty"`
      Progress  int    `json:"progress"`
      Error     string `json:"error,omitempty"`
      CreatedAt string `json:"created_at"`
      UpdatedAt string `json:"updated_at"`
  }

  // JobsResponse for GET /jobs
  type JobsResponse struct {
      Jobs []JobResponse `json:"jobs"`
  }

  // ErrorResponse for all error responses
  type ErrorResponse struct {
      Error string `json:"error"`
      Code  string `json:"code,omitempty"`
  }

  // Conversion helpers
  func SourceToResponse(s *catalog.Source) SourceResponse {
      return SourceResponse{
          ID:            s.ID,
          Type:          s.Type,
          Path:          s.Path,
          DisplayName:   s.DisplayName,
          DriveNickname: s.DriveNickname,
          Present:       s.Present,
          CreatedAt:     s.CreatedAt.Format(time.RFC3339),
      }
  }

  func JobToResponse(j *catalog.Job) JobResponse {
      return JobResponse{
          ID:        j.ID,
          Type:      j.Type,
          Status:    j.Status,
          SourceID:  j.SourceID,
          Progress:  j.Progress,
          Error:     j.Error,
          CreatedAt: j.CreatedAt.Format(time.RFC3339),
          UpdatedAt: j.UpdatedAt.Format(time.RFC3339),
      }
  }
  ```

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Pure data structures
  - **Skills**: `[]`
    - No special skills needed
  - **Skills Evaluated but Omitted**:
    - None

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 5 (with Tasks 12, 13)
  - **Blocks**: Task 14
  - **Blocked By**: Task 2

  **References**:
  - Pattern: DTO pattern for API layer
  - Docs: encoding/json struct tags

  **Acceptance Criteria**:

  **Agent-Executed QA Scenarios:**

  ```
  Scenario: Schemas compile correctly
    Tool: Bash
    Steps:
      1. go build ./internal/api/...
    Expected Result: No compilation errors
    Evidence: Build output

  Scenario: JSON marshaling works
    Tool: Bash
    Steps:
      1. Create test that marshals each response type
      2. Verify JSON has expected fields
    Expected Result: Valid JSON output
    Evidence: JSON output
  ```

  **Commit**: YES (groups with Task 12)
  - Message: `feat(api): add request/response schemas`
  - Files: `internal/api/schemas.go`

---

- [ ] 12. API Middleware

  **What to do**:
  - Create `internal/api/middleware.go` with HTTP middleware
  - Implement Bearer token authentication middleware
  - Implement request logging middleware (using slog)
  - Implement panic recovery middleware
  - Implement request ID injection middleware
  - Auth: check `Authorization: Bearer <token>` against config table

  **Must NOT do**:
  - Do NOT log full tokens (use SanitizeToken)
  - Do NOT allow auth bypass

  **Files to create**:
  ```
  internal/api/middleware.go
  ```

  **Key implementation details**:

  ```go
  // internal/api/middleware.go
  package api

  import (
      "context"
      "log/slog"
      "net/http"
      "strings"
      "time"

      "github.com/heimdex/heimdex-agent/internal/catalog"
      "github.com/heimdex/heimdex-agent/internal/logging"
  )

  type contextKey string

  const RequestIDKey contextKey = "request_id"

  // AuthMiddleware validates Bearer token authentication
  func AuthMiddleware(repo catalog.Repository, logger *slog.Logger) func(http.Handler) http.Handler {
      return func(next http.Handler) http.Handler {
          return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
              auth := r.Header.Get("Authorization")
              if auth == "" {
                  WriteError(w, http.StatusUnauthorized, "missing authorization header", "UNAUTHORIZED")
                  return
              }

              if !strings.HasPrefix(auth, "Bearer ") {
                  WriteError(w, http.StatusUnauthorized, "invalid authorization format", "UNAUTHORIZED")
                  return
              }

              token := strings.TrimPrefix(auth, "Bearer ")
              
              // Get stored token from config
              storedToken, err := repo.GetConfig(r.Context(), "auth_token")
              if err != nil || storedToken == "" {
                  logger.Error("failed to get auth token from config", "error", err)
                  WriteError(w, http.StatusInternalServerError, "auth configuration error", "INTERNAL_ERROR")
                  return
              }

              if token != storedToken {
                  logger.Warn("invalid auth token", "provided", logging.SanitizeToken(token))
                  WriteError(w, http.StatusUnauthorized, "invalid token", "UNAUTHORIZED")
                  return
              }

              next.ServeHTTP(w, r)
          })
      }
  }

  // LoggingMiddleware logs HTTP requests
  func LoggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
      return func(next http.Handler) http.Handler {
          return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
              start := time.Now()
              
              // Wrap response writer to capture status
              wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}
              
              next.ServeHTTP(wrapped, r)
              
              logger.Info("http request",
                  "method", r.Method,
                  "path", r.URL.Path,
                  "status", wrapped.status,
                  "duration_ms", time.Since(start).Milliseconds(),
                  "request_id", r.Context().Value(RequestIDKey),
              )
          })
      }
  }

  // RecoveryMiddleware recovers from panics
  func RecoveryMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
      return func(next http.Handler) http.Handler {
          return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
              defer func() {
                  if err := recover(); err != nil {
                      logger.Error("panic recovered", "error", err, "request_id", r.Context().Value(RequestIDKey))
                      WriteError(w, http.StatusInternalServerError, "internal server error", "INTERNAL_ERROR")
                  }
              }()
              next.ServeHTTP(w, r)
          })
      }
  }

  // RequestIDMiddleware adds request ID to context
  func RequestIDMiddleware() func(http.Handler) http.Handler {
      return func(next http.Handler) http.Handler {
          return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
              requestID := catalog.NewID()[:8] // Short ID for request
              ctx := context.WithValue(r.Context(), RequestIDKey, requestID)
              w.Header().Set("X-Request-ID", requestID)
              next.ServeHTTP(w, r.WithContext(ctx))
          })
      }
  }

  type responseWriter struct {
      http.ResponseWriter
      status int
  }

  func (w *responseWriter) WriteHeader(status int) {
      w.status = status
      w.ResponseWriter.WriteHeader(status)
  }

  // WriteError writes a JSON error response
  func WriteError(w http.ResponseWriter, status int, message, code string) {
      w.Header().Set("Content-Type", "application/json")
      w.WriteHeader(status)
      json.NewEncoder(w).Encode(ErrorResponse{Error: message, Code: code})
  }
  ```

  **Recommended Agent Profile**:
  - **Category**: `unspecified-low`
    - Reason: HTTP middleware patterns, security considerations
  - **Skills**: `[]`
    - No special skills needed
  - **Skills Evaluated but Omitted**:
    - None

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 5 (with Tasks 11, 13)
  - **Blocks**: Task 14
  - **Blocked By**: Tasks 2, 3

  **References**:
  - Docs: go-chi middleware patterns
  - Pattern: Bearer token authentication
  - Research: See librarian findings on chi middleware

  **Acceptance Criteria**:

  **Agent-Executed QA Scenarios:**

  ```
  Scenario: Auth middleware rejects missing token
    Tool: Bash
    Steps:
      1. Send request without Authorization header
      2. Assert status 401
      3. Assert response contains "missing authorization"
    Expected Result: 401 Unauthorized
    Evidence: curl output

  Scenario: Auth middleware rejects invalid token
    Tool: Bash
    Steps:
      1. Send request with invalid Bearer token
      2. Assert status 401
    Expected Result: 401 Unauthorized
    Evidence: curl output

  Scenario: Auth middleware accepts valid token
    Tool: Bash
    Steps:
      1. Get token from config table
      2. Send request with valid Bearer token
      3. Assert request succeeds (passes to handler)
    Expected Result: Request processed
    Evidence: Response from handler

  Scenario: Recovery middleware handles panics
    Tool: Bash
    Steps:
      1. Create handler that panics
      2. Wrap with recovery middleware
      3. Send request
    Expected Result: 500 response (not crash)
    Evidence: Response status
  ```

  **Commit**: YES (groups with Task 11)
  - Message: `feat(api): add authentication and logging middleware`
  - Files: `internal/api/middleware.go`

---

- [ ] 13. Cloud Stubs

  **What to do**:
  - Create `internal/cloud/client.go` with cloud communication interface
  - Create `internal/cloud/auth.go` with cloud auth stub
  - Create `internal/cloud/upload.go` with upload stub
  - All methods log intent but perform no real API calls
  - Define interfaces for future implementation

  **Must NOT do**:
  - Do NOT make real HTTP calls
  - Do NOT store cloud credentials

  **Files to create**:
  ```
  internal/cloud/client.go
  internal/cloud/auth.go
  internal/cloud/upload.go
  ```

  **Key implementation details**:

  ```go
  // internal/cloud/client.go
  package cloud

  import "log/slog"

  // Client is the cloud communication interface
  type Client interface {
      Auth() AuthService
      Upload() UploadService
  }

  // StubClient is a no-op implementation for v0
  type StubClient struct {
      auth   *StubAuth
      upload *StubUpload
  }

  func NewStubClient(logger *slog.Logger) *StubClient {
      return &StubClient{
          auth:   &StubAuth{logger: logger},
          upload: &StubUpload{logger: logger},
      }
  }

  func (c *StubClient) Auth() AuthService     { return c.auth }
  func (c *StubClient) Upload() UploadService { return c.upload }
  ```

  ```go
  // internal/cloud/auth.go
  package cloud

  import "log/slog"

  // AuthService handles cloud authentication
  type AuthService interface {
      Login(email, password string) error
      Logout() error
      IsAuthenticated() bool
  }

  // StubAuth is a no-op implementation
  type StubAuth struct {
      logger *slog.Logger
  }

  func (s *StubAuth) Login(email, password string) error {
      s.logger.Info("cloud auth stub: login requested", "email", email)
      return nil
  }

  func (s *StubAuth) Logout() error {
      s.logger.Info("cloud auth stub: logout requested")
      return nil
  }

  func (s *StubAuth) IsAuthenticated() bool {
      s.logger.Debug("cloud auth stub: checking auth status")
      return false
  }
  ```

  ```go
  // internal/cloud/upload.go
  package cloud

  import "log/slog"

  // UploadService handles file uploads to cloud
  type UploadService interface {
      UploadMetadata(fileID string, metadata map[string]interface{}) error
      UploadFile(fileID, filePath string) error
  }

  // StubUpload is a no-op implementation
  type StubUpload struct {
      logger *slog.Logger
  }

  func (s *StubUpload) UploadMetadata(fileID string, metadata map[string]interface{}) error {
      s.logger.Info("cloud upload stub: metadata upload requested", "file_id", fileID)
      return nil
  }

  func (s *StubUpload) UploadFile(fileID, filePath string) error {
      s.logger.Info("cloud upload stub: file upload requested", "file_id", fileID, "path", filePath)
      return nil
  }
  ```

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Simple stub implementations
  - **Skills**: `[]`
    - No special skills needed
  - **Skills Evaluated but Omitted**:
    - None

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 5 (with Tasks 11, 12)
  - **Blocks**: Task 17
  - **Blocked By**: Tasks 2, 3

  **References**:
  - Pattern: Interface-based stubs for future implementation

  **Acceptance Criteria**:

  **Agent-Executed QA Scenarios:**

  ```
  Scenario: Cloud stubs compile
    Tool: Bash
    Steps:
      1. go build ./internal/cloud/...
    Expected Result: No errors
    Evidence: Build output

  Scenario: Stubs log intent
    Tool: Bash
    Steps:
      1. Call StubAuth.Login("test@test.com", "pass")
      2. Verify log contains "login requested"
    Expected Result: Intent logged
    Evidence: Log output

  Scenario: Stubs return no errors
    Tool: Bash
    Steps:
      1. Call all stub methods
      2. Verify all return nil error
    Expected Result: All succeed (no-op)
    Evidence: Test output
  ```

  **Commit**: YES
  - Message: `feat(cloud): add cloud communication stubs`
  - Files: `internal/cloud/client.go`, `internal/cloud/auth.go`, `internal/cloud/upload.go`

---

### Wave 6: API Routes

---

- [ ] 14. API Routes and Handlers

  **What to do**:
  - Create `internal/api/server.go` with HTTP server setup
  - Create `internal/api/routes.go` with chi router configuration
  - Implement all endpoint handlers:
    - GET /health (no auth)
    - GET /status (auth)
    - POST /sources/folders (auth)
    - GET /sources (auth)
    - POST /scan (auth)
    - GET /jobs (auth)
    - GET /playback/file?file_id=... (auth)
  - Bind to 127.0.0.1 only
  - Support graceful shutdown

  **Must NOT do**:
  - Do NOT bind to 0.0.0.0
  - Do NOT expose file paths (use file_id lookup)

  **Files to create**:
  ```
  internal/api/server.go
  internal/api/routes.go
  ```

  **Key implementation details**:

  ```go
  // internal/api/server.go
  package api

  import (
      "context"
      "fmt"
      "log/slog"
      "net/http"
      "time"

      "github.com/heimdex/heimdex-agent/internal/catalog"
      "github.com/heimdex/heimdex-agent/internal/playback"
  )

  // Server is the HTTP API server
  type Server struct {
      httpServer *http.Server
      logger     *slog.Logger
  }

  // Config holds server configuration
  type ServerConfig struct {
      Port           int
      CatalogService catalog.CatalogService
      PlaybackServer playback.PlaybackService
      Repository     catalog.Repository
      Logger         *slog.Logger
  }

  // NewServer creates a new API server
  func NewServer(cfg ServerConfig) *Server {
      router := NewRouter(cfg)
      
      return &Server{
          httpServer: &http.Server{
              Addr:         fmt.Sprintf("127.0.0.1:%d", cfg.Port),
              Handler:      router,
              ReadTimeout:  15 * time.Second,
              WriteTimeout: 0, // Disabled for video streaming
              IdleTimeout:  60 * time.Second,
          },
          logger: cfg.Logger,
      }
  }

  // Start begins listening for requests
  func (s *Server) Start() error {
      s.logger.Info("starting HTTP server", "addr", s.httpServer.Addr)
      return s.httpServer.ListenAndServe()
  }

  // Shutdown gracefully stops the server
  func (s *Server) Shutdown(ctx context.Context) error {
      s.logger.Info("shutting down HTTP server")
      return s.httpServer.Shutdown(ctx)
  }
  ```

  ```go
  // internal/api/routes.go
  package api

  import (
      "encoding/json"
      "net/http"

      "github.com/go-chi/chi/v5"
  )

  // NewRouter creates the chi router with all routes
  func NewRouter(cfg ServerConfig) *chi.Mux {
      r := chi.NewRouter()

      // Global middleware
      r.Use(RequestIDMiddleware())
      r.Use(RecoveryMiddleware(cfg.Logger))
      r.Use(LoggingMiddleware(cfg.Logger))

      // Public routes (no auth)
      r.Get("/health", healthHandler())

      // Protected routes (auth required)
      r.Group(func(r chi.Router) {
          r.Use(AuthMiddleware(cfg.Repository, cfg.Logger))
          
          r.Get("/status", statusHandler(cfg.CatalogService, cfg.Repository))
          r.Post("/sources/folders", addFolderHandler(cfg.CatalogService))
          r.Get("/sources", listSourcesHandler(cfg.CatalogService))
          r.Post("/scan", scanHandler(cfg.CatalogService))
          r.Get("/jobs", listJobsHandler(cfg.Repository))
          r.Get("/playback/file", playbackHandler(cfg.CatalogService, cfg.PlaybackServer))
      })

      return r
  }

  func healthHandler() http.HandlerFunc {
      return func(w http.ResponseWriter, r *http.Request) {
          writeJSON(w, http.StatusOK, HealthResponse{
              Status:  "ok",
              Version: "0.1.0",
          })
      }
  }

  func statusHandler(svc catalog.CatalogService, repo catalog.Repository) http.HandlerFunc {
      return func(w http.ResponseWriter, r *http.Request) {
          ctx := r.Context()
          sources, _ := svc.GetSources(ctx)
          jobs, _ := repo.ListJobs(ctx)
          
          status := "idle"
          var activeJob *JobResponse
          for _, j := range jobs {
              if j.Status == catalog.JobStatusRunning {
                  status = "indexing"
                  resp := JobToResponse(j)
                  activeJob = &resp
                  break
              }
          }

          writeJSON(w, http.StatusOK, StatusResponse{
              Status:    status,
              Sources:   len(sources),
              Files:     0, // TODO: count files
              ActiveJob: activeJob,
          })
      }
  }

  func addFolderHandler(svc catalog.CatalogService) http.HandlerFunc {
      return func(w http.ResponseWriter, r *http.Request) {
          var req AddFolderRequest
          if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
              WriteError(w, http.StatusBadRequest, "invalid request body", "BAD_REQUEST")
              return
          }

          if req.Path == "" {
              WriteError(w, http.StatusBadRequest, "path is required", "BAD_REQUEST")
              return
          }

          source, err := svc.AddFolder(r.Context(), req.Path, req.DisplayName)
          if err != nil {
              WriteError(w, http.StatusBadRequest, err.Error(), "BAD_REQUEST")
              return
          }

          writeJSON(w, http.StatusCreated, SourceToResponse(source))
      }
  }

  func listSourcesHandler(svc catalog.CatalogService) http.HandlerFunc {
      return func(w http.ResponseWriter, r *http.Request) {
          sources, err := svc.GetSources(r.Context())
          if err != nil {
              WriteError(w, http.StatusInternalServerError, "failed to list sources", "INTERNAL_ERROR")
              return
          }

          resp := SourcesResponse{Sources: make([]SourceResponse, len(sources))}
          for i, s := range sources {
              resp.Sources[i] = SourceToResponse(s)
          }
          writeJSON(w, http.StatusOK, resp)
      }
  }

  func scanHandler(svc catalog.CatalogService) http.HandlerFunc {
      return func(w http.ResponseWriter, r *http.Request) {
          var req ScanRequest
          if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
              WriteError(w, http.StatusBadRequest, "invalid request body", "BAD_REQUEST")
              return
          }

          job, err := svc.ScanSource(r.Context(), req.SourceID)
          if err != nil {
              WriteError(w, http.StatusBadRequest, err.Error(), "BAD_REQUEST")
              return
          }

          writeJSON(w, http.StatusAccepted, JobToResponse(job))
      }
  }

  func listJobsHandler(repo catalog.Repository) http.HandlerFunc {
      return func(w http.ResponseWriter, r *http.Request) {
          jobs, err := repo.ListJobs(r.Context())
          if err != nil {
              WriteError(w, http.StatusInternalServerError, "failed to list jobs", "INTERNAL_ERROR")
              return
          }

          resp := JobsResponse{Jobs: make([]JobResponse, len(jobs))}
          for i, j := range jobs {
              resp.Jobs[i] = JobToResponse(j)
          }
          writeJSON(w, http.StatusOK, resp)
      }
  }

  func playbackHandler(svc catalog.CatalogService, pb playback.PlaybackService) http.HandlerFunc {
      return func(w http.ResponseWriter, r *http.Request) {
          fileID := r.URL.Query().Get("file_id")
          if fileID == "" {
              WriteError(w, http.StatusBadRequest, "file_id is required", "BAD_REQUEST")
              return
          }

          file, err := svc.GetFile(r.Context(), fileID)
          if err != nil || file == nil {
              WriteError(w, http.StatusNotFound, "file not found", "NOT_FOUND")
              return
          }

          if err := pb.ServeFile(w, r, file.Path); err != nil {
              // Error already written to response by ServeFile
              return
          }
      }
  }

  func writeJSON(w http.ResponseWriter, status int, data interface{}) {
      w.Header().Set("Content-Type", "application/json")
      w.WriteHeader(status)
      json.NewEncoder(w).Encode(data)
  }
  ```

  **Recommended Agent Profile**:
  - **Category**: `unspecified-low`
    - Reason: HTTP handlers with multiple endpoints, moderate complexity
  - **Skills**: `[]`
    - No special skills needed
  - **Skills Evaluated but Omitted**:
    - None - standard chi patterns

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 6 (with Task 15)
  - **Blocks**: Task 17
  - **Blocked By**: Tasks 8, 9, 11, 12

  **References**:
  - Research: See librarian findings on chi router patterns
  - Docs: go-chi/chi v5

  **Acceptance Criteria**:

  **Agent-Executed QA Scenarios:**

  ```
  Scenario: Health endpoint works without auth
    Tool: Bash
    Steps:
      1. Start server
      2. curl http://127.0.0.1:8787/health
      3. Assert status 200
      4. Assert response contains "ok"
    Expected Result: {"status":"ok","version":"0.1.0"}
    Evidence: curl output

  Scenario: Protected endpoints require auth
    Tool: Bash
    Steps:
      1. curl http://127.0.0.1:8787/status (no auth)
      2. Assert status 401
    Expected Result: 401 Unauthorized
    Evidence: curl output

  Scenario: Add folder creates source
    Tool: Bash
    Steps:
      1. Create temp directory
      2. POST /sources/folders with path
      3. Assert status 201
      4. GET /sources
      5. Assert new source in list
    Expected Result: Source created and listed
    Evidence: API responses

  Scenario: Playback returns video with Range support
    Tool: Bash
    Steps:
      1. Add folder with video file
      2. Scan folder
      3. Get file_id from files
      4. curl -H "Range: bytes=0-99" /playback/file?file_id=...
      5. Assert status 206
    Expected Result: 206 Partial Content
    Evidence: curl output with headers

  Scenario: Server binds to 127.0.0.1 only
    Tool: Bash
    Steps:
      1. Start server
      2. netstat -an | grep 8787
      3. Assert bound to 127.0.0.1:8787, not 0.0.0.0:8787
    Expected Result: Only localhost binding
    Evidence: netstat output
  ```

  **Commit**: YES
  - Message: `feat(api): add HTTP server with all endpoints`
  - Files: `internal/api/server.go`, `internal/api/routes.go`

---

- [ ] 15. Watcher and Pipeline Stubs

  **What to do**:
  - Create `internal/watcher/watcher.go` with file watcher interface stub
  - Create `internal/pipeline/pipeline.go` with processing pipeline interface stub
  - Create `internal/pipeline/ffmpeg.go` with ffmpeg integration stub
  - Define interfaces for future implementation
  - Log intent only, no real watching or processing

  **Must NOT do**:
  - Do NOT implement real file watching (v1 feature)
  - Do NOT implement real ffmpeg processing

  **Files to create**:
  ```
  internal/watcher/watcher.go
  internal/pipeline/pipeline.go
  internal/pipeline/ffmpeg.go
  ```

  **Key implementation details**:

  ```go
  // internal/watcher/watcher.go
  package watcher

  import (
      "context"
      "log/slog"
  )

  // Watcher monitors directories for file changes
  type Watcher interface {
      Watch(ctx context.Context, path string) error
      Stop() error
  }

  // StubWatcher is a no-op implementation for v0
  type StubWatcher struct {
      logger *slog.Logger
  }

  func NewStubWatcher(logger *slog.Logger) *StubWatcher {
      return &StubWatcher{logger: logger}
  }

  func (w *StubWatcher) Watch(ctx context.Context, path string) error {
      w.logger.Info("watcher stub: watch requested", "path", path)
      return nil
  }

  func (w *StubWatcher) Stop() error {
      w.logger.Info("watcher stub: stop requested")
      return nil
  }
  ```

  ```go
  // internal/pipeline/pipeline.go
  package pipeline

  import (
      "context"
      "log/slog"
  )

  // Pipeline processes video files
  type Pipeline interface {
      Process(ctx context.Context, fileID, filePath string) error
  }

  // StubPipeline is a no-op implementation for v0
  type StubPipeline struct {
      logger *slog.Logger
  }

  func NewStubPipeline(logger *slog.Logger) *StubPipeline {
      return &StubPipeline{logger: logger}
  }

  func (p *StubPipeline) Process(ctx context.Context, fileID, filePath string) error {
      p.logger.Info("pipeline stub: process requested", "file_id", fileID, "path", filePath)
      return nil
  }
  ```

  ```go
  // internal/pipeline/ffmpeg.go
  package pipeline

  import "log/slog"

  // FFmpeg wraps ffmpeg/ffprobe operations
  type FFmpeg interface {
      Probe(filePath string) (map[string]interface{}, error)
      GenerateThumbnail(filePath, outputPath string) error
  }

  // StubFFmpeg is a no-op implementation for v0
  type StubFFmpeg struct {
      logger *slog.Logger
  }

  func NewStubFFmpeg(logger *slog.Logger) *StubFFmpeg {
      return &StubFFmpeg{logger: logger}
  }

  func (f *StubFFmpeg) Probe(filePath string) (map[string]interface{}, error) {
      f.logger.Info("ffmpeg stub: probe requested", "path", filePath)
      return map[string]interface{}{}, nil
  }

  func (f *StubFFmpeg) GenerateThumbnail(filePath, outputPath string) error {
      f.logger.Info("ffmpeg stub: thumbnail requested", "input", filePath, "output", outputPath)
      return nil
  }
  ```

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Simple stub implementations
  - **Skills**: `[]`
    - No special skills needed
  - **Skills Evaluated but Omitted**:
    - None

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 6 (with Task 14)
  - **Blocks**: Task 17
  - **Blocked By**: Task 1

  **References**:
  - Pattern: Interface stubs for future implementation

  **Acceptance Criteria**:

  **Agent-Executed QA Scenarios:**

  ```
  Scenario: Stubs compile
    Tool: Bash
    Steps:
      1. go build ./internal/watcher/... ./internal/pipeline/...
    Expected Result: No errors
    Evidence: Build output

  Scenario: Stubs log intent
    Tool: Bash
    Steps:
      1. Create instances of all stubs
      2. Call methods
      3. Verify logs contain intent
    Expected Result: Intent logged for each call
    Evidence: Log output
  ```

  **Commit**: YES
  - Message: `feat: add watcher and pipeline stubs`
  - Files: `internal/watcher/watcher.go`, `internal/pipeline/pipeline.go`, `internal/pipeline/ffmpeg.go`

---

### Wave 7: UI & Integration

---

- [ ] 16. System Tray UI

  **What to do**:
  - Create `internal/ui/tray.go` with system tray implementation
  - Use getlantern/systray library
  - Display status menu item (Idle/Indexing/Paused/Error)
  - Display connected sources count
  - Add "Pause/Resume" toggle menu item
  - Add "Add Folder" menu item (opens file dialog)
  - Add "Quit" menu item
  - Update status dynamically based on job runner state

  **Must NOT do**:
  - Do NOT block the main goroutine with menu operations
  - Do NOT forget to call systray.Quit() on shutdown

  **Files to create**:
  ```
  internal/ui/tray.go
  internal/ui/icon.go (embed icon bytes)
  ```

  **Key implementation details**:

  ```go
  // internal/ui/tray.go
  package ui

  import (
      "context"
      "log/slog"
      "sync"

      "github.com/getlantern/systray"
      "github.com/heimdex/heimdex-agent/internal/catalog"
  )

  // Tray manages the system tray UI
  type Tray struct {
      catalogSvc catalog.CatalogService
      logger     *slog.Logger
      
      statusItem  *systray.MenuItem
      sourcesItem *systray.MenuItem
      pauseItem   *systray.MenuItem
      
      paused bool
      mu     sync.Mutex
      
      onAddFolder func() error
      onQuit      func()
  }

  // Config for tray initialization
  type TrayConfig struct {
      CatalogService catalog.CatalogService
      Logger         *slog.Logger
      OnAddFolder    func() error
      OnQuit         func()
  }

  // NewTray creates a new system tray manager
  func NewTray(cfg TrayConfig) *Tray {
      return &Tray{
          catalogSvc:  cfg.CatalogService,
          logger:      cfg.Logger,
          onAddFolder: cfg.OnAddFolder,
          onQuit:      cfg.OnQuit,
      }
  }

  // Run starts the system tray (blocks)
  func (t *Tray) Run() {
      systray.Run(t.onReady, t.onExit)
  }

  func (t *Tray) onReady() {
      systray.SetIcon(iconBytes)
      systray.SetTitle("Heimdex")
      systray.SetTooltip("Heimdex Agent")

      // Status display (disabled, just for display)
      t.statusItem = systray.AddMenuItem("Status: Idle", "Current agent status")
      t.statusItem.Disable()

      // Sources count
      t.sourcesItem = systray.AddMenuItem("Sources: 0", "Connected sources")
      t.sourcesItem.Disable()

      systray.AddSeparator()

      // Pause/Resume toggle
      t.pauseItem = systray.AddMenuItem("Pause", "Pause indexing")

      // Add folder
      addFolderItem := systray.AddMenuItem("Add Folder...", "Add a folder to index")

      systray.AddSeparator()

      // Quit
      quitItem := systray.AddMenuItem("Quit", "Quit Heimdex Agent")

      // Handle menu clicks in goroutine
      go func() {
          for {
              select {
              case <-t.pauseItem.ClickedCh:
                  t.togglePause()
              case <-addFolderItem.ClickedCh:
                  t.handleAddFolder()
              case <-quitItem.ClickedCh:
                  t.logger.Info("quit requested from tray")
                  if t.onQuit != nil {
                      t.onQuit()
                  }
                  systray.Quit()
                  return
              }
          }
      }()

      t.logger.Info("system tray ready")
  }

  func (t *Tray) onExit() {
      t.logger.Info("system tray exiting")
  }

  func (t *Tray) togglePause() {
      t.mu.Lock()
      defer t.mu.Unlock()
      
      t.paused = !t.paused
      if t.paused {
          t.pauseItem.SetTitle("Resume")
          t.statusItem.SetTitle("Status: Paused")
      } else {
          t.pauseItem.SetTitle("Pause")
          t.statusItem.SetTitle("Status: Idle")
      }
  }

  func (t *Tray) handleAddFolder() {
      if t.onAddFolder != nil {
          if err := t.onAddFolder(); err != nil {
              t.logger.Error("failed to add folder", "error", err)
          }
      }
  }

  // UpdateStatus updates the status display
  func (t *Tray) UpdateStatus(status string) {
      t.mu.Lock()
      defer t.mu.Unlock()
      
      if !t.paused {
          t.statusItem.SetTitle("Status: " + status)
      }
  }

  // UpdateSourcesCount updates the sources count display
  func (t *Tray) UpdateSourcesCount(count int) {
      t.sourcesItem.SetTitle(fmt.Sprintf("Sources: %d", count))
  }

  // IsPaused returns whether indexing is paused
  func (t *Tray) IsPaused() bool {
      t.mu.Lock()
      defer t.mu.Unlock()
      return t.paused
  }
  ```

  ```go
  // internal/ui/icon.go
  package ui

  // iconBytes contains the tray icon
  // TODO: Replace with actual icon bytes
  var iconBytes = []byte{
      // Placeholder - generate actual icon
  }
  ```

  **Recommended Agent Profile**:
  - **Category**: `unspecified-low`
    - Reason: Desktop UI with callbacks, threading considerations
  - **Skills**: `[]`
    - No special skills needed
  - **Skills Evaluated but Omitted**:
    - `frontend-ui-ux`: Not web frontend

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 7 (with Task 17 - but 17 depends on 16)
  - **Blocks**: Task 17
  - **Blocked By**: Tasks 2, 3

  **References**:
  - Research: See librarian findings on getlantern/systray
  - Docs: getlantern/systray README

  **Acceptance Criteria**:

  **Agent-Executed QA Scenarios:**

  ```
  Scenario: Tray compiles (CGO check)
    Tool: Bash
    Steps:
      1. go build ./internal/ui/...
    Expected Result: Compiles (may need CGO for systray)
    Evidence: Build output

  Scenario: Tray shows menu items
    Tool: Bash (limited - visual verification needed)
    Steps:
      1. Build and run agent
      2. Verify tray icon appears
      3. Click tray icon
      4. Verify menu shows: Status, Sources, Pause, Add Folder, Quit
    Expected Result: All menu items visible
    Evidence: Manual verification (screenshot if possible)

  Scenario: Pause/Resume toggles
    Tool: Bash (limited)
    Steps:
      1. Click Pause
      2. Verify status shows "Paused"
      3. Click Resume
      4. Verify status shows "Idle"
    Expected Result: Toggle works
    Evidence: Visual verification
  ```

  **Commit**: YES
  - Message: `feat(ui): add system tray UI`
  - Files: `internal/ui/tray.go`, `internal/ui/icon.go`

---

- [ ] 17. Main Entry Point

  **What to do**:
  - Create `cmd/agent/main.go` that wires everything together
  - Initialize config, logging, database
  - Create all services with dependency injection
  - Generate auth token on first run (store in config table)
  - Print auth token to stdout on startup
  - Start job runner in background goroutine
  - Start HTTP server in background goroutine
  - Run systray (blocks main thread)
  - Handle graceful shutdown (SIGINT/SIGTERM)

  **Must NOT do**:
  - Do NOT use global variables
  - Do NOT skip graceful shutdown

  **Files to create**:
  ```
  cmd/agent/main.go
  ```

  **Key implementation details**:

  ```go
  // cmd/agent/main.go
  package main

  import (
      "context"
      "crypto/rand"
      "encoding/hex"
      "fmt"
      "log"
      "os"
      "os/signal"
      "syscall"
      "time"

      "github.com/heimdex/heimdex-agent/internal/api"
      "github.com/heimdex/heimdex-agent/internal/catalog"
      "github.com/heimdex/heimdex-agent/internal/cloud"
      "github.com/heimdex/heimdex-agent/internal/config"
      "github.com/heimdex/heimdex-agent/internal/db"
      "github.com/heimdex/heimdex-agent/internal/logging"
      "github.com/heimdex/heimdex-agent/internal/playback"
      "github.com/heimdex/heimdex-agent/internal/ui"
  )

  func main() {
      if err := run(); err != nil {
          log.Fatalf("fatal error: %v", err)
      }
  }

  func run() error {
      // Load configuration
      cfg, err := config.New()
      if err != nil {
          return fmt.Errorf("failed to load config: %w", err)
      }

      // Ensure data directory exists
      if err := os.MkdirAll(cfg.DataDir(), 0755); err != nil {
          return fmt.Errorf("failed to create data dir: %w", err)
      }

      // Initialize logging
      logger := logging.NewLogger(cfg.LogLevel())
      logger.Info("starting heimdex agent", "version", "0.1.0", "data_dir", cfg.DataDir())

      // Initialize database
      database, err := db.New(cfg.DBPath(), logger)
      if err != nil {
          return fmt.Errorf("failed to initialize database: %w", err)
      }
      defer database.Close()

      // Initialize repository
      repo := catalog.NewRepository(database.Conn())

      // Ensure auth token exists
      authToken, err := ensureAuthToken(repo)
      if err != nil {
          return fmt.Errorf("failed to ensure auth token: %w", err)
      }
      fmt.Printf("\n=== Heimdex Agent ===\n")
      fmt.Printf("Auth Token: %s\n", authToken)
      fmt.Printf("API URL: http://127.0.0.1:%d\n", cfg.Port())
      fmt.Printf("=====================\n\n")

      // Initialize services
      catalogSvc := catalog.NewService(repo, logger)
      playbackSvc := playback.NewServer(logger)
      cloudClient := cloud.NewStubClient(logger)
      _ = cloudClient // For future use

      // Context for graceful shutdown
      ctx, cancel := context.WithCancel(context.Background())
      defer cancel()

      // Start job runner
      runner := catalog.NewRunner(catalogSvc, repo, logger)
      go runner.Start(ctx)

      // Start HTTP server
      apiServer := api.NewServer(api.ServerConfig{
          Port:           cfg.Port(),
          CatalogService: catalogSvc,
          PlaybackServer: playbackSvc,
          Repository:     repo,
          Logger:         logger,
      })
      go func() {
          if err := apiServer.Start(); err != nil {
              logger.Error("HTTP server error", "error", err)
          }
      }()

      // Setup signal handling
      sigCh := make(chan os.Signal, 1)
      signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

      // Handle quit from tray or signal
      quitCh := make(chan struct{})
      
      // Start system tray (blocks)
      tray := ui.NewTray(ui.TrayConfig{
          CatalogService: catalogSvc,
          Logger:         logger,
          OnAddFolder: func() error {
              // TODO: Open file dialog
              logger.Info("add folder requested from tray")
              return nil
          },
          OnQuit: func() {
              close(quitCh)
          },
      })

      go func() {
          select {
          case <-sigCh:
              logger.Info("received shutdown signal")
              close(quitCh)
          case <-quitCh:
              // Already closed by tray
          }
      }()

      // Run tray (blocks until quit)
      go tray.Run()

      // Wait for quit
      <-quitCh

      // Graceful shutdown
      logger.Info("initiating graceful shutdown")
      cancel() // Stop job runner

      shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
      defer shutdownCancel()

      if err := apiServer.Shutdown(shutdownCtx); err != nil {
          logger.Error("failed to shutdown HTTP server", "error", err)
      }

      logger.Info("shutdown complete")
      return nil
  }

  func ensureAuthToken(repo catalog.Repository) (string, error) {
      ctx := context.Background()
      
      existing, err := repo.GetConfig(ctx, "auth_token")
      if err == nil && existing != "" {
          return existing, nil
      }

      // Generate new token
      tokenBytes := make([]byte, 32)
      if _, err := rand.Read(tokenBytes); err != nil {
          return "", err
      }
      token := hex.EncodeToString(tokenBytes)

      if err := repo.SetConfig(ctx, "auth_token", token); err != nil {
          return "", err
      }

      return token, nil
  }
  ```

  **Recommended Agent Profile**:
  - **Category**: `unspecified-low`
    - Reason: Application wiring, signal handling, concurrency
  - **Skills**: `[]`
    - No special skills needed
  - **Skills Evaluated but Omitted**:
    - None

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 7 (standalone, depends on all)
  - **Blocks**: Tasks 18, 19, 20
  - **Blocked By**: Tasks 8, 10, 14, 16

  **References**:
  - Pattern: Dependency injection in Go main
  - Pattern: Graceful shutdown with context

  **Acceptance Criteria**:

  **Agent-Executed QA Scenarios:**

  ```
  Scenario: Agent starts successfully
    Tool: Bash
    Steps:
      1. go run ./cmd/agent &
      2. Wait 2 seconds
      3. curl http://127.0.0.1:8787/health
      4. Kill process
    Expected Result: Health endpoint responds
    Evidence: curl output

  Scenario: Auth token printed on startup
    Tool: Bash
    Steps:
      1. go run ./cmd/agent | head -10
      2. Grep for "Auth Token:"
    Expected Result: Token printed
    Evidence: Output containing token

  Scenario: Graceful shutdown on SIGINT
    Tool: Bash
    Steps:
      1. Start agent in background
      2. Send SIGINT
      3. Check logs for "shutdown complete"
    Expected Result: Clean shutdown
    Evidence: Log output

  Scenario: Auth token persists across restarts
    Tool: Bash
    Steps:
      1. Start agent, capture token
      2. Stop agent
      3. Start agent again, capture token
      4. Compare tokens
    Expected Result: Same token both times
    Evidence: Token comparison
  ```

  **Commit**: YES
  - Message: `feat: add main entry point with DI wiring`
  - Files: `cmd/agent/main.go`

---

### Wave 8: Quality

---

- [ ] 18. Unit Tests (Range, Migrations)

  **What to do**:
  - Enhance `internal/playback/range_test.go` with comprehensive edge cases
  - Create `internal/db/migrations_test.go` for migration testing
  - Test migration applies cleanly to fresh database
  - Test migration is idempotent (can apply twice)

  **Must NOT do**:
  - Do NOT skip edge cases
  - Do NOT leave tests incomplete

  **Files to create/modify**:
  ```
  internal/playback/range_test.go (enhance)
  internal/db/migrations_test.go
  ```

  **Key implementation details**:

  Enhanced `range_test.go`:
  ```go
  func TestParseRange_EdgeCases(t *testing.T) {
      tests := []struct {
          name    string
          header  string
          size    int64
          want    *Range
          wantErr error
      }{
          // Standard cases
          {"full range", "bytes=0-999", 1000, &Range{0, 999}, nil},
          {"partial start", "bytes=500-", 1000, &Range{500, 999}, nil},
          {"suffix range", "bytes=-500", 1000, &Range{500, 999}, nil},
          
          // Edge cases
          {"empty header", "", 1000, nil, nil},
          {"single byte", "bytes=0-0", 1000, &Range{0, 0}, nil},
          {"beyond size clamped", "bytes=0-2000", 1000, &Range{0, 999}, nil},
          {"suffix larger than file", "bytes=-2000", 500, &Range{0, 499}, nil},
          {"last byte", "bytes=999-", 1000, &Range{999, 999}, nil},
          
          // Error cases
          {"unsatisfiable start", "bytes=1000-", 1000, nil, ErrUnsatisfiable},
          {"start after end", "bytes=500-400", 1000, nil, ErrInvalidRange},
          {"invalid format", "invalid", 1000, nil, ErrInvalidRange},
          {"wrong unit", "chars=0-100", 1000, nil, ErrInvalidRange},
          {"negative suffix", "bytes=--100", 1000, nil, ErrInvalidRange},
          
          // Multi-range (take first)
          {"multi range takes first", "bytes=0-99, 200-299", 1000, &Range{0, 99}, nil},
      }

      for _, tt := range tests {
          t.Run(tt.name, func(t *testing.T) {
              got, err := ParseRange(tt.header, tt.size)
              
              if tt.wantErr != nil {
                  if err != tt.wantErr {
                      t.Errorf("ParseRange() error = %v, wantErr %v", err, tt.wantErr)
                  }
                  return
              }
              
              if err != nil {
                  t.Errorf("ParseRange() unexpected error: %v", err)
                  return
              }
              
              if tt.want == nil {
                  if got != nil {
                      t.Errorf("ParseRange() = %v, want nil", got)
                  }
                  return
              }
              
              if got.Start != tt.want.Start || got.End != tt.want.End {
                  t.Errorf("ParseRange() = %v, want %v", got, tt.want)
              }
          })
      }
  }

  func TestRange_ContentLength(t *testing.T) {
      r := &Range{Start: 0, End: 99}
      if r.ContentLength() != 100 {
          t.Errorf("ContentLength() = %d, want 100", r.ContentLength())
      }
  }

  func TestRange_ContentRange(t *testing.T) {
      r := &Range{Start: 0, End: 99}
      if r.ContentRange(1000) != "bytes 0-99/1000" {
          t.Errorf("ContentRange() = %s, want 'bytes 0-99/1000'", r.ContentRange(1000))
      }
  }
  ```

  `migrations_test.go`:
  ```go
  package db

  import (
      "os"
      "path/filepath"
      "testing"
  )

  func TestMigrations_ApplyCleanly(t *testing.T) {
      // Create temp database
      tmpDir := t.TempDir()
      dbPath := filepath.Join(tmpDir, "test.db")
      
      // First application
      db, err := New(dbPath, nil)
      if err != nil {
          t.Fatalf("failed to create database: %v", err)
      }
      
      // Verify tables exist
      tables := []string{"sources", "files", "jobs", "config"}
      for _, table := range tables {
          var name string
          err := db.conn.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
          if err != nil {
              t.Errorf("table %s not found: %v", table, err)
          }
      }
      
      db.Close()
  }

  func TestMigrations_Idempotent(t *testing.T) {
      tmpDir := t.TempDir()
      dbPath := filepath.Join(tmpDir, "test.db")
      
      // Apply migrations twice
      db1, err := New(dbPath, nil)
      if err != nil {
          t.Fatalf("first migration failed: %v", err)
      }
      db1.Close()
      
      db2, err := New(dbPath, nil)
      if err != nil {
          t.Fatalf("second migration failed: %v", err)
      }
      db2.Close()
  }
  ```

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Unit tests with clear requirements
  - **Skills**: `[]`
    - No special skills needed
  - **Skills Evaluated but Omitted**:
    - None

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 8 (with Tasks 19, 20)
  - **Blocks**: Task 21
  - **Blocked By**: Tasks 7, 17

  **References**:
  - Docs: testing package
  - Pattern: Table-driven tests

  **Acceptance Criteria**:

  **Agent-Executed QA Scenarios:**

  ```
  Scenario: All range tests pass
    Tool: Bash
    Steps:
      1. go test -v ./internal/playback/...
    Expected Result: All tests PASS
    Evidence: Test output

  Scenario: All migration tests pass
    Tool: Bash
    Steps:
      1. go test -v ./internal/db/...
    Expected Result: All tests PASS
    Evidence: Test output

  Scenario: Tests run with race detector
    Tool: Bash
    Steps:
      1. go test -race ./internal/playback/... ./internal/db/...
    Expected Result: No race conditions
    Evidence: Test output
  ```

  **Commit**: YES
  - Message: `test: add unit tests for Range parsing and migrations`
  - Files: `internal/playback/range_test.go`, `internal/db/migrations_test.go`

---

- [ ] 19. Integration Tests (Scan)

  **What to do**:
  - Create `internal/catalog/service_test.go` with integration tests
  - Test folder scanning with temp directory
  - Create temp video files (can be empty with correct extensions)
  - Verify files are discovered and fingerprinted
  - Verify hidden folders are skipped

  **Must NOT do**:
  - Do NOT rely on external video files
  - Do NOT leave temp files after tests

  **Files to create**:
  ```
  internal/catalog/service_test.go
  ```

  **Key implementation details**:

  ```go
  // internal/catalog/service_test.go
  package catalog

  import (
      "context"
      "os"
      "path/filepath"
      "testing"
  )

  func TestService_ScanFolder(t *testing.T) {
      // Setup temp directory with test files
      tmpDir := t.TempDir()
      
      // Create test video files
      createFile(t, filepath.Join(tmpDir, "video1.mp4"), []byte("fake mp4 content"))
      createFile(t, filepath.Join(tmpDir, "video2.mov"), []byte("fake mov content"))
      createFile(t, filepath.Join(tmpDir, "document.txt"), []byte("not a video"))
      
      // Create hidden folder (should be skipped)
      hiddenDir := filepath.Join(tmpDir, ".hidden")
      os.Mkdir(hiddenDir, 0755)
      createFile(t, filepath.Join(hiddenDir, "hidden.mp4"), []byte("hidden video"))
      
      // Create subdirectory with video
      subDir := filepath.Join(tmpDir, "subdir")
      os.Mkdir(subDir, 0755)
      createFile(t, filepath.Join(subDir, "nested.mkv"), []byte("nested video"))
      
      // Setup in-memory database
      db := setupTestDB(t)
      repo := NewRepository(db.Conn())
      svc := NewService(repo, nil)
      
      // Add source
      ctx := context.Background()
      source, err := svc.AddFolder(ctx, tmpDir, "Test Folder")
      if err != nil {
          t.Fatalf("AddFolder failed: %v", err)
      }
      
      // Create and run scan job
      job, err := svc.ScanSource(ctx, source.ID)
      if err != nil {
          t.Fatalf("ScanSource failed: %v", err)
      }
      
      // Execute scan directly (normally done by runner)
      err = svc.ScanFolder(ctx, job.ID, source.ID, source.Path)
      if err != nil {
          t.Fatalf("ScanFolder failed: %v", err)
      }
      
      // Verify results
      files, err := repo.GetFilesBySource(ctx, source.ID)
      if err != nil {
          t.Fatalf("GetFilesBySource failed: %v", err)
      }
      
      // Should find: video1.mp4, video2.mov, nested.mkv (3 files)
      // Should NOT find: document.txt, hidden.mp4
      if len(files) != 3 {
          t.Errorf("expected 3 files, got %d", len(files))
          for _, f := range files {
              t.Logf("found: %s", f.Path)
          }
      }
      
      // Verify fingerprints are computed
      for _, f := range files {
          if f.Fingerprint == "" {
              t.Errorf("file %s has no fingerprint", f.Filename)
          }
      }
  }

  func TestService_AddFolder_Validation(t *testing.T) {
      db := setupTestDB(t)
      repo := NewRepository(db.Conn())
      svc := NewService(repo, nil)
      ctx := context.Background()
      
      // Test non-existent path
      _, err := svc.AddFolder(ctx, "/nonexistent/path", "Test")
      if err == nil {
          t.Error("expected error for non-existent path")
      }
      
      // Test file path (not directory)
      tmpFile := filepath.Join(t.TempDir(), "file.txt")
      createFile(t, tmpFile, []byte("test"))
      
      _, err = svc.AddFolder(ctx, tmpFile, "Test")
      if err == nil {
          t.Error("expected error for file path")
      }
  }

  func createFile(t *testing.T, path string, content []byte) {
      t.Helper()
      if err := os.WriteFile(path, content, 0644); err != nil {
          t.Fatalf("failed to create test file: %v", err)
      }
  }

  func setupTestDB(t *testing.T) *db.DB {
      t.Helper()
      // Create in-memory or temp file database for testing
      tmpDir := t.TempDir()
      database, err := db.New(filepath.Join(tmpDir, "test.db"), nil)
      if err != nil {
          t.Fatalf("failed to setup test db: %v", err)
      }
      t.Cleanup(func() { database.Close() })
      return database
  }
  ```

  **Recommended Agent Profile**:
  - **Category**: `unspecified-low`
    - Reason: Integration tests with file system, moderate complexity
  - **Skills**: `[]`
    - No special skills needed
  - **Skills Evaluated but Omitted**:
    - None

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 8 (with Tasks 18, 20)
  - **Blocks**: Task 21
  - **Blocked By**: Tasks 8, 17

  **References**:
  - Pattern: Testing with temp directories
  - Docs: testing.T.TempDir()

  **Acceptance Criteria**:

  **Agent-Executed QA Scenarios:**

  ```
  Scenario: Scan integration tests pass
    Tool: Bash
    Steps:
      1. go test -v ./internal/catalog/...
    Expected Result: All tests PASS
    Evidence: Test output

  Scenario: Tests clean up temp files
    Tool: Bash
    Steps:
      1. Run tests
      2. Check /tmp for leftover test files
    Expected Result: No orphaned test files
    Evidence: ls output
  ```

  **Commit**: YES
  - Message: `test: add integration tests for folder scanning`
  - Files: `internal/catalog/service_test.go`

---

- [ ] 20. Documentation

  **What to do**:
  - Create `docs/architecture.md` with system overview
  - Create `docs/threat_model.md` with security considerations
  - Create `docs/api.md` with API reference
  - Document all endpoints with examples

  **Must NOT do**:
  - Do NOT include sensitive information
  - Do NOT leave TODO placeholders

  **Files to create**:
  ```
  docs/architecture.md
  docs/threat_model.md
  docs/api.md
  ```

  **Key implementation details**:

  `docs/architecture.md`:
  ```markdown
  # Heimdex Agent Architecture

  ## Overview
  
  Heimdex Agent is a desktop application that catalogs local video files
  and provides a localhost API for accessing file metadata and playback.

  ## Component Diagram
  
  ```
  ┌─────────────────────────────────────────────────────────────┐
  │                     Heimdex Agent                           │
  ├─────────────────────────────────────────────────────────────┤
  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
  │  │  Tray UI │  │ API      │  │ Playback │  │ Job      │   │
  │  │          │  │ Server   │  │ Server   │  │ Runner   │   │
  │  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘   │
  │       │             │             │             │          │
  │  ┌────┴─────────────┴─────────────┴─────────────┴────┐    │
  │  │              Catalog Service                       │    │
  │  └────────────────────────┬──────────────────────────┘    │
  │                           │                                │
  │  ┌────────────────────────┴──────────────────────────┐    │
  │  │              Repository (SQLite)                   │    │
  │  └────────────────────────────────────────────────────┘    │
  └─────────────────────────────────────────────────────────────┘
  ```

  ## Key Design Decisions

  1. **Localhost Only**: API binds to 127.0.0.1 for security
  2. **Pure Go SQLite**: No CGO for easy cross-compilation
  3. **Clean Architecture**: API → Service → Repository → DB
  4. **Dependency Injection**: All components created via constructors
  ```

  `docs/api.md`:
  ```markdown
  # Heimdex Agent API Reference

  Base URL: `http://127.0.0.1:8787`

  ## Authentication

  All endpoints except `/health` require Bearer token authentication:
  
  ```
  Authorization: Bearer <token>
  ```
  
  The token is printed to stdout on first run and stored in the database.

  ## Endpoints

  ### GET /health
  
  Health check (no auth required).
  
  **Response:**
  ```json
  {
    "status": "ok",
    "version": "0.1.0"
  }
  ```

  ### GET /status
  
  Get agent status.
  
  **Response:**
  ```json
  {
    "status": "idle",
    "sources": 2,
    "files": 150,
    "active_job": null
  }
  ```

  ### POST /sources/folders
  
  Add a folder to index.
  
  **Request:**
  ```json
  {
    "path": "/Users/me/Videos",
    "display_name": "My Videos"
  }
  ```
  
  **Response:** (201 Created)
  ```json
  {
    "id": "abc123",
    "type": "folder",
    "path": "/Users/me/Videos",
    "display_name": "My Videos",
    "present": true,
    "created_at": "2024-01-15T10:00:00Z"
  }
  ```

  ... (continue for all endpoints)
  ```

  **Recommended Agent Profile**:
  - **Category**: `writing`
    - Reason: Documentation requires clear technical writing
  - **Skills**: `[]`
    - No special skills needed
  - **Skills Evaluated but Omitted**:
    - None

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 8 (with Tasks 18, 19)
  - **Blocks**: Task 21
  - **Blocked By**: Task 17

  **References**:
  - Pattern: API documentation style
  - Pattern: Threat modeling

  **Acceptance Criteria**:

  **Agent-Executed QA Scenarios:**

  ```
  Scenario: Documentation files exist
    Tool: Bash
    Steps:
      1. ls docs/
      2. Verify architecture.md, threat_model.md, api.md exist
    Expected Result: All 3 files present
    Evidence: ls output

  Scenario: Markdown is valid
    Tool: Bash
    Steps:
      1. If markdownlint available: markdownlint docs/*.md
    Expected Result: No lint errors
    Evidence: Lint output
  ```

  **Commit**: YES
  - Message: `docs: add architecture, threat model, and API documentation`
  - Files: `docs/architecture.md`, `docs/threat_model.md`, `docs/api.md`

---

- [ ] 21. Packaging Templates

  **What to do**:
  - Create `packaging/macos/installer/com.heimdex.agent.plist` for LaunchAgent
  - Create placeholder for Windows installer
  - Document installation process

  **Must NOT do**:
  - Do NOT create full installer scripts (v1 feature)

  **Files to create**:
  ```
  packaging/macos/installer/com.heimdex.agent.plist
  packaging/windows/installer/README.md
  packaging/README.md
  ```

  **Key implementation details**:

  `packaging/macos/installer/com.heimdex.agent.plist`:
  ```xml
  <?xml version="1.0" encoding="UTF-8"?>
  <!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
  <plist version="1.0">
  <dict>
      <key>Label</key>
      <string>com.heimdex.agent</string>
      
      <key>ProgramArguments</key>
      <array>
          <string>/usr/local/bin/heimdex-agent</string>
      </array>
      
      <key>RunAtLoad</key>
      <true/>
      
      <key>KeepAlive</key>
      <dict>
          <key>SuccessfulExit</key>
          <false/>
      </dict>
      
      <key>StandardOutPath</key>
      <string>/tmp/heimdex-agent.log</string>
      
      <key>StandardErrorPath</key>
      <string>/tmp/heimdex-agent.err</string>
      
      <key>EnvironmentVariables</key>
      <dict>
          <key>HEIMDEX_LOG_LEVEL</key>
          <string>info</string>
      </dict>
  </dict>
  </plist>
  ```

  `packaging/README.md`:
  ```markdown
  # Heimdex Agent Packaging

  ## macOS

  ### Manual Installation

  1. Build the agent:
     ```bash
     make build
     ```

  2. Copy binary to /usr/local/bin:
     ```bash
     sudo cp bin/heimdex-agent /usr/local/bin/
     ```

  3. Install LaunchAgent:
     ```bash
     cp packaging/macos/installer/com.heimdex.agent.plist ~/Library/LaunchAgents/
     launchctl load ~/Library/LaunchAgents/com.heimdex.agent.plist
     ```

  ### Uninstallation

  ```bash
  launchctl unload ~/Library/LaunchAgents/com.heimdex.agent.plist
  rm ~/Library/LaunchAgents/com.heimdex.agent.plist
  sudo rm /usr/local/bin/heimdex-agent
  rm -rf ~/.heimdex
  ```

  ## Windows

  Windows installer will be added in v1. For now, run manually:

  ```powershell
  .\heimdex-agent.exe
  ```

  To run at startup, add a shortcut to the Startup folder.
  ```

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Template files with no logic
  - **Skills**: `[]`
    - No special skills needed
  - **Skills Evaluated but Omitted**:
    - None

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 8 (with Tasks 18, 19, 20)
  - **Blocks**: None (final task)
  - **Blocked By**: Task 17

  **References**:
  - Docs: Apple LaunchAgent format

  **Acceptance Criteria**:

  **Agent-Executed QA Scenarios:**

  ```
  Scenario: Plist is valid XML
    Tool: Bash
    Steps:
      1. plutil -lint packaging/macos/installer/com.heimdex.agent.plist
    Expected Result: "OK" output
    Evidence: plutil output

  Scenario: Packaging files exist
    Tool: Bash
    Steps:
      1. ls -la packaging/
      2. ls -la packaging/macos/installer/
    Expected Result: All expected files present
    Evidence: ls output
  ```

  **Commit**: YES
  - Message: `chore: add packaging templates for macOS LaunchAgent`
  - Files: `packaging/macos/installer/com.heimdex.agent.plist`, `packaging/README.md`, `packaging/windows/installer/README.md`

---

## Commit Strategy

| After Task | Message | Key Files | Verification |
|------------|---------|-----------|--------------|
| 1 | `feat(init): initialize project structure` | go.mod, Makefile | go mod verify |
| 2,3 | `feat(core): add config and logging` | config.go, logging.go | go build |
| 4,5 | `feat(db): add database and models` | db.go, models.go | go build |
| 6 | `feat(catalog): add repository` | repository.go | go build |
| 7 | `feat(playback): add Range parser` | range.go, range_test.go | go test |
| 8,9,10 | `feat(core): add services and runner` | service.go, server.go, runner.go | go build |
| 11,12 | `feat(api): add schemas and middleware` | schemas.go, middleware.go | go build |
| 13 | `feat(cloud): add stubs` | client.go, auth.go, upload.go | go build |
| 14,15 | `feat(api): add routes and stubs` | routes.go, watcher.go | go build |
| 16 | `feat(ui): add system tray` | tray.go | go build |
| 17 | `feat: wire up main entry point` | main.go | make dev |
| 18,19 | `test: add unit and integration tests` | *_test.go | make test |
| 20 | `docs: add documentation` | docs/*.md | ls docs/ |
| 21 | `chore: add packaging` | packaging/ | plutil -lint |

---

## Success Criteria

### Verification Commands

```bash
# Build succeeds
make build
# Expected: Binary created in bin/

# Tests pass
make test
# Expected: All tests pass

# Lint passes
make lint
# Expected: No errors

# Agent starts
./bin/heimdex-agent &
sleep 2
curl http://127.0.0.1:8787/health
# Expected: {"status":"ok","version":"0.1.0"}

# Auth works
TOKEN=$(grep "Auth Token:" /tmp/heimdex-agent.log | awk '{print $3}')
curl -H "Authorization: Bearer $TOKEN" http://127.0.0.1:8787/status
# Expected: {"status":"idle",...}

# Kill agent
pkill heimdex-agent
```

### Final Checklist

- [ ] `make build` produces binaries
- [ ] `make test` passes all tests
- [ ] `make lint` has no errors
- [ ] Agent shows tray icon on startup
- [ ] Auth token printed on first run
- [ ] GET /health works without auth
- [ ] All other endpoints require auth
- [ ] Can add folder via API
- [ ] Can trigger scan via API
- [ ] Playback returns video with Range support
- [ ] Graceful shutdown works
- [ ] Documentation is complete
