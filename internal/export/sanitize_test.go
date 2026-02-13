package export

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeName_ControlChars(t *testing.T) {
	got := SanitizeName(" A\nB\rC\tD\x00 ", 100)
	if strings.ContainsAny(got, "\n\r\t\x00") {
		t.Fatalf("sanitize output contains control chars: %q", got)
	}
	if got != "ABCD" {
		t.Fatalf("SanitizeName control char behavior mismatch, got %q", got)
	}
}

func TestSanitizeName_MaxLength(t *testing.T) {
	got := SanitizeName("abcdefghijklmnopqrstuvwxyz", 10)
	if len([]rune(got)) != 10 {
		t.Fatalf("expected length 10, got %d (%q)", len([]rune(got)), got)
	}
}

func TestSanitizeName_AllowedChars(t *testing.T) {
	input := "Az09 -_.,()"
	got := SanitizeName(input, 100)
	if got != input {
		t.Fatalf("SanitizeName changed allowed chars: got %q want %q", got, input)
	}
}

func TestSanitizeName_ReplacesDisallowed(t *testing.T) {
	got := SanitizeName("bad<>|\"name", 100)
	if got != "bad____name" {
		t.Fatalf("SanitizeName disallowed replacement mismatch: got %q", got)
	}
}

func TestValidateOutputDir_Valid(t *testing.T) {
	dir := t.TempDir()
	if err := ValidateOutputDir(dir); err != nil {
		t.Fatalf("ValidateOutputDir(%q) error = %v, want nil", dir, err)
	}
}

func TestValidateOutputDir_NotExist(t *testing.T) {
	base := t.TempDir()
	missing := filepath.Join(base, "missing")
	if err := ValidateOutputDir(missing); err == nil {
		t.Fatalf("ValidateOutputDir(%q) expected error for non-existent path", missing)
	}
}

func TestValidateOutputDir_PathTraversal(t *testing.T) {
	path := "/tmp/../etc"
	if err := ValidateOutputDir(path); err == nil {
		t.Fatalf("ValidateOutputDir(%q) expected traversal error", path)
	}
}

func TestValidateOutputDir_NotADir(t *testing.T) {
	tmp := t.TempDir()
	filePath := filepath.Join(tmp, "file.txt")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	if err := ValidateOutputDir(filePath); err == nil {
		t.Fatalf("ValidateOutputDir(%q) expected non-directory error", filePath)
	}
}
