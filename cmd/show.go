package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/ernestl/revmap/store"
	"github.com/spf13/cobra"
)

var fields string

var showCmd = &cobra.Command{
	Use:   "show <snap> <revision>",
	Short: "Show details of a specific snap revision",
	Long: `Show full details for a specific revision of a snap.

Requires authentication. Run 'revmap login' first.
If not authenticated or lacking permissions, cached data is
used automatically when available.

Examples:
  revmap show snapd 17339
  revmap show snapd 17339 -f version,status,architectures`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		snapName := args[0]
		revision := args[1]

		// If not authenticated, try to use cached data.
		if !store.CredentialsExist() {
			return showFromCache(snapName, revision, "run 'revmap login' for live results")
		}

		client := store.NewClient()
		return showRevision(client, snapName, revision)
	},
}

// showRevision fetches and displays a single revision.
func showRevision(client *store.Client, snapName, revision string) error {
	info, err := client.GetRevision(snapName, revision)
	if err != nil {
		if isCacheFallbackErr(err) {
			return showFromCache(snapName, revision, "insufficient permissions for live data")
		}
		return err
	}

	result := filterFields(info.Raw, fields)

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot format output: %w", err)
	}
	fmt.Fprintln(os.Stdout, string(output))
	return nil
}

// filterFields returns only the requested fields from the API response.
// If fieldList is empty, the full response is returned unchanged.
// Fields are looked up inside the top-level "revision" object if present,
// otherwise from the top level directly.
func filterFields(raw map[string]interface{}, fieldList string) interface{} {
	if fieldList == "" {
		return raw
	}

	// Use the nested "revision" object if it exists, otherwise top level.
	source := raw
	if rev, ok := raw["revision"].(map[string]interface{}); ok {
		source = rev
	}

	requested := parseFieldList(fieldList)
	filtered := make(map[string]interface{}, len(requested))
	for _, f := range requested {
		if val, ok := source[f]; ok {
			filtered[f] = val
		}
	}
	return filtered
}

// parseFieldList splits a comma-separated field list into trimmed field names.
func parseFieldList(fieldList string) []string {
	parts := strings.Split(fieldList, ",")
	var result []string
	for _, f := range parts {
		f = strings.TrimSpace(f)
		if f != "" {
			result = append(result, f)
		}
	}
	return result
}

// showFromCache attempts to serve the show request from the
// pre-built cache. If no cache is available, it returns an error
// with the given reason context.
func showFromCache(snapName, revision, reason string) error {
	cachePath := store.FindCacheFile(snapName)
	if cachePath == "" {
		return fmt.Errorf("no cache available for %q (%s)", snapName, reason)
	}

	cacheData, err := store.ReadCache(cachePath)
	if err != nil {
		return fmt.Errorf("cannot read cache: %w", err)
	}

	raw, ok := cacheData.Details[revision]
	if !ok {
		return fmt.Errorf("revision %s not found in cache for %q", revision, snapName)
	}

	fmt.Fprintf(os.Stderr, "Using cached data from %s (%s)\n\n",
		cacheData.CachedAt.Format("2006-01-02"), reason)

	result := filterFields(raw, fields)

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot format output: %w", err)
	}
	fmt.Fprintln(os.Stdout, string(output))
	return nil
}
