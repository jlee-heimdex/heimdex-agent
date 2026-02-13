package api

import (
	"time"

	"github.com/heimdex/heimdex-agent/internal/catalog"
)

type HealthResponse struct {
	Status   string `json:"status"`
	Version  string `json:"version"`
	UptimeS  int64  `json:"uptime_s"`
	DeviceID string `json:"device_id"`
}

type StatusResponse struct {
	State        string                  `json:"state"`
	LastError    string                  `json:"last_error,omitempty"`
	SourcesCount int                     `json:"sources_count"`
	FilesCount   int                     `json:"files_count"`
	JobsRunning  int                     `json:"jobs_running"`
	ActiveJob    *JobResponse            `json:"active_job,omitempty"`
	Pipelines    *PipelineStatusResponse `json:"pipelines,omitempty"`
	Constraints  *ConstraintsResponse    `json:"constraints,omitempty"`
}

type PipelineStatusResponse struct {
	HasFaces    bool   `json:"has_faces"`
	HasSpeech   bool   `json:"has_speech"`
	HasScenes   bool   `json:"has_scenes"`
	HasOCR      bool   `json:"has_ocr"`
	LastProbeAt string `json:"last_probe_at,omitempty"`
	DepsAvail   int    `json:"deps_available"`
	DepsTotal   int    `json:"deps_total"`
}

type ConstraintsResponse struct {
	ScenesRequiresSpeech bool `json:"scenes_requires_speech"`
}

type AddFolderRequest struct {
	Path        string `json:"path"`
	DisplayName string `json:"display_name,omitempty"`
}

type AddFolderResponse struct {
	SourceID string `json:"source_id"`
}

type SourceResponse struct {
	ID            string `json:"id"`
	Type          string `json:"type"`
	Path          string `json:"path"`
	DisplayName   string `json:"display_name"`
	DriveNickname string `json:"drive_nickname,omitempty"`
	Present       bool   `json:"present"`
	CreatedAt     string `json:"created_at"`
}

type SourcesResponse struct {
	Sources []SourceResponse `json:"sources"`
}

type ScanRequest struct {
	SourceID string `json:"source_id,omitempty"`
}

type ScanResponse struct {
	JobID string `json:"job_id"`
}

type JobResponse struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	SourceID  string `json:"source_id,omitempty"`
	FileID    string `json:"file_id,omitempty"`
	Progress  int    `json:"progress"`
	Error     string `json:"error,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type JobsResponse struct {
	Jobs []JobResponse `json:"jobs"`
}

type FileResponse struct {
	ID          string `json:"id"`
	SourceID    string `json:"source_id"`
	Path        string `json:"path"`
	Filename    string `json:"filename"`
	Size        int64  `json:"size"`
	Fingerprint string `json:"fingerprint"`
	CreatedAt   string `json:"created_at"`
}

type FilesResponse struct {
	Files []FileResponse `json:"files"`
}

type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

func SourceToResponse(s *catalog.Source) SourceResponse {
	return SourceResponse{
		ID:            s.ID,
		Type:          s.Type,
		Path:          s.Path,
		DisplayName:   s.DisplayName,
		DriveNickname: s.DriveNickname,
		Present:       s.Present,
		CreatedAt:     s.CreatedAt.Format(time.RFC3339),
	}
}

func JobToResponse(j *catalog.Job) JobResponse {
	return JobResponse{
		ID:        j.ID,
		Type:      j.Type,
		Status:    j.Status,
		SourceID:  j.SourceID,
		FileID:    j.FileID,
		Progress:  j.Progress,
		Error:     j.Error,
		CreatedAt: j.CreatedAt.Format(time.RFC3339),
		UpdatedAt: j.UpdatedAt.Format(time.RFC3339),
	}
}

func FileToResponse(f *catalog.File) FileResponse {
	return FileResponse{
		ID:          f.ID,
		SourceID:    f.SourceID,
		Path:        f.Path,
		Filename:    f.Filename,
		Size:        f.Size,
		Fingerprint: f.Fingerprint,
		CreatedAt:   f.CreatedAt.Format(time.RFC3339),
	}
}
