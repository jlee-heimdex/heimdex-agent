package config

import (
	"os"
	"testing"
)

func TestCloudLibraryID_Default(t *testing.T) {
	os.Unsetenv(EnvCloudLibraryID)

	cfg, err := New()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.CloudLibraryID() != "" {
		t.Errorf("default CloudLibraryID = %q, want empty", cfg.CloudLibraryID())
	}
}

func TestCloudLibraryID_FromEnv(t *testing.T) {
	os.Setenv(EnvCloudLibraryID, "test-lib-uuid")
	defer os.Unsetenv(EnvCloudLibraryID)

	cfg, err := New()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.CloudLibraryID() != "test-lib-uuid" {
		t.Errorf("CloudLibraryID = %q, want %q", cfg.CloudLibraryID(), "test-lib-uuid")
	}
}

func TestOCREnabled_DefaultFalse(t *testing.T) {
	os.Unsetenv(EnvOCREnabled)

	cfg, err := New()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.OCREnabled() {
		t.Errorf("default OCREnabled = %v, want false", cfg.OCREnabled())
	}
}

func TestOCREnabled_FromEnv(t *testing.T) {
	os.Setenv(EnvOCREnabled, "true")
	defer os.Unsetenv(EnvOCREnabled)

	cfg, err := New()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.OCREnabled() {
		t.Errorf("OCREnabled = %v, want true", cfg.OCREnabled())
	}
}

func TestOCRRedactPII_DefaultFalse(t *testing.T) {
	os.Unsetenv(EnvOCRRedactPII)

	cfg, err := New()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.OCRRedactPII() {
		t.Errorf("default OCRRedactPII = %v, want false", cfg.OCRRedactPII())
	}
}

func TestOCRRedactPII_FromEnv(t *testing.T) {
	os.Setenv(EnvOCRRedactPII, "1")
	defer os.Unsetenv(EnvOCRRedactPII)

	cfg, err := New()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.OCRRedactPII() {
		t.Errorf("OCRRedactPII = %v, want true", cfg.OCRRedactPII())
	}
}

func TestParallelFacesWithSpeech_DefaultFalse(t *testing.T) {
	os.Unsetenv(EnvParallelFacesWithSpeech)

	cfg, err := New()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ParallelFacesWithSpeech() {
		t.Errorf("default ParallelFacesWithSpeech = %v, want false", cfg.ParallelFacesWithSpeech())
	}
}

func TestParallelFacesWithSpeech_FromEnv(t *testing.T) {
	os.Setenv(EnvParallelFacesWithSpeech, "true")
	defer os.Unsetenv(EnvParallelFacesWithSpeech)

	cfg, err := New()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.ParallelFacesWithSpeech() {
		t.Errorf("ParallelFacesWithSpeech = %v, want true", cfg.ParallelFacesWithSpeech())
	}
}
