package pipeline

import "log/slog"

type FFmpeg interface {
	Probe(filePath string) (*ProbeResult, error)
	GenerateThumbnail(filePath, outputPath string, timeOffset float64) error
	ExtractAudio(filePath, outputPath string) error
}

type ProbeResult struct {
	Duration    float64
	Width       int
	Height      int
	Codec       string
	Bitrate     int64
	FrameRate   float64
	AudioCodec  string
	AudioSample int
}

type StubFFmpeg struct {
	logger *slog.Logger
}

func NewStubFFmpeg(logger *slog.Logger) *StubFFmpeg {
	return &StubFFmpeg{logger: logger}
}

func (f *StubFFmpeg) Probe(filePath string) (*ProbeResult, error) {
	f.logger.Info("ffmpeg stub: probe requested (v0 does not implement real ffmpeg)",
		"path", filePath)
	return &ProbeResult{}, nil
}

func (f *StubFFmpeg) GenerateThumbnail(filePath, outputPath string, timeOffset float64) error {
	f.logger.Info("ffmpeg stub: thumbnail requested",
		"input", filePath, "output", outputPath, "offset", timeOffset)
	return nil
}

func (f *StubFFmpeg) ExtractAudio(filePath, outputPath string) error {
	f.logger.Info("ffmpeg stub: audio extraction requested",
		"input", filePath, "output", outputPath)
	return nil
}
