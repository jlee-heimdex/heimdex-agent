package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

type LibraryService interface {
	GetOrCreate(ctx context.Context, name string) (*LibraryResult, error)
	List(ctx context.Context) ([]LibraryResult, error)
}

type LibraryResult struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Created bool   `json:"created"`
}

type HTTPLibraryService struct {
	client *HTTPClient
}

func (s *HTTPLibraryService) GetOrCreate(ctx context.Context, name string) (*LibraryResult, error) {
	body, err := json.Marshal(map[string]string{"name": name})
	if err != nil {
		return nil, fmt.Errorf("marshal library request: %w", err)
	}

	url := fmt.Sprintf("%s/api/libraries", s.client.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.client.token)
	if s.client.orgSlug != "" {
		req.Host = s.client.orgSlug + ".app.heimdex.local"
	}

	resp, err := s.client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &UploadError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}

	var result LibraryResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal library response: %w", err)
	}

	return &result, nil
}

func (s *HTTPLibraryService) List(ctx context.Context) ([]LibraryResult, error) {
	url := fmt.Sprintf("%s/api/libraries", s.client.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.client.token)
	if s.client.orgSlug != "" {
		req.Host = s.client.orgSlug + ".app.heimdex.local"
	}

	resp, err := s.client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 65536))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &UploadError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}

	var wrapper struct {
		Libraries []LibraryResult `json:"libraries"`
	}
	if err := json.Unmarshal(respBody, &wrapper); err != nil {
		return nil, fmt.Errorf("unmarshal libraries response: %w", err)
	}

	return wrapper.Libraries, nil
}

type StubLibraryService struct {
	logger *slog.Logger
}

func (s *StubLibraryService) GetOrCreate(ctx context.Context, name string) (*LibraryResult, error) {
	s.logger.Info("cloud stub: library get-or-create requested", "name", name)
	return &LibraryResult{ID: "stub-library-id", Name: name, Created: true}, nil
}

func (s *StubLibraryService) List(ctx context.Context) ([]LibraryResult, error) {
	s.logger.Info("cloud stub: library list requested")
	return nil, nil
}
