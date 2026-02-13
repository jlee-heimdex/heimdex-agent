package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/heimdex/heimdex-agent/internal/cloud"
	"github.com/heimdex/heimdex-agent/internal/db"
	"github.com/heimdex/heimdex-agent/internal/pipeline"
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

	runner := NewRunner(svc, repo, fake, pipeline.NewStubFFmpeg(logger), doctor, logger)
	return runner, repo
}

type fakePipeRunner struct {
	speechCalled atomic.Int32
	facesCalled  atomic.Int32
	scenesCalled atomic.Int32

	speechFn        func(ctx context.Context, videoPath, outPath string) (pipelines.RunResult, error)
	facesFn         func(ctx context.Context, videoPath, outPath string) (pipelines.RunResult, error)
	scenesFn        func(ctx context.Context, videoPath, videoID, speechResultPath, outPath string, ocrEnabled, redactPII bool) (pipelines.RunResult, error)
	validateFn      func(path string) (*pipelines.PipelineOutput, error)
	validateSceneFn func(path string) (*pipelines.PipelineOutput, error)
	artifacts       string
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

func (f *fakePipeRunner) RunScenes(ctx context.Context, videoPath, videoID, speechResultPath, outPath string, ocrEnabled, redactPII bool) (pipelines.RunResult, error) {
	f.scenesCalled.Add(1)
	if f.scenesFn != nil {
		return f.scenesFn(ctx, videoPath, videoID, speechResultPath, outPath, ocrEnabled, redactPII)
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

func (f *fakePipeRunner) ValidateSceneOutput(path string) (*pipelines.PipelineOutput, error) {
	if f.validateSceneFn != nil {
		return f.validateSceneFn(path)
	}
	return f.ValidateOutput(path)
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

type fakeOCRConfig struct {
	enabled bool
	redact  bool
}

func (c fakeOCRConfig) OCREnabled() bool {
	return c.enabled
}

func (c fakeOCRConfig) OCRRedactPII() bool {
	return c.redact
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

func (f *fakeDoctorRunner) RunScenes(ctx context.Context, videoPath, videoID, speechResultPath, outPath string, ocrEnabled, redactPII bool) (pipelines.RunResult, error) {
	return pipelines.RunResult{}, nil
}

func (f *fakeDoctorRunner) ValidateOutput(path string) (*pipelines.PipelineOutput, error) {
	return &pipelines.PipelineOutput{SchemaVersion: "1.0", PipelineVersion: "0.1.0", ModelVersion: "test"}, nil
}

func (f *fakeDoctorRunner) ValidateSceneOutput(path string) (*pipelines.PipelineOutput, error) {
	return f.ValidateOutput(path)
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

func TestProcessIndexJob_PassesOCRFlagsToScenes(t *testing.T) {
	gotOCREnabled := false
	gotRedactPII := false

	fake := &fakePipeRunner{
		scenesFn: func(ctx context.Context, videoPath, videoID, speechResultPath, outPath string, ocrEnabled, redactPII bool) (pipelines.RunResult, error) {
			gotOCREnabled = ocrEnabled
			gotRedactPII = redactPII
			return pipelines.RunResult{ExitCode: 0, OutputPath: outPath, Duration: 10 * time.Millisecond}, nil
		},
	}
	caps := &pipelines.Capabilities{
		HasSpeech: true,
		HasFaces:  false,
		HasScenes: true,
		HasOCR:    true,
		ProbedAt:  time.Now(),
	}

	runner, repo := setupRunnerTest(t, fake, caps)
	runner.SetOCRConfig(fakeOCRConfig{enabled: true, redact: true})
	job, _ := createTestJobAndFile(t, repo)

	runner.processIndexJob(context.Background(), job)

	updatedJob, _ := repo.GetJob(context.Background(), job.ID)
	if updatedJob.Status != JobStatusCompleted {
		t.Errorf("job status = %s, want %s", updatedJob.Status, JobStatusCompleted)
	}
	if !gotOCREnabled {
		t.Error("ocrEnabled = false, want true")
	}
	if !gotRedactPII {
		t.Error("redactPII = false, want true")
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

	runner := NewRunner(svc, repo, nil, nil, nil, logger)

	job, _ := createTestJobAndFile(t, repo)
	runner.processIndexJob(context.Background(), job)

	updatedJob, _ := repo.GetJob(context.Background(), job.ID)
	if updatedJob.Status != JobStatusFailed {
		t.Errorf("job status = %s, want %s", updatedJob.Status, JobStatusFailed)
	}
}

func TestProcessIndexJob_CancelledContext(t *testing.T) {
	fake := &fakePipeRunner{}
	caps := &pipelines.Capabilities{
		HasSpeech: true,
		HasFaces:  true,
		HasScenes: true,
		ProbedAt:  time.Now(),
	}

	runner, repo := setupRunnerTest(t, fake, caps)
	job, _ := createTestJobAndFile(t, repo)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	runner.processIndexJob(ctx, job)

	updatedJob, _ := repo.GetJob(context.Background(), job.ID)
	if updatedJob.Status != JobStatusFailed {
		t.Errorf("job status = %s, want %s", updatedJob.Status, JobStatusFailed)
	}

	if fake.speechCalled.Load() != 0 {
		t.Errorf("speech called %d times, want 0", fake.speechCalled.Load())
	}
	if fake.facesCalled.Load() != 0 {
		t.Errorf("faces called %d times, want 0", fake.facesCalled.Load())
	}
	if fake.scenesCalled.Load() != 0 {
		t.Errorf("scenes called %d times, want 0", fake.scenesCalled.Load())
	}
}

func TestProcessIndexJob_FacesFailsScenesStillDrains(t *testing.T) {
	scenesExited := make(chan struct{})

	fake := &fakePipeRunner{
		facesFn: func(ctx context.Context, videoPath, outPath string) (pipelines.RunResult, error) {
			return pipelines.RunResult{ExitCode: 1, StderrTail: "faces failed"}, nil
		},
		scenesFn: func(ctx context.Context, videoPath, videoID, speechResultPath, outPath string, ocrEnabled, redactPII bool) (pipelines.RunResult, error) {
			defer close(scenesExited)

			select {
			case <-ctx.Done():
				return pipelines.RunResult{}, ctx.Err()
			case <-time.After(100 * time.Millisecond):
				if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
					return pipelines.RunResult{}, err
				}
				if err := os.WriteFile(outPath, []byte(`{
					"schema_version":"1.0",
					"pipeline_version":"0.2.0",
					"model_version":"ffmpeg-scenecut",
					"video_id":"video-1",
					"scenes":[]
				}`), 0644); err != nil {
					return pipelines.RunResult{}, err
				}
				return pipelines.RunResult{ExitCode: 0, OutputPath: outPath}, nil
			}
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

	done := make(chan struct{})
	go func() {
		runner.processIndexJob(context.Background(), job)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("processIndexJob did not complete")
	}

	select {
	case <-scenesExited:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("scenes goroutine did not exit")
	}

	updatedJob, _ := repo.GetJob(context.Background(), job.ID)
	if updatedJob.Status != JobStatusFailed {
		t.Errorf("job status = %s, want %s", updatedJob.Status, JobStatusFailed)
	}
}

type fakeSceneUploader struct {
	uploadFn func(ctx context.Context, payload cloud.SceneIngestPayload) error
}

func (f *fakeSceneUploader) UploadScenes(ctx context.Context, payload cloud.SceneIngestPayload) error {
	if f.uploadFn != nil {
		return f.uploadFn(ctx, payload)
	}
	return nil
}

type fakeCloudClient struct {
	scenes *fakeSceneUploader
}

func (f *fakeCloudClient) Auth() cloud.AuthService              { return nil }
func (f *fakeCloudClient) Upload() cloud.UploadService          { return nil }
func (f *fakeCloudClient) Scenes() cloud.SceneUploader          { return f.scenes }
func (f *fakeCloudClient) Libraries() cloud.LibraryService      { return &fakeLibraryService{} }
func (f *fakeCloudClient) RegisterDevice(deviceID string) error { return nil }

type fakeLibraryService struct{}

func (f *fakeLibraryService) GetOrCreate(ctx context.Context, name string) (*cloud.LibraryResult, error) {
	return &cloud.LibraryResult{ID: "auto-lib-" + name, Name: name, Created: true}, nil
}

func (f *fakeLibraryService) List(ctx context.Context) ([]cloud.LibraryResult, error) {
	return nil, nil
}

func writeSceneResult(t *testing.T, artifactsDir, fileID string) {
	t.Helper()
	writeSceneResultWithPayload(t, artifactsDir, fileID, pipelines.SceneOutputPayload{
		PipelineOutput: pipelines.PipelineOutput{
			SchemaVersion:   "1.0",
			PipelineVersion: "0.3.0",
			ModelVersion:    "ffmpeg-scenecut",
		},
		VideoID:         "video-1",
		TotalDurationMs: 60000,
		Scenes: []pipelines.SceneBoundary{
			{
				SceneID:             "video-1_scene_0",
				Index:               0,
				StartMs:             0,
				EndMs:               5000,
				KeyframeTimestampMs: 2500,
				TranscriptRaw:       "지금 이 수분크림을 소개합니다",
				SpeechSegmentCount:  2,
				PeopleClusterIDs:    []string{"cluster-abc", "cluster-def"},
				KeywordTags:         []string{"cta", "feature"},
				ProductTags:         []string{"skincare"},
				ProductEntities:     []string{"수분크림"},
			},
		},
	})
}

func writeSceneResultWithPayload(t *testing.T, artifactsDir, fileID string, payload pipelines.SceneOutputPayload) {
	t.Helper()
	scenesDir := filepath.Join(artifactsDir, fileID, "scenes")
	if err := os.MkdirAll(scenesDir, 0755); err != nil {
		t.Fatalf("mkdir scenes: %v", err)
	}
	data, _ := json.Marshal(payload)
	if err := os.WriteFile(filepath.Join(scenesDir, "result.json"), data, 0644); err != nil {
		t.Fatalf("write scene result: %v", err)
	}
}

func TestUploadScenesToCloud_PermanentError_NoRetryJob(t *testing.T) {
	fake := &fakePipeRunner{}
	caps := &pipelines.Capabilities{HasSpeech: true, HasFaces: true, HasScenes: true, ProbedAt: time.Now()}

	runner, repo := setupRunnerTest(t, fake, caps)

	tmpDir := t.TempDir()
	fake.artifacts = tmpDir

	cc := &fakeCloudClient{scenes: &fakeSceneUploader{
		uploadFn: func(_ context.Context, _ cloud.SceneIngestPayload) error {
			return &cloud.UploadError{StatusCode: 422, Body: "unprocessable"}
		},
	}}
	runner.SetCloudClient(cc, "lib-1")

	_, file := createTestJobAndFile(t, repo)
	writeSceneResult(t, tmpDir, file.ID)

	job := &Job{
		ID:        NewID(),
		Type:      JobTypeIndex,
		Status:    JobStatusRunning,
		FileID:    file.ID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.CreateJob(context.Background(), job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	runner.uploadScenesToCloud(context.Background(), job, file, filepath.Join(tmpDir, file.ID))

	jobs, _ := repo.ListJobs(context.Background(), 100)
	for _, j := range jobs {
		if j.Type == JobTypeUploadScenes {
			t.Error("expected NO upload_scenes retry job for permanent 422 error, but found one")
		}
	}
}

func TestUploadScenesToCloud_RetryableError_CreatesRetryJob(t *testing.T) {
	fake := &fakePipeRunner{}
	caps := &pipelines.Capabilities{HasSpeech: true, HasFaces: true, HasScenes: true, ProbedAt: time.Now()}

	runner, repo := setupRunnerTest(t, fake, caps)

	tmpDir := t.TempDir()
	fake.artifacts = tmpDir

	cc := &fakeCloudClient{scenes: &fakeSceneUploader{
		uploadFn: func(_ context.Context, _ cloud.SceneIngestPayload) error {
			return &cloud.UploadError{StatusCode: 500, Body: "internal server error"}
		},
	}}
	runner.SetCloudClient(cc, "lib-1")

	_, file := createTestJobAndFile(t, repo)
	writeSceneResult(t, tmpDir, file.ID)

	job := &Job{
		ID:        NewID(),
		Type:      JobTypeIndex,
		Status:    JobStatusRunning,
		FileID:    file.ID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.CreateJob(context.Background(), job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	runner.uploadScenesToCloud(context.Background(), job, file, filepath.Join(tmpDir, file.ID))

	found := false
	jobs, _ := repo.ListJobs(context.Background(), 100)
	for _, j := range jobs {
		if j.Type == JobTypeUploadScenes {
			found = true
		}
	}
	if !found {
		t.Error("expected upload_scenes retry job for 500 error, but none found")
	}
}

func TestUploadScenesToCloud_NetworkError_CreatesRetryJob(t *testing.T) {
	fake := &fakePipeRunner{}
	caps := &pipelines.Capabilities{HasSpeech: true, HasFaces: true, HasScenes: true, ProbedAt: time.Now()}

	runner, repo := setupRunnerTest(t, fake, caps)

	tmpDir := t.TempDir()
	fake.artifacts = tmpDir

	cc := &fakeCloudClient{scenes: &fakeSceneUploader{
		uploadFn: func(_ context.Context, _ cloud.SceneIngestPayload) error {
			return fmt.Errorf("connection refused")
		},
	}}
	runner.SetCloudClient(cc, "lib-1")

	_, file := createTestJobAndFile(t, repo)
	writeSceneResult(t, tmpDir, file.ID)

	job := &Job{
		ID:        NewID(),
		Type:      JobTypeIndex,
		Status:    JobStatusRunning,
		FileID:    file.ID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.CreateJob(context.Background(), job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	runner.uploadScenesToCloud(context.Background(), job, file, filepath.Join(tmpDir, file.ID))

	found := false
	jobs, _ := repo.ListJobs(context.Background(), 100)
	for _, j := range jobs {
		if j.Type == JobTypeUploadScenes {
			found = true
		}
	}
	if !found {
		t.Error("expected upload_scenes retry job for network error, but none found")
	}
}

func TestUploadBackoff(t *testing.T) {
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 10 * time.Second},
		{1, 30 * time.Second},
		{2, 90 * time.Second},
		{3, 270 * time.Second},
		{4, 600 * time.Second},
		{5, 600 * time.Second},
		{10, 600 * time.Second},
	}
	for _, tc := range cases {
		got := uploadBackoff(tc.attempt)
		if got != tc.want {
			t.Errorf("uploadBackoff(%d) = %v, want %v", tc.attempt, got, tc.want)
		}
	}
}

func TestBuildSceneIngestDocs_PropagatesAllFields(t *testing.T) {
	input := []pipelines.SceneBoundary{
		{
			SceneID:             "vid_scene_0",
			Index:               0,
			StartMs:             0,
			EndMs:               5000,
			KeyframeTimestampMs: 2500,
			TranscriptRaw:       "hello world",
			SpeechSegmentCount:  3,
			PeopleClusterIDs:    []string{"p1", "p2"},
			KeywordTags:         []string{"cta", "price"},
			ProductTags:         []string{"skincare"},
			ProductEntities:     []string{"세럼", "수분크림"},
		},
		{
			SceneID:             "vid_scene_1",
			Index:               1,
			StartMs:             5000,
			EndMs:               12000,
			KeyframeTimestampMs: 8000,
			TranscriptRaw:       "second scene transcript",
			SpeechSegmentCount:  1,
			PeopleClusterIDs:    nil,
			KeywordTags:         nil,
			ProductTags:         nil,
			ProductEntities:     nil,
		},
	}

	docs := buildSceneIngestDocs(input, "local")

	if len(docs) != 2 {
		t.Fatalf("got %d docs, want 2", len(docs))
	}

	d := docs[0]
	if d.SourceType != "local" {
		t.Errorf("SourceType = %q, want %q", d.SourceType, "local")
	}
	if d.SceneID != "vid_scene_0" {
		t.Errorf("SceneID = %q, want %q", d.SceneID, "vid_scene_0")
	}
	if d.Index != 0 {
		t.Errorf("Index = %d, want 0", d.Index)
	}
	if d.StartMs != 0 || d.EndMs != 5000 {
		t.Errorf("StartMs/EndMs = %d/%d, want 0/5000", d.StartMs, d.EndMs)
	}
	if d.KeyframeTimestampMs != 2500 {
		t.Errorf("KeyframeTimestampMs = %d, want 2500", d.KeyframeTimestampMs)
	}
	if d.TranscriptRaw != "hello world" {
		t.Errorf("TranscriptRaw = %q, want %q", d.TranscriptRaw, "hello world")
	}
	if d.SpeechSegmentCount != 3 {
		t.Errorf("SpeechSegmentCount = %d, want 3", d.SpeechSegmentCount)
	}
	if len(d.PeopleClusterIDs) != 2 || d.PeopleClusterIDs[0] != "p1" || d.PeopleClusterIDs[1] != "p2" {
		t.Errorf("PeopleClusterIDs = %v, want [p1 p2]", d.PeopleClusterIDs)
	}
	if len(d.KeywordTags) != 2 || d.KeywordTags[0] != "cta" || d.KeywordTags[1] != "price" {
		t.Errorf("KeywordTags = %v, want [cta price]", d.KeywordTags)
	}
	if len(d.ProductTags) != 1 || d.ProductTags[0] != "skincare" {
		t.Errorf("ProductTags = %v, want [skincare]", d.ProductTags)
	}
	if len(d.ProductEntities) != 2 || d.ProductEntities[0] != "세럼" {
		t.Errorf("ProductEntities = %v, want [세럼 수분크림]", d.ProductEntities)
	}

	d1 := docs[1]
	if d1.TranscriptRaw != "second scene transcript" {
		t.Errorf("docs[1].TranscriptRaw = %q, want %q", d1.TranscriptRaw, "second scene transcript")
	}
	if d1.PeopleClusterIDs != nil {
		t.Errorf("docs[1].PeopleClusterIDs = %v, want nil (omitempty safe)", d1.PeopleClusterIDs)
	}
	if d1.KeywordTags != nil {
		t.Errorf("docs[1].KeywordTags = %v, want nil", d1.KeywordTags)
	}
	if d1.ProductTags != nil {
		t.Errorf("docs[1].ProductTags = %v, want nil", d1.ProductTags)
	}
	if d1.ProductEntities != nil {
		t.Errorf("docs[1].ProductEntities = %v, want nil", d1.ProductEntities)
	}
}

func TestBuildSceneIngestDocs_EmptyInput(t *testing.T) {
	docs := buildSceneIngestDocs(nil, "local")
	if len(docs) != 0 {
		t.Errorf("got %d docs for nil input, want 0", len(docs))
	}
	docs = buildSceneIngestDocs([]pipelines.SceneBoundary{}, "gdrive")
	if len(docs) != 0 {
		t.Errorf("got %d docs for empty input, want 0", len(docs))
	}
}

func TestBuildSceneIngestDocs_BackwardCompat_MissingFields(t *testing.T) {
	input := []pipelines.SceneBoundary{
		{SceneID: "vid_scene_0", StartMs: 0, EndMs: 5000},
	}
	docs := buildSceneIngestDocs(input, "removable_disk")
	if len(docs) != 1 {
		t.Fatalf("got %d docs, want 1", len(docs))
	}
	d := docs[0]
	if d.SourceType != "removable_disk" {
		t.Errorf("SourceType = %q, want %q", d.SourceType, "removable_disk")
	}
	if d.TranscriptRaw != "" {
		t.Errorf("TranscriptRaw = %q, want empty string", d.TranscriptRaw)
	}
	if d.SpeechSegmentCount != 0 {
		t.Errorf("SpeechSegmentCount = %d, want 0", d.SpeechSegmentCount)
	}
	if d.PeopleClusterIDs != nil {
		t.Errorf("PeopleClusterIDs = %v, want nil", d.PeopleClusterIDs)
	}
	if d.KeyframeTimestampMs != 0 {
		t.Errorf("KeyframeTimestampMs = %d, want 0", d.KeyframeTimestampMs)
	}
	if d.KeywordTags != nil {
		t.Errorf("KeywordTags = %v, want nil", d.KeywordTags)
	}
	if d.ProductTags != nil {
		t.Errorf("ProductTags = %v, want nil", d.ProductTags)
	}
	if d.ProductEntities != nil {
		t.Errorf("ProductEntities = %v, want nil", d.ProductEntities)
	}
}

func TestBuildSceneIngestDocs_PropagatesOCRFields(t *testing.T) {
	input := []pipelines.SceneBoundary{
		{
			SceneID:      "vid_scene_0",
			StartMs:      0,
			EndMs:        5000,
			OCRTextRaw:   "hello",
			OCRCharCount: 5,
		},
	}

	docs := buildSceneIngestDocs(input, "local")
	if len(docs) != 1 {
		t.Fatalf("got %d docs, want 1", len(docs))
	}
	if docs[0].OCRTextRaw != "hello" {
		t.Errorf("OCRTextRaw = %q, want %q", docs[0].OCRTextRaw, "hello")
	}
	if docs[0].OCRCharCount != 5 {
		t.Errorf("OCRCharCount = %d, want 5", docs[0].OCRCharCount)
	}
}

func TestBuildSceneIngestDocs_EmptyOCR(t *testing.T) {
	input := []pipelines.SceneBoundary{
		{
			SceneID: "vid_scene_0",
			StartMs: 0,
			EndMs:   5000,
		},
	}

	docs := buildSceneIngestDocs(input, "local")
	if len(docs) != 1 {
		t.Fatalf("got %d docs, want 1", len(docs))
	}
	if docs[0].OCRTextRaw != "" {
		t.Errorf("OCRTextRaw = %q, want empty", docs[0].OCRTextRaw)
	}
	if docs[0].OCRCharCount != 0 {
		t.Errorf("OCRCharCount = %d, want 0", docs[0].OCRCharCount)
	}
}

func TestUploadScenesToCloud_PropagatesFieldsToPayload(t *testing.T) {
	fake := &fakePipeRunner{}
	caps := &pipelines.Capabilities{HasSpeech: true, HasFaces: true, HasScenes: true, ProbedAt: time.Now()}

	runner, repo := setupRunnerTest(t, fake, caps)

	tmpDir := t.TempDir()
	fake.artifacts = tmpDir

	var captured cloud.SceneIngestPayload
	cc := &fakeCloudClient{scenes: &fakeSceneUploader{
		uploadFn: func(_ context.Context, p cloud.SceneIngestPayload) error {
			captured = p
			return nil
		},
	}}
	runner.SetCloudClient(cc, "lib-1")

	_, file := createTestJobAndFile(t, repo)
	writeSceneResult(t, tmpDir, file.ID)

	job := &Job{
		ID:        NewID(),
		Type:      JobTypeIndex,
		Status:    JobStatusRunning,
		FileID:    file.ID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.CreateJob(context.Background(), job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	runner.uploadScenesToCloud(context.Background(), job, file, filepath.Join(tmpDir, file.ID))

	if captured.VideoID != "video-1" {
		t.Errorf("payload.VideoID = %q, want %q", captured.VideoID, "video-1")
	}
	if captured.VideoTitle != "clip" {
		t.Errorf("payload.VideoTitle = %q, want %q", captured.VideoTitle, "clip")
	}
	if captured.TotalDurationMs != 60000 {
		t.Errorf("payload.TotalDurationMs = %d, want 60000", captured.TotalDurationMs)
	}
	if len(captured.Scenes) != 1 {
		t.Fatalf("payload.Scenes count = %d, want 1", len(captured.Scenes))
	}

	s := captured.Scenes[0]
	if s.SourceType != "local" {
		t.Errorf("SourceType = %q, want %q (folder source resolves to local)", s.SourceType, "local")
	}
	if s.TranscriptRaw != "지금 이 수분크림을 소개합니다" {
		t.Errorf("TranscriptRaw = %q, want Korean transcript", s.TranscriptRaw)
	}
	if s.SpeechSegmentCount != 2 {
		t.Errorf("SpeechSegmentCount = %d, want 2", s.SpeechSegmentCount)
	}
	if len(s.PeopleClusterIDs) != 2 {
		t.Errorf("PeopleClusterIDs = %v, want [cluster-abc cluster-def]", s.PeopleClusterIDs)
	}
	if s.KeyframeTimestampMs != 2500 {
		t.Errorf("KeyframeTimestampMs = %d, want 2500", s.KeyframeTimestampMs)
	}
	if len(s.KeywordTags) != 2 || s.KeywordTags[0] != "cta" || s.KeywordTags[1] != "feature" {
		t.Errorf("KeywordTags = %v, want [cta feature]", s.KeywordTags)
	}
	if len(s.ProductTags) != 1 || s.ProductTags[0] != "skincare" {
		t.Errorf("ProductTags = %v, want [skincare]", s.ProductTags)
	}
	if len(s.ProductEntities) != 1 || s.ProductEntities[0] != "수분크림" {
		t.Errorf("ProductEntities = %v, want [수분크림]", s.ProductEntities)
	}
}

func TestUploadScenesToCloud_OlderPipelineOutput_StillWorks(t *testing.T) {
	fake := &fakePipeRunner{}
	caps := &pipelines.Capabilities{HasSpeech: true, HasFaces: true, HasScenes: true, ProbedAt: time.Now()}

	runner, repo := setupRunnerTest(t, fake, caps)

	tmpDir := t.TempDir()
	fake.artifacts = tmpDir

	var captured cloud.SceneIngestPayload
	cc := &fakeCloudClient{scenes: &fakeSceneUploader{
		uploadFn: func(_ context.Context, p cloud.SceneIngestPayload) error {
			captured = p
			return nil
		},
	}}
	runner.SetCloudClient(cc, "lib-1")

	_, file := createTestJobAndFile(t, repo)

	writeSceneResultWithPayload(t, tmpDir, file.ID, pipelines.SceneOutputPayload{
		PipelineOutput: pipelines.PipelineOutput{
			SchemaVersion:   "1.0",
			PipelineVersion: "0.1.0",
			ModelVersion:    "ffmpeg-scenecut",
		},
		VideoID: "old-video",
		Scenes: []pipelines.SceneBoundary{
			{SceneID: "old-video_scene_0", StartMs: 0, EndMs: 3000},
		},
	})

	job := &Job{
		ID:        NewID(),
		Type:      JobTypeIndex,
		Status:    JobStatusRunning,
		FileID:    file.ID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.CreateJob(context.Background(), job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	runner.uploadScenesToCloud(context.Background(), job, file, filepath.Join(tmpDir, file.ID))

	if captured.VideoID != "old-video" {
		t.Fatalf("upload not called or wrong video_id: %q", captured.VideoID)
	}
	if captured.VideoTitle != "clip" {
		t.Errorf("payload.VideoTitle = %q, want %q", captured.VideoTitle, "clip")
	}

	s := captured.Scenes[0]
	if s.SourceType != "local" {
		t.Errorf("SourceType = %q, want %q (folder source resolves to local)", s.SourceType, "local")
	}
	if s.TranscriptRaw != "" {
		t.Errorf("TranscriptRaw = %q, want empty for old pipeline output", s.TranscriptRaw)
	}
	if s.SpeechSegmentCount != 0 {
		t.Errorf("SpeechSegmentCount = %d, want 0 for old pipeline output", s.SpeechSegmentCount)
	}
	if s.PeopleClusterIDs != nil {
		t.Errorf("PeopleClusterIDs = %v, want nil for old pipeline output", s.PeopleClusterIDs)
	}
	if s.KeywordTags != nil {
		t.Errorf("KeywordTags = %v, want nil for old pipeline output", s.KeywordTags)
	}
	if s.ProductTags != nil {
		t.Errorf("ProductTags = %v, want nil for old pipeline output", s.ProductTags)
	}
	if s.ProductEntities != nil {
		t.Errorf("ProductEntities = %v, want nil for old pipeline output", s.ProductEntities)
	}
}

func TestProcessUploadScenesJob_RespectsBackoff(t *testing.T) {
	fake := &fakePipeRunner{}
	caps := &pipelines.Capabilities{HasSpeech: true, HasFaces: true, HasScenes: true, ProbedAt: time.Now()}

	runner, repo := setupRunnerTest(t, fake, caps)

	tmpDir := t.TempDir()
	fake.artifacts = tmpDir

	uploadCalled := false
	cc := &fakeCloudClient{scenes: &fakeSceneUploader{
		uploadFn: func(_ context.Context, _ cloud.SceneIngestPayload) error {
			uploadCalled = true
			return nil
		},
	}}
	runner.SetCloudClient(cc, "lib-1")

	_, file := createTestJobAndFile(t, repo)
	writeSceneResult(t, tmpDir, file.ID)

	now := time.Now()
	job := &Job{
		ID:        NewID(),
		Type:      JobTypeUploadScenes,
		Status:    JobStatusPending,
		FileID:    file.ID,
		Progress:  1,
		UpdatedAt: now,
		CreatedAt: now,
	}
	if err := repo.CreateJob(context.Background(), job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	runner.processUploadScenesJob(context.Background(), job)

	if uploadCalled {
		t.Error("expected upload to be skipped due to backoff, but it was called")
	}

	updatedJob, _ := repo.GetJob(context.Background(), job.ID)
	if updatedJob.Status != JobStatusPending {
		t.Errorf("job should remain pending during backoff, got %s", updatedJob.Status)
	}
}

func TestSceneBoundary_JSONUnmarshal_WithTags(t *testing.T) {
	raw := `{
		"scene_id": "vid_scene_0",
		"index": 0,
		"start_ms": 0,
		"end_ms": 5000,
		"keyframe_timestamp_ms": 2500,
		"transcript_raw": "지금 세럼 소개",
		"speech_segment_count": 1,
		"people_cluster_ids": [],
		"keyword_tags": ["cta", "feature"],
		"product_tags": ["skincare"],
		"product_entities": ["세럼"]
	}`

	var sb pipelines.SceneBoundary
	if err := json.Unmarshal([]byte(raw), &sb); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(sb.KeywordTags) != 2 || sb.KeywordTags[0] != "cta" {
		t.Errorf("KeywordTags = %v, want [cta feature]", sb.KeywordTags)
	}
	if len(sb.ProductTags) != 1 || sb.ProductTags[0] != "skincare" {
		t.Errorf("ProductTags = %v, want [skincare]", sb.ProductTags)
	}
	if len(sb.ProductEntities) != 1 || sb.ProductEntities[0] != "세럼" {
		t.Errorf("ProductEntities = %v, want [세럼]", sb.ProductEntities)
	}
}

func TestSceneBoundary_JSONUnmarshal_WithoutTags(t *testing.T) {
	raw := `{
		"scene_id": "vid_scene_0",
		"index": 0,
		"start_ms": 0,
		"end_ms": 5000
	}`

	var sb pipelines.SceneBoundary
	if err := json.Unmarshal([]byte(raw), &sb); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if sb.KeywordTags != nil {
		t.Errorf("KeywordTags = %v, want nil for missing field", sb.KeywordTags)
	}
	if sb.ProductTags != nil {
		t.Errorf("ProductTags = %v, want nil for missing field", sb.ProductTags)
	}
	if sb.ProductEntities != nil {
		t.Errorf("ProductEntities = %v, want nil for missing field", sb.ProductEntities)
	}
}

func TestSceneIngestDoc_JSONMarshal_OmitsEmptyTags(t *testing.T) {
	doc := cloud.SceneIngestDoc{
		SceneID: "vid_scene_0",
		Index:   0,
		StartMs: 0,
		EndMs:   5000,
	}

	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	raw := string(data)
	if strings.Contains(raw, "keyword_tags") {
		t.Error("nil KeywordTags should be omitted from JSON")
	}
	if strings.Contains(raw, "product_tags") {
		t.Error("nil ProductTags should be omitted from JSON")
	}
	if strings.Contains(raw, "product_entities") {
		t.Error("nil ProductEntities should be omitted from JSON")
	}
}

func TestSceneIngestDoc_JSONMarshal_IncludesTags(t *testing.T) {
	doc := cloud.SceneIngestDoc{
		SceneID:         "vid_scene_0",
		Index:           0,
		StartMs:         0,
		EndMs:           5000,
		KeywordTags:     []string{"cta"},
		ProductTags:     []string{"skincare"},
		ProductEntities: []string{"세럼"},
	}

	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed map[string]interface{}
	json.Unmarshal(data, &parsed)

	if _, ok := parsed["keyword_tags"]; !ok {
		t.Error("KeywordTags should be present in JSON when non-nil")
	}
	if _, ok := parsed["product_tags"]; !ok {
		t.Error("ProductTags should be present in JSON when non-nil")
	}
	if _, ok := parsed["product_entities"]; !ok {
		t.Error("ProductEntities should be present in JSON when non-nil")
	}
}

func TestResolveSourceType(t *testing.T) {
	cases := []struct {
		name   string
		source *Source
		want   string
	}{
		{"nil source defaults to local", nil, "local"},
		{"folder maps to local", &Source{Type: "folder"}, "local"},
		{"gdrive stays gdrive", &Source{Type: "gdrive"}, "gdrive"},
		{"removable_disk stays removable_disk", &Source{Type: "removable_disk"}, "removable_disk"},
		{"unknown type defaults to local", &Source{Type: "something_else"}, "local"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveSourceType(tc.source)
			if got != tc.want {
				t.Errorf("resolveSourceType(%v) = %q, want %q", tc.source, got, tc.want)
			}
		})
	}
}

func TestBackfillCloudUploads_CreatesJobsForIndexedFiles(t *testing.T) {
	fake := &fakePipeRunner{}
	caps := &pipelines.Capabilities{HasSpeech: true, HasScenes: true, ProbedAt: time.Now()}

	runner, repo := setupRunnerTest(t, fake, caps)

	tmpDir := t.TempDir()
	fake.artifacts = tmpDir

	cc := &fakeCloudClient{scenes: &fakeSceneUploader{}}
	runner.SetCloudClient(cc, "lib-1")

	_, file1 := createTestJobAndFile(t, repo)

	ctx := context.Background()

	file2 := &File{
		ID:          NewID(),
		SourceID:    file1.SourceID,
		Path:        "/test/videos/clip2.mp4",
		Filename:    "clip2.mp4",
		Size:        2048,
		Mtime:       time.Now(),
		Fingerprint: "def456",
		CreatedAt:   time.Now(),
	}
	if err := repo.CreateFile(ctx, file2); err != nil {
		t.Fatalf("create file2: %v", err)
	}

	repo.CreateJob(ctx, &Job{ID: NewID(), Type: JobTypeIndex, Status: JobStatusCompleted, FileID: file1.ID, CreatedAt: time.Now(), UpdatedAt: time.Now()})
	repo.CreateJob(ctx, &Job{ID: NewID(), Type: JobTypeIndex, Status: JobStatusCompleted, FileID: file2.ID, CreatedAt: time.Now(), UpdatedAt: time.Now()})

	writeSceneResult(t, tmpDir, file1.ID)
	writeSceneResult(t, tmpDir, file2.ID)

	runner.backfillCloudUploads(ctx)

	jobs, _ := repo.ListJobs(ctx, 100)
	uploadCount := 0
	for _, j := range jobs {
		if j.Type == JobTypeUploadScenes {
			uploadCount++
		}
	}
	if uploadCount != 2 {
		t.Errorf("expected 2 upload_scenes jobs, got %d", uploadCount)
	}
}

func TestBackfillCloudUploads_SkipsAlreadyUploaded(t *testing.T) {
	fake := &fakePipeRunner{}
	caps := &pipelines.Capabilities{HasSpeech: true, HasScenes: true, ProbedAt: time.Now()}

	runner, repo := setupRunnerTest(t, fake, caps)

	tmpDir := t.TempDir()
	fake.artifacts = tmpDir

	cc := &fakeCloudClient{scenes: &fakeSceneUploader{}}
	runner.SetCloudClient(cc, "lib-1")

	_, file := createTestJobAndFile(t, repo)

	ctx := context.Background()

	repo.CreateJob(ctx, &Job{ID: NewID(), Type: JobTypeIndex, Status: JobStatusCompleted, FileID: file.ID, CreatedAt: time.Now(), UpdatedAt: time.Now()})
	repo.CreateJob(ctx, &Job{ID: NewID(), Type: JobTypeUploadScenes, Status: JobStatusCompleted, FileID: file.ID, CreatedAt: time.Now(), UpdatedAt: time.Now()})

	writeSceneResult(t, tmpDir, file.ID)

	runner.backfillCloudUploads(ctx)

	jobs, _ := repo.ListJobs(ctx, 100)
	uploadCount := 0
	for _, j := range jobs {
		if j.Type == JobTypeUploadScenes {
			uploadCount++
		}
	}
	if uploadCount != 1 {
		t.Errorf("expected 1 upload_scenes job (pre-existing), got %d", uploadCount)
	}
}

func TestProcessIndexJob_FacesParallelWithSpeech(t *testing.T) {
	facesStarted := make(chan struct{})
	speechDone := make(chan struct{})
	facesStartedBeforeSpeechDone := atomic.Bool{}

	fake := &fakePipeRunner{
		speechFn: func(ctx context.Context, videoPath, outPath string) (pipelines.RunResult, error) {
			time.Sleep(50 * time.Millisecond)
			close(speechDone)
			os.MkdirAll(filepath.Dir(outPath), 0755)
			os.WriteFile(outPath, []byte(`{"schema_version":"1.0","pipeline_version":"0.2.0","model_version":"whisper-large-v3"}`), 0644)
			return pipelines.RunResult{ExitCode: 0, OutputPath: outPath, Duration: 50 * time.Millisecond}, nil
		},
		facesFn: func(ctx context.Context, videoPath, outPath string) (pipelines.RunResult, error) {
			close(facesStarted)
			select {
			case <-speechDone:
			default:
				facesStartedBeforeSpeechDone.Store(true)
			}
			os.MkdirAll(filepath.Dir(outPath), 0755)
			os.WriteFile(outPath, []byte(`{"schema_version":"1.0","pipeline_version":"0.2.0","model_version":"scrfd"}`), 0644)
			return pipelines.RunResult{ExitCode: 0, OutputPath: outPath, Duration: 10 * time.Millisecond}, nil
		},
	}
	caps := &pipelines.Capabilities{
		HasSpeech: true,
		HasFaces:  true,
		HasScenes: true,
		ProbedAt:  time.Now(),
	}

	runner, repo := setupRunnerTest(t, fake, caps)
	runner.SetParallelFacesWithSpeech(true)
	job, _ := createTestJobAndFile(t, repo)

	runner.processIndexJob(context.Background(), job)

	updatedJob, _ := repo.GetJob(context.Background(), job.ID)
	if updatedJob.Status != JobStatusCompleted {
		t.Errorf("job status = %s, want %s", updatedJob.Status, JobStatusCompleted)
	}
	if !facesStartedBeforeSpeechDone.Load() {
		t.Error("faces should have started before speech completed when flag is true")
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

func TestProcessIndexJob_FacesParallelFlag_False_PreservesOrder(t *testing.T) {
	speechDone := make(chan struct{})
	facesStartedBeforeSpeechDone := atomic.Bool{}

	fake := &fakePipeRunner{
		speechFn: func(ctx context.Context, videoPath, outPath string) (pipelines.RunResult, error) {
			time.Sleep(50 * time.Millisecond)
			close(speechDone)
			os.MkdirAll(filepath.Dir(outPath), 0755)
			os.WriteFile(outPath, []byte(`{"schema_version":"1.0","pipeline_version":"0.2.0","model_version":"whisper-large-v3"}`), 0644)
			return pipelines.RunResult{ExitCode: 0, OutputPath: outPath, Duration: 50 * time.Millisecond}, nil
		},
		facesFn: func(ctx context.Context, videoPath, outPath string) (pipelines.RunResult, error) {
			select {
			case <-speechDone:
			default:
				facesStartedBeforeSpeechDone.Store(true)
			}
			os.MkdirAll(filepath.Dir(outPath), 0755)
			os.WriteFile(outPath, []byte(`{"schema_version":"1.0","pipeline_version":"0.2.0","model_version":"scrfd"}`), 0644)
			return pipelines.RunResult{ExitCode: 0, OutputPath: outPath, Duration: 10 * time.Millisecond}, nil
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
	if updatedJob.Status != JobStatusCompleted {
		t.Errorf("job status = %s, want %s", updatedJob.Status, JobStatusCompleted)
	}
	if facesStartedBeforeSpeechDone.Load() {
		t.Error("faces should NOT start before speech when flag is false")
	}
}

func TestProcessIndexJob_FacesParallel_SpeechFails_FacesStillCompletes(t *testing.T) {
	facesCompleted := atomic.Bool{}

	fake := &fakePipeRunner{
		speechFn: func(ctx context.Context, videoPath, outPath string) (pipelines.RunResult, error) {
			time.Sleep(20 * time.Millisecond)
			return pipelines.RunResult{ExitCode: 1, StderrTail: "speech failed"}, nil
		},
		facesFn: func(ctx context.Context, videoPath, outPath string) (pipelines.RunResult, error) {
			time.Sleep(50 * time.Millisecond)
			facesCompleted.Store(true)
			os.MkdirAll(filepath.Dir(outPath), 0755)
			os.WriteFile(outPath, []byte(`{"schema_version":"1.0","pipeline_version":"0.2.0","model_version":"scrfd"}`), 0644)
			return pipelines.RunResult{ExitCode: 0, OutputPath: outPath, Duration: 50 * time.Millisecond}, nil
		},
	}
	caps := &pipelines.Capabilities{
		HasSpeech: true,
		HasFaces:  true,
		HasScenes: true,
		ProbedAt:  time.Now(),
	}

	runner, repo := setupRunnerTest(t, fake, caps)
	runner.SetParallelFacesWithSpeech(true)
	job, _ := createTestJobAndFile(t, repo)

	done := make(chan struct{})
	go func() {
		runner.processIndexJob(context.Background(), job)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("processIndexJob did not complete")
	}

	updatedJob, _ := repo.GetJob(context.Background(), job.ID)
	if updatedJob.Status != JobStatusFailed {
		t.Errorf("job status = %s, want %s (speech failed)", updatedJob.Status, JobStatusFailed)
	}

	if fake.facesCalled.Load() != 1 {
		t.Errorf("faces called %d times, want 1 (should have been launched early)", fake.facesCalled.Load())
	}
	if fake.scenesCalled.Load() != 0 {
		t.Errorf("scenes called %d times, want 0 (speech failed)", fake.scenesCalled.Load())
	}
}

func TestBackfillCloudUploads_SkipsMissingArtifacts(t *testing.T) {
	fake := &fakePipeRunner{}
	caps := &pipelines.Capabilities{HasSpeech: true, HasScenes: true, ProbedAt: time.Now()}

	runner, repo := setupRunnerTest(t, fake, caps)

	tmpDir := t.TempDir()
	fake.artifacts = tmpDir

	cc := &fakeCloudClient{scenes: &fakeSceneUploader{}}
	runner.SetCloudClient(cc, "lib-1")

	_, file := createTestJobAndFile(t, repo)

	ctx := context.Background()

	repo.CreateJob(ctx, &Job{ID: NewID(), Type: JobTypeIndex, Status: JobStatusCompleted, FileID: file.ID, CreatedAt: time.Now(), UpdatedAt: time.Now()})

	runner.backfillCloudUploads(ctx)

	jobs, _ := repo.ListJobs(ctx, 100)
	uploadCount := 0
	for _, j := range jobs {
		if j.Type == JobTypeUploadScenes {
			uploadCount++
		}
	}
	if uploadCount != 0 {
		t.Errorf("expected 0 upload_scenes jobs (no artifacts), got %d", uploadCount)
	}
}
