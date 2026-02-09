package catalog

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/heimdex/heimdex-agent/internal/pipelines"
)

type Runner struct {
	service      *Service
	repo         Repository
	pipeRunner   pipelines.Runner
	doctor       *pipelines.CachedDoctor
	logger       *slog.Logger
	pollInterval time.Duration
	running      atomic.Bool
	paused       atomic.Bool
}

func NewRunner(service *Service, repo Repository, pipeRunner pipelines.Runner, doctor *pipelines.CachedDoctor, logger *slog.Logger) *Runner {
	return &Runner{
		service:      service,
		repo:         repo,
		pipeRunner:   pipeRunner,
		doctor:       doctor,
		logger:       logger,
		pollInterval: 5 * time.Second,
	}
}

func (r *Runner) Start(ctx context.Context) {
	if r.running.Swap(true) {
		return
	}

	r.logger.Info("job runner started")

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

	default:
		r.logger.Warn("unknown job type", "type", job.Type)
		r.repo.UpdateJobStatus(ctx, job.ID, JobStatusFailed, "unknown job type")
	}
}

func (r *Runner) processIndexJob(ctx context.Context, job *Job) {
	if r.pipeRunner == nil || r.doctor == nil {
		r.repo.UpdateJobStatus(ctx, job.ID, JobStatusFailed, "pipeline runner not configured")
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

	if !caps.HasSpeech && !caps.HasFaces {
		r.repo.UpdateJobStatus(ctx, job.ID, JobStatusFailed, "no pipeline capabilities available")
		return
	}

	artifactsBase := filepath.Join(r.pipeRunner.ArtifactsDir(), file.ID)
	progress := 0
	totalSteps := 0
	if caps.HasSpeech {
		totalSteps++
	}
	if caps.HasFaces {
		totalSteps++
	}
	completedSteps := 0

	if caps.HasSpeech {
		outPath := filepath.Join(artifactsBase, "speech", "result.json")
		r.logger.Info("running speech pipeline", "job_id", job.ID, "file_id", file.ID)

		result, err := r.pipeRunner.RunSpeech(ctx, file.Path, outPath)
		if err != nil {
			r.repo.UpdateJobStatus(ctx, job.ID, JobStatusFailed, fmt.Sprintf("speech pipeline error: %v", err))
			return
		}
		if !result.IsSuccess() {
			r.repo.UpdateJobStatus(ctx, job.ID, JobStatusFailed,
				fmt.Sprintf("speech pipeline exited %d: %s", result.ExitCode, truncateStr(result.StderrTail, 512)))
			return
		}

		if _, err := r.pipeRunner.ValidateOutput(outPath); err != nil {
			r.repo.UpdateJobStatus(ctx, job.ID, JobStatusFailed, fmt.Sprintf("speech output invalid: %v", err))
			return
		}

		completedSteps++
		progress = completedSteps * 100 / totalSteps
		r.repo.UpdateJobProgress(ctx, job.ID, progress)
		r.logger.Info("speech pipeline completed", "job_id", job.ID, "duration", result.Duration)
	}

	if caps.HasFaces {
		outPath := filepath.Join(artifactsBase, "faces", "result.json")
		r.logger.Info("running faces pipeline", "job_id", job.ID, "file_id", file.ID)

		result, err := r.pipeRunner.RunFaces(ctx, file.Path, outPath)
		if err != nil {
			r.repo.UpdateJobStatus(ctx, job.ID, JobStatusFailed, fmt.Sprintf("faces pipeline error: %v", err))
			return
		}
		if !result.IsSuccess() {
			r.repo.UpdateJobStatus(ctx, job.ID, JobStatusFailed,
				fmt.Sprintf("faces pipeline exited %d: %s", result.ExitCode, truncateStr(result.StderrTail, 512)))
			return
		}

		if _, err := r.pipeRunner.ValidateOutput(outPath); err != nil {
			r.repo.UpdateJobStatus(ctx, job.ID, JobStatusFailed, fmt.Sprintf("faces output invalid: %v", err))
			return
		}

		completedSteps++
		progress = completedSteps * 100 / totalSteps
		r.repo.UpdateJobProgress(ctx, job.ID, progress)
		r.logger.Info("faces pipeline completed", "job_id", job.ID, "duration", result.Duration)
	}

	r.repo.UpdateJobStatus(ctx, job.ID, JobStatusCompleted, "")
	r.logger.Info("index job completed", "job_id", job.ID, "file_id", file.ID)
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
