package playback

import (
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path/filepath"
)

type PlaybackService interface {
	ServeFile(w http.ResponseWriter, r *http.Request, filePath string) error
}

type Server struct {
	logger *slog.Logger
}

func NewServer(logger *slog.Logger) *Server {
	return &Server{logger: logger}
}

func (s *Server) ServeFile(w http.ResponseWriter, r *http.Request, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "file not found", http.StatusNotFound)
			return nil
		}
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	size := stat.Size()
	contentType := mime.TypeByExtension(filepath.Ext(filePath))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Type", contentType)

	rangeHeader := r.Header.Get("Range")
	parsedRange, err := ParseRange(rangeHeader, size)

	if err == ErrUnsatisfiable {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", size))
		http.Error(w, "Range Not Satisfiable", http.StatusRequestedRangeNotSatisfiable)
		return nil
	}

	if err != nil && err != ErrInvalidRange {
		return err
	}

	if parsedRange == nil {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", size))
		w.WriteHeader(http.StatusOK)
		io.Copy(w, file)
		return nil
	}

	w.Header().Set("Content-Length", fmt.Sprintf("%d", parsedRange.ContentLength()))
	w.Header().Set("Content-Range", parsedRange.ContentRange(size))
	w.WriteHeader(http.StatusPartialContent)

	if _, err := file.Seek(parsedRange.Start, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek: %w", err)
	}

	io.CopyN(w, file, parsedRange.ContentLength())
	return nil
}
