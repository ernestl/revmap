package cmd

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ernestl/revmap/store"
	"github.com/spf13/cobra"
)

var (
	filterArch   string
	filterVer    string
	filterVerRe  *regexp.Regexp
	filterStatus string
	filterBuild  string
	since        string
	until        string
	limit        int
	fetchAll     bool
	columns      string
)

// column defines a table column with its header and value extractor.
type column struct {
	header string
	value  func(store.RevisionEntry) string
	fixed  bool // fixed-width columns are not shrunk
}

// allColumns maps column names to their definitions.
var allColumns = map[string]column{
	"revision": {
		header: "REVISION",
		value:  func(r store.RevisionEntry) string { return fmt.Sprintf("%d", r.Revision) },
		fixed:  true,
	},
	"version": {
		header: "VERSION",
		value:  func(r store.RevisionEntry) string { return r.Version },
	},
	"arch": {
		header: "ARCH",
		value:  func(r store.RevisionEntry) string { return strings.Join(r.Architectures, ",") },
	},
	"status": {
		header: "STATUS",
		value:  func(r store.RevisionEntry) string { return r.Status },
	},
	"created": {
		header: "CREATED",
		value: func(r store.RevisionEntry) string {
			if len(r.CreatedAt) > 10 {
				return r.CreatedAt[:10]
			}
			return r.CreatedAt
		},
		fixed: true,
	},
	"confinement": {
		header: "CONFINEMENT",
		value:  func(r store.RevisionEntry) string { return r.Confinement },
	},
	"base": {
		header: "BASE",
		value: func(r store.RevisionEntry) string {
			if r.Base != nil {
				return *r.Base
			}
			return ""
		},
	},
	"size": {
		header: "SIZE",
		value: func(r store.RevisionEntry) string {
			if r.Size < 1024 {
				return fmt.Sprintf("%d B", r.Size)
			}
			if r.Size < 1024*1024 {
				return fmt.Sprintf("%.1f KB", float64(r.Size)/1024)
			}
			return fmt.Sprintf("%.1f MB", float64(r.Size)/(1024*1024))
		},
		fixed: true,
	},
}

const defaultColumns = "revision,version,arch,status,created"

// columnNames returns the sorted list of available column names.
func columnNames() string {
	return "revision, version, arch, status, created, confinement, base, size"
}

var listCmd = &cobra.Command{
	Use:   "list <snap>",
	Short: "List revision history of a snap",
	Long: `List the revision history of a snap published in the Snap Store.

Requires authentication. Run 'revmap login' first.
By default only the last 90 days are shown.

Examples:
  revmap list snapd
  revmap list snapd --since 7d -a amd64
  revmap list snapd --since 2024-01-01 --until 2024-06-30
  revmap list snapd --all -b release -c revision,version,size`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		snapName := args[0]

		if !store.CredentialsExist() {
			return fmt.Errorf("not logged in (run 'revmap login' first)")
		}

		// Compile version regex if provided.
		if filterVer != "" {
			re, err := regexp.Compile("(?i)" + filterVer)
			if err != nil {
				return fmt.Errorf("invalid --version regex %q: %v", filterVer, err)
			}
			filterVerRe = re
		}

		client := store.NewClient()
		return listRevisions(client, snapName)
	},
}

// listRevisions fetches revisions and displays them as a table,
// sorted by revision number descending.
func listRevisions(client *store.Client, snapName string) error {
	opts, err := parseTimeWindow(since, until, limit, fetchAll)
	if err != nil {
		return err
	}

	// Resolve columns.
	cols, err := resolveColumns(columns)
	if err != nil {
		return err
	}

	releases, err := client.GetReleases(snapName, opts)
	if err != nil {
		return err
	}

	if len(releases.Revisions) == 0 {
		fmt.Println("No revisions found.")
		return nil
	}

	// Sort by revision number descending (newest first).
	sort.Slice(releases.Revisions, func(i, j int) bool {
		return releases.Revisions[i].Revision > releases.Revisions[j].Revision
	})

	// Apply row filters.
	filtered := applyFilters(releases.Revisions)

	if len(filtered) == 0 {
		fmt.Println("No revisions match the given filters.")
		return nil
	}

	printTable(cols, filtered)

	fmt.Printf("\nTotal: %d revisions\n", len(filtered))
	return nil
}

// resolveColumns parses the --columns flag and returns the ordered
// list of column definitions.
func resolveColumns(colStr string) ([]column, error) {
	if colStr == "" {
		colStr = defaultColumns
	}

	parts := strings.Split(colStr, ",")
	cols := make([]column, 0, len(parts))
	for _, p := range parts {
		name := strings.TrimSpace(strings.ToLower(p))
		if name == "" {
			continue
		}
		col, ok := allColumns[name]
		if !ok {
			return nil, fmt.Errorf("unknown column %q (available: %s)", name, columnNames())
		}
		cols = append(cols, col)
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("no columns specified")
	}
	return cols, nil
}

// parseTimeWindow builds FetchOptions from the --since, --until, --limit,
// and --all flags.
func parseTimeWindow(sinceVal, untilVal string, limitVal int, all bool) (store.FetchOptions, error) {
	// Validate mutual exclusivity.
	if all && (sinceVal != "" || untilVal != "") {
		return store.FetchOptions{}, fmt.Errorf("cannot use --since or --until with --all")
	}

	if all {
		return store.FetchOptions{FetchAll: true, MaxRevisions: limitVal}, nil
	}

	var opts store.FetchOptions
	opts.MaxRevisions = limitVal

	// Parse --since.
	if sinceVal != "" {
		t, err := parseTimeValue(sinceVal, "--since")
		if err != nil {
			return store.FetchOptions{}, err
		}
		opts.Since = t
	} else if untilVal == "" {
		// Default: 90 days when neither --since nor --until is set.
		opts.Since = time.Now().AddDate(0, 0, -90)
	} else {
		// --until without --since: fetch all pages up to the cutoff.
		opts.FetchAll = true
	}

	// Parse --until.
	if untilVal != "" {
		t, err := parseTimeValue(untilVal, "--until")
		if err != nil {
			return store.FetchOptions{}, err
		}
		// Set until to end of day so the date is inclusive.
		opts.Until = t.Add(24*time.Hour - time.Second)
	}

	// Validate that since is before until.
	if !opts.Since.IsZero() && !opts.Until.IsZero() && !opts.Since.Before(opts.Until) {
		return store.FetchOptions{}, fmt.Errorf("--since date must be before --until date")
	}

	return opts, nil
}

// parseTimeValue parses a time value that can be either a relative
// duration (Nd, Nw, Nm, Ny) or an absolute date (yyyy-mm-dd).
// flagName is used in error messages.
func parseTimeValue(value, flagName string) (time.Time, error) {
	// Try absolute date first (yyyy-mm-dd).
	if t, err := time.Parse("2006-01-02", value); err == nil {
		return t, nil
	}

	if len(value) < 2 {
		return time.Time{}, fmt.Errorf("invalid %s value %q (use Nd, Nw, Nm, Ny, or yyyy-mm-dd)", flagName, value)
	}

	suffix := value[len(value)-1]
	numStr := value[:len(value)-1]
	n, err := strconv.Atoi(numStr)
	if err != nil || n <= 0 {
		return time.Time{}, fmt.Errorf("invalid %s value %q (use Nd, Nw, Nm, Ny, or yyyy-mm-dd)", flagName, value)
	}

	now := time.Now()
	switch suffix {
	case 'd':
		return now.AddDate(0, 0, -n), nil
	case 'w':
		return now.AddDate(0, 0, -n*7), nil
	case 'm':
		return now.AddDate(0, -n, 0), nil
	case 'y':
		return now.AddDate(-n, 0, 0), nil
	default:
		return time.Time{}, fmt.Errorf("invalid %s suffix %q (use d, w, m, or y)", flagName, string(suffix))
	}
}

// applyFilters returns only the revisions matching all active filter flags.
func applyFilters(revisions []store.RevisionEntry) []store.RevisionEntry {
	if filterArch == "" && filterVer == "" && filterStatus == "" && filterBuild == "" {
		return revisions
	}

	var result []store.RevisionEntry
	for _, rev := range revisions {
		if !matchesFilters(rev) {
			continue
		}
		result = append(result, rev)
	}
	return result
}

// matchesFilters returns true if a revision matches all active filters.
func matchesFilters(rev store.RevisionEntry) bool {
	if filterArch != "" && !containsArch(rev.Architectures, filterArch) {
		return false
	}
	if filterStatus != "" && !strings.EqualFold(rev.Status, filterStatus) {
		return false
	}
	if filterVerRe != nil && !filterVerRe.MatchString(rev.Version) {
		return false
	}
	if filterBuild != "" && !matchesBuildType(rev.Version, filterBuild) {
		return false
	}
	return true
}

// matchesBuildType checks whether a version string matches the requested
// build type. Recognised types:
//
//	release - base version only, no "+" or "~" suffix (e.g. "2.75.2")
//	git     - has +g or +git suffix, excluding fips/dirty/pre/rc (e.g. "2.75.2+g307.abc")
//	fips    - has +fips anywhere in the version (e.g. "2.75.2+g307.abc+fips")
//	pre     - pre-release builds with ~pre (e.g. "2.63~pre1+git10.g930660d")
//	rc      - release candidates with ~rc (e.g. "2.54~rc1")
//	dirty   - builds from uncommitted trees with -dirty (e.g. "2.38+git4.g7de2afe-dirty")
func matchesBuildType(version, buildType string) bool {
	ver := strings.ToLower(version)
	switch strings.ToLower(buildType) {
	case "release":
		return !strings.Contains(ver, "+") && !strings.Contains(ver, "~")
	case "git":
		hasGit := strings.Contains(ver, "+g") || strings.Contains(ver, "+git")
		hasFips := strings.Contains(ver, "+fips")
		hasDirty := strings.Contains(ver, "-dirty")
		hasPre := strings.Contains(ver, "~pre")
		hasRC := strings.Contains(ver, "~rc")
		return hasGit && !hasFips && !hasDirty && !hasPre && !hasRC
	case "fips":
		return strings.Contains(ver, "+fips")
	case "pre":
		return strings.Contains(ver, "~pre")
	case "rc":
		return strings.Contains(ver, "~rc")
	case "dirty":
		return strings.Contains(ver, "-dirty")
	default:
		return true
	}
}

// containsArch checks if the architecture list contains the given value
// (case-insensitive).
func containsArch(archs []string, target string) bool {
	target = strings.ToLower(target)
	for _, a := range archs {
		if strings.ToLower(a) == target {
			return true
		}
	}
	return false
}

const maxTableWidth = 80

// printTable prints revisions as a fixed-width table that never exceeds
// maxTableWidth columns. If the natural widths overflow, the widest
// shrinkable column is iteratively shrunk until everything fits.
// Values that exceed their column width are truncated with "...".
func printTable(cols []column, revisions []store.RevisionEntry) {
	n := len(cols)

	// Compute natural widths (start with header widths).
	widths := make([]int, n)
	for i, col := range cols {
		widths[i] = len(col.header)
	}

	// Build all cell values and measure widths.
	rows := make([][]string, len(revisions))
	for r, rev := range revisions {
		row := make([]string, n)
		for c, col := range cols {
			row[c] = col.value(rev)
			if len(row[c]) > widths[c] {
				widths[c] = len(row[c])
			}
		}
		rows[r] = row
	}

	// Gaps: 2 spaces between each pair of columns.
	gaps := (n - 1) * 2
	budget := maxTableWidth - gaps

	// Shrink shrinkable columns (non-fixed) until total fits.
	for {
		total := 0
		for _, w := range widths {
			total += w
		}
		if total <= budget {
			break
		}

		// Find the widest shrinkable column.
		maxIdx, maxW := -1, 0
		for i, col := range cols {
			if !col.fixed && widths[i] > maxW {
				maxW = widths[i]
				maxIdx = i
			}
		}
		if maxIdx == -1 || widths[maxIdx] <= len(cols[maxIdx].header) {
			break // can't shrink further
		}

		widths[maxIdx]--
	}

	// Ensure no column is narrower than its header.
	for i, col := range cols {
		if widths[i] < len(col.header) {
			widths[i] = len(col.header)
		}
	}

	// Build format string. Last column is not padded.
	var fmtParts []string
	for i := range cols {
		if i < n-1 {
			fmtParts = append(fmtParts, fmt.Sprintf("%%-%ds", widths[i]))
		} else {
			fmtParts = append(fmtParts, "%s")
		}
	}
	fmtStr := strings.Join(fmtParts, "  ") + "\n"

	// Print headers.
	headerVals := make([]interface{}, n)
	sepVals := make([]interface{}, n)
	for i, col := range cols {
		headerVals[i] = col.header
		sepVals[i] = strings.Repeat("-", widths[i])
	}
	fmt.Printf(fmtStr, headerVals...)
	fmt.Printf(fmtStr, sepVals...)

	// Print rows.
	for _, row := range rows {
		vals := make([]interface{}, n)
		for i, v := range row {
			vals[i] = truncate(v, widths[i])
		}
		fmt.Printf(fmtStr, vals...)
	}
}

// truncate shortens s to maxLen, replacing the tail with "..." if needed.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func init() {
	listCmd.Flags().SortFlags = false

	// Scope.
	listCmd.Flags().StringVar(&since, "since", "", "start of time window (Nd, Nw, Nm, Ny, or yyyy-mm-dd; default 90d)")
	listCmd.Flags().StringVar(&until, "until", "", "end of time window (Nd, Nw, Nm, Ny, or yyyy-mm-dd)")
	listCmd.Flags().IntVarP(&limit, "limit", "n", 0, "maximum number of revisions to return")
	listCmd.Flags().BoolVar(&fetchAll, "all", false, "fetch the complete revision history")

	// Filters.
	listCmd.Flags().StringVarP(&filterArch, "arch", "a", "", "filter by architecture (e.g. amd64, arm64)")
	listCmd.Flags().StringVarP(&filterStatus, "status", "s", "", "filter by status (e.g. Published)")
	listCmd.Flags().StringVar(&filterVer, "version", "", "filter by version (regex, e.g. '2\\.75\\.2$' or 'fips')")
	listCmd.Flags().StringVarP(&filterBuild, "build", "b", "", "filter by build type: release, git, fips, pre, rc, dirty")

	// Display.
	listCmd.Flags().StringVarP(&columns, "columns", "c", defaultColumns, "comma-separated list of columns to display")

	rootCmd.AddCommand(listCmd)
}
