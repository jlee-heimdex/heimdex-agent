package export

import (
	"strings"
	"testing"
)

func TestGenerateEDL_SingleClip(t *testing.T) {
	clips := []ResolvedClip{{
		ClipName:  "Intro",
		MediaPath: "/media/intro.mp4",
		StartMs:   0,
		EndMs:     2000,
	}}

	edl := GenerateEDL(clips, "Project One", 30.0)

	if !strings.Contains(edl, "TITLE: Project One") {
		t.Fatalf("missing title in EDL: %q", edl)
	}
	if !strings.Contains(edl, "FCM: NON-DROP FRAME") {
		t.Fatalf("missing non-drop-frame FCM: %q", edl)
	}
	if !strings.Contains(edl, "001  AX       V     C        00:00:00:00 00:00:02:00 00:00:00:00 00:00:02:00") {
		t.Fatalf("missing event line: %q", edl)
	}
	if !strings.Contains(edl, "* FROM CLIP NAME:  Intro") {
		t.Fatalf("missing clip name comment: %q", edl)
	}
	if !strings.Contains(edl, "* MEDIA PATH:  /media/intro.mp4") {
		t.Fatalf("missing media path comment: %q", edl)
	}
}

func TestGenerateEDL_MultipleClips(t *testing.T) {
	clips := []ResolvedClip{
		{ClipName: "Clip A", MediaPath: "/a.mp4", StartMs: 0, EndMs: 1000},
		{ClipName: "Clip B", MediaPath: "/b.mp4", StartMs: 1000, EndMs: 2500},
	}

	edl := GenerateEDL(clips, "Multi", 30.0)

	if !strings.Contains(edl, "001  AX       V     C        00:00:00:00 00:00:01:00 00:00:00:00 00:00:01:00") {
		t.Fatalf("first event line mismatch: %q", edl)
	}
	if !strings.Contains(edl, "002  AX       V     C        00:00:01:00 00:00:02:15 00:00:01:00 00:00:02:15") {
		t.Fatalf("second event line mismatch or bad record offset: %q", edl)
	}
}

func TestGenerateEDL_DropFrame(t *testing.T) {
	clips := []ResolvedClip{{ClipName: "Clip", MediaPath: "/x.mp4", StartMs: 0, EndMs: 1000}}
	edl := GenerateEDL(clips, "Drop", 29.97)

	if !strings.Contains(edl, "FCM: DROP FRAME") {
		t.Fatalf("expected drop frame FCM, got: %q", edl)
	}
}

func TestMsToTimecode(t *testing.T) {
	tests := []struct {
		name string
		ms   int
		fps  int
		want string
	}{
		{name: "zero", ms: 0, fps: 30, want: "00:00:00:00"},
		{name: "one second", ms: 1000, fps: 30, want: "00:00:01:00"},
		{name: "fractional second", ms: 500, fps: 30, want: "00:00:00:15"},
		{name: "one minute", ms: 60000, fps: 30, want: "00:01:00:00"},
		{name: "one hour", ms: 3600000, fps: 30, want: "01:00:00:00"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := msToTimecode(tc.ms, tc.fps)
			if got != tc.want {
				t.Fatalf("msToTimecode(%d, %d) = %q, want %q", tc.ms, tc.fps, got, tc.want)
			}
		})
	}
}
