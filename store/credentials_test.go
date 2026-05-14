package store

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/macaroon.v1"
)

// withTempDataHome sets XDG_DATA_HOME to a temp dir and clears the
// credentials env var and SNAP_USER_COMMON for the duration of the
// test. It restores original values on cleanup.
func withTempDataHome(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	oldData := os.Getenv("XDG_DATA_HOME")
	oldCreds := os.Getenv(CredentialsEnvVar)
	oldSnap := os.Getenv("SNAP_USER_COMMON")
	os.Setenv("XDG_DATA_HOME", tmp)
	os.Unsetenv(CredentialsEnvVar)
	os.Unsetenv("SNAP_USER_COMMON")
	t.Cleanup(func() {
		os.Setenv("XDG_DATA_HOME", oldData)
		if oldCreds != "" {
			os.Setenv(CredentialsEnvVar, oldCreds)
		} else {
			os.Unsetenv(CredentialsEnvVar)
		}
		if oldSnap != "" {
			os.Setenv("SNAP_USER_COMMON", oldSnap)
		} else {
			os.Unsetenv("SNAP_USER_COMMON")
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

func TestCredentialsExistOnDiskNoFile(t *testing.T) {
	withTempDataHome(t)

	if CredentialsExistOnDisk() {
		t.Error("CredentialsExistOnDisk should return false when no file exists")
	}
}

func TestCredentialsExistOnDiskIgnoresEnvVar(t *testing.T) {
	withTempDataHome(t)

	creds := Credentials{Root: "r", Discharge: "d"}
	data, _ := json.Marshal(creds)
	encoded := base64.StdEncoding.EncodeToString(data)
	os.Setenv(CredentialsEnvVar, encoded)

	if CredentialsExistOnDisk() {
		t.Error("CredentialsExistOnDisk should return false when only env var is set")
	}
}

func TestCredentialsExistOnDiskWithFile(t *testing.T) {
	withTempDataHome(t)

	err := SaveCredentials("root", "discharge")
	if err != nil {
		t.Fatalf("SaveCredentials failed: %v", err)
	}

	if !CredentialsExistOnDisk() {
		t.Error("CredentialsExistOnDisk should return true when file exists")
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

func TestParseSnapcraftCredentials(t *testing.T) {
	input := "[login.ubuntu.com]\n" +
		"macaroon = root-mac-value\n" +
		"unbound_discharge = discharge-mac-value\n" +
		"email = user@example.com\n"

	creds, err := parseSnapcraftCredentials(input)
	if err != nil {
		t.Fatalf("parseSnapcraftCredentials failed: %v", err)
	}
	if creds.Root != "root-mac-value" {
		t.Errorf("Root = %q, want %q", creds.Root, "root-mac-value")
	}
	if creds.Discharge != "discharge-mac-value" {
		t.Errorf("Discharge = %q, want %q", creds.Discharge, "discharge-mac-value")
	}
}

func TestParseSnapcraftCredentialsMissingFields(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"no macaroon", "[login.ubuntu.com]\nunbound_discharge = d\n"},
		{"no discharge", "[login.ubuntu.com]\nmacaroon = r\n"},
		{"empty", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseSnapcraftCredentials(tc.input)
			if err == nil {
				t.Fatal("expected error for incomplete credentials, got nil")
			}
		})
	}
}

func TestLoadCredentialsFromSnapcraftEnvVar(t *testing.T) {
	withTempDataHome(t)

	snapcraftCreds := "[login.ubuntu.com]\n" +
		"macaroon = sc-root\n" +
		"unbound_discharge = sc-discharge\n"
	os.Setenv(CredentialsEnvVar, snapcraftCreds)

	loaded, err := LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials from snapcraft env var failed: %v", err)
	}
	if loaded.Root != "sc-root" {
		t.Errorf("Root = %q, want %q", loaded.Root, "sc-root")
	}
	if loaded.Discharge != "sc-discharge" {
		t.Errorf("Discharge = %q, want %q", loaded.Discharge, "sc-discharge")
	}
}

func TestLoadCredentialsNoFile(t *testing.T) {
	withTempDataHome(t)

	_, err := LoadCredentials()
	if err == nil {
		t.Fatal("expected error when no credentials exist, got nil")
	}
}

func TestCredentialsFilePathSnap(t *testing.T) {
	tmp := withTempDataHome(t)
	snapCommon := filepath.Join(tmp, "snap-common")
	os.MkdirAll(snapCommon, 0700)
	os.Setenv("SNAP_USER_COMMON", snapCommon)

	// Save credentials — should land in $SNAP_USER_COMMON.
	err := SaveCredentials("snap-root", "snap-discharge")
	if err != nil {
		t.Fatalf("SaveCredentials failed: %v", err)
	}

	expected := filepath.Join(snapCommon, "credentials.json")
	if _, err := os.Stat(expected); err != nil {
		t.Fatalf("credentials not at snap path %s: %v", expected, err)
	}

	// Verify XDG path was NOT used.
	xdgPath := filepath.Join(tmp, AppName, "credentials.json")
	if _, err := os.Stat(xdgPath); err == nil {
		t.Error("credentials should not exist at XDG path when SNAP_USER_COMMON is set")
	}

	// Load should read from snap path.
	creds, err := LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials failed: %v", err)
	}
	if creds.Root != "snap-root" {
		t.Errorf("Root = %q, want %q", creds.Root, "snap-root")
	}
}

func TestCredentialsExpiry(t *testing.T) {
	withTempDataHome(t)

	// Create a macaroon with a time-before caveat.
	m, err := macaroon.New([]byte("key"), "id", "location")
	if err != nil {
		t.Fatalf("cannot create macaroon: %v", err)
	}
	err = m.AddFirstPartyCaveat("time-before 2027-06-15T12:00:00.000000")
	if err != nil {
		t.Fatalf("cannot add caveat: %v", err)
	}

	root, err := MacaroonSerialize(m)
	if err != nil {
		t.Fatalf("cannot serialize macaroon: %v", err)
	}

	if err := SaveCredentials(root, "discharge-placeholder"); err != nil {
		t.Fatalf("SaveCredentials failed: %v", err)
	}

	expires, err := CredentialsExpiry()
	if err != nil {
		t.Fatalf("CredentialsExpiry failed: %v", err)
	}
	if expires.IsZero() {
		t.Fatal("expected non-zero expiry time")
	}
	if expires.Year() != 2027 || expires.Month() != 6 || expires.Day() != 15 {
		t.Errorf("unexpected expiry: %v", expires)
	}
}

func TestCredentialsExpiryStoreFormat(t *testing.T) {
	withTempDataHome(t)

	// Create a macaroon with the Snap Store pipe-delimited expiry caveat.
	m, err := macaroon.New([]byte("key"), "id", "location")
	if err != nil {
		t.Fatalf("cannot create macaroon: %v", err)
	}
	err = m.AddFirstPartyCaveat("myapps.developer.ubuntu.com|expires|2027-05-20T20:30:22.896621")
	if err != nil {
		t.Fatalf("cannot add caveat: %v", err)
	}

	root, err := MacaroonSerialize(m)
	if err != nil {
		t.Fatalf("cannot serialize macaroon: %v", err)
	}

	if err := SaveCredentials(root, "discharge-placeholder"); err != nil {
		t.Fatalf("SaveCredentials failed: %v", err)
	}

	expires, err := CredentialsExpiry()
	if err != nil {
		t.Fatalf("CredentialsExpiry failed: %v", err)
	}
	if expires.IsZero() {
		t.Fatal("expected non-zero expiry time")
	}
	if expires.Year() != 2027 || expires.Month() != 5 || expires.Day() != 20 {
		t.Errorf("unexpected expiry: %v", expires)
	}
}

func TestCredentialsExpiryNoCaveat(t *testing.T) {
	withTempDataHome(t)

	// Create a macaroon without a time-before caveat.
	m, err := macaroon.New([]byte("key"), "id", "location")
	if err != nil {
		t.Fatalf("cannot create macaroon: %v", err)
	}

	root, err := MacaroonSerialize(m)
	if err != nil {
		t.Fatalf("cannot serialize macaroon: %v", err)
	}

	if err := SaveCredentials(root, "discharge-placeholder"); err != nil {
		t.Fatalf("SaveCredentials failed: %v", err)
	}

	expires, err := CredentialsExpiry()
	if err != nil {
		t.Fatalf("CredentialsExpiry failed: %v", err)
	}
	if !expires.IsZero() {
		t.Errorf("expected zero expiry for macaroon without time-before, got %v", expires)
	}
}
