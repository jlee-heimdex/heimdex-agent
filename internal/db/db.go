package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type DB struct {
	conn   *sql.DB
	logger *slog.Logger
}

func New(dbPath string, logger *slog.Logger) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
	}
	for _, pragma := range pragmas {
		if _, err := conn.Exec(pragma); err != nil {
			return nil, fmt.Errorf("failed to execute %s: %w", pragma, err)
		}
	}

	db := &DB{conn: conn, logger: logger}

	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	if err := db.markInterruptedJobs(); err != nil && logger != nil {
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

func (d *DB) migrate() error {
	migrations, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("failed to read migrations: %w", err)
	}

	for _, m := range migrations {
		if m.IsDir() {
			continue
		}

		name := m.Name()

		if d.isMigrationApplied(name) {
			continue
		}

		content, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", name, err)
		}

		if _, err := d.conn.Exec(string(content)); err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", name, err)
		}

		if _, err := d.conn.Exec("INSERT INTO _migrations (name) VALUES (?)", name); err != nil {
			return fmt.Errorf("failed to record migration %s: %w", name, err)
		}

		if d.logger != nil {
			d.logger.Info("applied migration", "name", name)
		}
	}

	return nil
}

func (d *DB) isMigrationApplied(name string) bool {
	var exists int
	err := d.conn.QueryRow("SELECT 1 FROM sqlite_master WHERE type='table' AND name='_migrations'").Scan(&exists)
	if err != nil {
		return false
	}

	var applied int
	err = d.conn.QueryRow("SELECT 1 FROM _migrations WHERE name = ?", name).Scan(&applied)
	return err == nil && applied == 1
}

func (d *DB) markInterruptedJobs() error {
	_, err := d.conn.ExecContext(context.Background(),
		`UPDATE jobs SET status = 'failed', error = 'interrupted by restart', updated_at = datetime('now') WHERE status = 'running'`)
	return err
}
