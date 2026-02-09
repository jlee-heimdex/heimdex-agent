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
