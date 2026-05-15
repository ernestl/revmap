package store

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"gopkg.in/macaroon.v1"
)

// Sentinel errors for the login flow.
var (
	ErrTwoFactorRequired  = errors.New("two-factor authentication required")
	ErrTwoFactorFailed    = errors.New("two-factor authentication failed")
	ErrInvalidCredentials = errors.New("invalid email or password")
)

// ssoError represents an error response from Ubuntu One SSO.
type ssoError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Extra   map[string]string `json:"extra"`
}

// MacaroonSerialize returns a store-compatible serialized representation
// of the given macaroon (URL-safe base64, no padding).
func MacaroonSerialize(m *macaroon.Macaroon) (string, error) {
	marshalled, err := m.MarshalBinary()
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(marshalled), nil
}

// MacaroonDeserialize returns a deserialized macaroon from a
// store-compatible serialization (URL-safe base64, no padding).
func MacaroonDeserialize(serializedMacaroon string) (*macaroon.Macaroon, error) {
	var m macaroon.Macaroon
	decoded, err := base64.RawURLEncoding.DecodeString(serializedMacaroon)
	if err != nil {
		return nil, err
	}
	if err := m.UnmarshalBinary(decoded); err != nil {
		return nil, err
	}
	return &m, nil
}

// loginCaveatID extracts the third-party caveat ID from the root macaroon
// that needs to be discharged by Ubuntu One SSO.
func loginCaveatID(m *macaroon.Macaroon) (string, error) {
	for _, caveat := range m.Caveats() {
		if caveat.Location == UbuntuOneLocation {
			return caveat.Id, nil
		}
	}
	return "", fmt.Errorf("missing login caveat for %s", UbuntuOneLocation)
}

// RequestStoreMacaroon requests a root macaroon from the store with
// the specified permissions.
func RequestStoreMacaroon(httpClient *http.Client) (string, error) {
	const errPrefix = "cannot get snap access permission from store: "

	data := map[string]interface{}{
		"permissions": []string{"package_access"},
	}
	body, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf(errPrefix+"%v", err)
	}

	req, err := http.NewRequest("POST", MacaroonACLAPI, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf(errPrefix+"%v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf(errPrefix+"%v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf(errPrefix+"store returned status %d", resp.StatusCode)
	}

	var result struct {
		Macaroon string `json:"macaroon"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf(errPrefix+"%v", err)
	}
	if result.Macaroon == "" {
		return "", fmt.Errorf(errPrefix + "empty macaroon returned")
	}

	return result.Macaroon, nil
}

// DischargeAuthCaveat discharges the store macaroon's login caveat via
// Ubuntu One SSO using the provided credentials.
func DischargeAuthCaveat(httpClient *http.Client, caveatID, email, password, otp string) (string, error) {
	data := map[string]string{
		"email":     email,
		"password":  password,
		"caveat_id": caveatID,
	}
	if otp != "" {
		data["otp"] = otp
	}
	return requestDischargeMacaroon(httpClient, DischargeAPI, data)
}

// RefreshDischargeMacaroon refreshes an expired discharge macaroon.
func RefreshDischargeMacaroon(httpClient *http.Client, discharge string) (string, error) {
	data := map[string]string{
		"discharge_macaroon": discharge,
	}
	return requestDischargeMacaroon(httpClient, RefreshDischargeAPI, data)
}

// requestDischargeMacaroon posts to the given SSO endpoint to obtain
// or refresh a discharge macaroon.
func requestDischargeMacaroon(httpClient *http.Client, endpoint string, data map[string]string) (string, error) {
	const errPrefix = "cannot authenticate to snap store: "

	body, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf(errPrefix+"%v", err)
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf(errPrefix+"%v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf(errPrefix+"%v", err)
	}
	defer resp.Body.Close()

	// Decode response body for both success and error paths.
	var result struct {
		Macaroon string `json:"discharge_macaroon"`
	}
	var ssoErr ssoError

	// Read body once and try to decode both.
	rawBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf(errPrefix+"cannot read response body: %v", err)
	}

	// Always try to decode the error structure for 4xx responses.
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		_ = json.Unmarshal(rawBytes, &ssoErr)
		switch ssoErr.Code {
		case "TWOFACTOR_REQUIRED":
			return "", ErrTwoFactorRequired
		case "TWOFACTOR_FAILURE":
			return "", ErrTwoFactorFailed
		case "INVALID_CREDENTIALS":
			return "", ErrInvalidCredentials
		default:
			if ssoErr.Message != "" {
				return "", fmt.Errorf(errPrefix+"%s", ssoErr.Message)
			}
			return "", fmt.Errorf(errPrefix+"server returned status %d", resp.StatusCode)
		}
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf(errPrefix+"server returned status %d", resp.StatusCode)
	}

	if err := json.Unmarshal(rawBytes, &result); err != nil {
		return "", fmt.Errorf(errPrefix+"%v", err)
	}
	if result.Macaroon == "" {
		return "", fmt.Errorf(errPrefix + "empty discharge macaroon returned")
	}

	return result.Macaroon, nil
}

// Login performs the full store login flow:
// 1. Request a root macaroon from the store
// 2. Extract the SSO caveat ID
// 3. Discharge the caveat via Ubuntu One SSO
// 4. Save credentials to disk
func Login(email, password, otp string) error {
	httpClient := &http.Client{}

	// Step 1: Request root macaroon from the store.

	rootMacaroon, err := RequestStoreMacaroon(httpClient)
	if err != nil {
		return err
	}

	// Step 2: Deserialize and extract the SSO caveat ID.
	root, err := MacaroonDeserialize(rootMacaroon)
	if err != nil {
		return fmt.Errorf("cannot deserialize root macaroon: %v", err)
	}

	caveatID, err := loginCaveatID(root)
	if err != nil {
		return err
	}

	// Step 3: Discharge the caveat via Ubuntu One SSO.
	dischargeMacaroon, err := DischargeAuthCaveat(httpClient, caveatID, email, password, otp)
	if err != nil {
		return err
	}

	// Step 4: Save credentials.
	if err := SaveCredentials(rootMacaroon, dischargeMacaroon); err != nil {
		return err
	}

	return nil
}
