package catalog

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/heimdex/heimdex-agent/internal/db"
	"github.com/heimdex/heimdex-agent/internal/pipelines"
)

func setupRunnerTest(t *testing.T, fake *fakePipeRunner, caps *pipelines.Capabilities) (*Runner, Repository) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := db.New(dbPath, nil)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	repo := NewRepository(database.Conn())
	svc := NewService(repo, nil)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	doctor := pipelines.NewCachedDoctor(&fakeDoctorRunner{caps: caps}, logger)

	runner := NewRunner(svc, repo, fake, doctor, logger)
	return runner, repo
}

type fakePipeRunner struct {
	speechCalled atomic.Int32
	facesCalled  atomic.Int32
	scenesCalled atomic.Int32

	speechFn   func(ctx context.Context, videoPath, outPath string) (pipelines.RunResult, error)
	facesFn    func(ctx context.Context, videoPath, outPath string) (pipelines.RunResult, error)
	scenesFn   func(ctx context.Context, videoPath, speechResultPath, outPath string) (pipelines.RunResult, error)
	validateFn func(path string) (*pipelines.PipelineOutput, error)
	artifacts  string
}

func (f *fakePipeRunner) RunDoctor(ctx context.Context) (*pipelines.Capabilities, error) {
	return nil, fmt.Errorf("not implemented in test")
}

func (f *fakePipeRunner) RunSpeech(ctx context.Context, videoPath, outPath string) (pipelines.RunResult, error) {
	f.speechCalled.Add(1)
	if f.speechFn != nil {
		return f.speechFn(ctx, videoPath, outPath)
	}
	os.MkdirAll(filepath.Dir(outPath), 0755)
	os.WriteFile(outPath, []byte(`{"schema_version":"1.0","pipeline_version":"0.2.0","model_version":"whisper-large-v3"}`), 0644)
	return pipelines.RunResult{ExitCode: 0, OutputPath: outPath, Duration: 100 * time.Millisecond}, nil
}

func (f *fakePipeRunner) RunFaces(ctx context.Context, videoPath, outPath string) (pipelines.RunResult, error) {
	f.facesCalled.Add(1)
	if f.facesFn != nil {
		return f.facesFn(ctx, videoPath, outPath)
	}
	os.MkdirAll(filepath.Dir(outPath), 0755)
	os.WriteFile(outPath, []byte(`{"schema_version":"1.0","pipeline_version":"0.2.0","model_version":"scrfd"}`), 0644)
	return pipelines.RunResult{ExitCode: 0, OutputPath: outPath, Duration: 50 * time.Millisecond}, nil
}

func (f *fakePipeRunner) RunScenes(ctx context.Context, videoPath, speechResultPath, outPath string) (pipelines.RunResult, error) {
	f.scenesCalled.Add(1)
	if f.scenesFn != nil {
		return f.scenesFn(ctx, videoPath, speechResultPath, outPath)
	}
	os.MkdirAll(filepath.Dir(outPath), 0755)
	os.WriteFile(outPath, []byte(`{"schema_version":"1.0","pipeline_version":"0.2.0","model_version":"ffmpeg-scenecut"}`), 0644)
	return pipelines.RunResult{ExitCode: 0, OutputPath: outPath, Duration: 50 * time.Millisecond}, nil
}

func (f *fakePipeRunner) ValidateOutput(path string) (*pipelines.PipelineOutput, error) {
	if f.validateFn != nil {
		return f.validateFn(path)
	}
	return &pipelines.PipelineOutput{SchemaVersion: "1.0", PipelineVersion: "0.2.0", ModelVersion: "test"}, nil
}

func (f *fakePipeRunner) ArtifactsDir() string {
	if f.artifacts != "" {
		return f.artifacts
	}
	return "/tmp/test-artifacts"
}

type fakeDoctorRunner struct {
	caps *pipelines.Capabilities
}

func (f *fakeDoctorRunner) RunDoctor(ctx context.Context) (*pipelines.Capabilities, error) {
	return f.caps, nil
}

func (f *fakeDoctorRunner) RunSpeech(ctx context.Context, videoPath, outPath string) (pipelines.RunResult, error) {
	return pipelines.RunResult{}, nil
}

func (f *fakeDoctorRunner) RunFaces(ctx context.Context, videoPath, outPath string) (pipelines.RunResult, error) {
	return pipelines.RunResult{}, nil
}

func (f *fakeDoctorRunner) RunScenes(ctx context.Context, videoPath, speechResultPath, outPath string) (pipelines.RunResult, error) {
	return pipelines.RunResult{}, nil
}

func (f *fakeDoctorRunner) ValidateOutput(path string) (*pipelines.PipelineOutput, error) {
	return &pipelines.PipelineOutput{SchemaVersion: "1.0", PipelineVersion: "0.1.0", ModelVersion: "test"}, nil
}

func (f *fakeDoctorRunner) ArtifactsDir() string {
	return "/tmp/test-artifacts"
}

func createTestJobAndFile(t *testing.T, repo Repository) (*Job, *File) {
	t.Helper()
	ctx := context.Background()

	source := &Source{
		ID:          NewID(),
		Type:        "folder",
		Path:        "/test/videos",
		DisplayName: "Test",
		Present:     true,
		CreatedAt:   time.Now(),
	}
	if err := repo.CreateSource(ctx, source); err != nil {
		t.Fatalf("create source: %v", err)
	}

	file := &File{
		ID:          NewID(),
		SourceID:    source.ID,
		Path:        "/test/videos/clip.mp4",
		Filename:    "clip.mp4",
		Size:        1024,
		Mtime:       time.Now(),
		Fingerprint: "abc123",
		CreatedAt:   time.Now(),
	}
	if err := repo.CreateFile(ctx, file); err != nil {
		t.Fatalf("create file: %v", err)
	}

	now := time.Now()
	job := &Job{
		ID:        NewID(),
		Type:      JobTypeIndex,
		Status:    JobStatusPending,
		SourceID:  source.ID,
		FileID:    file.ID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := repo.CreateJob(ctx, job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	return job, file
}

func TestProcessIndexJob_WithScenes(t *testing.T) {
	fake := &fakePipeRunner{}
	caps := &pipelines.Capabilities{
		HasSpeech: true,
		HasFaces:  true,
		HasScenes: true,
		ProbedAt:  time.Now(),
	}

	runner, repo := setupRunnerTest(t, fake, caps)
	job, _ := createTestJobAndFile(t, repo)

	runner.processIndexJob(context.Background(), job)

	updatedJob, _ := repo.GetJob(context.Background(), job.ID)
	if updatedJob.Status != JobStatusCompleted {
		t.Errorf("job status = %s, want %s", updatedJob.Status, JobStatusCompleted)
	}

	if fake.speechCalled.Load() != 1 {
		t.Errorf("speech called %d times, want 1", fake.speechCalled.Load())
	}
	if fake.facesCalled.Load() != 1 {
		t.Errorf("faces called %d times, want 1", fake.facesCalled.Load())
	}
	if fake.scenesCalled.Load() != 1 {
		t.Errorf("scenes called %d times, want 1", fake.scenesCalled.Load())
	}
}

func TestProcessIndexJob_ScenesDisabled(t *testing.T) {
	fake := &fakePipeRunner{}
	caps := &pipelines.Capabilities{
		HasSpeech: true,
		HasFaces:  true,
		HasScenes: false,
		ProbedAt:  time.Now(),
	}

	runner, repo := setupRunnerTest(t, fake, caps)
	job, _ := createTestJobAndFile(t, repo)

	runner.processIndexJob(context.Background(), job)

	updatedJob, _ := repo.GetJob(context.Background(), job.ID)
	if updatedJob.Status != JobStatusCompleted {
		t.Errorf("job status = %s, want %s", updatedJob.Status, JobStatusCompleted)
	}

	if fake.speechCalled.Load() != 1 {
		t.Errorf("speech called %d times, want 1", fake.speechCalled.Load())
	}
	if fake.facesCalled.Load() != 1 {
		t.Errorf("faces called %d times, want 1", fake.facesCalled.Load())
	}
	if fake.scenesCalled.Load() != 0 {
		t.Errorf("scenes called %d times, want 0 (disabled)", fake.scenesCalled.Load())
	}
}

func TestProcessIndexJob_SpeechFailsNoScenes(t *testing.T) {
	fake := &fakePipeRunner{
		speechFn: func(ctx context.Context, videoPath, outPath string) (pipelines.RunResult, error) {
			return pipelines.RunResult{ExitCode: 1, StderrTail: "speech failed"}, nil
		},
	}
	caps := &pipelines.Capabilities{
		HasSpeech: true,
		HasFaces:  true,
		HasScenes: true,
		ProbedAt:  time.Now(),
	}

	runner, repo := setupRunnerTest(t, fake, caps)
	job, _ := createTestJobAndFile(t, repo)

	runner.processIndexJob(context.Background(), job)

	updatedJob, _ := repo.GetJob(context.Background(), job.ID)
	if updatedJob.Status != JobStatusFailed {
		t.Errorf("job status = %s, want %s", updatedJob.Status, JobStatusFailed)
	}

	if fake.scenesCalled.Load() != 0 {
		t.Errorf("scenes called %d times, want 0 (speech failed)", fake.scenesCalled.Load())
	}
}

func TestProcessIndexJob_ScenesOnly(t *testing.T) {
	fake := &fakePipeRunner{}
	caps := &pipelines.Capabilities{
		HasSpeech: false,
		HasFaces:  false,
		HasScenes: true,
		ProbedAt:  time.Now(),
	}

	runner, repo := setupRunnerTest(t, fake, caps)
	job, _ := createTestJobAndFile(t, repo)

	runner.processIndexJob(context.Background(), job)

	updatedJob, _ := repo.GetJob(context.Background(), job.ID)
	// Scenes requires speech, so with HasSpeech=false and speechOK=false, scenes won't run.
	// But the job should still complete (no steps ran successfully, but no failures either).
	if updatedJob.Status != JobStatusCompleted {
		t.Errorf("job status = %s, want %s", updatedJob.Status, JobStatusCompleted)
	}

	if fake.scenesCalled.Load() != 0 {
		t.Errorf("scenes called %d times, want 0 (no speech available)", fake.scenesCalled.Load())
	}
}

func TestProcessIndexJob_ProgressWithAllThreeSteps(t *testing.T) {
	fake := &fakePipeRunner{}
	caps := &pipelines.Capabilities{
		HasSpeech: true,
		HasFaces:  true,
		HasScenes: true,
		ProbedAt:  time.Now(),
	}

	runner, repo := setupRunnerTest(t, fake, caps)
	job, _ := createTestJobAndFile(t, repo)

	runner.processIndexJob(context.Background(), job)

	updatedJob, _ := repo.GetJob(context.Background(), job.ID)
	if updatedJob.Status != JobStatusCompleted {
		t.Errorf("job status = %s, want %s", updatedJob.Status, JobStatusCompleted)
	}
	if updatedJob.Progress != 100 {
		t.Errorf("job progress = %d, want 100", updatedJob.Progress)
	}
}

func TestProcessIndexJob_NoPipelineRunner(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := db.New(dbPath, nil)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	defer database.Close()

	repo := NewRepository(database.Conn())
	svc := NewService(repo, nil)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	runner := NewRunner(svc, repo, nil, nil, logger)

	job, _ := createTestJobAndFile(t, repo)
	runner.processIndexJob(context.Background(), job)

	updatedJob, _ := repo.GetJob(context.Background(), job.ID)
	if updatedJob.Status != JobStatusFailed {
		t.Errorf("job status = %s, want %s", updatedJob.Status, JobStatusFailed)
	}
}
