package cloud

import (
	"context"
	"log/slog"
)

type Client interface {
	Auth() AuthService
	Upload() UploadService
	Scenes() SceneUploader
	RegisterDevice(deviceID string) error
}

type SceneUploader interface {
	UploadScenes(ctx context.Context, payload SceneIngestPayload) error
}

type StubClient struct {
	auth   *StubAuth
	upload *StubUpload
	scenes *StubSceneUploader
	logger *slog.Logger
}

func NewStubClient(logger *slog.Logger) *StubClient {
	return &StubClient{
		auth:   NewStubAuth(logger),
		upload: NewStubUpload(logger),
		scenes: &StubSceneUploader{logger: logger},
		logger: logger,
	}
}

func (c *StubClient) Auth() AuthService {
	return c.auth
}

func (c *StubClient) Upload() UploadService {
	return c.upload
}

func (c *StubClient) Scenes() SceneUploader {
	return c.scenes
}

func (c *StubClient) RegisterDevice(deviceID string) error {
	c.logger.Info("cloud stub: device registration requested", "device_id", deviceID)
	return nil
}

type StubSceneUploader struct {
	logger *slog.Logger
}

func (s *StubSceneUploader) UploadScenes(ctx context.Context, payload SceneIngestPayload) error {
	s.logger.Info("cloud stub: scene upload requested",
		"video_id", payload.VideoID,
		"scene_count", len(payload.Scenes),
	)
	return nil
}
