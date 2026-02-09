package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const fingerprintSize = 64 * 1024

type CatalogService interface {
	AddFolder(ctx context.Context, path, displayName string) (*Source, error)
	RemoveSource(ctx context.Context, id string) error
	GetSources(ctx context.Context) ([]*Source, error)
	GetSource(ctx context.Context, id string) (*Source, error)
	GetFiles(ctx context.Context, sourceID string) ([]*File, error)
	GetFile(ctx context.Context, id string) (*File, error)
	CountFiles(ctx context.Context) (int, error)
	ScanSource(ctx context.Context, sourceID string) (*Job, error)
	ExecuteScan(ctx context.Context, jobID, sourceID, path string) error
}

type Service struct {
	repo   Repository
	logger *slog.Logger
}

func NewService(repo Repository, logger *slog.Logger) *Service {
	return &Service{repo: repo, logger: logger}
}

func (s *Service) AddFolder(ctx context.Context, path, displayName string) (*Source, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("path does not exist: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory")
	}

	existing, err := s.repo.GetSourceByPath(ctx, absPath)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	if displayName == "" {
		displayName = filepath.Base(absPath)
	}

	source := &Source{
		ID:          NewID(),
		Type:        "folder",
		Path:        absPath,
		DisplayName: displayName,
		Present:     true,
		CreatedAt:   time.Now(),
	}

	if err := s.repo.CreateSource(ctx, source); err != nil {
		return nil, err
	}

	if s.logger != nil {
		s.logger.Info("folder added", "source_id", source.ID, "path", absPath)
	}
	return source, nil
}

func (s *Service) RemoveSource(ctx context.Context, id string) error {
	if err := s.repo.DeleteFilesBySource(ctx, id); err != nil {
		return err
	}
	return s.repo.DeleteSource(ctx, id)
}

func (s *Service) GetSources(ctx context.Context) ([]*Source, error) {
	return s.repo.ListSources(ctx)
}

func (s *Service) GetSource(ctx context.Context, id string) (*Source, error) {
	return s.repo.GetSource(ctx, id)
}

func (s *Service) GetFiles(ctx context.Context, sourceID string) ([]*File, error) {
	return s.repo.GetFilesBySource(ctx, sourceID)
}

func (s *Service) GetFile(ctx context.Context, id string) (*File, error) {
	return s.repo.GetFile(ctx, id)
}

func (s *Service) CountFiles(ctx context.Context) (int, error) {
	return s.repo.CountFiles(ctx)
}

func (s *Service) ScanSource(ctx context.Context, sourceID string) (*Job, error) {
	source, err := s.repo.GetSource(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	if source == nil {
		return nil, fmt.Errorf("source not found")
	}

	now := time.Now()
	job := &Job{
		ID:        NewID(),
		Type:      JobTypeScan,
		Status:    JobStatusPending,
		SourceID:  sourceID,
		Progress:  0,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repo.CreateJob(ctx, job); err != nil {
		return nil, err
	}

	if s.logger != nil {
		s.logger.Info("scan job created", "job_id", job.ID, "source_id", sourceID)
	}
	return job, nil
}

func (s *Service) ExecuteScan(ctx context.Context, jobID, sourceID, path string) error {
	s.repo.UpdateJobStatus(ctx, jobID, JobStatusRunning, "")
	if s.logger != nil {
		s.logger.Info("starting scan", "job_id", jobID, "path", path)
	}

	var files []string
	err := filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") {
			return filepath.SkipDir
		}
		if !d.IsDir() && IsVideoFile(d.Name()) {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		s.repo.UpdateJobStatus(ctx, jobID, JobStatusFailed, err.Error())
		return err
	}

	total := len(files)
	if s.logger != nil {
		s.logger.Info("found video files", "count", total)
	}

	for i, filePath := range files {
		select {
		case <-ctx.Done():
			s.repo.UpdateJobStatus(ctx, jobID, JobStatusFailed, "cancelled")
			return ctx.Err()
		default:
		}

		if err := s.processFile(ctx, sourceID, filePath); err != nil {
			if s.logger != nil {
				s.logger.Warn("failed to process file", "path", filePath, "error", err)
			}
		}

		progress := 0
		if total > 0 {
			progress = (i + 1) * 100 / total
		}
		s.repo.UpdateJobProgress(ctx, jobID, progress)
	}

	s.repo.UpdateJobStatus(ctx, jobID, JobStatusCompleted, "")
	if s.logger != nil {
		s.logger.Info("scan completed", "job_id", jobID, "files_processed", total)
	}

	s.createIndexJobs(ctx, sourceID)
	return nil
}

func (s *Service) createIndexJobs(ctx context.Context, sourceID string) {
	files, err := s.repo.GetFilesBySource(ctx, sourceID)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("failed to list files for index job creation", "source_id", sourceID, "error", err)
		}
		return
	}

	existingJobs, err := s.repo.ListJobs(ctx, 10000)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("failed to list existing jobs", "error", err)
		}
		return
	}

	indexed := make(map[string]bool)
	for _, j := range existingJobs {
		if j.Type == JobTypeIndex && j.FileID != "" &&
			(j.Status == JobStatusPending || j.Status == JobStatusRunning || j.Status == JobStatusCompleted) {
			indexed[j.FileID] = true
		}
	}

	created := 0
	for _, f := range files {
		if indexed[f.ID] {
			continue
		}
		now := time.Now()
		job := &Job{
			ID:        NewID(),
			Type:      JobTypeIndex,
			Status:    JobStatusPending,
			SourceID:  sourceID,
			FileID:    f.ID,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := s.repo.CreateJob(ctx, job); err != nil {
			if s.logger != nil {
				s.logger.Warn("failed to create index job", "file_id", f.ID, "error", err)
			}
			continue
		}
		created++
	}

	if s.logger != nil {
		s.logger.Info("created index jobs", "source_id", sourceID, "count", created)
	}
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
	lr := io.LimitReader(f, fingerprintSize)
	if _, err := io.Copy(h, lr); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
