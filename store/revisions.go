package store

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// StoreError represents an HTTP error response from the store API.
type StoreError struct {
	StatusCode int
	Message    string
}

func (e *StoreError) Error() string {
	return e.Message
}

// RevisionsAPI returns the endpoint URL for fetching a specific snap revision.
func RevisionsAPI(snapName string, revision string) string {
	return fmt.Sprintf("%sapi/v2/snaps/%s/revisions/%s", StoreDashboardURL, snapName, revision)
}

// ReleasesAPI returns the endpoint URL for fetching the first page of
// releases and revisions of a snap with maximum page size.
func ReleasesAPI(snapName string) string {
	return fmt.Sprintf("%sapi/v2/snaps/%s/releases?page=1&size=500", StoreDashboardURL, snapName)
}

// RevisionInfo represents the response from the store's revision endpoint.
type RevisionInfo struct {
	// Raw holds the full decoded JSON response for flexible access.
	Raw map[string]interface{}
}

// RevisionEntry represents a single revision in the releases response.
type RevisionEntry struct {
	Revision      int      `json:"revision"`
	Version       string   `json:"version"`
	Architectures []string `json:"architectures"`
	Status        string   `json:"status"`
	CreatedAt     string   `json:"created_at"`
	Confinement   string   `json:"confinement"`
	Base          *string  `json:"base"`
	Size          int64    `json:"size"`
}

// paginationLinks represents the _links object in a paginated response.
type paginationLinks struct {
	Next string `json:"next"`
}

// releasesPage represents a single page of the releases response.
type releasesPage struct {
	Revisions []RevisionEntry          `json:"revisions"`
	Releases  []map[string]interface{} `json:"releases"`
	Links     paginationLinks          `json:"_links"`
}

// ReleasesResponse represents the aggregated response from all pages
// of the releases endpoint.
type ReleasesResponse struct {
	Revisions []RevisionEntry
}

// FetchOptions controls how many pages of revisions to fetch.
type FetchOptions struct {
	// MaxRevisions stops pagination after collecting this many unique
	// revisions. Zero means no count limit.
	MaxRevisions int

	// Since stops pagination when all revisions on a page are older
	// than this time. Zero value means no time limit.
	Since time.Time

	// Until excludes revisions newer than this time.
	// Zero value means no upper time limit.
	Until time.Time

	// FetchAll fetches every page (subject to MaxRevisions count limit).
	// Set automatically when --limit is used without explicit time bounds.
	FetchAll bool
}

// GetRevision fetches information about a specific revision of a snap.
func (c *Client) GetRevision(snapName string, revision string) (*RevisionInfo, error) {
	url := RevisionsAPI(snapName, revision)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &StoreError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("store returned status %d for %s revision %s", resp.StatusCode, snapName, revision),
		}
	}

	var raw map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("cannot decode revision response: %w", err)
	}

	return &RevisionInfo{Raw: raw}, nil
}

// GetReleases fetches releases and revisions for a snap, respecting
// the provided FetchOptions to limit pagination. Revisions are
// deduplicated by revision number.
func (c *Client) GetReleases(snapName string, opts FetchOptions) (*ReleasesResponse, error) {
	seen := make(map[int]bool)
	var allRevisions []RevisionEntry

	nextURL := ReleasesAPI(snapName)
	for nextURL != "" {
		page, err := c.fetchReleasesPage(nextURL)
		if err != nil {
			return nil, err
		}

		// Track how many revisions from this page fall within
		// the time window (used for time-based early exit).
		pageInWindow := 0

		for _, rev := range page.Revisions {
			if seen[rev.Revision] {
				continue
			}
			seen[rev.Revision] = true

			// Parse the creation time for time-based filtering.
			var created time.Time
			var hasTime bool
			if !opts.Since.IsZero() || !opts.Until.IsZero() {
				t, err := time.Parse(time.RFC3339, rev.CreatedAt)
				if err == nil {
					created = t
					hasTime = true
				}
			}

			// Skip revisions newer than the upper bound.
			if hasTime && !opts.Until.IsZero() && created.After(opts.Until) {
				continue
			}

			// For time-based limits, skip revisions older than
			// the cutoff but keep processing the page to check
			// if any are still within the window.
			if hasTime && !opts.Since.IsZero() && created.Before(opts.Since) {
				continue
			}

			if hasTime {
				pageInWindow++
			}

			allRevisions = append(allRevisions, rev)
		}

		// Count-based limit: stop once we have enough.
		if opts.MaxRevisions > 0 && len(allRevisions) >= opts.MaxRevisions {
			allRevisions = allRevisions[:opts.MaxRevisions]
			break
		}

		// Time-based early exit: when --since is set and no revisions on
		// this page fell within the window, all remaining pages are older
		// so we can stop. This only applies when Since is set because
		// pages are ordered newest-first; --until alone must page through
		// newer revisions to reach the target window.
		if !opts.Since.IsZero() && pageInWindow == 0 {
			break
		}

		nextURL = page.Links.Next
	}

	return &ReleasesResponse{Revisions: allRevisions}, nil
}

// fetchReleasesPage fetches a single page from the given URL.
func (c *Client) fetchReleasesPage(url string) (*releasesPage, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &StoreError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("store returned status %d fetching releases", resp.StatusCode),
		}
	}

	var page releasesPage
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return nil, fmt.Errorf("cannot decode releases response: %w", err)
	}

	return &page, nil
}
