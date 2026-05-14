package cmd

import (
	"testing"
)

func TestSetVersionExplicit(t *testing.T) {
	// Simulate a release build where ldflags sets the version.
	SetVersion("1.2.3")
	if rootCmd.Version != "1.2.3" {
		t.Errorf("expected version 1.2.3, got %s", rootCmd.Version)
	}
}

func TestSetVersionFallback(t *testing.T) {
	// Simulate a dev build where ldflags is empty.
	SetVersion("")
	if rootCmd.Version == "" {
		t.Error("expected non-empty version from fallback")
	}
}

func TestVersionFromBuildInfo(t *testing.T) {
	v := versionFromBuildInfo()
	// In a test binary the VCS info may or may not be
	// present, but the function must always return a
	// non-empty string.
	if v == "" {
		t.Error("expected non-empty version string")
	}
}
