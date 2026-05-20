package store

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
)

// DefaultSeries is the store series used for snap registrations.
const DefaultSeries = "16"

// AccountInfo holds account details returned by the store.
type AccountInfo struct {
	Email       string
	Username    string
	DisplayName string
	Snaps       []string // sorted snap names (Approved only)
}

// GetAccount fetches account information from the store.
func (c *Client) GetAccount() (*AccountInfo, error) {
	req, err := http.NewRequest("GET", AccountAPI, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("credentials expired or invalid (try 'revmap logout && revmap login')")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("store returned status %d", resp.StatusCode)
	}

	var raw struct {
		Email       string `json:"email"`
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		Snaps       map[string]map[string]struct {
			Status string `json:"status"`
		} `json:"snaps"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("cannot decode account response: %w", err)
	}

	info := &AccountInfo{
		Email:       raw.Email,
		Username:    raw.Username,
		DisplayName: raw.DisplayName,
	}

	// Extract approved snap names from the default series.
	if series, ok := raw.Snaps[DefaultSeries]; ok {
		for name, snap := range series {
			if snap.Status == "Approved" {
				info.Snaps = append(info.Snaps, name)
			}
		}
	}
	sort.Strings(info.Snaps)

	return info, nil
}
