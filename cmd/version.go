package cmd

import (
	"fmt"
	"runtime/debug"
)

// SetVersion configures the version displayed by --version.
// If v is non-empty it is used directly (release build via
// ldflags). Otherwise the version is derived from VCS info
// embedded by the Go toolchain.
func SetVersion(v string) {
	if v == "" {
		v = versionFromBuildInfo()
	}
	rootCmd.Version = v
	rootCmd.SetVersionTemplate(
		fmt.Sprintf("snaprev %s\n", v))
}

// versionFromBuildInfo extracts VCS revision and dirty flag
// from runtime/debug.ReadBuildInfo.
func versionFromBuildInfo() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}

	var revision string
	var dirty bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}

	if revision == "" {
		return "dev"
	}

	// Shorten to 7 chars like git's short hash.
	if len(revision) > 7 {
		revision = revision[:7]
	}

	if dirty {
		return fmt.Sprintf("dev (%s, dirty)", revision)
	}
	return fmt.Sprintf("dev (%s)", revision)
}
