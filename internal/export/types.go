package export

type ExportRequest struct {
	ProjectName string      `json:"project_name"`
	Format      string      `json:"format"`
	FrameRate   float64     `json:"frame_rate"`
	OutputDir   string      `json:"output_dir"`
	Clips       []ClipInput `json:"clips"`
}

type ClipInput struct {
	VideoID  string `json:"video_id"`
	SceneID  string `json:"scene_id"`
	ClipName string `json:"clip_name"`
	StartMs  int    `json:"start_ms"`
	EndMs    int    `json:"end_ms"`
}

type ResolvedClip struct {
	ClipName  string
	MediaPath string
	StartMs   int
	EndMs     int
	SceneID   string
}

type ExportResponse struct {
	Status          string   `json:"status"`
	Format          string   `json:"format"`
	OutputPath      string   `json:"output_path"`
	ClipCount       int      `json:"clip_count"`
	UnresolvedClips []string `json:"unresolved_clips"`
}
