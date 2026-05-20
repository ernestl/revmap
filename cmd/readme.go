package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var embeddedReadme string

// SetReadme stores the embedded README content for the readme command.
func SetReadme(content string) {
	embeddedReadme = content
}

var readmeCmd = &cobra.Command{
	Use:   "readme",
	Short: "Display the full README documentation",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print(embeddedReadme)
	},
}
