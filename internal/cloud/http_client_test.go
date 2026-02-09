package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestHTTPClient_UploadScenes_Success(t *testing.T) {
	var receivedPayload SceneIngestPayload
	var receivedAuth string
	var receivedHost string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/ingest/scenes" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}

		receivedAuth = r.Header.Get("Authorization")
		receivedHost = r.Host

		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedPayload)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(SceneIngestResponse{
			IndexedCount: 2,
			VideoID:      "vid123",
		})
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, "test-token", "devorg", testLogger())

	payload := SceneIngestPayload{
		VideoID:   "vid123",
		LibraryID: "lib-uuid-1",
		Scenes: []SceneIngestDoc{
			{SceneID: "vid123_scene_0", Index: 0, StartMs: 0, EndMs: 5000},
			{SceneID: "vid123_scene_1", Index: 1, StartMs: 5000, EndMs: 10000},
		},
	}

	err := client.UploadScenes(context.Background(), payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedAuth != "Bearer test-token" {
		t.Errorf("auth = %q, want %q", receivedAuth, "Bearer test-token")
	}

	if receivedHost != "devorg.app.heimdex.local" {
		t.Errorf("host = %q, want %q", receivedHost, "devorg.app.heimdex.local")
	}

	if receivedPayload.VideoID != "vid123" {
		t.Errorf("video_id = %q, want %q", receivedPayload.VideoID, "vid123")
	}

	if len(receivedPayload.Scenes) != 2 {
		t.Errorf("scenes count = %d, want 2", len(receivedPayload.Scenes))
	}
}

func TestHTTPClient_UploadScenes_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"detail":"internal server error"}`))
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, "test-token", "devorg", testLogger())

	err := client.UploadScenes(context.Background(), SceneIngestPayload{
		VideoID: "vid1",
		Scenes:  []SceneIngestDoc{{SceneID: "vid1_scene_0", Index: 0, StartMs: 0, EndMs: 1000}},
	})

	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestUploadError_IsRetryable(t *testing.T) {
	if !(&UploadError{StatusCode: http.StatusInternalServerError}).IsRetryable() {
		t.Fatal("expected 5xx upload error to be retryable")
	}
	if (&UploadError{StatusCode: http.StatusBadRequest}).IsRetryable() {
		t.Fatal("expected 4xx upload error to be permanent")
	}
}

func TestHTTPClient_UploadScenes_Returns_UploadError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"detail":"invalid library"}`))
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, "test-token", "devorg", testLogger())

	err := client.UploadScenes(context.Background(), SceneIngestPayload{
		VideoID: "vid1",
		Scenes:  []SceneIngestDoc{{SceneID: "vid1_scene_0", Index: 0, StartMs: 0, EndMs: 1000}},
	})

	if err == nil {
		t.Fatal("expected error for 400 response")
	}

	var uploadErr *UploadError
	if !errors.As(err, &uploadErr) {
		t.Fatalf("expected UploadError, got %T", err)
	}
	if uploadErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("status_code = %d, want %d", uploadErr.StatusCode, http.StatusBadRequest)
	}
	if !strings.Contains(uploadErr.Body, "invalid library") {
		t.Fatalf("body = %q, want to contain invalid library", uploadErr.Body)
	}
}

func TestHTTPClient_SetDeviceID(t *testing.T) {
	var receivedDeviceID string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedDeviceID = r.Header.Get("X-Heimdex-Device-Id")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(SceneIngestResponse{IndexedCount: 1, VideoID: "vid1"})
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, "test-token", "devorg", testLogger())
	client.SetDeviceID("device-123")

	err := client.UploadScenes(context.Background(), SceneIngestPayload{
		VideoID: "vid1",
		Scenes:  []SceneIngestDoc{{SceneID: "vid1_scene_0", Index: 0, StartMs: 0, EndMs: 1000}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedDeviceID != "device-123" {
		t.Fatalf("device_id_header = %q, want %q", receivedDeviceID, "device-123")
	}
}

func TestHTTPClient_UploadScenes_SendsCorrelationHeaders(t *testing.T) {
	var requestID string
	var deviceID string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID = r.Header.Get("X-Heimdex-Request-Id")
		deviceID = r.Header.Get("X-Heimdex-Device-Id")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(SceneIngestResponse{IndexedCount: 1, VideoID: "vid1"})
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, "test-token", "devorg", testLogger())
	client.SetDeviceID("device-xyz")

	err := client.UploadScenes(context.Background(), SceneIngestPayload{
		VideoID: "vid1",
		Scenes:  []SceneIngestDoc{{SceneID: "vid1_scene_0", Index: 0, StartMs: 0, EndMs: 1000}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if requestID == "" {
		t.Fatal("expected X-Heimdex-Request-Id header")
	}
	if deviceID != "device-xyz" {
		t.Fatalf("device_id_header = %q, want %q", deviceID, "device-xyz")
	}
}

func TestHTTPClient_UploadScenes_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"detail":"Invalid agent API key"}`))
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, "wrong-token", "devorg", testLogger())

	err := client.UploadScenes(context.Background(), SceneIngestPayload{
		VideoID: "vid1",
		Scenes:  []SceneIngestDoc{{SceneID: "vid1_scene_0", Index: 0, StartMs: 0, EndMs: 1000}},
	})

	if err == nil {
		t.Fatal("expected error for 401 response")
	}
}

func TestHTTPClient_UploadScenes_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(SceneIngestResponse{IndexedCount: 1, VideoID: "vid1"})
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, "test-token", "devorg", testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := client.UploadScenes(ctx, SceneIngestPayload{
		VideoID: "vid1",
		Scenes:  []SceneIngestDoc{{SceneID: "vid1_scene_0", Index: 0, StartMs: 0, EndMs: 1000}},
	})

	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestHTTPClient_ImplementsClientInterface(t *testing.T) {
	var _ Client = (*HTTPClient)(nil)
}

func TestStubClient_ImplementsClientInterface(t *testing.T) {
	var _ Client = (*StubClient)(nil)
}

func TestStubSceneUploader_NoOp(t *testing.T) {
	stub := &StubSceneUploader{logger: testLogger()}
	err := stub.UploadScenes(context.Background(), SceneIngestPayload{
		VideoID: "vid1",
		Scenes:  []SceneIngestDoc{{SceneID: "vid1_scene_0", Index: 0, StartMs: 0, EndMs: 1000}},
	})
	if err != nil {
		t.Fatalf("stub should not error: %v", err)
	}
}

func TestHTTPClient_EmptyOrgSlug(t *testing.T) {
	var receivedHost string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHost = r.Host
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(SceneIngestResponse{IndexedCount: 0, VideoID: "vid1"})
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, "test-token", "", testLogger())

	err := client.UploadScenes(context.Background(), SceneIngestPayload{
		VideoID: "vid1",
		Scenes:  []SceneIngestDoc{},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With empty org slug, Host should not be overridden (uses server's default)
	if receivedHost == ".app.heimdex.local" {
		t.Error("host should not have empty slug prefix")
	}
}
