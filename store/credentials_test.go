package store

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// withTempDataHome sets XDG_DATA_HOME to a temp dir and clears the
// credentials env var for the duration of the test. It restores
// original values on cleanup.
func withTempDataHome(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	oldData := os.Getenv("XDG_DATA_HOME")
	oldCreds := os.Getenv(CredentialsEnvVar)
	os.Setenv("XDG_DATA_HOME", tmp)
	os.Unsetenv(CredentialsEnvVar)
	t.Cleanup(func() {
		os.Setenv("XDG_DATA_HOME", oldData)
		if oldCreds != "" {
			os.Setenv(CredentialsEnvVar, oldCreds)
		} else {
			os.Unsetenv(CredentialsEnvVar)
		}
	})
	return tmp
}

func TestSaveAndLoadCredentials(t *testing.T) {
	withTempDataHome(t)

	// Save.
	err := SaveCredentials("root-mac", "discharge-mac")
	if err != nil {
		t.Fatalf("SaveCredentials failed: %v", err)
	}

	// Load.
	creds, err := LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials failed: %v", err)
	}
	if creds.Root != "root-mac" {
		t.Errorf("Root = %q, want %q", creds.Root, "root-mac")
	}
	if creds.Discharge != "discharge-mac" {
		t.Errorf("Discharge = %q, want %q", creds.Discharge, "discharge-mac")
	}
}

func TestSaveCredentialsCreatesDir(t *testing.T) {
	tmp := withTempDataHome(t)

	err := SaveCredentials("root", "discharge")
	if err != nil {
		t.Fatalf("SaveCredentials failed: %v", err)
	}

	// Verify the directory was created with correct permissions.
	dir := filepath.Join(tmp, AppName)
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory, got file")
	}
}

func TestSaveCredentialsFilePermissions(t *testing.T) {
	tmp := withTempDataHome(t)

	err := SaveCredentials("root", "discharge")
	if err != nil {
		t.Fatalf("SaveCredentials failed: %v", err)
	}

	path := filepath.Join(tmp, AppName, "credentials.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}
}

func TestClearCredentials(t *testing.T) {
	withTempDataHome(t)

	// Save, then clear.
	err := SaveCredentials("root", "discharge")
	if err != nil {
		t.Fatalf("SaveCredentials failed: %v", err)
	}

	err = ClearCredentials()
	if err != nil {
		t.Fatalf("ClearCredentials failed: %v", err)
	}

	// Load should fail.
	_, err = LoadCredentials()
	if err == nil {
		t.Fatal("expected error after clearing credentials, got nil")
	}
}

func TestClearCredentialsNonExistent(t *testing.T) {
	withTempDataHome(t)

	// Clearing when no file exists should not error.
	err := ClearCredentials()
	if err != nil {
		t.Fatalf("ClearCredentials on non-existent file failed: %v", err)
	}
}

func TestCredentialsExist(t *testing.T) {
	withTempDataHome(t)

	if CredentialsExist() {
		t.Error("CredentialsExist should return false before saving")
	}

	err := SaveCredentials("root", "discharge")
	if err != nil {
		t.Fatalf("SaveCredentials failed: %v", err)
	}

	if !CredentialsExist() {
		t.Error("CredentialsExist should return true after saving")
	}
}

func TestCredentialsExistViaEnvVar(t *testing.T) {
	withTempDataHome(t)

	creds := Credentials{Root: "r", Discharge: "d"}
	data, _ := json.Marshal(creds)
	encoded := base64.StdEncoding.EncodeToString(data)
	os.Setenv(CredentialsEnvVar, encoded)

	if !CredentialsExist() {
		t.Error("CredentialsExist should return true when env var is set")
	}
}

func TestLoadCredentialsFromEnvVar(t *testing.T) {
	withTempDataHome(t)

	creds := Credentials{Root: "env-root", Discharge: "env-discharge"}
	data, _ := json.Marshal(creds)
	encoded := base64.StdEncoding.EncodeToString(data)
	os.Setenv(CredentialsEnvVar, encoded)

	loaded, err := LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials from env var failed: %v", err)
	}
	if loaded.Root != "env-root" {
		t.Errorf("Root = %q, want %q", loaded.Root, "env-root")
	}
	if loaded.Discharge != "env-discharge" {
		t.Errorf("Discharge = %q, want %q", loaded.Discharge, "env-discharge")
	}
}

func TestDecodeEnvCredentialsInvalidBase64(t *testing.T) {
	_, err := decodeEnvCredentials("!!!invalid!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64, got nil")
	}
}

func TestDecodeEnvCredentialsInvalidJSON(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("not-json"))
	_, err := decodeEnvCredentials(encoded)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestLoadCredentialsNoFile(t *testing.T) {
	withTempDataHome(t)

	_, err := LoadCredentials()
	if err == nil {
		t.Fatal("expected error when no credentials exist, got nil")
	}
}
