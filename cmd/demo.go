package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var noPause bool

func init() {
	demoCmd.Flags().BoolVar(&noPause, "no-pause", false, "run without pausing between commands")
}

var demoCmd = &cobra.Command{
	Use:   "demo",
	Short: "Run an interactive demo using the snapd snap",
	Long: `Run an interactive demo that showcases revmap's capabilities
using the snapd snap as an example. Requires authentication.

The demo walks through various list and show commands,
pausing between each for review. Use --no-pause to run
without pauses.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		script := findDemoScript()
		if script == "" {
			return fmt.Errorf("demo.sh not found (expected next to the revmap binary)")
		}

		// Find our own binary path to pass as REVMAP.
		self, err := os.Executable()
		if err != nil {
			self = os.Args[0]
		}

		var cmdArgs []string
		if noPause {
			cmdArgs = []string{script, "--no-pause"}
		} else {
			cmdArgs = []string{script}
		}

		c := exec.Command("/bin/bash", cmdArgs...)
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		c.Env = append(os.Environ(), "REVMAP="+self)

		return c.Run()
	},
}

// findDemoScript locates demo.sh relative to the binary.
// It checks: $SNAP/bin/demo.sh, then next to the executable.
func findDemoScript() string {
	// When running as a snap, $SNAP points to the snap's root.
	if snap := os.Getenv("SNAP"); snap != "" {
		p := filepath.Join(snap, "bin", "demo.sh")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// Next to the executable.
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), "demo.sh")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// Current working directory (dev builds).
	if _, err := os.Stat("demo.sh"); err == nil {
		return "demo.sh"
	}

	return ""
}
