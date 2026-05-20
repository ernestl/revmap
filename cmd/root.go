package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "revmap",
	Short: "Inspect snap revision and version history",
	Long: `revmap is a CLI tool for inspecting the revision and version
history of snaps published in the Snap Store.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	CompletionOptions: cobra.CompletionOptions{
		DisableDefaultCmd: true,
	},
}

func init() {
	cobra.EnableCommandSorting = false

	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true, GroupID: "learn"})

	rootCmd.AddGroup(
		&cobra.Group{ID: "auth", Title: "Auth:"},
		&cobra.Group{ID: "query", Title: "Query:"},
		&cobra.Group{ID: "learn", Title: "Learn:"},
	)

	loginCmd.GroupID = "auth"
	whoamiCmd.GroupID = "auth"
	logoutCmd.GroupID = "auth"

	listCmd.GroupID = "query"
	showCmd.GroupID = "query"

	readmeCmd.GroupID = "learn"
	demoCmd.GroupID = "learn"

	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(whoamiCmd)
	rootCmd.AddCommand(logoutCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(showCmd)
	rootCmd.AddCommand(readmeCmd)
	rootCmd.AddCommand(demoCmd)
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
