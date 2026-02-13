// Package pipelines provides subprocess-based execution of heimdex-media-pipelines
// Python CLI commands (doctor, faces, speech) with structured result parsing.
package pipelines

import "time"

// Capabilities represents what the installed Python pipelines can do,
// as reported by the `doctor --json` command.
type Capabilities struct {
	PackageVersion string             `json:"package_version"`
	Python         PythonInfo         `json:"python"`
	Dependencies   map[string]DepInfo `json:"dependencies"`
	Executables    map[string]DepInfo `json:"executables"`
	GPU            GPUInfo            `json:"gpu"`
	Summary        SummaryInfo        `json:"summary"`
	Pipelines      PipelinesInfo      `json:"pipelines"`

	HasFaces  bool      `json:"-"`
	HasSpeech bool      `json:"-"`
	HasScenes bool      `json:"-"`
	HasOCR    bool      `json:"-"`
	ProbedAt  time.Time `json:"-"`
}

// PipelinesInfo reports per-pipeline availability from doctor JSON.
type PipelinesInfo struct {
	Speech bool `json:"speech"`
	Faces  bool `json:"faces"`
	Scenes bool `json:"scenes"`
	OCR    bool `json:"ocr"`
}

// PythonInfo holds Python runtime information.
type PythonInfo struct {
	Version    string `json:"version"`
	Executable string `json:"executable"`
}

// DepInfo represents the availability status of a single dependency.
type DepInfo struct {
	Available bool   `json:"available"`
	Version   string `json:"version,omitempty"`
	Path      string `json:"path,omitempty"`
	Error     string `json:"error,omitempty"`
}

// GPUInfo holds GPU availability information.
type GPUInfo struct {
	CUDAAvailable bool   `json:"cuda_available"`
	DeviceCount   int    `json:"device_count,omitempty"`
	Error         string `json:"error,omitempty"`
}

// SummaryInfo summarises overall dependency status.
type SummaryInfo struct {
	Available int  `json:"available"`
	Total     int  `json:"total"`
	AllOK     bool `json:"all_ok"`
}

// RunResult is the structured outcome of executing a pipeline subprocess.
type RunResult struct {
	ExitCode   int           `json:"exit_code"`
	OutputPath string        `json:"output_path,omitempty"` // path to the --out JSON file
	StderrTail string        `json:"stderr_tail,omitempty"` // last N bytes of stderr
	Duration   time.Duration `json:"duration"`
}

// IsSuccess returns true when the subprocess exited cleanly.
func (r RunResult) IsSuccess() bool { return r.ExitCode == 0 }

// PipelineOutput represents the required metadata fields the agent validates
// in every pipeline JSON output file.
type PipelineOutput struct {
	SchemaVersion   string `json:"schema_version"`
	PipelineVersion string `json:"pipeline_version"`
	ModelVersion    string `json:"model_version"`
}

type SceneOutputPayload struct {
	PipelineOutput
	VideoID         string          `json:"video_id"`
	TotalDurationMs int             `json:"total_duration_ms"`
	Scenes          []SceneBoundary `json:"scenes"`
}

type SceneBoundary struct {
	SceneID             string   `json:"scene_id"`
	Index               int      `json:"index"`
	StartMs             int      `json:"start_ms"`
	EndMs               int      `json:"end_ms"`
	KeyframeTimestampMs int      `json:"keyframe_timestamp_ms"`
	TranscriptRaw       string   `json:"transcript_raw"`
	SpeechSegmentCount  int      `json:"speech_segment_count"`
	PeopleClusterIDs    []string `json:"people_cluster_ids"`
	KeywordTags         []string `json:"keyword_tags"`
	ProductTags         []string `json:"product_tags"`
	ProductEntities     []string `json:"product_entities"`
	OCRTextRaw          string   `json:"ocr_text_raw,omitempty"`
	OCRCharCount        int      `json:"ocr_char_count,omitempty"`
}

// RequiredFieldsPresent checks the hard invariants the agent enforces.
func (p PipelineOutput) RequiredFieldsPresent() bool {
	return p.SchemaVersion != "" && p.PipelineVersion != "" && p.ModelVersion != ""
}
