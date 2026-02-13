package export

import (
	"fmt"
	"math"
	"strings"
)

func GenerateEDL(clips []ResolvedClip, title string, frameRate float64) string {
	fps := int(math.Round(frameRate))
	if fps <= 0 {
		fps = 30
	}

	isDropFrame := math.Abs(frameRate-29.97) < 0.01 || math.Abs(frameRate-59.94) < 0.01

	lines := []string{fmt.Sprintf("TITLE: %s", title)}
	if isDropFrame {
		lines = append(lines, "FCM: DROP FRAME")
	} else {
		lines = append(lines, "FCM: NON-DROP FRAME")
	}
	lines = append(lines, "")

	recordOffsetMs := 0
	for i, clip := range clips {
		srcIn := msToTimecode(clip.StartMs, fps)
		srcOut := msToTimecode(clip.EndMs, fps)
		recIn := msToTimecode(recordOffsetMs, fps)
		durationMs := clip.EndMs - clip.StartMs
		recOut := msToTimecode(recordOffsetMs+durationMs, fps)

		lines = append(lines,
			fmt.Sprintf("%03d  %-8s %-5s C        %s %s %s %s", i+1, "AX", "V", srcIn, srcOut, recIn, recOut),
			fmt.Sprintf("* FROM CLIP NAME:  %s", clip.ClipName),
			fmt.Sprintf("* MEDIA PATH:  %s", clip.MediaPath),
		)

		recordOffsetMs += durationMs
	}

	lines = append(lines, "")
	return strings.Join(lines, "\n")
}

func msToTimecode(ms int, fps int) string {
	totalFrames := int(math.Round(float64(ms) * float64(fps) / 1000.0))
	frames := totalFrames % fps
	totalSeconds := totalFrames / fps
	seconds := totalSeconds % 60
	totalMinutes := totalSeconds / 60
	minutes := totalMinutes % 60
	hours := totalMinutes / 60
	return fmt.Sprintf("%02d:%02d:%02d:%02d", hours, minutes, seconds, frames)
}
