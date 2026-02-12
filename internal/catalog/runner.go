package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/heimdex/heimdex-agent/internal/cloud"
	"github.com/heimdex/heimdex-agent/internal/pipeline"
	"github.com/heimdex/heimdex-agent/internal/pipelines"
)

type Runner struct {
	service           *Service
	repo              Repository
	pipeRunner        pipelines.Runner
	ffmpeg            pipeline.FFmpeg
	doctor            *pipelines.CachedDoctor
	cloudClient       cloud.Client
	fallbackLibraryID string
	logger            *slog.Logger
	pollInterval      time.Duration
	running           atomic.Bool
	paused            atomic.Bool
}

func NewRunner(service *Service, repo Repository, pipeRunner pipelines.Runner, ffmpeg pipeline.FFmpeg, doctor *pipelines.CachedDoctor, logger *slog.Logger) *Runner {
	return &Runner{
		service:      service,
		repo:         repo,
		pipeRunner:   pipeRunner,
		ffmpeg:       ffmpeg,
		doctor:       doctor,
		logger:       logger,
		pollInterval: 5 * time.Second,
	}
}

// SetCloudClient configures the cloud client for scene upload after indexing.
// If not set (nil), scene upload is skipped silently.
func (r *Runner) SetCloudClient(client cloud.Client, libraryID string) {
	r.cloudClient = client
	r.fallbackLibraryID = libraryID
}

func (r *Runner) resolveLibraryID(ctx context.Context, source *Source) (string, error) {
	if source != nil && source.CloudLibraryID != "" {
		return source.CloudLibraryID, nil
	}

	if r.cloudClient != nil && source != nil {
		result, err := r.cloudClient.Libraries().GetOrCreate(ctx, source.DisplayName)
		if err != nil {
			r.logger.Warn("library auto-create failed, using fallback",
				"source_id", source.ID,
				"source_name", source.DisplayName,
				"error", err,
			)
		} else {
			if updateErr := r.repo.UpdateSourceCloudLibraryID(ctx, source.ID, result.ID); updateErr != nil {
				r.logger.Warn("failed to store library mapping", "source_id", source.ID, "error", updateErr)
			} else {
				r.logger.Info("library resolved for source",
					"source_id", source.ID,
					"source_name", source.DisplayName,
					"library_id", result.ID,
					"created", result.Created,
				)
			}
			return result.ID, nil
		}
	}

	if r.fallbackLibraryID != "" {
		return r.fallbackLibraryID, nil
	}

	return "", fmt.Errorf("no library ID available: source has no mapping and no fallback configured")
}

func (r *Runner) backfillCloudUploads(ctx context.Context) {
	jobs, err := r.repo.ListJobs(ctx, 10000)
	if err != nil {
		r.logger.Warn("backfill: cannot list jobs", "error", err)
		return
	}

	completedIndex := make(map[string]bool)
	hasUpload := make(map[string]bool)
	for _, j := range jobs {
		if j.Type == JobTypeIndex && j.Status == JobStatusCompleted && j.FileID != "" {
			completedIndex[j.FileID] = true
		}
		if j.Type == JobTypeUploadScenes && j.FileID != "" {
			hasUpload[j.FileID] = true
		}
	}

	created := 0
	for fileID := range completedIndex {
		if hasUpload[fileID] {
			continue
		}
		scenePath := filepath.Join(r.pipeRunner.ArtifactsDir(), fileID, "scenes", "result.json")
		if _, err := os.Stat(scenePath); err != nil {
			continue
		}
		now := time.Now()
		job := &Job{
			ID:        NewID(),
			Type:      JobTypeUploadScenes,
			Status:    JobStatusPending,
			FileID:    fileID,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := r.repo.CreateJob(ctx, job); err != nil {
			r.logger.Warn("backfill: cannot create upload job", "file_id", fileID, "error", err)
			continue
		}
		created++
	}

	r.logger.Info("backfill: scan complete",
		"completed_index_jobs", len(completedIndex),
		"already_uploaded", len(hasUpload),
		"created", created,
	)
}

func (r *Runner) Start(ctx context.Context) {
	if r.running.Swap(true) {
		return
	}

	r.logger.Info("job runner started")

	// Retroactively create upload_scenes jobs for files that were indexed
	// before cloud was enabled.  This is a one-shot best-effort pass.
	if r.cloudClient != nil && r.pipeRunner != nil {
		r.backfillCloudUploads(ctx)
	}
	if r.pipeRunner != nil && r.ffmpeg != nil {
		r.backfillThumbnails(ctx)
	}

	ticker := time.NewTicker(r.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("job runner stopping")
			r.running.Store(false)
			return
		case <-ticker.C:
			if !r.paused.Load() {
				r.processNextJob(ctx)
			}
		}
	}
}

func (r *Runner) Pause() {
	r.paused.Store(true)
	r.logger.Info("job runner paused")
}

func (r *Runner) Resume() {
	r.paused.Store(false)
	r.logger.Info("job runner resumed")
}

func (r *Runner) IsPaused() bool {
	return r.paused.Load()
}

func (r *Runner) IsRunning() bool {
	return r.running.Load()
}

func (r *Runner) processNextJob(ctx context.Context) {
	jobs, err := r.repo.ListPendingJobs(ctx)
	if err != nil {
		r.logger.Error("failed to list pending jobs", "error", err)
		return
	}

	if len(jobs) == 0 {
		return
	}

	job := jobs[0]
	r.logger.Info("processing job", "job_id", job.ID, "type", job.Type)

	switch job.Type {
	case JobTypeScan:
		source, err := r.repo.GetSource(ctx, job.SourceID)
		if err != nil || source == nil {
			r.repo.UpdateJobStatus(ctx, job.ID, JobStatusFailed, "source not found")
			return
		}

		if err := r.service.ExecuteScan(ctx, job.ID, source.ID, source.Path); err != nil {
			r.logger.Error("scan failed", "job_id", job.ID, "error", err)
		}

	case JobTypeIndex:
		r.processIndexJob(ctx, job)

	case JobTypeUploadScenes:
		r.processUploadScenesJob(ctx, job)

	case JobTypeGenerateThumbnails:
		r.processGenerateThumbnailsJob(ctx, job)

	default:
		r.logger.Warn("unknown job type", "type", job.Type)
		r.repo.UpdateJobStatus(ctx, job.ID, JobStatusFailed, "unknown job type")
	}
}

func (r *Runner) backfillThumbnails(ctx context.Context) {
	files, err := r.repo.ListFiles(ctx)
	if err != nil {
		r.logger.Warn("thumbnail backfill: cannot list files", "error", err)
		return
	}

	jobs, err := r.repo.ListJobs(ctx, 1000)
	if err != nil {
		return
	}

	hasThumbJob := make(map[string]bool)
	for _, j := range jobs {
		if j.Type == JobTypeGenerateThumbnails {
			hasThumbJob[j.FileID] = true
		}
	}

	for _, file := range files {
		if hasThumbJob[file.ID] {
			continue
		}
		scenePath := filepath.Join(r.pipeRunner.ArtifactsDir(), file.ID, "scenes", "result.json")
		if _, err := os.Stat(scenePath); os.IsNotExist(err) {
			continue
		}
		thumbDir := filepath.Join(r.pipeRunner.ArtifactsDir(), file.ID, "thumbnails")
		if info, err := os.Stat(thumbDir); err == nil && info.IsDir() {
			entries, _ := os.ReadDir(thumbDir)
			if len(entries) > 0 {
				continue
			}
		}
		job := &Job{
			ID:        NewID(),
			Type:      JobTypeGenerateThumbnails,
			Status:    JobStatusPending,
			FileID:    file.ID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := r.repo.CreateJob(ctx, job); err != nil {
			r.logger.Warn("thumbnail backfill: create job failed", "file_id", file.ID, "error", err)
		}
	}
}

func (r *Runner) processIndexJob(ctx context.Context, job *Job) {
	if r.pipeRunner == nil || r.doctor == nil {
		r.repo.UpdateJobStatus(ctx, job.ID, JobStatusFailed, "pipeline runner not configured")
		return
	}

	if err := ctx.Err(); err != nil {
		r.repo.UpdateJobStatus(context.WithoutCancel(ctx), job.ID, JobStatusFailed, fmt.Sprintf("index job cancelled: %v", err))
		return
	}

	file, err := r.repo.GetFile(ctx, job.FileID)
	if err != nil || file == nil {
		r.repo.UpdateJobStatus(ctx, job.ID, JobStatusFailed, "file not found")
		return
	}

	r.repo.UpdateJobStatus(ctx, job.ID, JobStatusRunning, "")

	caps, err := r.doctor.Get(ctx)
	if err != nil {
		r.repo.UpdateJobStatus(ctx, job.ID, JobStatusFailed, fmt.Sprintf("doctor probe failed: %v", err))
		return
	}

	if !caps.HasSpeech && !caps.HasFaces && !caps.HasScenes {
		r.repo.UpdateJobStatus(ctx, job.ID, JobStatusFailed, "no pipeline capabilities available")
		return
	}

	artifactsBase := filepath.Join(r.pipeRunner.ArtifactsDir(), file.ID)
	totalSteps := 0
	if caps.HasSpeech {
		totalSteps++
	}
	if caps.HasFaces {
		totalSteps++
	}
	if caps.HasScenes {
		totalSteps++
	}
	completedSteps := 0

	speechOutPath := filepath.Join(artifactsBase, "speech", "result.json")
	speechOK := false

	if caps.HasSpeech {
		r.logger.Info("running speech pipeline", "job_id", job.ID, "file_id", file.ID)

		result, err := r.pipeRunner.RunSpeech(ctx, file.Path, speechOutPath)
		if err != nil {
			r.repo.UpdateJobStatus(ctx, job.ID, JobStatusFailed, fmt.Sprintf("speech pipeline error: %v", err))
			return
		}
		if !result.IsSuccess() {
			r.repo.UpdateJobStatus(ctx, job.ID, JobStatusFailed,
				fmt.Sprintf("speech pipeline exited %d: %s", result.ExitCode, truncateStr(result.StderrTail, 512)))
			return
		}

		if _, err := r.pipeRunner.ValidateOutput(speechOutPath); err != nil {
			r.repo.UpdateJobStatus(ctx, job.ID, JobStatusFailed, fmt.Sprintf("speech output invalid: %v", err))
			return
		}

		speechOK = true
		completedSteps++
		r.repo.UpdateJobProgress(ctx, job.ID, completedSteps*100/totalSteps)
		r.logger.Info("speech pipeline completed", "job_id", job.ID, "duration", result.Duration)
	}

	// Faces and scenes run in parallel (both depend on speech, not on each other)
	runFaces := caps.HasFaces
	runScenes := caps.HasScenes && speechOK

	if runFaces || runScenes {
		type stepResult struct {
			name string
			err  error
		}

		parallelCtx, parallelCancel := context.WithCancel(ctx)
		defer parallelCancel()

		var wg sync.WaitGroup
		results := make(chan stepResult, 2)

		if runFaces {
			wg.Add(1)
			go func() {
				defer wg.Done()
				outPath := filepath.Join(artifactsBase, "faces", "result.json")
				r.logger.Info("running faces pipeline", "job_id", job.ID, "file_id", file.ID)

				result, err := r.pipeRunner.RunFaces(parallelCtx, file.Path, outPath)
				if err != nil {
					results <- stepResult{"faces", fmt.Errorf("faces pipeline error: %w", err)}
					return
				}
				if !result.IsSuccess() {
					results <- stepResult{"faces", fmt.Errorf("faces pipeline exited %d: %s", result.ExitCode, truncateStr(result.StderrTail, 512))}
					return
				}
				if _, err := r.pipeRunner.ValidateOutput(outPath); err != nil {
					results <- stepResult{"faces", fmt.Errorf("faces output invalid: %w", err)}
					return
				}
				r.logger.Info("faces pipeline completed", "job_id", job.ID, "duration", result.Duration)
				results <- stepResult{"faces", nil}
			}()
		}

		if runScenes {
			wg.Add(1)
			go func() {
				defer wg.Done()
				outPath := filepath.Join(artifactsBase, "scenes", "result.json")
				r.logger.Info("running scenes pipeline", "job_id", job.ID, "file_id", file.ID)

				result, err := r.pipeRunner.RunScenes(parallelCtx, file.Path, file.ID, speechOutPath, outPath)
				if err != nil {
					results <- stepResult{"scenes", fmt.Errorf("scenes pipeline error: %w", err)}
					return
				}
				if !result.IsSuccess() {
					results <- stepResult{"scenes", fmt.Errorf("scenes pipeline exited %d: %s", result.ExitCode, truncateStr(result.StderrTail, 512))}
					return
				}
				if _, err := r.pipeRunner.ValidateSceneOutput(outPath); err != nil {
					results <- stepResult{"scenes", fmt.Errorf("scenes output invalid: %w", err)}
					return
				}
				r.logger.Info("scenes pipeline completed", "job_id", job.ID, "duration", result.Duration)
				results <- stepResult{"scenes", nil}
			}()
		}

		go func() {
			wg.Wait()
			close(results)
		}()

		var firstErr error
		for sr := range results {
			if sr.err != nil {
				if firstErr == nil {
					firstErr = sr.err
					parallelCancel()
				}
				continue
			}
			if firstErr == nil {
				completedSteps++
				r.repo.UpdateJobProgress(ctx, job.ID, completedSteps*100/totalSteps)
			}
		}

		if firstErr != nil {
			r.repo.UpdateJobStatus(ctx, job.ID, JobStatusFailed, firstErr.Error())
			return
		}
	}

	r.repo.UpdateJobStatus(ctx, job.ID, JobStatusCompleted, "")
	r.logger.Info("index job completed", "job_id", job.ID, "file_id", file.ID)

	if r.cloudClient != nil && caps.HasScenes && speechOK {
		r.uploadScenesToCloud(ctx, job, file, artifactsBase)
	}
}

// buildSceneIngestDocs converts pipeline SceneBoundary output into the cloud
// ingest payload. This is the single mapping point — every field the SaaS
// accepts should be forwarded here.  Missing fields in older pipeline outputs
// default safely (empty string / empty slice / zero).
func buildSceneIngestDocs(scenes []pipelines.SceneBoundary, sourceType string) []cloud.SceneIngestDoc {
	docs := make([]cloud.SceneIngestDoc, 0, len(scenes))
	for _, s := range scenes {
		docs = append(docs, cloud.SceneIngestDoc{
			SceneID:             s.SceneID,
			Index:               s.Index,
			StartMs:             s.StartMs,
			EndMs:               s.EndMs,
			KeyframeTimestampMs: s.KeyframeTimestampMs,
			TranscriptRaw:       s.TranscriptRaw,
			SpeechSegmentCount:  s.SpeechSegmentCount,
			PeopleClusterIDs:    s.PeopleClusterIDs,
			KeywordTags:         s.KeywordTags,
			ProductTags:         s.ProductTags,
			ProductEntities:     s.ProductEntities,
			SourceType:          sourceType,
		})
	}
	return docs
}

// resolveSourceType maps the agent's internal Source.Type to the cloud API
// source_type value. Unknown source types default to "local".
func resolveSourceType(source *Source) string {
	if source == nil {
		return "local"
	}
	switch source.Type {
	case "gdrive":
		return "gdrive"
	case "removable_disk":
		return "removable_disk"
	default:
		// "folder" and any future local source types map to "local"
		return "local"
	}
}

func (r *Runner) uploadScenesToCloud(ctx context.Context, job *Job, file *File, artifactsBase string) {
	scenePath := filepath.Join(artifactsBase, "scenes", "result.json")

	data, err := os.ReadFile(scenePath)
	if err != nil {
		r.logger.Warn("scene upload skipped: cannot read scene output", "job_id", job.ID, "error", err)
		return
	}

	var sceneOutput pipelines.SceneOutputPayload
	if err := json.Unmarshal(data, &sceneOutput); err != nil {
		r.logger.Warn("scene upload skipped: invalid scene JSON", "job_id", job.ID, "error", err)
		return
	}

	if len(sceneOutput.Scenes) == 0 {
		r.logger.Info("scene upload skipped: no scenes detected", "job_id", job.ID)
		return
	}

	source, _ := r.repo.GetSource(ctx, file.SourceID)
	libraryID, err := r.resolveLibraryID(ctx, source)
	if err != nil {
		r.logger.Warn("scene upload skipped: no library available", "job_id", job.ID, "error", err)
		return
	}
	sourceType := resolveSourceType(source)
	scenes := buildSceneIngestDocs(sceneOutput.Scenes, sourceType)

	payload := cloud.SceneIngestPayload{
		VideoID:         sceneOutput.VideoID,
		VideoTitle:      strings.TrimSuffix(file.Filename, filepath.Ext(file.Filename)),
		LibraryID:       libraryID,
		PipelineVersion: sceneOutput.PipelineVersion,
		ModelVersion:    sceneOutput.ModelVersion,
		TotalDurationMs: sceneOutput.TotalDurationMs,
		Scenes:          scenes,
	}

	uploadCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := r.cloudClient.Scenes().UploadScenes(uploadCtx, payload); err != nil {
		r.logger.Warn("scene upload failed (non-blocking)", "job_id", job.ID, "video_id", sceneOutput.VideoID, "error", err)

		var uploadErr *cloud.UploadError
		if errors.As(err, &uploadErr) && !uploadErr.IsRetryable() {
			r.logger.Warn("scene upload permanent failure, no retry",
				"job_id", job.ID, "status_code", uploadErr.StatusCode, "error", err)
			return
		}

		retryJob := &Job{
			ID:        NewID(),
			Type:      JobTypeUploadScenes,
			Status:    JobStatusPending,
			FileID:    file.ID,
			Progress:  0,
			Error:     err.Error(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if createErr := r.repo.CreateJob(ctx, retryJob); createErr != nil {
			r.logger.Error("failed to create upload retry job", "error", createErr)
		} else {
			r.logger.Info("created upload retry job", "retry_job_id", retryJob.ID, "file_id", file.ID)
		}
		return
	}

	r.logger.Info("scene upload succeeded", "job_id", job.ID, "video_id", sceneOutput.VideoID, "scene_count", len(scenes))

	// Record a completed upload_scenes job so backfillCloudUploads won't
	// create a duplicate upload for this file on next restart.
	now := time.Now()
	uploadJob := &Job{
		ID:        NewID(),
		Type:      JobTypeUploadScenes,
		Status:    JobStatusCompleted,
		FileID:    file.ID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if createErr := r.repo.CreateJob(ctx, uploadJob); createErr != nil {
		r.logger.Warn("failed to record upload job (non-critical)", "file_id", file.ID, "error", createErr)
	}
}

func uploadBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		return 10 * time.Second
	}
	backoff := 10 * time.Second
	for i := 0; i < attempt; i++ {
		backoff *= 3
	}
	const maxBackoff = 10 * time.Minute
	if backoff > maxBackoff {
		backoff = maxBackoff
	}
	return backoff
}

func (r *Runner) processUploadScenesJob(ctx context.Context, job *Job) {
	const maxRetries = 5

	delay := uploadBackoff(job.Progress)
	if time.Since(job.UpdatedAt) < delay {
		return
	}

	attempt := job.Progress + 1
	if attempt > maxRetries {
		r.repo.UpdateJobStatus(ctx, job.ID, JobStatusFailed, fmt.Sprintf("max retries (%d) exceeded: %s", maxRetries, job.Error))
		r.logger.Warn("upload retry abandoned", "job_id", job.ID, "attempts", attempt)
		return
	}

	if r.pipeRunner == nil {
		r.repo.UpdateJobStatus(ctx, job.ID, JobStatusFailed, "pipeline runner not configured")
		return
	}
	if r.cloudClient == nil {
		r.repo.UpdateJobStatus(ctx, job.ID, JobStatusFailed, "cloud client not configured")
		return
	}

	file, err := r.repo.GetFile(ctx, job.FileID)
	if err != nil || file == nil {
		r.repo.UpdateJobStatus(ctx, job.ID, JobStatusFailed, "file not found for retry")
		return
	}

	r.repo.UpdateJobStatus(ctx, job.ID, JobStatusRunning, "")
	r.repo.UpdateJobProgress(ctx, job.ID, attempt)

	artifactsBase := filepath.Join(r.pipeRunner.ArtifactsDir(), file.ID)
	r.uploadScenesToCloudRetry(ctx, job, file, artifactsBase, attempt)
}

func (r *Runner) processGenerateThumbnailsJob(ctx context.Context, job *Job) {
	if r.pipeRunner == nil || r.ffmpeg == nil {
		r.repo.UpdateJobStatus(ctx, job.ID, JobStatusFailed, "thumbnail generation not configured")
		return
	}

	r.repo.UpdateJobStatus(ctx, job.ID, JobStatusRunning, "")

	file, err := r.repo.GetFile(ctx, job.FileID)
	if err != nil || file == nil {
		r.repo.UpdateJobStatus(ctx, job.ID, JobStatusFailed, "file not found")
		return
	}

	scenePath := filepath.Join(r.pipeRunner.ArtifactsDir(), file.ID, "scenes", "result.json")
	data, err := os.ReadFile(scenePath)
	if err != nil {
		r.repo.UpdateJobStatus(ctx, job.ID, JobStatusFailed, "cannot read scene result: "+err.Error())
		return
	}

	var sceneOutput pipelines.SceneOutputPayload
	if err := json.Unmarshal(data, &sceneOutput); err != nil {
		r.repo.UpdateJobStatus(ctx, job.ID, JobStatusFailed, "invalid scene JSON")
		return
	}

	thumbDir := filepath.Join(r.pipeRunner.ArtifactsDir(), file.ID, "thumbnails")
	os.MkdirAll(thumbDir, 0o755)

	generated := 0
	for _, scene := range sceneOutput.Scenes {
		outPath := filepath.Join(thumbDir, scene.SceneID+".jpg")
		if _, err := os.Stat(outPath); err == nil {
			generated++
			continue
		}
		ts := float64(scene.KeyframeTimestampMs) / 1000.0
		if err := r.ffmpeg.GenerateThumbnail(file.Path, outPath, ts); err != nil {
			r.logger.Warn("thumbnail generation failed", "scene_id", scene.SceneID, "error", err)
			continue
		}
		generated++
	}

	r.logger.Info("thumbnails generated", "file_id", file.ID, "count", generated, "total", len(sceneOutput.Scenes))
	r.repo.UpdateJobStatus(ctx, job.ID, JobStatusCompleted, "")
}

func (r *Runner) uploadScenesToCloudRetry(ctx context.Context, job *Job, file *File, artifactsBase string, attempt int) {
	scenePath := filepath.Join(artifactsBase, "scenes", "result.json")

	data, err := os.ReadFile(scenePath)
	if err != nil {
		r.repo.UpdateJobStatus(ctx, job.ID, JobStatusFailed, fmt.Sprintf("cannot read scene output: %v", err))
		return
	}

	var sceneOutput pipelines.SceneOutputPayload
	if err := json.Unmarshal(data, &sceneOutput); err != nil {
		r.repo.UpdateJobStatus(ctx, job.ID, JobStatusFailed, fmt.Sprintf("invalid scene JSON: %v", err))
		return
	}

	if len(sceneOutput.Scenes) == 0 {
		r.repo.UpdateJobStatus(ctx, job.ID, JobStatusCompleted, "")
		return
	}

	source, _ := r.repo.GetSource(ctx, file.SourceID)
	libraryID, err := r.resolveLibraryID(ctx, source)
	if err != nil {
		r.repo.UpdateJobStatus(ctx, job.ID, JobStatusFailed, fmt.Sprintf("no library available: %v", err))
		return
	}
	sourceType := resolveSourceType(source)
	scenes := buildSceneIngestDocs(sceneOutput.Scenes, sourceType)

	payload := cloud.SceneIngestPayload{
		VideoID:         sceneOutput.VideoID,
		VideoTitle:      strings.TrimSuffix(file.Filename, filepath.Ext(file.Filename)),
		LibraryID:       libraryID,
		PipelineVersion: sceneOutput.PipelineVersion,
		ModelVersion:    sceneOutput.ModelVersion,
		TotalDurationMs: sceneOutput.TotalDurationMs,
		Scenes:          scenes,
	}

	uploadCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := r.cloudClient.Scenes().UploadScenes(uploadCtx, payload); err != nil {
		var uploadErr *cloud.UploadError
		if errors.As(err, &uploadErr) && !uploadErr.IsRetryable() {
			r.repo.UpdateJobStatus(ctx, job.ID, JobStatusFailed, fmt.Sprintf("permanent error (HTTP %d): %s", uploadErr.StatusCode, uploadErr.Body))
			return
		}

		r.logger.Warn("upload retry failed", "job_id", job.ID, "attempt", attempt, "error", err)
		r.repo.UpdateJobStatus(ctx, job.ID, JobStatusPending, err.Error())
		return
	}

	r.repo.UpdateJobStatus(ctx, job.ID, JobStatusCompleted, "")
	r.logger.Info("upload retry succeeded", "job_id", job.ID, "attempt", attempt, "video_id", sceneOutput.VideoID)
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[len(s)-maxLen:]
}

func (r *Runner) GetActiveJobCount(ctx context.Context) int {
	jobs, err := r.repo.ListJobs(ctx, 100)
	if err != nil {
		return 0
	}
	count := 0
	for _, j := range jobs {
		if j.Status == JobStatusRunning {
			count++
		}
	}
	return count
}
