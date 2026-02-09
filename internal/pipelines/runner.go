package pipelines

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	maxStderrBytes = 8 * 1024 // 8 KB tail of stderr kept for diagnostics
)

// Runner executes Python pipeline commands as subprocesses.
// It is the single implementation of the pipeline execution contract
// used throughout the agent.
type Runner interface {
	// RunDoctor executes `python -m <module> doctor --json --out <path>` and
	// returns parsed capabilities.
	RunDoctor(ctx context.Context) (*Capabilities, error)

	// RunSpeech executes the speech pipeline for a video file.
	RunSpeech(ctx context.Context, videoPath, outPath string) (RunResult, error)

	// RunFaces executes the face detection pipeline for a video file.
	RunFaces(ctx context.Context, videoPath, outPath string) (RunResult, error)

	// RunScenes executes the scene detection pipeline for a video file.
	// It requires the speech result path so scenes can aggregate transcripts.
	RunScenes(ctx context.Context, videoPath, speechResultPath, outPath string) (RunResult, error)

	// ValidateOutput reads a pipeline output JSON and checks required fields.
	ValidateOutput(path string) (*PipelineOutput, error)

	// ArtifactsDir returns the base directory for pipeline outputs.
	ArtifactsDir() string
}

// Config holds the runner's configuration.
type Config struct {
	PythonPath    string        // path to python binary; empty = auto-detect
	ModuleName    string        // default "heimdex_media_pipelines"
	ArtifactsBase string        // base dir for outputs, e.g. ~/.heimdex/artifacts
	DoctorTimeout time.Duration // timeout for doctor command
	SpeechTimeout time.Duration // timeout for speech pipeline
	FacesTimeout  time.Duration // timeout for faces pipeline
	ScenesTimeout time.Duration // timeout for scenes pipeline
	Logger        *slog.Logger
	DebugPaths    bool // if true, log full file paths; otherwise sanitise
}

// DefaultConfig returns production-ready defaults.
func DefaultConfig(dataDir string, logger *slog.Logger) Config {
	return Config{
		PythonPath:    "", // auto-detect
		ModuleName:    "heimdex_media_pipelines",
		ArtifactsBase: filepath.Join(dataDir, "artifacts"),
		DoctorTimeout: 30 * time.Second,
		SpeechTimeout: 30 * time.Minute,
		FacesTimeout:  15 * time.Minute,
		ScenesTimeout: 10 * time.Minute,
		Logger:        logger,
		DebugPaths:    false,
	}
}

// SubprocessRunner is the production implementation of Runner.
type SubprocessRunner struct {
	cfg    Config
	python string // resolved python path
}

// NewRunner creates a SubprocessRunner, resolving the Python binary path.
func NewRunner(cfg Config) (*SubprocessRunner, error) {
	python, err := resolvePython(cfg.PythonPath)
	if err != nil {
		return nil, fmt.Errorf("cannot locate python: %w", err)
	}

	if err := os.MkdirAll(cfg.ArtifactsBase, 0755); err != nil {
		return nil, fmt.Errorf("cannot create artifacts dir: %w", err)
	}

	cfg.Logger.Info("pipeline runner initialised",
		"python", python,
		"module", cfg.ModuleName,
		"artifacts_dir", cfg.ArtifactsBase,
	)

	return &SubprocessRunner{cfg: cfg, python: python}, nil
}

func (r *SubprocessRunner) ArtifactsDir() string {
	return r.cfg.ArtifactsBase
}

// RunDoctor probes the installed pipelines environment.
func (r *SubprocessRunner) RunDoctor(ctx context.Context) (*Capabilities, error) {
	outPath := filepath.Join(r.cfg.ArtifactsBase, ".doctor.json")

	ctx, cancel := context.WithTimeout(ctx, r.cfg.DoctorTimeout)
	defer cancel()

	result := r.exec(ctx, outPath, "doctor", "--json", "--out", outPath)
	if !result.IsSuccess() {
		return nil, fmt.Errorf("doctor exited %d: %s", result.ExitCode, result.StderrTail)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read doctor output: %w", err)
	}

	var caps Capabilities
	if err := json.Unmarshal(data, &caps); err != nil {
		return nil, fmt.Errorf("cannot parse doctor JSON: %w", err)
	}

	// Derive capability flags
	caps.HasFaces = isAvailable(caps.Dependencies, "cv2") &&
		isAvailable(caps.Dependencies, "insightface")
	caps.HasSpeech = isAvailable(caps.Dependencies, "whisper") &&
		isAvailable(caps.Executables, "ffmpeg")
	caps.HasScenes = isAvailable(caps.Executables, "ffmpeg")
	caps.ProbedAt = time.Now()

	r.cfg.Logger.Info("doctor probe complete",
		"faces", caps.HasFaces,
		"speech", caps.HasSpeech,
		"scenes", caps.HasScenes,
		"deps_available", caps.Summary.Available,
		"deps_total", caps.Summary.Total,
	)

	return &caps, nil
}

// RunSpeech runs the speech pipeline CLI.
func (r *SubprocessRunner) RunSpeech(ctx context.Context, videoPath, outPath string) (RunResult, error) {
	ctx, cancel := context.WithTimeout(ctx, r.cfg.SpeechTimeout)
	defer cancel()

	result := r.exec(ctx, outPath,
		"speech", "pipeline",
		"--video", videoPath,
		"--out", outPath,
	)
	return result, nil
}

// RunFaces runs the face detection pipeline CLI.
func (r *SubprocessRunner) RunFaces(ctx context.Context, videoPath, outPath string) (RunResult, error) {
	ctx, cancel := context.WithTimeout(ctx, r.cfg.FacesTimeout)
	defer cancel()

	result := r.exec(ctx, outPath,
		"faces", "detect",
		"--video", videoPath,
		"--fps", "1.0",
		"--out", outPath,
	)
	return result, nil
}

func (r *SubprocessRunner) RunScenes(ctx context.Context, videoPath, speechResultPath, outPath string) (RunResult, error) {
	ctx, cancel := context.WithTimeout(ctx, r.cfg.ScenesTimeout)
	defer cancel()

	result := r.exec(ctx, outPath,
		"scenes", "pipeline",
		"--video", videoPath,
		"--speech-result", speechResultPath,
		"--out", outPath,
	)
	return result, nil
}

// ValidateOutput reads a pipeline JSON output and checks required metadata fields.
func (r *SubprocessRunner) ValidateOutput(path string) (*PipelineOutput, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read output file %s: %w", r.safePath(path), err)
	}

	var out PipelineOutput
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("cannot parse output JSON: %w", err)
	}

	if !out.RequiredFieldsPresent() {
		missing := []string{}
		if out.SchemaVersion == "" {
			missing = append(missing, "schema_version")
		}
		if out.PipelineVersion == "" {
			missing = append(missing, "pipeline_version")
		}
		if out.ModelVersion == "" {
			missing = append(missing, "model_version")
		}
		return &out, fmt.Errorf("pipeline output missing required fields: %s", strings.Join(missing, ", "))
	}

	return &out, nil
}

// exec is the core subprocess execution helper.
func (r *SubprocessRunner) exec(ctx context.Context, outPath string, args ...string) RunResult {
	start := time.Now()

	// Ensure output directory exists
	if outPath != "" {
		if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
			r.cfg.Logger.Error("cannot create output dir", "error", err)
			return RunResult{ExitCode: -1, StderrTail: err.Error(), Duration: time.Since(start)}
		}
	}

	cmdArgs := append([]string{"-m", r.cfg.ModuleName}, args...)
	cmd := exec.CommandContext(ctx, r.python, cmdArgs...)

	// Capture stderr with bounded buffer
	var stderrBuf bytes.Buffer
	cmd.Stderr = io.Writer(&limitedWriter{w: &stderrBuf, limit: maxStderrBytes})
	cmd.Stdout = io.Discard // CLI writes to --out file, not stdout

	r.cfg.Logger.Info("executing pipeline command",
		"args", cmdArgs,
		"timeout", ctx.Deadline,
	)

	err := cmd.Run()
	elapsed := time.Since(start)

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	stderrTail := stderrBuf.String()

	if exitCode != 0 {
		r.cfg.Logger.Warn("pipeline command failed",
			"exit_code", exitCode,
			"duration_ms", elapsed.Milliseconds(),
			"stderr_tail", truncate(stderrTail, 512),
		)
	} else {
		r.cfg.Logger.Info("pipeline command succeeded",
			"duration_ms", elapsed.Milliseconds(),
			"output", r.safePath(outPath),
		)
	}

	return RunResult{
		ExitCode:   exitCode,
		OutputPath: outPath,
		StderrTail: stderrTail,
		Duration:   elapsed,
	}
}

func (r *SubprocessRunner) safePath(path string) string {
	if r.cfg.DebugPaths {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Base(path)
	}
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return filepath.Base(path)
}

// resolvePython finds a usable python binary.
func resolvePython(preferred string) (string, error) {
	if preferred != "" {
		if p, err := exec.LookPath(preferred); err == nil {
			return p, nil
		}
		return "", fmt.Errorf("configured python %q not found", preferred)
	}
	for _, name := range []string{"python3", "python"} {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("no python binary found on PATH (tried python3, python)")
}

func isAvailable(deps map[string]DepInfo, name string) bool {
	d, ok := deps[name]
	return ok && d.Available
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return "..." + s[len(s)-maxLen:]
}

// limitedWriter is an io.Writer that keeps only the last `limit` bytes.
type limitedWriter struct {
	w     *bytes.Buffer
	limit int
}

func (lw *limitedWriter) Write(p []byte) (int, error) {
	n := len(p)
	lw.w.Write(p)
	if lw.w.Len() > lw.limit {
		// Keep only the tail
		b := lw.w.Bytes()
		lw.w.Reset()
		lw.w.Write(b[len(b)-lw.limit:])
	}
	return n, nil
}
