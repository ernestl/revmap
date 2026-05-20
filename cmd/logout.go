package cmd

import (
	"fmt"

	"github.com/ernestl/revmap/store"
	"github.com/spf13/cobra"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove stored Snap Store credentials",
	Long:  `Remove locally stored Snap Store credentials.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !store.CredentialsExist() {
			fmt.Println("No credentials found.")
			return nil
		}

		if err := store.ClearCredentials(); err != nil {
			return err
		}

		fmt.Println("Credentials cleared.")
		return nil
	},
}
