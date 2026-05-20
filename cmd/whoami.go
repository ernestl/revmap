package cmd

import (
	"fmt"
	"strings"

	"github.com/ernestl/revmap/store"
	"github.com/spf13/cobra"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the logged-in account",
	Long: `Show account information for the currently authenticated user,
including email, username, and registered snaps.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !store.CredentialsExist() {
			return fmt.Errorf("not logged in (use 'revmap login' or set SNAPCRAFT_STORE_CREDENTIALS)")
		}

		client := store.NewClient()
		info, err := client.GetAccount()
		if err != nil {
			return err
		}

		fmt.Printf("email:    %s\n", info.Email)
		fmt.Printf("username: %s\n", info.Username)
		fmt.Printf("source:   %s\n", store.CredentialsSource())

		if expires, err := store.CredentialsExpiry(); err == nil && !expires.IsZero() {
			fmt.Printf("expires:  %s\n", expires.Format("2006-01-02"))
		}

		if len(info.Snaps) == 0 {
			fmt.Printf("snaps:    (none)\n")
		} else {
			printSnaps(info.Snaps)
		}

		return nil
	},
}

const (
	snapsPrefix = "snaps:    "
	snapsCont   = "          "
	lineWidth   = 80
	snapCols    = 3
)

// printSnaps prints snap names in a 3-column grid within 80 chars.
func printSnaps(snaps []string) {
	available := lineWidth - len(snapsPrefix)
	colWidth := available / snapCols // 23 chars per column

	for i, name := range snaps {
		if i == 0 {
			fmt.Print(snapsPrefix)
		} else if i%snapCols == 0 {
			fmt.Print("\n" + snapsCont)
		}

		display := truncateSnap(name, colWidth-1) // -1 for inter-column space
		if i%snapCols < snapCols-1 && i < len(snaps)-1 {
			fmt.Printf("%-*s", colWidth, display)
		} else {
			fmt.Print(display)
		}
	}
	fmt.Println()
}

// truncateSnap truncates a snap name to maxLen, adding "..." if needed.
func truncateSnap(name string, maxLen int) string {
	if len(name) <= maxLen {
		return name
	}
	if maxLen <= 3 {
		return name[:maxLen]
	}
	return strings.TrimRight(name[:maxLen-3], "-") + "..."
}
