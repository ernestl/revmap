package store

import (
	"testing"

	"gopkg.in/macaroon.v1"
)

func TestMacaroonSerializeDeserializeRoundtrip(t *testing.T) {
	// Create a fresh macaroon.
	m, err := macaroon.New([]byte("root-key"), "test-id", "test-location")
	if err != nil {
		t.Fatalf("cannot create macaroon: %v", err)
	}

	// Serialize.
	serialized, err := MacaroonSerialize(m)
	if err != nil {
		t.Fatalf("MacaroonSerialize failed: %v", err)
	}

	if serialized == "" {
		t.Fatal("serialized macaroon is empty")
	}

	// Verify URL-safe encoding: no '+', '/', or '='.
	for _, c := range serialized {
		if c == '+' || c == '/' || c == '=' {
			t.Errorf("serialized macaroon contains non-URL-safe character: %c", c)
		}
	}

	// Deserialize and verify round-trip.
	restored, err := MacaroonDeserialize(serialized)
	if err != nil {
		t.Fatalf("MacaroonDeserialize failed: %v", err)
	}

	if restored.Id() != m.Id() {
		t.Errorf("ID mismatch: got %q, want %q", restored.Id(), m.Id())
	}
	if restored.Location() != m.Location() {
		t.Errorf("location mismatch: got %q, want %q", restored.Location(), m.Location())
	}
}

func TestMacaroonDeserializeInvalid(t *testing.T) {
	// Invalid base64.
	_, err := MacaroonDeserialize("!!!not-valid-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64, got nil")
	}

	// Valid base64 but not a macaroon.
	_, err = MacaroonDeserialize("aGVsbG8") // "hello" in base64url
	if err == nil {
		t.Fatal("expected error for non-macaroon data, got nil")
	}
}

func TestLoginCaveatID(t *testing.T) {
	// Create a macaroon with a third-party caveat at UbuntuOneLocation.
	m, err := macaroon.New([]byte("root-key"), "test-id", "test-location")
	if err != nil {
		t.Fatalf("cannot create macaroon: %v", err)
	}

	err = m.AddThirdPartyCaveat([]byte("third-party-key"), "caveat-id-123", UbuntuOneLocation)
	if err != nil {
		t.Fatalf("cannot add third-party caveat: %v", err)
	}

	// Should extract the caveat ID.
	id, err := loginCaveatID(m)
	if err != nil {
		t.Fatalf("loginCaveatID failed: %v", err)
	}
	if id != "caveat-id-123" {
		t.Errorf("got caveat ID %q, want %q", id, "caveat-id-123")
	}
}

func TestLoginCaveatIDMissing(t *testing.T) {
	// Macaroon with no third-party caveats.
	m, err := macaroon.New([]byte("root-key"), "test-id", "test-location")
	if err != nil {
		t.Fatalf("cannot create macaroon: %v", err)
	}

	_, err = loginCaveatID(m)
	if err == nil {
		t.Fatal("expected error for missing caveat, got nil")
	}
}

func TestLoginCaveatIDWrongLocation(t *testing.T) {
	// Macaroon with a third-party caveat at a different location.
	m, err := macaroon.New([]byte("root-key"), "test-id", "test-location")
	if err != nil {
		t.Fatalf("cannot create macaroon: %v", err)
	}

	err = m.AddThirdPartyCaveat([]byte("key"), "caveat-id", "other.location.com")
	if err != nil {
		t.Fatalf("cannot add third-party caveat: %v", err)
	}

	_, err = loginCaveatID(m)
	if err == nil {
		t.Fatal("expected error for wrong-location caveat, got nil")
	}
}
