package cloud

// SceneIngestPayload is the request body sent to POST /api/ingest/scenes.
// Matches the SaaS IngestScenesRequest Pydantic schema.
type SceneIngestPayload struct {
	VideoID         string           `json:"video_id"`
	LibraryID       string           `json:"library_id"`
	PipelineVersion string           `json:"pipeline_version,omitempty"`
	ModelVersion    string           `json:"model_version,omitempty"`
	TotalDurationMs int              `json:"total_duration_ms,omitempty"`
	Scenes          []SceneIngestDoc `json:"scenes"`
}

type SceneIngestDoc struct {
	SceneID               string   `json:"scene_id"`
	Index                 int      `json:"index"`
	StartMs               int      `json:"start_ms"`
	EndMs                 int      `json:"end_ms"`
	KeyframeTimestampMs   int      `json:"keyframe_timestamp_ms,omitempty"`
	TranscriptRaw         string   `json:"transcript_raw,omitempty"`
	SpeechSegmentCount    int      `json:"speech_segment_count,omitempty"`
	PeopleClusterIDs      []string `json:"people_cluster_ids,omitempty"`
	SourceType            string   `json:"source_type,omitempty"`
	RequiredDriveNickname string   `json:"required_drive_nickname,omitempty"`
}

// SceneIngestResponse is the response from POST /api/ingest/scenes.
type SceneIngestResponse struct {
	IndexedCount int    `json:"indexed_count"`
	VideoID      string `json:"video_id"`
	SkippedCount int    `json:"skipped_count"`
}
