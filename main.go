package main

import (
	_ "embed"

	"github.com/ernestl/revmap/cmd"
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

//go:embed DESIGN.md
var design string

func main() {
	cmd.SetVersion(version)
	cmd.SetReadme(readme)
	cmd.SetDesign(design)
	cmd.Execute()
}
