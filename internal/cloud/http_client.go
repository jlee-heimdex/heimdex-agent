package cloud

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// UploadError represents an error from the scene upload endpoint.
type UploadError struct {
	StatusCode int
	Body       string
}

func (e *UploadError) Error() string {
	return fmt.Sprintf("scene upload failed: HTTP %d: %s", e.StatusCode, e.Body)
}

// IsRetryable returns true for server errors (5xx) and network errors.
// Client errors (4xx) are considered permanent.
func (e *UploadError) IsRetryable() bool {
	return e.StatusCode >= 500
}

// HTTPClient is a real cloud client that communicates with the Heimdex SaaS.
// It sends scene ingestion payloads via HTTP to the SaaS ingest endpoint.
type HTTPClient struct {
	baseURL    string
	token      string
	orgSlug    string
	deviceID   string
	httpClient *http.Client
	logger     *slog.Logger

	auth   *StubAuth
	upload *StubUpload
}

func NewHTTPClient(baseURL, token, orgSlug string, logger *slog.Logger) *HTTPClient {
	return &HTTPClient{
		baseURL: baseURL,
		token:   token,
		orgSlug: orgSlug,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		logger: logger,
		auth:   NewStubAuth(logger),
		upload: NewStubUpload(logger),
	}
}

func (c *HTTPClient) Auth() AuthService {
	return c.auth
}

func (c *HTTPClient) Upload() UploadService {
	return c.upload
}

func (c *HTTPClient) Scenes() SceneUploader {
	return c
}

func (c *HTTPClient) RegisterDevice(deviceID string) error {
	c.logger.Info("cloud http: device registration requested", "device_id", deviceID)
	return nil
}

func (c *HTTPClient) SetDeviceID(id string) {
	c.deviceID = id
}

func (c *HTTPClient) UploadScenes(ctx context.Context, payload SceneIngestPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal scene payload: %w", err)
	}

	// Build URL with org slug as subdomain: {org}.app.heimdex.local -> baseURL
	url := fmt.Sprintf("%s/api/ingest/scenes", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("X-Heimdex-Request-Id", generateRequestID())
	if c.deviceID != "" {
		req.Header.Set("X-Heimdex-Device-Id", c.deviceID)
	}

	// Set Host header for tenancy resolution
	// The SaaS resolves org from the Host header subdomain
	if c.orgSlug != "" {
		req.Host = c.orgSlug + ".app.heimdex.local"
	}

	c.logger.Info("uploading scenes to cloud",
		"url", url,
		"host", req.Host,
		"video_id", payload.VideoID,
		"scene_count", len(payload.Scenes),
		"body_bytes", len(body),
	)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var result SceneIngestResponse
		if err := json.Unmarshal(respBody, &result); err == nil {
			c.logger.Info("scene upload succeeded",
				"video_id", result.VideoID,
				"indexed_count", result.IndexedCount,
			)
		}
		return nil
	}

	return &UploadError{StatusCode: resp.StatusCode, Body: string(respBody)}
}

func generateRequestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
