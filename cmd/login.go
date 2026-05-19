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

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with the Snap Store",
	Long: `Authenticate with the Snap Store using your Ubuntu One SSO
credentials. Credentials are stored locally for subsequent use.

You can also set the REVMAP_STORE_CREDENTIALS environment variable
with snapcraft export-login output to skip interactive login.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
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
		return nil
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)
}
