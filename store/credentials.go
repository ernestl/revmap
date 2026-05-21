package store

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Credentials holds the root and discharge macaroons for store authentication.
type Credentials struct {
	Root      string `json:"r"`
	Discharge string `json:"d"`
}

// credentialsFilePath returns the path to the credentials file.
// Inside a snap it uses $SNAP_USER_COMMON (persists across snap
// refreshes). Otherwise it respects XDG_DATA_HOME.
func credentialsFilePath() string {
	if common := os.Getenv("SNAP_USER_COMMON"); common != "" {
		return filepath.Join(common, "credentials.json")
	}
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataHome, AppName, "credentials.json")
}

// SaveCredentials writes the root and discharge macaroons to the
// credentials file with restricted permissions.
func SaveCredentials(root, discharge string) error {
	creds := Credentials{Root: root, Discharge: discharge}
	data, err := json.Marshal(creds)
	if err != nil {
		return fmt.Errorf("cannot marshal credentials: %w", err)
	}

	path := credentialsFilePath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("cannot create credentials directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("cannot write credentials file: %w", err)
	}
	return nil
}

// LoadCredentials returns stored credentials. It checks the environment
// variable first (snapcraft export format or base64-encoded JSON), then
// falls back to the credentials file.
func LoadCredentials() (*Credentials, error) {
	// Check environment variable first.
	if envCreds := os.Getenv(CredentialsEnvVar); envCreds != "" {
		return decodeEnvCredentials(envCreds)
	}

	// Fall back to file.
	path := credentialsFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no credentials found (use 'revmap login' first)")
		}
		return nil, fmt.Errorf("cannot read credentials file: %w", err)
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("cannot parse credentials file: %w", err)
	}
	return &creds, nil
}

// ClearCredentials removes the stored credentials file.
func ClearCredentials() error {
	path := credentialsFilePath()
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cannot remove credentials file: %w", err)
	}
	return nil
}

// CredentialsExist returns true if credentials are available, either
// via the environment variable or the credentials file.
func CredentialsExist() bool {
	if os.Getenv(CredentialsEnvVar) != "" {
		return true
	}
	return CredentialsExistOnDisk()
}

// CredentialsExistOnDisk returns true if credentials are stored in the
// credentials file, ignoring the SNAPCRAFT_STORE_CREDENTIALS environment
// variable.
func CredentialsExistOnDisk() bool {
	path := credentialsFilePath()
	_, err := os.Stat(path)
	return err == nil
}

// CredentialsSource returns a human-readable description of where the
// current credentials are loaded from.
func CredentialsSource() string {
	if os.Getenv(CredentialsEnvVar) != "" {
		return CredentialsEnvVar
	}
	return credentialsFilePath()
}

// CredentialsPermissions returns the permissions (ACL) encoded in the
// root macaroon's first-party caveats. It looks for caveats in the
// format "<location>|acl|[...]".
// Returns nil if no permissions caveat is found.
func CredentialsPermissions() ([]string, error) {
	creds, err := LoadCredentials()
	if err != nil {
		return nil, err
	}

	m, err := MacaroonDeserialize(creds.Root)
	if err != nil {
		return nil, fmt.Errorf("cannot deserialize root macaroon: %w", err)
	}

	for _, caveat := range m.Caveats() {
		if caveat.Location != "" {
			continue // skip third-party caveats
		}
		if strings.Contains(caveat.Id, "|acl|") {
			parts := strings.SplitN(caveat.Id, "|acl|", 2)
			if len(parts) != 2 {
				continue
			}
			var perms []string
			if err := json.Unmarshal([]byte(parts[1]), &perms); err != nil {
				continue
			}
			return perms, nil
		}
	}

	return nil, nil
}

// CredentialsExpiry returns the expiry time of the stored credentials
// by inspecting the root macaroon's first-party caveats. It checks for:
//   - "time-before <timestamp>" (standard macaroon format)
//   - "<location>|expires|<timestamp>" (Snap Store format)
//
// Returns zero time if no expiry is found.
func CredentialsExpiry() (time.Time, error) {
	creds, err := LoadCredentials()
	if err != nil {
		return time.Time{}, err
	}

	m, err := MacaroonDeserialize(creds.Root)
	if err != nil {
		return time.Time{}, fmt.Errorf("cannot deserialize root macaroon: %w", err)
	}

	for _, caveat := range m.Caveats() {
		if caveat.Location != "" {
			continue // skip third-party caveats
		}

		var ts string
		switch {
		case strings.HasPrefix(caveat.Id, "time-before "):
			ts = strings.TrimPrefix(caveat.Id, "time-before ")
		case strings.Contains(caveat.Id, "|expires|"):
			parts := strings.SplitN(caveat.Id, "|expires|", 2)
			if len(parts) == 2 {
				ts = parts[1]
			}
		default:
			continue
		}

		// Try multiple timestamp formats the store may use.
		for _, layout := range []string{
			"2006-01-02T15:04:05.000000",
			"2006-01-02T15:04:05.999999",
			"2006-01-02T15:04:05",
			time.RFC3339,
		} {
			if t, err := time.Parse(layout, ts); err == nil {
				return t, nil
			}
		}
		return time.Time{}, fmt.Errorf("cannot parse expiry time: %s", ts)
	}

	return time.Time{}, nil
}

// decodeEnvCredentials decodes credentials from the environment variable.
// It supports two formats:
//
//  1. Snapcraft export format (from "snapcraft export-login"):
//     [login.ubuntu.com]
//     macaroon = <root>
//     unbound_discharge = <discharge>
//
//  2. Base64-encoded JSON (legacy revmap format):
//     base64({"r":"<root>","d":"<discharge>"})
func decodeEnvCredentials(value string) (*Credentials, error) {
	// Try snapcraft INI format first.
	if creds, err := parseSnapcraftCredentials(value); err == nil {
		return creds, nil
	}

	// Fall back to base64-encoded JSON.
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("cannot decode %s: not a valid snapcraft export or base64 JSON", CredentialsEnvVar)
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("cannot parse %s: %w", CredentialsEnvVar, err)
	}
	return &creds, nil
}

// ExportCredentials writes the stored credentials to the given file in
// the snapcraft INI format compatible with SNAPCRAFT_STORE_CREDENTIALS.
// When running inside a snap, relative paths are resolved under
// $SNAP_USER_COMMON since the snap cannot write to arbitrary directories.
func ExportCredentials(path string) error {
	creds, err := LoadCredentials()
	if err != nil {
		return err
	}

	path = resolveExportPath(path)

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("cannot create export directory: %w", err)
	}

	content := fmt.Sprintf("[login.ubuntu.com]\nmacaroon = %s\nunbound_discharge = %s\n",
		creds.Root, creds.Discharge)

	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return fmt.Errorf("cannot write export file: %w", err)
	}
	return nil
}

// ExportPath returns the resolved path that ExportCredentials would
// write to, so the caller can display it to the user.
func ExportPath(path string) string {
	return resolveExportPath(path)
}

// resolveExportPath resolves relative paths under $SNAP_USER_COMMON
// when running inside a snap (strict confinement cannot write to
// arbitrary directories).
func resolveExportPath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	if common := os.Getenv("SNAP_USER_COMMON"); common != "" {
		return filepath.Join(common, path)
	}
	return path
}

// produced by "snapcraft export-login". The expected format is:
//
//	[login.ubuntu.com]
//	macaroon = <serialized root macaroon>
//	unbound_discharge = <serialized discharge macaroon>
func parseSnapcraftCredentials(text string) (*Credentials, error) {
	var root, discharge string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "macaroon":
			root = strings.TrimSpace(value)
		case "unbound_discharge":
			discharge = strings.TrimSpace(value)
		}
	}
	if root == "" || discharge == "" {
		return nil, fmt.Errorf("missing macaroon or unbound_discharge fields")
	}
	return &Credentials{Root: root, Discharge: discharge}, nil
}
