package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/heimdex/heimdex-agent/internal/export"
)

func exportPremiereHandler(cfg ServerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req export.ExportRequest
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&req); err != nil {
			WriteError(w, http.StatusBadRequest, "invalid request body", "BAD_REQUEST")
			return
		}

		if strings.ToLower(req.Format) != "edl" {
			WriteError(w, http.StatusBadRequest, "format must be edl", "BAD_REQUEST")
			return
		}

		if err := export.ValidateOutputDir(req.OutputDir); err != nil {
			WriteError(w, http.StatusBadRequest, err.Error(), "BAD_REQUEST")
			return
		}

		if len(req.Clips) == 0 {
			WriteError(w, http.StatusBadRequest, "clips must not be empty", "BAD_REQUEST")
			return
		}

		projectName := export.SanitizeName(req.ProjectName, 120)
		if projectName == "" {
			projectName = "heimdex_export"
		}

		frameRate := req.FrameRate
		if frameRate <= 0 {
			frameRate = 30.0
		}

		resolvedClips := make([]export.ResolvedClip, 0, len(req.Clips))
		unresolvedClips := make([]string, 0)

		for _, clip := range req.Clips {
			if clip.VideoID == "" {
				WriteError(w, http.StatusBadRequest, "video_id is required", "BAD_REQUEST")
				return
			}
			if clip.StartMs >= clip.EndMs {
				WriteError(w, http.StatusBadRequest, "start_ms must be less than end_ms", "BAD_REQUEST")
				return
			}

			file, err := cfg.CatalogService.GetFile(r.Context(), clip.VideoID)
			if err != nil {
				WriteError(w, http.StatusInternalServerError, err.Error(), "INTERNAL_ERROR")
				return
			}
			if file == nil {
				unresolvedClips = append(unresolvedClips, clip.VideoID)
				continue
			}

			clipName := export.SanitizeName(clip.ClipName, 160)
			if clipName == "" {
				clipName = clip.VideoID
			}

			resolvedClips = append(resolvedClips, export.ResolvedClip{
				ClipName:  clipName,
				MediaPath: file.Path,
				StartMs:   clip.StartMs,
				EndMs:     clip.EndMs,
				SceneID:   clip.SceneID,
			})
		}

		if len(resolvedClips) == 0 {
			WriteError(w, http.StatusUnprocessableEntity, "no clips could be resolved", "UNRESOLVABLE_CLIPS")
			return
		}

		edl := export.GenerateEDL(resolvedClips, projectName, frameRate)
		outputPath := filepath.Join(req.OutputDir, projectName+".edl")
		if err := os.WriteFile(outputPath, []byte(edl), 0o644); err != nil {
			WriteError(w, http.StatusInternalServerError, "failed to write export file", "INTERNAL_ERROR")
			return
		}

		WriteJSON(w, http.StatusOK, export.ExportResponse{
			Status:          "ok",
			Format:          "edl",
			OutputPath:      outputPath,
			ClipCount:       len(resolvedClips),
			UnresolvedClips: unresolvedClips,
		})
	}
}
