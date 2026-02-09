package cloud

import "log/slog"

type Client interface {
	Auth() AuthService
	Upload() UploadService
}

type StubClient struct {
	auth   *StubAuth
	upload *StubUpload
	logger *slog.Logger
}

func NewStubClient(logger *slog.Logger) *StubClient {
	return &StubClient{
		auth:   NewStubAuth(logger),
		upload: NewStubUpload(logger),
		logger: logger,
	}
}

func (c *StubClient) Auth() AuthService {
	return c.auth
}

func (c *StubClient) Upload() UploadService {
	return c.upload
}

func (c *StubClient) RegisterDevice(deviceID string) error {
	c.logger.Info("cloud stub: device registration requested", "device_id", deviceID)
	return nil
}
