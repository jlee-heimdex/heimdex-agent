package pipelines

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunResult_IsSuccess(t *testing.T) {
	tests := []struct {
		exitCode int
		want     bool
	}{
		{0, true},
		{1, false},
		{-1, false},
		{127, false},
	}
	for _, tt := range tests {
		r := RunResult{ExitCode: tt.exitCode}
		if got := r.IsSuccess(); got != tt.want {
			t.Errorf("RunResult{ExitCode: %d}.IsSuccess() = %v, want %v", tt.exitCode, got, tt.want)
		}
	}
}

func TestPipelineOutput_RequiredFieldsPresent(t *testing.T) {
	tests := []struct {
		name string
		out  PipelineOutput
		want bool
	}{
		{"all present", PipelineOutput{"1.0", "0.1.0", "scrfd"}, true},
		{"missing schema", PipelineOutput{"", "0.1.0", "scrfd"}, false},
		{"missing pipeline", PipelineOutput{"1.0", "", "scrfd"}, false},
		{"missing model", PipelineOutput{"1.0", "0.1.0", ""}, false},
		{"all empty", PipelineOutput{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.out.RequiredFieldsPresent(); got != tt.want {
				t.Errorf("RequiredFieldsPresent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLimitedWriter_KeepsOnlyTail(t *testing.T) {
	var buf bytes.Buffer
	lw := &limitedWriter{w: &buf, limit: 10}

	lw.Write([]byte("hello"))
	if buf.String() != "hello" {
		t.Errorf("after short write got %q, want %q", buf.String(), "hello")
	}

	lw.Write([]byte(" world of test data"))
	got := buf.String()
	if len(got) > 10 {
		t.Errorf("buffer length %d exceeds limit 10", len(got))
	}

	want := " test data"
	if got != want {
		t.Errorf("after overflow got %q, want %q", got, want)
	}
}

func TestLimitedWriter_ExactLimit(t *testing.T) {
	var buf bytes.Buffer
	lw := &limitedWriter{w: &buf, limit: 5}

	n, err := lw.Write([]byte("12345"))
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != 5 {
		t.Errorf("Write returned %d, want 5", n)
	}
	if buf.String() != "12345" {
		t.Errorf("got %q, want %q", buf.String(), "12345")
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 5, "...world"},
	}
	for _, tt := range tests {
		got := truncate(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}

func TestResolvePython_PreferredNotFound(t *testing.T) {
	_, err := resolvePython("/nonexistent/python999")
	if err == nil {
		t.Fatal("expected error for nonexistent python")
	}
}

func TestResolvePython_AutoDetect(t *testing.T) {
	p, err := resolvePython("")
	if err != nil {
		t.Skipf("no python on PATH: %v", err)
	}
	if p == "" {
		t.Error("resolved python path is empty")
	}
}

func TestIsAvailable(t *testing.T) {
	deps := map[string]DepInfo{
		"cv2":     {Available: true, Version: "4.13"},
		"whisper": {Available: false, Error: "not installed"},
	}

	if !isAvailable(deps, "cv2") {
		t.Error("cv2 should be available")
	}
	if isAvailable(deps, "whisper") {
		t.Error("whisper should not be available")
	}
	if isAvailable(deps, "nonexistent") {
		t.Error("nonexistent should not be available")
	}
}

func TestValidateOutput_ValidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "result.json")

	data := PipelineOutput{
		SchemaVersion:   "1.0",
		PipelineVersion: "0.1.0",
		ModelVersion:    "scrfd",
	}
	b, _ := json.Marshal(data)
	os.WriteFile(path, b, 0644)

	cfg := DefaultConfig(dir, nil)
	cfg.Logger = nil
	r := &SubprocessRunner{cfg: cfg, python: "python3"}

	out, err := r.ValidateOutput(path)
	if err != nil {
		t.Fatalf("ValidateOutput error: %v", err)
	}
	if !out.RequiredFieldsPresent() {
		t.Error("expected all required fields present")
	}
}

func TestValidateOutput_MissingFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "result.json")

	data := map[string]string{"schema_version": "1.0"}
	b, _ := json.Marshal(data)
	os.WriteFile(path, b, 0644)

	cfg := DefaultConfig(dir, nil)
	cfg.Logger = nil
	r := &SubprocessRunner{cfg: cfg, python: "python3"}

	_, err := r.ValidateOutput(path)
	if err == nil {
		t.Fatal("expected error for missing fields")
	}
}

func TestValidateOutput_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig(dir, nil)
	cfg.Logger = nil
	r := &SubprocessRunner{cfg: cfg, python: "python3"}

	_, err := r.ValidateOutput(filepath.Join(dir, "nonexistent.json"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestValidateSceneOutput_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "scene.json")
	data := `{
		"schema_version":"1.0",
		"pipeline_version":"0.2.0",
		"model_version":"ffmpeg-scenecut",
		"video_id":"video-1",
		"scenes":[
			{"scene_id":"video-1_scene_0","start_ms":0,"end_ms":1000}
		]
	}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("write scene output: %v", err)
	}

	cfg := DefaultConfig(dir, nil)
	r := &SubprocessRunner{cfg: cfg, python: "python3"}

	out, err := r.ValidateSceneOutput(path)
	if err != nil {
		t.Fatalf("ValidateSceneOutput error: %v", err)
	}
	if !out.RequiredFieldsPresent() {
		t.Error("expected required fields present")
	}
}

func TestValidateSceneOutput_EmptyScenes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "scene.json")
	data := `{
		"schema_version":"1.0",
		"pipeline_version":"0.2.0",
		"model_version":"ffmpeg-scenecut",
		"video_id":"video-1",
		"scenes":[]
	}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("write scene output: %v", err)
	}

	cfg := DefaultConfig(dir, nil)
	r := &SubprocessRunner{cfg: cfg, python: "python3"}

	if _, err := r.ValidateSceneOutput(path); err != nil {
		t.Fatalf("ValidateSceneOutput error: %v", err)
	}
}

func TestValidateSceneOutput_MissingVideoID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "scene.json")
	data := `{
		"schema_version":"1.0",
		"pipeline_version":"0.2.0",
		"model_version":"ffmpeg-scenecut",
		"scenes":[]
	}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("write scene output: %v", err)
	}

	cfg := DefaultConfig(dir, nil)
	r := &SubprocessRunner{cfg: cfg, python: "python3"}

	_, err := r.ValidateSceneOutput(path)
	if err == nil || !strings.Contains(err.Error(), "video_id is required") {
		t.Fatalf("expected missing video_id error, got: %v", err)
	}
}

func TestValidateSceneOutput_NegativeTimestamp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "scene.json")
	data := `{
		"schema_version":"1.0",
		"pipeline_version":"0.2.0",
		"model_version":"ffmpeg-scenecut",
		"video_id":"video-1",
		"scenes":[{"scene_id":"video-1_scene_0","start_ms":-1,"end_ms":1000}]
	}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("write scene output: %v", err)
	}

	cfg := DefaultConfig(dir, nil)
	r := &SubprocessRunner{cfg: cfg, python: "python3"}

	_, err := r.ValidateSceneOutput(path)
	if err == nil || !strings.Contains(err.Error(), "start_ms must be >= 0") {
		t.Fatalf("expected negative timestamp error, got: %v", err)
	}
}

func TestValidateSceneOutput_EndBeforeStart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "scene.json")
	data := `{
		"schema_version":"1.0",
		"pipeline_version":"0.2.0",
		"model_version":"ffmpeg-scenecut",
		"video_id":"video-1",
		"scenes":[{"scene_id":"video-1_scene_0","start_ms":1000,"end_ms":1000}]
	}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("write scene output: %v", err)
	}

	cfg := DefaultConfig(dir, nil)
	r := &SubprocessRunner{cfg: cfg, python: "python3"}

	_, err := r.ValidateSceneOutput(path)
	if err == nil || !strings.Contains(err.Error(), "end_ms must be greater than start_ms") {
		t.Fatalf("expected end before start error, got: %v", err)
	}
}

func TestValidateSceneOutput_NonMonotonic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "scene.json")
	data := `{
		"schema_version":"1.0",
		"pipeline_version":"0.2.0",
		"model_version":"ffmpeg-scenecut",
		"video_id":"video-1",
		"scenes":[
			{"scene_id":"video-1_scene_0","start_ms":0,"end_ms":1000},
			{"scene_id":"video-1_scene_1","start_ms":500,"end_ms":1500}
		]
	}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("write scene output: %v", err)
	}

	cfg := DefaultConfig(dir, nil)
	r := &SubprocessRunner{cfg: cfg, python: "python3"}

	_, err := r.ValidateSceneOutput(path)
	if err == nil || !strings.Contains(err.Error(), "start_ms must be >= previous scene end_ms") {
		t.Fatalf("expected non-monotonic error, got: %v", err)
	}
}

func TestValidateSceneOutput_InvalidSceneID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "scene.json")
	data := `{
		"schema_version":"1.0",
		"pipeline_version":"0.2.0",
		"model_version":"ffmpeg-scenecut",
		"video_id":"video-1",
		"scenes":[{"scene_id":"video-1","start_ms":0,"end_ms":1000}]
	}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("write scene output: %v", err)
	}

	cfg := DefaultConfig(dir, nil)
	r := &SubprocessRunner{cfg: cfg, python: "python3"}

	_, err := r.ValidateSceneOutput(path)
	if err == nil || !strings.Contains(err.Error(), "scene_id must match") {
		t.Fatalf("expected invalid scene_id error, got: %v", err)
	}
}

func TestValidateSceneOutput_MissingMetaFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "scene.json")
	data := `{
		"schema_version":"",
		"pipeline_version":"0.2.0",
		"model_version":"ffmpeg-scenecut",
		"video_id":"video-1",
		"scenes":[]
	}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("write scene output: %v", err)
	}

	cfg := DefaultConfig(dir, nil)
	r := &SubprocessRunner{cfg: cfg, python: "python3"}

	_, err := r.ValidateSceneOutput(path)
	if err == nil || !strings.Contains(err.Error(), "schema_version is required") {
		t.Fatalf("expected missing meta field error, got: %v", err)
	}
}

func TestCachedDoctor_TTL(t *testing.T) {
	calls := 0
	fake := &fakeRunner{
		doctorFn: func(ctx context.Context) (*Capabilities, error) {
			calls++
			return &Capabilities{
				HasFaces:  true,
				HasSpeech: false,
				ProbedAt:  time.Now(),
				Summary:   SummaryInfo{Available: 5, Total: 9},
			}, nil
		},
	}

	doc := NewCachedDoctor(fake, nil)
	doc.ttl = 100 * time.Millisecond
	ctx := context.Background()

	caps1, err := doc.Get(ctx)
	if err != nil {
		t.Fatalf("first Get: %v", err)
	}
	if !caps1.HasFaces {
		t.Error("expected HasFaces=true")
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}

	caps2, err := doc.Get(ctx)
	if err != nil {
		t.Fatalf("second Get: %v", err)
	}
	if caps2.ProbedAt != caps1.ProbedAt {
		t.Error("expected cached result on second call")
	}
	if calls != 1 {
		t.Errorf("expected 1 call (cached), got %d", calls)
	}

	time.Sleep(150 * time.Millisecond)

	_, err = doc.Get(ctx)
	if err != nil {
		t.Fatalf("third Get (after TTL): %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls after TTL expiry, got %d", calls)
	}
}

func TestCachedDoctor_Invalidate(t *testing.T) {
	calls := 0
	fake := &fakeRunner{
		doctorFn: func(ctx context.Context) (*Capabilities, error) {
			calls++
			return &Capabilities{ProbedAt: time.Now()}, nil
		},
	}

	doc := NewCachedDoctor(fake, nil)
	ctx := context.Background()

	doc.Get(ctx)
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}

	doc.Invalidate()
	doc.Get(ctx)
	if calls != 2 {
		t.Errorf("expected 2 calls after Invalidate, got %d", calls)
	}
}

func TestSafePath_DebugMode(t *testing.T) {
	r := &SubprocessRunner{
		cfg: Config{DebugPaths: true},
	}
	path := "/Users/test/secret/file.json"
	if got := r.safePath(path); got != path {
		t.Errorf("debug mode: safePath(%q) = %q, want full path", path, got)
	}
}

func TestSafePath_ProductionMode(t *testing.T) {
	r := &SubprocessRunner{
		cfg: Config{DebugPaths: false},
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}
	path := filepath.Join(home, ".heimdex", "artifacts", "result.json")
	got := r.safePath(path)
	if got == path {
		t.Errorf("production mode should sanitise path, got full path: %q", got)
	}
	if got != "~/.heimdex/artifacts/result.json" {
		t.Errorf("safePath() = %q, want %q", got, "~/.heimdex/artifacts/result.json")
	}
}

func TestParseDoctorJSON_WithPipelines(t *testing.T) {
	raw := []byte(`{
		"dependencies": {
			"cv2": {"available": true},
			"insightface": {"available": true},
			"whisper": {"available": false}
		},
		"executables": {
			"ffmpeg": {"available": false}
		},
		"pipelines": {
			"speech": true,
			"faces": false,
			"scenes": true
		}
	}`)

	var caps Capabilities
	if err := json.Unmarshal(raw, &caps); err != nil {
		t.Fatalf("unmarshal doctor JSON: %v", err)
	}

	DeriveCapabilities(&caps)

	if !caps.HasSpeech {
		t.Error("HasSpeech = false, want true from pipelines")
	}
	if caps.HasFaces {
		t.Error("HasFaces = true, want false from pipelines")
	}
	if !caps.HasScenes {
		t.Error("HasScenes = false, want true from pipelines")
	}
	if caps.ProbedAt.IsZero() {
		t.Error("ProbedAt is zero, want non-zero")
	}
}

func TestParseDoctorJSON_LegacyFallback(t *testing.T) {
	raw := []byte(`{
		"dependencies": {
			"cv2": {"available": true},
			"insightface": {"available": true},
			"whisper": {"available": true}
		},
		"executables": {
			"ffmpeg": {"available": true}
		}
	}`)

	var caps Capabilities
	if err := json.Unmarshal(raw, &caps); err != nil {
		t.Fatalf("unmarshal doctor JSON: %v", err)
	}

	DeriveCapabilities(&caps)

	if !caps.HasFaces {
		t.Error("HasFaces = false, want true from legacy inference")
	}
	if !caps.HasSpeech {
		t.Error("HasSpeech = false, want true from legacy inference")
	}
	if !caps.HasScenes {
		t.Error("HasScenes = false, want true from legacy inference")
	}
	if caps.ProbedAt.IsZero() {
		t.Error("ProbedAt is zero, want non-zero")
	}
}

func TestParseDoctorJSON_ScenesWithoutSpeech(t *testing.T) {
	raw := []byte(`{
		"dependencies": {
			"whisper": {"available": true}
		},
		"executables": {
			"ffmpeg": {"available": true}
		},
		"pipelines": {
			"speech": false,
			"faces": true,
			"scenes": true
		}
	}`)

	var caps Capabilities
	if err := json.Unmarshal(raw, &caps); err != nil {
		t.Fatalf("unmarshal doctor JSON: %v", err)
	}

	DeriveCapabilities(&caps)

	if caps.HasSpeech {
		t.Error("HasSpeech = true, want false from pipelines")
	}
	if !caps.HasScenes {
		t.Error("HasScenes = false, want true from pipelines")
	}
}

type fakeRunner struct {
	doctorFn        func(ctx context.Context) (*Capabilities, error)
	speechFn        func(ctx context.Context, videoPath, outPath string) (RunResult, error)
	facesFn         func(ctx context.Context, videoPath, outPath string) (RunResult, error)
	scenesFn        func(ctx context.Context, videoPath, speechResultPath, outPath string) (RunResult, error)
	validateFn      func(path string) (*PipelineOutput, error)
	validateSceneFn func(path string) (*PipelineOutput, error)
}

func (f *fakeRunner) RunDoctor(ctx context.Context) (*Capabilities, error) {
	return f.doctorFn(ctx)
}

func (f *fakeRunner) RunSpeech(ctx context.Context, videoPath, outPath string) (RunResult, error) {
	if f.speechFn != nil {
		return f.speechFn(ctx, videoPath, outPath)
	}
	return RunResult{ExitCode: 0, OutputPath: outPath}, nil
}

func (f *fakeRunner) RunFaces(ctx context.Context, videoPath, outPath string) (RunResult, error) {
	if f.facesFn != nil {
		return f.facesFn(ctx, videoPath, outPath)
	}
	return RunResult{ExitCode: 0, OutputPath: outPath}, nil
}

func (f *fakeRunner) RunScenes(ctx context.Context, videoPath, speechResultPath, outPath string) (RunResult, error) {
	if f.scenesFn != nil {
		return f.scenesFn(ctx, videoPath, speechResultPath, outPath)
	}
	return RunResult{ExitCode: 0, OutputPath: outPath}, nil
}

func (f *fakeRunner) ValidateOutput(path string) (*PipelineOutput, error) {
	if f.validateFn != nil {
		return f.validateFn(path)
	}
	return &PipelineOutput{SchemaVersion: "1.0", PipelineVersion: "0.1.0", ModelVersion: "test"}, nil
}

func (f *fakeRunner) ValidateSceneOutput(path string) (*PipelineOutput, error) {
	if f.validateSceneFn != nil {
		return f.validateSceneFn(path)
	}
	return f.ValidateOutput(path)
}

func (f *fakeRunner) ArtifactsDir() string {
	return "/tmp/artifacts"
}
