package catalog

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
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

		var wg sync.WaitGroup
		results := make(chan stepResult, 2)

		if runFaces {
			wg.Add(1)
			go func() {
				defer wg.Done()
				outPath := filepath.Join(artifactsBase, "faces", "result.json")
				r.logger.Info("running faces pipeline", "job_id", job.ID, "file_id", file.ID)

				result, err := r.pipeRunner.RunFaces(ctx, file.Path, outPath)
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

				result, err := r.pipeRunner.RunScenes(ctx, file.Path, speechOutPath, outPath)
				if err != nil {
					results <- stepResult{"scenes", fmt.Errorf("scenes pipeline error: %w", err)}
					return
				}
				if !result.IsSuccess() {
					results <- stepResult{"scenes", fmt.Errorf("scenes pipeline exited %d: %s", result.ExitCode, truncateStr(result.StderrTail, 512))}
					return
				}
				if _, err := r.pipeRunner.ValidateOutput(outPath); err != nil {
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

		for sr := range results {
			if sr.err != nil {
				r.repo.UpdateJobStatus(ctx, job.ID, JobStatusFailed, sr.err.Error())
				return
			}
			completedSteps++
			r.repo.UpdateJobProgress(ctx, job.ID, completedSteps*100/totalSteps)
		}
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
