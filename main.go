package main

import (
	_ "embed"

	"github.com/ernestl/snaprev/cmd"
)

// version is set at build time via:
//
//	go build -ldflags "-X main.version=1.0.0"
//
// When unset, the cmd package falls back to VCS info from
// runtime/debug.ReadBuildInfo.
var version string

//go:embed README.md
var readme string

func main() {
	cmd.SetVersion(version)
	cmd.SetReadme(readme)
	cmd.Execute()
}
