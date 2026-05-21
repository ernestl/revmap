package cmd

import (
	"fmt"

	"github.com/charmbracelet/glamour"
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
	Long: `Display the full README rendered with terminal styling.
Useful as a quick reference for available commands, flags,
and workflows without leaving the terminal.`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		out, err := glamour.Render(embeddedReadme, "auto")
		if err != nil {
			// Fallback to raw markdown on render failure.
			fmt.Print(embeddedReadme)
			return
		}
		fmt.Print(out)
	},
}
