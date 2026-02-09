package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/heimdex/heimdex-agent/internal/catalog"
)

func NewRouter(cfg ServerConfig) *chi.Mux {
	r := chi.NewRouter()

	r.Use(RequestIDMiddleware())
	r.Use(RecoveryMiddleware(cfg.Logger))
	r.Use(LoggingMiddleware(cfg.Logger))

	r.Get("/health", healthHandler(cfg))

	r.Group(func(r chi.Router) {
		r.Use(AuthMiddleware(cfg.Repository, cfg.Logger))

		r.Get("/status", statusHandler(cfg))
		r.Get("/sources", listSourcesHandler(cfg))
		r.Post("/sources/folders", addFolderHandler(cfg))
		r.Delete("/sources/{id}", deleteSourceHandler(cfg))
		r.Get("/sources/{id}/files", listFilesHandler(cfg))
		r.Post("/scan", scanHandler(cfg))
		r.Get("/jobs", listJobsHandler(cfg))
		r.Get("/jobs/{id}", getJobHandler(cfg))
		r.Get("/playback/file", playbackHandler(cfg))
	})

	return r
}

func healthHandler(cfg ServerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uptime := int64(time.Since(cfg.StartTime).Seconds())
		WriteJSON(w, http.StatusOK, HealthResponse{
			Status:   "ok",
			Version:  "0.1.0",
			UptimeS:  uptime,
			DeviceID: cfg.DeviceID,
		})
	}
}

func statusHandler(cfg ServerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		sources, _ := cfg.CatalogService.GetSources(ctx)
		filesCount, _ := cfg.CatalogService.CountFiles(ctx)
		jobs, _ := cfg.Repository.ListJobs(ctx, 10)

		state := "idle"
		var activeJob *JobResponse
		jobsRunning := 0
		lastError := ""

		if cfg.Runner != nil && cfg.Runner.IsPaused() {
			state = "paused"
		}

		for _, j := range jobs {
			if j.Status == catalog.JobStatusRunning {
				state = "indexing"
				resp := JobToResponse(j)
				activeJob = &resp
				jobsRunning++
			}
			if j.Status == catalog.JobStatusFailed && lastError == "" {
				lastError = j.Error
			}
		}

		if lastError != "" && state == "idle" {
			state = "error"
		}

		resp := StatusResponse{
			State:        state,
			LastError:    lastError,
			SourcesCount: len(sources),
			FilesCount:   filesCount,
			JobsRunning:  jobsRunning,
			ActiveJob:    activeJob,
		}

		if cfg.Doctor != nil {
			caps, err := cfg.Doctor.Get(ctx)
			if err == nil && caps != nil {
				resp.Pipelines = &PipelineStatusResponse{
					HasFaces:    caps.HasFaces,
					HasSpeech:   caps.HasSpeech,
					LastProbeAt: caps.ProbedAt.Format(time.RFC3339),
					DepsAvail:   caps.Summary.Available,
					DepsTotal:   caps.Summary.Total,
				}
			}
		}

		WriteJSON(w, http.StatusOK, resp)
	}
}

func listSourcesHandler(cfg ServerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sources, err := cfg.CatalogService.GetSources(r.Context())
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "failed to list sources", "INTERNAL_ERROR")
			return
		}

		resp := SourcesResponse{Sources: make([]SourceResponse, len(sources))}
		for i, s := range sources {
			resp.Sources[i] = SourceToResponse(s)
		}
		WriteJSON(w, http.StatusOK, resp)
	}
}

func addFolderHandler(cfg ServerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req AddFolderRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, http.StatusBadRequest, "invalid request body", "BAD_REQUEST")
			return
		}

		if req.Path == "" {
			WriteError(w, http.StatusBadRequest, "path is required", "BAD_REQUEST")
			return
		}

		source, err := cfg.CatalogService.AddFolder(r.Context(), req.Path, req.DisplayName)
		if err != nil {
			WriteError(w, http.StatusBadRequest, err.Error(), "BAD_REQUEST")
			return
		}

		WriteJSON(w, http.StatusCreated, AddFolderResponse{SourceID: source.ID})
	}
}

func deleteSourceHandler(cfg ServerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if id == "" {
			WriteError(w, http.StatusBadRequest, "source id required", "BAD_REQUEST")
			return
		}

		if err := cfg.CatalogService.RemoveSource(r.Context(), id); err != nil {
			WriteError(w, http.StatusInternalServerError, err.Error(), "INTERNAL_ERROR")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func listFilesHandler(cfg ServerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sourceID := chi.URLParam(r, "id")
		if sourceID == "" {
			WriteError(w, http.StatusBadRequest, "source id required", "BAD_REQUEST")
			return
		}

		files, err := cfg.CatalogService.GetFiles(r.Context(), sourceID)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, err.Error(), "INTERNAL_ERROR")
			return
		}

		resp := FilesResponse{Files: make([]FileResponse, len(files))}
		for i, f := range files {
			resp.Files[i] = FileToResponse(f)
		}
		WriteJSON(w, http.StatusOK, resp)
	}
}

func scanHandler(cfg ServerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req ScanRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, http.StatusBadRequest, "invalid request body", "BAD_REQUEST")
			return
		}

		if req.SourceID == "" {
			sources, err := cfg.CatalogService.GetSources(r.Context())
			if err != nil {
				WriteError(w, http.StatusInternalServerError, err.Error(), "INTERNAL_ERROR")
				return
			}
			if len(sources) == 0 {
				WriteError(w, http.StatusBadRequest, "no sources configured", "BAD_REQUEST")
				return
			}
			req.SourceID = sources[0].ID
		}

		job, err := cfg.CatalogService.ScanSource(r.Context(), req.SourceID)
		if err != nil {
			WriteError(w, http.StatusBadRequest, err.Error(), "BAD_REQUEST")
			return
		}

		WriteJSON(w, http.StatusAccepted, ScanResponse{JobID: job.ID})
	}
}

func listJobsHandler(cfg ServerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jobs, err := cfg.Repository.ListJobs(r.Context(), 50)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "failed to list jobs", "INTERNAL_ERROR")
			return
		}

		resp := JobsResponse{Jobs: make([]JobResponse, len(jobs))}
		for i, j := range jobs {
			resp.Jobs[i] = JobToResponse(j)
		}
		WriteJSON(w, http.StatusOK, resp)
	}
}

func getJobHandler(cfg ServerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if id == "" {
			WriteError(w, http.StatusBadRequest, "job id required", "BAD_REQUEST")
			return
		}

		job, err := cfg.Repository.GetJob(r.Context(), id)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, err.Error(), "INTERNAL_ERROR")
			return
		}
		if job == nil {
			WriteError(w, http.StatusNotFound, "job not found", "NOT_FOUND")
			return
		}

		WriteJSON(w, http.StatusOK, JobToResponse(job))
	}
}

func playbackHandler(cfg ServerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fileID := r.URL.Query().Get("file_id")
		if fileID == "" {
			WriteError(w, http.StatusBadRequest, "file_id is required", "BAD_REQUEST")
			return
		}

		file, err := cfg.CatalogService.GetFile(r.Context(), fileID)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, err.Error(), "INTERNAL_ERROR")
			return
		}
		if file == nil {
			WriteError(w, http.StatusNotFound, "file not found", "NOT_FOUND")
			return
		}

		source, _ := cfg.CatalogService.GetSource(r.Context(), file.SourceID)
		if source != nil && !source.Present {
			WriteError(w, http.StatusNotFound,
				"file not available - drive '"+source.DriveNickname+"' is disconnected",
				"DRIVE_DISCONNECTED")
			return
		}

		if err := cfg.PlaybackServer.ServeFile(w, r, file.Path); err != nil {
			cfg.Logger.Error("playback error", "error", err, "file_id", fileID)
		}
	}
}
