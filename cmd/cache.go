package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ernestl/revmap/store"
	"github.com/spf13/cobra"
)

var cacheConcurrency int

var cacheBuildCmd = &cobra.Command{
	Use:   "cache-build",
	Short: "Build revision cache for configured snaps",
	Long: `Fetch the complete revision history and individual revision details
for all snaps listed in cache-snaps.json, and write compressed cache
files to the cache/ directory.

This command requires authentication. If not already logged in,
it can authenticate automatically using REVMAP_EMAIL and
REVMAP_PASSWORD environment variables. The account must not have
two-factor authentication (2FA) enabled.

Example (local, already logged in):
  revmap login
  revmap cache-build

Example (CI, env vars):
  export REVMAP_EMAIL="user@example.com"
  export REVMAP_PASSWORD="secret"
  revmap cache-build`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// If not logged in, attempt non-interactive login from env vars.
		if !store.CredentialsExist() {
			email := os.Getenv("REVMAP_EMAIL")
			password := os.Getenv("REVMAP_PASSWORD")
			if email == "" || password == "" {
				return fmt.Errorf("not logged in (run 'revmap login' first, or set REVMAP_EMAIL and REVMAP_PASSWORD)")
			}
			fmt.Println("Authenticating via environment credentials...")
			if err := store.Login(email, password, ""); err != nil {
				return fmt.Errorf("automatic login failed: %w", err)
			}
		}

		configPath := store.FindCacheConfig()
		if configPath == "" {
			return fmt.Errorf("cache-snaps.json not found")
		}

		snaps, err := store.LoadCacheSnaps(configPath)
		if err != nil {
			return fmt.Errorf("cannot load cache config: %w", err)
		}

		if len(snaps) == 0 {
			fmt.Println("No snaps configured in cache-snaps.json.")
			return nil
		}

		client := store.NewClient()

		for i, snapName := range snaps {
			fmt.Printf("[%d/%d] Caching %s...\n", i+1, len(snaps), snapName)
			if err := buildCacheForSnap(client, snapName); err != nil {
				return fmt.Errorf("failed to cache %s: %w", snapName, err)
			}
		}

		fmt.Println("Done.")
		return nil
	},
}

func buildCacheForSnap(client *store.Client, snapName string) error {
	// Fetch all revisions.
	fmt.Printf("  Fetching revision list (all pages)...\n")
	opts := store.FetchOptions{FetchAll: true}
	releases, err := client.GetReleases(snapName, opts)
	if err != nil {
		return fmt.Errorf("cannot fetch releases: %w", err)
	}
	fmt.Printf("  Found %d revisions.\n", len(releases.Revisions))

	// Fetch individual revision details concurrently.
	fmt.Printf("  Fetching revision details (%d workers)...\n", cacheConcurrency)
	details := make(map[string]map[string]interface{}, len(releases.Revisions))
	var mu sync.Mutex
	var fetchErr error

	sem := make(chan struct{}, cacheConcurrency)
	var wg sync.WaitGroup

	for i, rev := range releases.Revisions {
		if fetchErr != nil {
			break
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(revision int, idx int) {
			defer wg.Done()
			defer func() { <-sem }()

			revStr := strconv.Itoa(revision)
			info, err := client.GetRevision(snapName, revStr)
			if err != nil {
				// Skip 404s — some revisions in the releases list
				// may have been deleted from the revision endpoint.
				if isNotFoundErr(err) {
					mu.Lock()
					if (idx+1)%100 == 0 {
						fmt.Printf("    %d/%d details fetched\n", len(details), len(releases.Revisions))
					}
					mu.Unlock()
					return
				}
				mu.Lock()
				if fetchErr == nil {
					fetchErr = fmt.Errorf("revision %d: %w", revision, err)
				}
				mu.Unlock()
				return
			}

			mu.Lock()
			details[revStr] = info.Raw
			if (idx+1)%100 == 0 || idx+1 == len(releases.Revisions) {
				fmt.Printf("    %d/%d details fetched\n", len(details), len(releases.Revisions))
			}
			mu.Unlock()
		}(rev.Revision, i)
	}

	wg.Wait()

	if fetchErr != nil {
		return fetchErr
	}

	// Write cache file.
	cacheData := &store.CacheData{
		Snap:      snapName,
		CachedAt:  time.Now().UTC(),
		Revisions: releases.Revisions,
		Details:   details,
	}

	outPath := fmt.Sprintf("cache/%s.json.gz", snapName)
	fmt.Printf("  Writing %s...\n", outPath)
	if err := store.WriteCache(outPath, cacheData); err != nil {
		return err
	}

	return nil
}

func init() {
	cacheBuildCmd.Flags().IntVar(&cacheConcurrency, "workers", 10, "number of concurrent revision detail fetches")
	rootCmd.AddCommand(cacheBuildCmd)
}

// isNotFoundErr returns true if the error indicates a 404 response.
func isNotFoundErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "status 404")
}

// isCacheFallbackErr returns true if the error indicates a permission
// or access issue (401, 403, 404) where falling back to cache is appropriate.
func isCacheFallbackErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "status 401") ||
		strings.Contains(msg, "status 403") ||
		strings.Contains(msg, "status 404")
}
