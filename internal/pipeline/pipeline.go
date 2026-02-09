package pipeline

import (
	"context"
	"log/slog"
)

type Pipeline interface {
	Process(ctx context.Context, fileID, filePath string) error
	GetStatus(fileID string) (Status, error)
}

type Status struct {
	FileID   string
	Stage    string
	Progress int
	Error    string
}

type StubPipeline struct {
	logger *slog.Logger
}

func NewStubPipeline(logger *slog.Logger) *StubPipeline {
	return &StubPipeline{logger: logger}
}

func (p *StubPipeline) Process(ctx context.Context, fileID, filePath string) error {
	p.logger.Info("pipeline stub: process requested (v0 does not implement real processing)",
		"file_id", fileID, "path", filePath)
	return nil
}

func (p *StubPipeline) GetStatus(fileID string) (Status, error) {
	return Status{
		FileID: fileID,
		Stage:  "stub",
	}, nil
}
