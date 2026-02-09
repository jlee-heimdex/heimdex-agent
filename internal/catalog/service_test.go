package catalog

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/heimdex/heimdex-agent/internal/db"
)

func setupTestDB(t *testing.T) (*db.DB, Repository) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := db.New(dbPath, nil)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	repo := NewRepository(database.Conn())
	return database, repo
}

func TestService_AddFolder(t *testing.T) {
	database, repo := setupTestDB(t)
	defer database.Close()

	svc := NewService(repo, nil)

	tmpDir := t.TempDir()

	source, err := svc.AddFolder(context.Background(), tmpDir, "Test Folder")
	if err != nil {
		t.Fatalf("AddFolder() error = %v", err)
	}

	if source.ID == "" {
		t.Error("source.ID is empty")
	}
	if source.Path != tmpDir {
		t.Errorf("source.Path = %s, want %s", source.Path, tmpDir)
	}
	if source.DisplayName != "Test Folder" {
		t.Errorf("source.DisplayName = %s, want Test Folder", source.DisplayName)
	}
}

func TestService_AddFolder_InvalidPath(t *testing.T) {
	database, repo := setupTestDB(t)
	defer database.Close()

	svc := NewService(repo, nil)

	_, err := svc.AddFolder(context.Background(), "/nonexistent/path", "Test")
	if err == nil {
		t.Error("AddFolder() should return error for nonexistent path")
	}
}

func TestService_AddFolder_NotDirectory(t *testing.T) {
	database, repo := setupTestDB(t)
	defer database.Close()

	svc := NewService(repo, nil)

	tmpFile, err := os.CreateTemp("", "test")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	_, err = svc.AddFolder(context.Background(), tmpFile.Name(), "Test")
	if err == nil {
		t.Error("AddFolder() should return error for file path")
	}
}

func TestService_ExecuteScan(t *testing.T) {
	database, repo := setupTestDB(t)
	defer database.Close()

	svc := NewService(repo, nil)
	ctx := context.Background()

	tmpDir := t.TempDir()
	testVideo := filepath.Join(tmpDir, "test.mp4")
	if err := os.WriteFile(testVideo, []byte("fake video content for testing"), 0644); err != nil {
		t.Fatalf("failed to create test video: %v", err)
	}

	source, err := svc.AddFolder(ctx, tmpDir, "Test")
	if err != nil {
		t.Fatalf("AddFolder() error = %v", err)
	}

	job, err := svc.ScanSource(ctx, source.ID)
	if err != nil {
		t.Fatalf("ScanSource() error = %v", err)
	}

	err = svc.ExecuteScan(ctx, job.ID, source.ID, source.Path)
	if err != nil {
		t.Fatalf("ExecuteScan() error = %v", err)
	}

	files, err := svc.GetFiles(ctx, source.ID)
	if err != nil {
		t.Fatalf("GetFiles() error = %v", err)
	}

	if len(files) != 1 {
		t.Errorf("found %d files, want 1", len(files))
	}

	if len(files) > 0 && files[0].Filename != "test.mp4" {
		t.Errorf("file.Filename = %s, want test.mp4", files[0].Filename)
	}
}

func TestService_ExecuteScan_SkipsHiddenDirs(t *testing.T) {
	database, repo := setupTestDB(t)
	defer database.Close()

	svc := NewService(repo, nil)
	ctx := context.Background()

	tmpDir := t.TempDir()

	visibleVideo := filepath.Join(tmpDir, "visible.mp4")
	os.WriteFile(visibleVideo, []byte("visible"), 0644)

	hiddenDir := filepath.Join(tmpDir, ".hidden")
	os.Mkdir(hiddenDir, 0755)
	hiddenVideo := filepath.Join(hiddenDir, "hidden.mp4")
	os.WriteFile(hiddenVideo, []byte("hidden"), 0644)

	source, _ := svc.AddFolder(ctx, tmpDir, "Test")
	job, _ := svc.ScanSource(ctx, source.ID)
	svc.ExecuteScan(ctx, job.ID, source.ID, source.Path)

	files, _ := svc.GetFiles(ctx, source.ID)

	if len(files) != 1 {
		t.Errorf("found %d files, want 1 (should skip hidden)", len(files))
	}
}

func TestIsVideoFile(t *testing.T) {
	tests := []struct {
		filename string
		want     bool
	}{
		{"video.mp4", true},
		{"video.MP4", true},
		{"video.mov", true},
		{"video.mkv", true},
		{"video.avi", false},
		{"document.pdf", false},
		{"image.jpg", false},
		{"noextension", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			if got := IsVideoFile(tt.filename); got != tt.want {
				t.Errorf("IsVideoFile(%s) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}
