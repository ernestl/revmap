package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ernestl/revmap/store"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var loginExportFile string

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with the Snap Store",
	Long: `Authenticate with the Snap Store using your Ubuntu One SSO
credentials. Credentials are stored locally for subsequent use.

Use --export to write credentials to a file in the snapcraft INI
format, compatible with SNAPCRAFT_STORE_CREDENTIALS:

    revmap login --export credentials.txt
    export SNAPCRAFT_STORE_CREDENTIALS=$(cat credentials.txt)

If already logged in, --export writes the existing credentials
without re-authenticating.

When running as a snap, relative paths are resolved under the snap's
user data directory (~/snap/revmap/common/) since strict confinement
prevents writing to arbitrary locations. Absolute paths are used as-is.

You can also set the SNAPCRAFT_STORE_CREDENTIALS environment variable
with snapcraft export-login output to skip interactive login.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// If --export is used with existing on-disk credentials, just export.
		// We intentionally ignore SNAPCRAFT_STORE_CREDENTIALS here so that
		// --export always produces fresh credentials from an interactive login
		// rather than re-exporting the environment variable.
		if loginExportFile != "" && store.CredentialsExistOnDisk() {
			if err := store.ExportCredentials(loginExportFile); err != nil {
				return err
			}
			fmt.Printf("Credentials exported to %s.\n", store.ExportPath(loginExportFile))
			return nil
		}

		if store.CredentialsExist() {
			fmt.Println("You are already logged in. Run 'revmap logout' first to re-authenticate.")
			return nil
		}

		reader := bufio.NewReader(os.Stdin)

		fmt.Print("Email: ")
		email, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("cannot read email: %w", err)
		}
		email = strings.TrimSpace(email)

		fmt.Print("Password: ")
		passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println() // newline after hidden input
		if err != nil {
			return fmt.Errorf("cannot read password: %w", err)
		}
		password := string(passwordBytes)

		// First attempt without OTP.
		err = store.Login(email, password, "")
		if errors.Is(err, store.ErrTwoFactorRequired) {
			fmt.Print("Second factor: ")
			otp, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("cannot read second factor: %w", err)
			}
			otp = strings.TrimSpace(otp)

			// Retry with OTP.
			err = store.Login(email, password, otp)
			if err != nil {
				return err
			}
		} else if err != nil {
			return err
		}

		fmt.Println("Login successful.")

		// Export if requested.
		if loginExportFile != "" {
			if err := store.ExportCredentials(loginExportFile); err != nil {
				return err
			}
			fmt.Printf("Credentials exported to %s.\n", store.ExportPath(loginExportFile))
		}

		return nil
	},
}

func init() {
	loginCmd.Flags().StringVar(&loginExportFile, "export", "", "export credentials to file in snapcraft format")
}
