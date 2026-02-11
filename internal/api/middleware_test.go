package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/heimdex/heimdex-agent/internal/catalog"
)

func TestIsAllowedOrigin(t *testing.T) {
	allowed := []string{
		"http://localhost:3000",
		"http://localhost:8080",
		"http://localhost",
		"http://127.0.0.1:3000",
		"http://127.0.0.1",
		"https://acme.app.heimdex.co",
		"https://demo-org.app.heimdex.co",
		"https://acme.app.heimdex.local",
		"http://acme.app.heimdex.local",
		"http://devorg.app.heimdex.local:3000",
		"https://acme.app.heimdex.co:443",
		"http://acme.app.heimdex.local:8080",
		"https://a--b.app.heimdex.co",
		"https://a.app.heimdex.co",
	}

	for _, origin := range allowed {
		if !isAllowedOrigin(origin) {
			t.Errorf("isAllowedOrigin(%q) = false, want true", origin)
		}
	}

	denied := []string{
		"https://evil.com",
		"https://app.heimdex.co",
		"https://app.heimdex.co.evil.com",
		"https://evil.app.heimdex.co.evil.com",
		"https://acme.app.heimdex.co.evil.com",
		"http://192.168.1.1:3000",
		"https://heimdex.co",
		"",
		"ftp://localhost:3000",
		"http://localhost:not-a-port",
		"http://localhost:3000/path",
		"https://-bad.app.heimdex.co",
		"https://bad-.app.heimdex.co",
		"https://acme.app.heimdex.co:not-a-port",
		"https://acme.app.heimdex.co:3000/path",
	}

	for _, origin := range denied {
		if isAllowedOrigin(origin) {
			t.Errorf("isAllowedOrigin(%q) = true, want false", origin)
		}
	}
}

func TestIsLoopbackRemoteAddr(t *testing.T) {
	loopback := []string{
		"127.0.0.1:12345",
		"[::1]:12345",
	}

	for _, addr := range loopback {
		if !isLoopbackRemoteAddr(addr) {
			t.Errorf("isLoopbackRemoteAddr(%q) = false, want true", addr)
		}
	}

	nonLoopback := []string{
		"8.8.8.8:12345",
		"192.168.1.1:8080",
		"10.0.0.1:3000",
		"not-an-ip:1234",
	}

	for _, addr := range nonLoopback {
		if isLoopbackRemoteAddr(addr) {
			t.Errorf("isLoopbackRemoteAddr(%q) = true, want false", addr)
		}
	}
}

func TestCORSAllowlist_AllowedOrigin(t *testing.T) {
	handler := CORSAllowlist()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Errorf("ACAO = %q, want %q", got, "http://localhost:3000")
	}
	if got := rr.Header().Get("Vary"); got != "Origin" {
		t.Errorf("Vary = %q, want %q", got, "Origin")
	}
}

func TestCORSAllowlist_HeimdexSubdomain(t *testing.T) {
	handler := CORSAllowlist()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "https://acme.app.heimdex.local")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://acme.app.heimdex.local" {
		t.Errorf("ACAO = %q, want %q", got, "https://acme.app.heimdex.local")
	}
}

func TestCORSAllowlist_DeniedOrigin_GET(t *testing.T) {
	handler := CORSAllowlist()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "https://evil.com")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (request still served, just no ACAO)", rr.Code, http.StatusOK)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("ACAO = %q, want empty for denied origin", got)
	}
}

func TestCORSAllowlist_DeniedOrigin_Preflight(t *testing.T) {
	handler := CORSAllowlist()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called for denied preflight")
	}))

	req := httptest.NewRequest(http.MethodOptions, "/playback/file", nil)
	req.Header.Set("Origin", "https://evil.com")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d for denied preflight", rr.Code, http.StatusForbidden)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("ACAO = %q, want empty for denied preflight", got)
	}
}

func TestCORSAllowlist_NoOrigin(t *testing.T) {
	handler := CORSAllowlist()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("ACAO = %q, want empty when no Origin header", got)
	}
}

func TestCORSAllowlist_AllowedPreflight(t *testing.T) {
	handler := CORSAllowlist()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called for preflight")
	}))

	req := httptest.NewRequest(http.MethodOptions, "/playback/file", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Errorf("ACAO = %q, want %q", got, "http://localhost:3000")
	}
}

func TestCORSAllowlist_RangeHeaders(t *testing.T) {
	handler := CORSAllowlist()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/playback/file", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	allowHeaders := rr.Header().Get("Access-Control-Allow-Headers")
	for _, h := range []string{"Range", "Content-Type", "Authorization", "X-Heimdex-Request-Id", "X-Heimdex-Device-Id"} {
		if !containsHeader(allowHeaders, h) {
			t.Errorf("Access-Control-Allow-Headers missing %q, got %q", h, allowHeaders)
		}
	}

	exposeHeaders := rr.Header().Get("Access-Control-Expose-Headers")
	for _, h := range []string{"Content-Range", "Accept-Ranges", "Content-Length", "Content-Type"} {
		if !containsHeader(exposeHeaders, h) {
			t.Errorf("Access-Control-Expose-Headers missing %q, got %q", h, exposeHeaders)
		}
	}

	allowMethods := rr.Header().Get("Access-Control-Allow-Methods")
	for _, m := range []string{"GET", "HEAD", "OPTIONS"} {
		if !containsHeader(allowMethods, m) {
			t.Errorf("Access-Control-Allow-Methods missing %q, got %q", m, allowMethods)
		}
	}
}

func containsHeader(headerVal, target string) bool {
	for _, part := range splitTrim(headerVal) {
		if part == target {
			return true
		}
	}
	return false
}

func splitTrim(s string) []string {
	parts := make([]string, 0)
	for _, p := range splitCSV(s) {
		p = trimSpace(p)
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

func splitCSV(s string) []string {
	result := make([]string, 0)
	current := ""
	for _, c := range s {
		if c == ',' {
			result = append(result, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func trimSpace(s string) string {
	start := firstNonSpace(s)
	end := lastNonSpace(s)
	if start > end {
		return ""
	}
	return s[start : end+1]
}

func firstNonSpace(s string) int {
	for i, c := range s {
		if c != ' ' && c != '\t' {
			return i
		}
	}
	return len(s)
}

func lastNonSpace(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] != ' ' && s[i] != '\t' {
			return i
		}
	}
	return -1
}

func TestCORSAllowlist_VaryIsAdditive(t *testing.T) {
	handler := CORSAllowlist()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rr := httptest.NewRecorder()
	rr.Header().Set("Vary", "Accept-Encoding")

	handler.ServeHTTP(rr, req)

	vary := rr.Header().Values("Vary")
	hasEncoding := false
	hasOrigin := false
	for _, v := range vary {
		if v == "Accept-Encoding" {
			hasEncoding = true
		}
		if v == "Origin" {
			hasOrigin = true
		}
	}
	if !hasEncoding {
		t.Errorf("Vary lost Accept-Encoding, got %v", vary)
	}
	if !hasOrigin {
		t.Errorf("Vary missing Origin, got %v", vary)
	}
}

func TestCORSAllowlist_PreflightWithRequestHeaders(t *testing.T) {
	handler := CORSAllowlist()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called for preflight")
	}))

	req := httptest.NewRequest(http.MethodOptions, "/playback/file", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Headers", "range,authorization")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}

	allowHeaders := rr.Header().Get("Access-Control-Allow-Headers")
	if !containsHeader(allowHeaders, "Range") {
		t.Errorf("Access-Control-Allow-Headers missing Range, got %q", allowHeaders)
	}
	if !containsHeader(allowHeaders, "Authorization") {
		t.Errorf("Access-Control-Allow-Headers missing Authorization, got %q", allowHeaders)
	}
}

func TestIsLoopbackRemoteAddr_EdgeCases(t *testing.T) {
	cases := []struct {
		addr string
		want bool
	}{
		{"::1", true},
		{"", false},
		{"garbage", false},
		{"[::1]", true},
		{"127.0.0.1", true},
	}

	for _, tc := range cases {
		got := isLoopbackRemoteAddr(tc.addr)
		if got != tc.want {
			t.Errorf("isLoopbackRemoteAddr(%q) = %v, want %v", tc.addr, got, tc.want)
		}
	}
}

func TestLoopbackGuard_Rejects_NonLoopback(t *testing.T) {
	handler := LoopbackGuard()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called for non-loopback")
	}))

	req := httptest.NewRequest(http.MethodGet, "/playback/file?file_id=test", nil)
	req.RemoteAddr = "8.8.8.8:12345"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusForbidden)
	}

	body := decodeJSONBody(t, rr)
	if code, ok := body["code"].(string); !ok || code != "FORBIDDEN" {
		t.Errorf("error code = %v, want FORBIDDEN", body["code"])
	}
}

func TestLoopbackGuard_Allows_Loopback(t *testing.T) {
	called := false
	handler := LoopbackGuard()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/playback/file?file_id=test", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !called {
		t.Fatal("handler should have been called for loopback")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestLoopbackGuard_Allows_IPv6Loopback(t *testing.T) {
	called := false
	handler := LoopbackGuard()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/playback/file?file_id=test", nil)
	req.RemoteAddr = "[::1]:12345"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !called {
		t.Fatal("handler should have been called for IPv6 loopback")
	}
}

func TestPlaybackRoute_Loopback_Rejection_Integration(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := ServerConfig{
		CatalogService: &fakeServiceWithFile{
			fakeService: fakeService{},
			file: &catalog.File{
				ID:       "file-1",
				SourceID: "src-1",
				Path:     "/tmp/test.mp4",
			},
		},
		PlaybackServer: &fakePlayback{},
		Repository:     &fakeRepo{},
		Logger:         logger,
		StartTime:      time.Now(),
		DeviceID:       "test-device",
	}

	router := NewRouter(cfg)
	server := httptest.NewServer(router)
	defer server.Close()

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/playback/file?file_id=file-1", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d (loopback request via httptest should be allowed)", resp.StatusCode, http.StatusOK)
	}
}

func TestHealthRoute_CORS_Integration(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := ServerConfig{
		CatalogService: &fakeService{},
		Repository:     &fakeRepo{},
		Logger:         logger,
		StartTime:      time.Now(),
		DeviceID:       "test-device",
	}

	router := NewRouter(cfg)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Errorf("ACAO = %q, want %q", got, "http://localhost:3000")
	}
}

func TestHEAD_PlaybackFile_NoBody(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := ServerConfig{
		CatalogService: &fakeServiceWithFile{
			fakeService: fakeService{},
			file: &catalog.File{
				ID:       "file-1",
				SourceID: "src-1",
				Path:     "/tmp/test.mp4",
			},
		},
		PlaybackServer: &fakePlayback{},
		Repository:     &fakeRepo{},
		Logger:         logger,
		StartTime:      time.Now(),
		DeviceID:       "test-device",
	}

	router := NewRouter(cfg)
	server := httptest.NewServer(router)
	defer server.Close()

	req, _ := http.NewRequest(http.MethodHead, server.URL+"/playback/file?file_id=file-1", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, _ := io.ReadAll(resp.Body)
	if len(body) != 0 {
		t.Errorf("HEAD response body length = %d, want 0", len(body))
	}
}

func TestHEAD_PlaybackFile_MissingFileID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := ServerConfig{
		CatalogService: &fakeService{},
		PlaybackServer: &fakePlayback{},
		Repository:     &fakeRepo{},
		Logger:         logger,
		StartTime:      time.Now(),
		DeviceID:       "test-device",
	}

	router := NewRouter(cfg)
	server := httptest.NewServer(router)
	defer server.Close()

	req, _ := http.NewRequest(http.MethodHead, server.URL+"/playback/file", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	body, _ := io.ReadAll(resp.Body)
	if len(body) != 0 {
		t.Errorf("HEAD response body length = %d, want 0", len(body))
	}
}

type fakePlayback struct{}

func (f *fakePlayback) ServeFile(w http.ResponseWriter, r *http.Request, path string) error {
	w.Header().Set("Accept-Ranges", "bytes")
	w.WriteHeader(http.StatusOK)
	return nil
}

type fakeServiceWithFile struct {
	fakeService
	file *catalog.File
}

func (f *fakeServiceWithFile) GetFile(ctx context.Context, id string) (*catalog.File, error) {
	if f.file != nil && f.file.ID == id {
		return f.file, nil
	}
	return nil, nil
}
