// Command cache-build fetches snap revision data from the store and writes
// compressed cache files. It is a build-time/CI tool and is not included
// in the revmap snap.
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ernestl/revmap/store"
)

func main() {
	workers := flag.Int("workers", 10, "number of concurrent revision detail fetches")
	flag.Parse()

	if err := run(*workers); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(workers int) error {
	// If not logged in, attempt non-interactive login from env vars.
	if !store.CredentialsExist() {
		email := os.Getenv("REVMAP_EMAIL")
		password := os.Getenv("REVMAP_PASSWORD")
		if email == "" || password == "" {
			fmt.Fprintln(os.Stderr, "Error: no store credentials available.")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "To authenticate, use one of the following methods:")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "  1. Set SNAPCRAFT_STORE_CREDENTIALS (recommended for CI):")
			fmt.Fprintln(os.Stderr, "     export SNAPCRAFT_STORE_CREDENTIALS=$(cat credentials.txt)")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "  2. Set REVMAP_EMAIL and REVMAP_PASSWORD (non-2FA account):")
			fmt.Fprintln(os.Stderr, "     export REVMAP_EMAIL=user@example.com")
			fmt.Fprintln(os.Stderr, "     export REVMAP_PASSWORD=secret")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "  3. Run 'revmap login' first to store credentials locally.")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "For GitHub Actions, add the secret SNAPCRAFT_STORE_CREDENTIALS")
			fmt.Fprintln(os.Stderr, "to your repository (Settings > Secrets and variables > Actions).")
			return fmt.Errorf("credentials not configured")
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
		if err := buildCacheForSnap(client, snapName, workers); err != nil {
			return fmt.Errorf("failed to cache %s: %w", snapName, err)
		}
	}

	fmt.Println("Done.")
	return nil
}

func buildCacheForSnap(client *store.Client, snapName string, workers int) error {
	// Fetch all revisions.
	fmt.Printf("  Fetching revision list (all pages)...\n")
	opts := store.FetchOptions{FetchAll: true}
	releases, err := client.GetReleases(snapName, opts)
	if err != nil {
		return fmt.Errorf("cannot fetch releases: %w", err)
	}
	fmt.Printf("  Found %d revisions.\n", len(releases.Revisions))

	// Fetch individual revision details concurrently.
	fmt.Printf("  Fetching revision details (%d workers)...\n", workers)
	details := make(map[string]map[string]interface{}, len(releases.Revisions))
	var mu sync.Mutex
	var fetchErr error

	sem := make(chan struct{}, workers)
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
				if strings.Contains(err.Error(), "status 404") {
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
