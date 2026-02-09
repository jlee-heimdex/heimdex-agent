// Package config provides configuration management for the Heimdex Agent.
// Configuration is loaded from environment variables with sensible defaults.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const (
	// Default values
	DefaultPort     = 8787
	DefaultLogLevel = "info"
	DefaultDataDir  = ".heimdex"

	// Environment variable names
	EnvPort     = "HEIMDEX_PORT"
	EnvLogLevel = "HEIMDEX_LOG_LEVEL"
	EnvDataDir  = "HEIMDEX_DATA_DIR"

	// Pipeline environment variable names
	EnvPipelinesPython = "HEIMDEX_PIPELINES_PYTHON"
	EnvPipelinesModule = "HEIMDEX_PIPELINES_MODULE"

	// Database filename
	DBFilename = "heimdex.db"

	// Cache settings
	DefaultCacheMaxBytes = 10 * 1024 * 1024 * 1024 // 10GB

	// Pipeline defaults
	DefaultPipelinesModule        = "heimdex_media_pipelines"
	DefaultPipelinesTimeoutDoctor = 30   // seconds
	DefaultPipelinesTimeoutSpeech = 1800 // 30 minutes
	DefaultPipelinesTimeoutFaces  = 900  // 15 minutes
	DefaultPipelinesTimeoutScenes = 600  // 10 minutes
)

// Config defines the application configuration interface
type Config interface {
	Port() int
	LogLevel() string
	DataDir() string
	DBPath() string
	CacheDir() string
	CacheMaxBytes() int64
	PipelinesPython() string
	PipelinesModule() string
	PipelinesTimeoutDoctor() time.Duration
	PipelinesTimeoutSpeech() time.Duration
	PipelinesTimeoutFaces() time.Duration
	PipelinesTimeoutScenes() time.Duration
}

// EnvConfig reads configuration from environment variables
type EnvConfig struct {
	port          int
	logLevel      string
	dataDir       string
	cacheMaxBytes int64

	pipelinesPython string
	pipelinesModule string
}

// New creates a new EnvConfig with defaults and environment variable overrides
func New() (*EnvConfig, error) {
	cfg := &EnvConfig{
		port:          DefaultPort,
		logLevel:      DefaultLogLevel,
		dataDir:       defaultDataDir(),
		cacheMaxBytes: DefaultCacheMaxBytes,
	}

	// Override port from environment
	if p := os.Getenv(EnvPort); p != "" {
		port, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("invalid %s: %w", EnvPort, err)
		}
		if port < 1 || port > 65535 {
			return nil, fmt.Errorf("invalid %s: port must be between 1 and 65535", EnvPort)
		}
		cfg.port = port
	}

	// Override log level from environment
	if ll := os.Getenv(EnvLogLevel); ll != "" {
		cfg.logLevel = ll
	}

	// Override data directory from environment
	if dd := os.Getenv(EnvDataDir); dd != "" {
		cfg.dataDir = dd
	}

	cfg.pipelinesPython = os.Getenv(EnvPipelinesPython)

	if pm := os.Getenv(EnvPipelinesModule); pm != "" {
		cfg.pipelinesModule = pm
	}

	return cfg, nil
}

// Port returns the HTTP server port
func (c *EnvConfig) Port() int {
	return c.port
}

// LogLevel returns the log level (debug, info, warn, error)
func (c *EnvConfig) LogLevel() string {
	return c.logLevel
}

// DataDir returns the data directory path
func (c *EnvConfig) DataDir() string {
	return c.dataDir
}

// DBPath returns the full path to the SQLite database file
func (c *EnvConfig) DBPath() string {
	return filepath.Join(c.dataDir, DBFilename)
}

// CacheDir returns the cache directory path
func (c *EnvConfig) CacheDir() string {
	return filepath.Join(c.dataDir, "cache")
}

// CacheMaxBytes returns the maximum cache size in bytes
func (c *EnvConfig) CacheMaxBytes() int64 {
	return c.cacheMaxBytes
}

func (c *EnvConfig) PipelinesPython() string {
	return c.pipelinesPython
}

func (c *EnvConfig) PipelinesModule() string {
	if c.pipelinesModule != "" {
		return c.pipelinesModule
	}
	return DefaultPipelinesModule
}

func (c *EnvConfig) PipelinesTimeoutDoctor() time.Duration {
	return time.Duration(DefaultPipelinesTimeoutDoctor) * time.Second
}

func (c *EnvConfig) PipelinesTimeoutSpeech() time.Duration {
	return time.Duration(DefaultPipelinesTimeoutSpeech) * time.Second
}

func (c *EnvConfig) PipelinesTimeoutFaces() time.Duration {
	return time.Duration(DefaultPipelinesTimeoutFaces) * time.Second
}

func (c *EnvConfig) PipelinesTimeoutScenes() time.Duration {
	return time.Duration(DefaultPipelinesTimeoutScenes) * time.Second
}

// defaultDataDir returns the default data directory path
func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory if home is not available
		return DefaultDataDir
	}
	return filepath.Join(home, DefaultDataDir)
}

// Version information (set at build time via ldflags)
var (
	Version   = "0.1.0"
	BuildTime = "unknown"
	GitCommit = "unknown"
)
