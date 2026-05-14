package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// Client is an HTTP client that automatically attaches store
// macaroon authentication headers to requests.
type Client struct {
	httpClient *http.Client
}

// NewClient creates a new authenticated store client with connection
// pooling optimised for concurrent requests.
func NewClient() *Client {
	return NewClientWithWorkers(30)
}

// NewClientWithWorkers creates a new authenticated store client with
// the HTTP transport tuned for the given concurrency level.
func NewClientWithWorkers(workers int) *Client {
	transport := &http.Transport{
		MaxIdleConns:        workers + 10,
		MaxIdleConnsPerHost: workers + 10,
		IdleConnTimeout:     90 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}
	return &Client{
		httpClient: &http.Client{
			Transport: transport,
		},
	}
}

// Do executes an HTTP request with store authentication.
// If the store returns a 401 with "macaroon-needs-refresh", the
// discharge macaroon is refreshed and the request is retried once.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	// Buffer the request body so we can replay it on retry.
	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("cannot read request body: %v", err)
		}
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	if err := c.authorize(req); err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}

	// Check if the store is asking for a macaroon refresh.
	if !needsRefresh(resp) {
		return resp, nil
	}
	resp.Body.Close()

	// Attempt to refresh the discharge macaroon.
	if err := c.refreshDischarge(); err != nil {
		return nil, fmt.Errorf("cannot refresh credentials: %v", err)
	}

	// Replay the request with fresh credentials.
	if bodyBytes != nil {
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}
	if err := c.authorize(req); err != nil {
		return nil, err
	}
	return c.httpClient.Do(req)
}

// needsRefresh checks whether a 401 response contains the
// "macaroon-needs-refresh" error code from the store.
// It reads and restores the response body so the caller can
// still consume it if this returns false.
func needsRefresh(resp *http.Response) bool {
	var body struct {
		ErrorList []struct {
			Code string `json:"code"`
		} `json:"error_list"`
		// Some endpoints use "error-list" with a hyphen.
		ErrorListAlt []struct {
			Code string `json:"code"`
		} `json:"error-list"`
	}

	// Read the body, then restore it for the caller.
	raw, err := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewReader(raw))
	if err != nil {
		return false
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return false
	}

	allErrors := append(body.ErrorList, body.ErrorListAlt...)
	for _, e := range allErrors {
		if e.Code == "macaroon-needs-refresh" {
			return true
		}
	}
	return false
}

// refreshDischarge refreshes the stored discharge macaroon and
// persists the updated credentials.
func (c *Client) refreshDischarge() error {
	creds, err := LoadCredentials()
	if err != nil {
		return err
	}

	newDischarge, err := RefreshDischargeMacaroon(c.httpClient, creds.Discharge)
	if err != nil {
		return err
	}

	return SaveCredentials(creds.Root, newDischarge)
}

// authorize attaches the macaroon Authorization header to the request.
func (c *Client) authorize(req *http.Request) error {
	creds, err := LoadCredentials()
	if err != nil {
		return err
	}

	// Deserialize the root macaroon to get its signature for binding.
	root, err := MacaroonDeserialize(creds.Root)
	if err != nil {
		return fmt.Errorf("cannot deserialize root macaroon: %v", err)
	}

	// Deserialize the discharge macaroon and bind it to the root.
	discharge, err := MacaroonDeserialize(creds.Discharge)
	if err != nil {
		return fmt.Errorf("cannot deserialize discharge macaroon: %v", err)
	}
	discharge.Bind(root.Signature())

	// Re-serialize the bound discharge.
	boundDischarge, err := MacaroonSerialize(discharge)
	if err != nil {
		return fmt.Errorf("cannot serialize bound discharge macaroon: %v", err)
	}

	// Set the Authorization header in the format expected by the store.
	req.Header.Set("Authorization",
		fmt.Sprintf(`Macaroon root="%s", discharge="%s"`, creds.Root, boundDischarge))

	return nil
}
