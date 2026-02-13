package export

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

func SanitizeName(s string, maxLen int) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsControl(r) {
			continue
		}
		if isAllowedNameRune(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}

	cleaned := strings.TrimSpace(b.String())
	if maxLen > 0 {
		runes := []rune(cleaned)
		if len(runes) > maxLen {
			cleaned = string(runes[:maxLen])
		}
	}
	return cleaned
}

func isAllowedNameRune(r rune) bool {
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return true
	}
	switch r {
	case ' ', '-', '_', '.', ',', '(', ')':
		return true
	default:
		return false
	}
}

func ValidateOutputDir(dir string) error {
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("output_dir is required")
	}

	for _, part := range strings.Split(filepath.ToSlash(dir), "/") {
		if part == ".." {
			return fmt.Errorf("output_dir cannot contain path traversal")
		}
	}

	cleaned := filepath.Clean(dir)
	if cleaned != dir {
		return fmt.Errorf("output_dir must be clean path")
	}

	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("output_dir does not exist")
		}
		return fmt.Errorf("invalid output_dir: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("output_dir is not a directory")
	}

	return nil
}
