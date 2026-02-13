package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/heimdex/heimdex-agent/internal/catalog"
	exportpkg "github.com/heimdex/heimdex-agent/internal/export"
)

type fakeServiceForExport struct {
	fakeService
	files map[string]*catalog.File
}

func (f *fakeServiceForExport) GetFile(ctx context.Context, id string) (*catalog.File, error) {
	if file, ok := f.files[id]; ok {
		return file, nil
	}
	return nil, nil
}

func exportTestConfig(svc catalog.CatalogService) ServerConfig {
	return ServerConfig{
		CatalogService: svc,
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		StartTime:      time.Now(),
		DeviceID:       "test-device",
	}
}

func newExportRequest(t *testing.T, req exportpkg.ExportRequest) *http.Request {
	t.Helper()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}
	httpReq := httptest.NewRequest(http.MethodPost, "/export/premiere", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	return httpReq
}

func TestExportPremiere_HappyPath(t *testing.T) {
	outDir := t.TempDir()
	svc := &fakeServiceForExport{files: map[string]*catalog.File{
		"v1": {ID: "v1", Path: "/media/alpha.mp4"},
	}}
	cfg := exportTestConfig(svc)

	req := newExportRequest(t, exportpkg.ExportRequest{
		ProjectName: "Project One",
		Format:      "edl",
		FrameRate:   30,
		OutputDir:   outDir,
		Clips: []exportpkg.ClipInput{{
			VideoID:  "v1",
			SceneID:  "scene-1",
			ClipName: "Intro",
			StartMs:  0,
			EndMs:    2000,
		}},
	})
	rr := httptest.NewRecorder()

	exportPremiereHandler(cfg).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp exportpkg.ExportResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response unmarshal error: %v", err)
	}
	if resp.ClipCount != 1 {
		t.Fatalf("clip_count = %d, want 1", resp.ClipCount)
	}

	content, err := os.ReadFile(resp.OutputPath)
	if err != nil {
		t.Fatalf("failed reading output EDL: %v", err)
	}
	if !bytes.Contains(content, []byte("* MEDIA PATH:  /media/alpha.mp4")) {
		t.Fatalf("written EDL missing media path: %q", string(content))
	}
}

func TestExportPremiere_PartialResolution(t *testing.T) {
	outDir := t.TempDir()
	svc := &fakeServiceForExport{files: map[string]*catalog.File{
		"v1": {ID: "v1", Path: "/media/ok.mp4"},
	}}
	cfg := exportTestConfig(svc)

	req := newExportRequest(t, exportpkg.ExportRequest{
		ProjectName: "Partial",
		Format:      "edl",
		OutputDir:   outDir,
		Clips: []exportpkg.ClipInput{
			{VideoID: "v1", ClipName: "One", StartMs: 0, EndMs: 1000},
			{VideoID: "missing", ClipName: "Two", StartMs: 0, EndMs: 1000},
		},
	})
	rr := httptest.NewRecorder()

	exportPremiereHandler(cfg).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp exportpkg.ExportResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response unmarshal error: %v", err)
	}
	if len(resp.UnresolvedClips) != 1 || resp.UnresolvedClips[0] != "missing" {
		t.Fatalf("unresolved_clips = %v, want [missing]", resp.UnresolvedClips)
	}
	if resp.ClipCount != 1 {
		t.Fatalf("clip_count = %d, want 1", resp.ClipCount)
	}
}

func TestExportPremiere_AllUnresolved(t *testing.T) {
	cfg := exportTestConfig(&fakeServiceForExport{files: map[string]*catalog.File{}})
	req := newExportRequest(t, exportpkg.ExportRequest{
		ProjectName: "None",
		Format:      "edl",
		OutputDir:   t.TempDir(),
		Clips:       []exportpkg.ClipInput{{VideoID: "missing", StartMs: 0, EndMs: 1000}},
	})
	rr := httptest.NewRecorder()

	exportPremiereHandler(cfg).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnprocessableEntity)
	}
}

func TestExportPremiere_InvalidFormat(t *testing.T) {
	cfg := exportTestConfig(&fakeServiceForExport{files: map[string]*catalog.File{}})
	req := newExportRequest(t, exportpkg.ExportRequest{
		ProjectName: "Bad",
		Format:      "fcpxml",
		OutputDir:   t.TempDir(),
		Clips:       []exportpkg.ClipInput{{VideoID: "v1", StartMs: 0, EndMs: 1000}},
	})
	rr := httptest.NewRecorder()

	exportPremiereHandler(cfg).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestExportPremiere_InvalidOutputDir(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	cfg := exportTestConfig(&fakeServiceForExport{files: map[string]*catalog.File{}})
	req := newExportRequest(t, exportpkg.ExportRequest{
		ProjectName: "BadDir",
		Format:      "edl",
		OutputDir:   missing,
		Clips:       []exportpkg.ClipInput{{VideoID: "v1", StartMs: 0, EndMs: 1000}},
	})
	rr := httptest.NewRecorder()

	exportPremiereHandler(cfg).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestExportPremiere_PathTraversal(t *testing.T) {
	cfg := exportTestConfig(&fakeServiceForExport{files: map[string]*catalog.File{}})
	req := newExportRequest(t, exportpkg.ExportRequest{
		ProjectName: "Traversal",
		Format:      "edl",
		OutputDir:   "/tmp/../etc",
		Clips:       []exportpkg.ClipInput{{VideoID: "v1", StartMs: 0, EndMs: 1000}},
	})
	rr := httptest.NewRecorder()

	exportPremiereHandler(cfg).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestExportPremiere_EmptyClips(t *testing.T) {
	cfg := exportTestConfig(&fakeServiceForExport{files: map[string]*catalog.File{}})
	req := newExportRequest(t, exportpkg.ExportRequest{
		ProjectName: "Empty",
		Format:      "edl",
		OutputDir:   t.TempDir(),
		Clips:       []exportpkg.ClipInput{},
	})
	rr := httptest.NewRecorder()

	exportPremiereHandler(cfg).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestExportPremiere_InvalidTimecodes(t *testing.T) {
	cfg := exportTestConfig(&fakeServiceForExport{files: map[string]*catalog.File{}})
	req := newExportRequest(t, exportpkg.ExportRequest{
		ProjectName: "BadTime",
		Format:      "edl",
		OutputDir:   t.TempDir(),
		Clips:       []exportpkg.ClipInput{{VideoID: "v1", StartMs: 1000, EndMs: 1000}},
	})
	rr := httptest.NewRecorder()

	exportPremiereHandler(cfg).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestExportPremiereRoute_PreflightAllowsPost(t *testing.T) {
	cfg := exportTestConfig(&fakeServiceForExport{files: map[string]*catalog.File{}})
	router := NewRouter(cfg)

	req := httptest.NewRequest(http.MethodOptions, "/export/premiere", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
	allowMethods := rr.Header().Get("Access-Control-Allow-Methods")
	if !strings.Contains(allowMethods, "POST") {
		t.Fatalf("Access-Control-Allow-Methods = %q, want to include POST", allowMethods)
	}
}
