package cmd

import (
	"fmt"

	"github.com/charmbracelet/glamour"
	"github.com/spf13/cobra"
)

var embeddedDesign string

// SetDesign stores the embedded DESIGN content for the design command.
func SetDesign(content string) {
	embeddedDesign = content
}

var designCmd = &cobra.Command{
	Use:   "design",
	Short: "Display the architecture and design documentation",
	Long: `Display the full design document describing revmap's architecture,
data flow, and conventions. Useful for understanding the codebase
before contributing, or as context for a coding agent.`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		out, err := glamour.Render(embeddedDesign, "auto")
		if err != nil {
			fmt.Print(embeddedDesign)
			return
		}
		fmt.Print(out)
	},
}
