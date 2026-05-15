package store

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Credentials holds the root and discharge macaroons for store authentication.
type Credentials struct {
	Root      string `json:"r"`
	Discharge string `json:"d"`
}

// credentialsFilePath returns the path to the credentials file,
// respecting XDG_DATA_HOME.
func credentialsFilePath() string {
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
// variable first (base64-encoded JSON), then falls back to the credentials file.
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
			return nil, fmt.Errorf("no credentials found (use 'snaprev login' first)")
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
	path := credentialsFilePath()
	_, err := os.Stat(path)
	return err == nil
}

// decodeEnvCredentials decodes base64-encoded JSON credentials from
// the environment variable.
func decodeEnvCredentials(encoded string) (*Credentials, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("cannot decode %s: %w", CredentialsEnvVar, err)
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("cannot parse %s: %w", CredentialsEnvVar, err)
	}
	return &creds, nil
}
