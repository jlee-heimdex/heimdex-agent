package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

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

type RealFFmpeg struct {
	ffmpegBin string
	logger    *slog.Logger
}

func NewStubFFmpeg(logger *slog.Logger) *StubFFmpeg {
	return &StubFFmpeg{logger: logger}
}

func NewRealFFmpeg(logger *slog.Logger) *RealFFmpeg {
	bin := "ffmpeg"
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		bin = p
	}
	return &RealFFmpeg{ffmpegBin: bin, logger: logger}
}

func (f *RealFFmpeg) Probe(filePath string) (*ProbeResult, error) {
	return &ProbeResult{}, nil
}

func (f *RealFFmpeg) GenerateThumbnail(filePath, outputPath string, timeOffset float64) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create thumbnail dir: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, f.ffmpegBin, "-y",
		"-ss", fmt.Sprintf("%.3f", timeOffset),
		"-i", filePath,
		"-frames:v", "1",
		"-q:v", "2",
		outputPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg thumbnail failed: %w: %s", err, string(out))
	}

	info, err := os.Stat(outputPath)
	if err != nil || info.Size() == 0 {
		return fmt.Errorf("thumbnail output missing or empty: %s", outputPath)
	}
	return nil
}

func (f *RealFFmpeg) ExtractAudio(filePath, outputPath string) error {
	f.logger.Info("ffmpeg: audio extraction not yet implemented", "input", filePath)
	return nil
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
