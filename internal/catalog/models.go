package catalog

import (
	"crypto/rand"
	"fmt"
	"time"
)

type Source struct {
	ID             string    `json:"id"`
	Type           string    `json:"type"`
	Path           string    `json:"path"`
	DisplayName    string    `json:"display_name"`
	DriveNickname  string    `json:"drive_nickname,omitempty"`
	CloudLibraryID string    `json:"cloud_library_id,omitempty"`
	Present        bool      `json:"present"`
	CreatedAt      time.Time `json:"created_at"`
}

type File struct {
	ID          string    `json:"id"`
	SourceID    string    `json:"source_id"`
	Path        string    `json:"path"`
	Filename    string    `json:"filename"`
	Size        int64     `json:"size"`
	Mtime       time.Time `json:"mtime"`
	Fingerprint string    `json:"fingerprint"`
	CreatedAt   time.Time `json:"created_at"`
}

const (
	JobTypeScan               = "scan"
	JobTypeIndex              = "index"
	JobTypeUploadScenes       = "upload_scenes"
	JobTypeGenerateThumbnails = "generate_thumbnails"

	JobStatusPending   = "pending"
	JobStatusRunning   = "running"
	JobStatusCompleted = "completed"
	JobStatusFailed    = "failed"
)

type Job struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Status    string    `json:"status"`
	SourceID  string    `json:"source_id,omitempty"`
	FileID    string    `json:"file_id,omitempty"`
	Progress  int       `json:"progress"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ConfigEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

var VideoExtensions = map[string]bool{
	".mp4": true,
	".mov": true,
	".mkv": true,
}

func NewID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func IsVideoFile(filename string) bool {
	ext := ""
	for i := len(filename) - 1; i >= 0; i-- {
		if filename[i] == '.' {
			ext = filename[i:]
			break
		}
	}
	if ext == "" {
		return false
	}
	lower := make([]byte, len(ext))
	for i, c := range ext {
		if c >= 'A' && c <= 'Z' {
			lower[i] = byte(c + 32)
		} else {
			lower[i] = byte(c)
		}
	}
	return VideoExtensions[string(lower)]
}
