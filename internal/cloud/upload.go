package cloud

import "log/slog"

type UploadService interface {
	UploadMetadata(fileID string, metadata map[string]interface{}) error
	UploadSidecar(fileID, sidecarPath string) error
	GetPresignedURL(fileID string) (string, error)
}

type StubUpload struct {
	logger *slog.Logger
}

func NewStubUpload(logger *slog.Logger) *StubUpload {
	return &StubUpload{logger: logger}
}

func (s *StubUpload) UploadMetadata(fileID string, metadata map[string]interface{}) error {
	s.logger.Info("cloud upload stub: metadata upload requested", "file_id", fileID)
	return nil
}

func (s *StubUpload) UploadSidecar(fileID, sidecarPath string) error {
	s.logger.Info("cloud upload stub: sidecar upload requested", "file_id", fileID, "path", sidecarPath)
	return nil
}

func (s *StubUpload) GetPresignedURL(fileID string) (string, error) {
	s.logger.Info("cloud upload stub: presigned URL requested", "file_id", fileID)
	return "", nil
}
