package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/heimdex/heimdex-agent/internal/catalog"
	"github.com/heimdex/heimdex-agent/internal/pipelines"
)

func TestStatusHandler_NilDoctor(t *testing.T) {
	cfg := testStatusConfig(nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/status", nil)

	statusHandler(cfg).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rr.Code, http.StatusOK)
	}

	body := decodeJSONBody(t, rr)
	if _, ok := body["pipelines"]; ok {
		t.Fatal("pipelines should be omitted when doctor is nil")
	}

	constraints, ok := body["constraints"].(map[string]interface{})
	if !ok {
		t.Fatal("constraints missing from response")
	}
	if got, ok := constraints["scenes_requires_speech"].(bool); !ok || !got {
		t.Fatalf("constraints.scenes_requires_speech = %v, want true", constraints["scenes_requires_speech"])
	}
}

func TestStatusHandler_EmptyCache(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	doctor := pipelines.NewCachedDoctor(&fakeDoctorPipelineRunner{}, logger)
	cfg := testStatusConfig(doctor)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/status", nil)

	statusHandler(cfg).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rr.Code, http.StatusOK)
	}

	body := decodeJSONBody(t, rr)
	if _, ok := body["pipelines"]; ok {
		t.Fatal("pipelines should be omitted when cache is empty")
	}
}

func TestStatusHandler_WithCachedCaps(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	doctor := pipelines.NewCachedDoctor(&fakeDoctorPipelineRunner{
		caps: &pipelines.Capabilities{
			HasFaces:  true,
			HasSpeech: true,
			HasScenes: true,
			ProbedAt:  time.Now(),
			Summary:   pipelines.SummaryInfo{Available: 4, Total: 6},
		},
	}, logger)

	if _, err := doctor.Refresh(context.Background()); err != nil {
		t.Fatalf("doctor.Refresh() error = %v", err)
	}

	cfg := testStatusConfig(doctor)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/status", nil)

	statusHandler(cfg).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rr.Code, http.StatusOK)
	}

	body := decodeJSONBody(t, rr)
	pipelinesMap, ok := body["pipelines"].(map[string]interface{})
	if !ok {
		t.Fatal("pipelines missing from response")
	}

	if got, ok := pipelinesMap["has_scenes"].(bool); !ok || !got {
		t.Fatalf("pipelines.has_scenes = %v, want true", pipelinesMap["has_scenes"])
	}
}

func TestStatusHandler_ScenesRequiresSpeech(t *testing.T) {
	cfg := testStatusConfig(nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/status", nil)

	statusHandler(cfg).ServeHTTP(rr, req)

	body := decodeJSONBody(t, rr)
	constraints, ok := body["constraints"].(map[string]interface{})
	if !ok {
		t.Fatal("constraints missing from response")
	}

	if got, ok := constraints["scenes_requires_speech"].(bool); !ok || !got {
		t.Fatalf("constraints.scenes_requires_speech = %v, want true", constraints["scenes_requires_speech"])
	}
}

func TestStatusHandler_ZeroProbedAt(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	doctor := pipelines.NewCachedDoctor(&fakeDoctorPipelineRunner{
		caps: &pipelines.Capabilities{
			HasFaces:  true,
			HasSpeech: true,
			HasScenes: true,
			Summary:   pipelines.SummaryInfo{Available: 3, Total: 5},
		},
	}, logger)

	if _, err := doctor.Refresh(context.Background()); err != nil {
		t.Fatalf("doctor.Refresh() error = %v", err)
	}

	cfg := testStatusConfig(doctor)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/status", nil)

	statusHandler(cfg).ServeHTTP(rr, req)

	body := decodeJSONBody(t, rr)
	pipelinesMap, ok := body["pipelines"].(map[string]interface{})
	if !ok {
		t.Fatal("pipelines missing from response")
	}

	if _, ok := pipelinesMap["last_probe_at"]; ok {
		t.Fatal("last_probe_at should be omitted when ProbedAt is zero")
	}
}

func testStatusConfig(doctor *pipelines.CachedDoctor) ServerConfig {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	return ServerConfig{
		CatalogService: &fakeService{},
		Repository:     &fakeRepo{},
		Doctor:         doctor,
		Logger:         logger,
		StartTime:      time.Now().Add(-10 * time.Second),
		DeviceID:       "test-device",
	}
}

func decodeJSONBody(t *testing.T, rr *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()

	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	return body
}

type fakeService struct{}

func (f *fakeService) AddFolder(ctx context.Context, path, displayName string) (*catalog.Source, error) {
	return nil, nil
}

func (f *fakeService) RemoveSource(ctx context.Context, id string) error {
	return nil
}

func (f *fakeService) GetSources(ctx context.Context) ([]*catalog.Source, error) {
	return []*catalog.Source{}, nil
}

func (f *fakeService) GetSource(ctx context.Context, id string) (*catalog.Source, error) {
	return nil, nil
}

func (f *fakeService) GetFiles(ctx context.Context, sourceID string) ([]*catalog.File, error) {
	return []*catalog.File{}, nil
}

func (f *fakeService) GetFile(ctx context.Context, id string) (*catalog.File, error) {
	return nil, nil
}

func (f *fakeService) CountFiles(ctx context.Context) (int, error) {
	return 0, nil
}

func (f *fakeService) ScanSource(ctx context.Context, sourceID string) (*catalog.Job, error) {
	return nil, nil
}

func (f *fakeService) ExecuteScan(ctx context.Context, jobID, sourceID, path string) error {
	return nil
}

type fakeRepo struct{}

func (f *fakeRepo) CreateSource(ctx context.Context, source *catalog.Source) error {
	return nil
}

func (f *fakeRepo) GetSource(ctx context.Context, id string) (*catalog.Source, error) {
	return nil, nil
}

func (f *fakeRepo) GetSourceByPath(ctx context.Context, path string) (*catalog.Source, error) {
	return nil, nil
}

func (f *fakeRepo) ListSources(ctx context.Context) ([]*catalog.Source, error) {
	return []*catalog.Source{}, nil
}

func (f *fakeRepo) DeleteSource(ctx context.Context, id string) error {
	return nil
}

func (f *fakeRepo) UpdateSourcePresent(ctx context.Context, id string, present bool) error {
	return nil
}

func (f *fakeRepo) CreateFile(ctx context.Context, file *catalog.File) error {
	return nil
}

func (f *fakeRepo) GetFile(ctx context.Context, id string) (*catalog.File, error) {
	return nil, nil
}

func (f *fakeRepo) GetFilesBySource(ctx context.Context, sourceID string) ([]*catalog.File, error) {
	return []*catalog.File{}, nil
}

func (f *fakeRepo) DeleteFilesBySource(ctx context.Context, sourceID string) error {
	return nil
}

func (f *fakeRepo) UpsertFile(ctx context.Context, file *catalog.File) error {
	return nil
}

func (f *fakeRepo) CountFiles(ctx context.Context) (int, error) {
	return 0, nil
}

func (f *fakeRepo) CreateJob(ctx context.Context, job *catalog.Job) error {
	return nil
}

func (f *fakeRepo) GetJob(ctx context.Context, id string) (*catalog.Job, error) {
	return nil, nil
}

func (f *fakeRepo) ListJobs(ctx context.Context, limit int) ([]*catalog.Job, error) {
	return []*catalog.Job{}, nil
}

func (f *fakeRepo) ListPendingJobs(ctx context.Context) ([]*catalog.Job, error) {
	return []*catalog.Job{}, nil
}

func (f *fakeRepo) UpdateJobStatus(ctx context.Context, id, status, errorMsg string) error {
	return nil
}

func (f *fakeRepo) UpdateJobProgress(ctx context.Context, id string, progress int) error {
	return nil
}

func (f *fakeRepo) GetConfig(ctx context.Context, key string) (string, error) {
	return "", nil
}

func (f *fakeRepo) SetConfig(ctx context.Context, key, value string) error {
	return nil
}

type fakeDoctorPipelineRunner struct {
	caps *pipelines.Capabilities
}

func (f *fakeDoctorPipelineRunner) RunDoctor(ctx context.Context) (*pipelines.Capabilities, error) {
	if f.caps == nil {
		return &pipelines.Capabilities{}, nil
	}
	return f.caps, nil
}

func (f *fakeDoctorPipelineRunner) RunSpeech(ctx context.Context, videoPath, outPath string) (pipelines.RunResult, error) {
	return pipelines.RunResult{}, nil
}

func (f *fakeDoctorPipelineRunner) RunFaces(ctx context.Context, videoPath, outPath string) (pipelines.RunResult, error) {
	return pipelines.RunResult{}, nil
}

func (f *fakeDoctorPipelineRunner) RunScenes(ctx context.Context, videoPath, videoID, speechResultPath, outPath string) (pipelines.RunResult, error) {
	return pipelines.RunResult{}, nil
}

func (f *fakeDoctorPipelineRunner) ValidateOutput(path string) (*pipelines.PipelineOutput, error) {
	return &pipelines.PipelineOutput{SchemaVersion: "1.0", PipelineVersion: "0.1.0", ModelVersion: "test"}, nil
}

func (f *fakeDoctorPipelineRunner) ValidateSceneOutput(path string) (*pipelines.PipelineOutput, error) {
	return f.ValidateOutput(path)
}

func (f *fakeDoctorPipelineRunner) ArtifactsDir() string {
	return "/tmp/test-artifacts"
}
