package db

import (
	"path/filepath"
	"testing"
)

func TestNew_CreatesDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := New(dbPath, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer database.Close()

	tables := []string{"sources", "files", "jobs", "config", "_migrations"}
	for _, table := range tables {
		var name string
		err := database.Conn().QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %s not found: %v", table, err)
		}
	}
}

func TestNew_WALEnabled(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := New(dbPath, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer database.Close()

	var journalMode string
	err = database.Conn().QueryRow("PRAGMA journal_mode").Scan(&journalMode)
	if err != nil {
		t.Fatalf("PRAGMA journal_mode error = %v", err)
	}

	if journalMode != "wal" {
		t.Errorf("journal_mode = %s, want wal", journalMode)
	}
}

func TestNew_MigrationsIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db1, err := New(dbPath, nil)
	if err != nil {
		t.Fatalf("first New() error = %v", err)
	}
	db1.Close()

	db2, err := New(dbPath, nil)
	if err != nil {
		t.Fatalf("second New() error = %v", err)
	}
	defer db2.Close()

	var count int
	err = db2.Conn().QueryRow("SELECT COUNT(*) FROM _migrations").Scan(&count)
	if err != nil {
		t.Fatalf("count migrations error = %v", err)
	}

	if count != 3 {
		t.Errorf("migration count = %d, want 3", count)
	}
}

func TestMarkInterruptedJobs(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db1, err := New(dbPath, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = db1.Conn().Exec(`
		INSERT INTO jobs (id, type, status, progress, created_at, updated_at)
		VALUES ('test-job', 'scan', 'running', 50, datetime('now'), datetime('now'))
	`)
	if err != nil {
		t.Fatalf("insert job error = %v", err)
	}
	db1.Close()

	db2, err := New(dbPath, nil)
	if err != nil {
		t.Fatalf("second New() error = %v", err)
	}
	defer db2.Close()

	var status, errMsg string
	err = db2.Conn().QueryRow("SELECT status, error FROM jobs WHERE id = 'test-job'").Scan(&status, &errMsg)
	if err != nil {
		t.Fatalf("query job error = %v", err)
	}

	if status != "failed" {
		t.Errorf("job status = %s, want failed", status)
	}
	if errMsg != "interrupted by restart" {
		t.Errorf("job error = %s, want 'interrupted by restart'", errMsg)
	}
}
