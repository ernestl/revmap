package cmd

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ernestl/revmap/store"
)

func TestParseTimeValue(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		value   string
		flag    string
		wantErr string
		check   func(t *testing.T, got time.Time)
	}{
		{
			name:  "days",
			value: "30d",
			flag:  "--since",
			check: func(t *testing.T, got time.Time) {
				expected := now.AddDate(0, 0, -30)
				if got.Sub(expected).Abs() > time.Second {
					t.Errorf("expected ~%v, got %v", expected, got)
				}
			},
		},
		{
			name:  "weeks",
			value: "2w",
			flag:  "--since",
			check: func(t *testing.T, got time.Time) {
				expected := now.AddDate(0, 0, -14)
				if got.Sub(expected).Abs() > time.Second {
					t.Errorf("expected ~%v, got %v", expected, got)
				}
			},
		},
		{
			name:  "months",
			value: "3m",
			flag:  "--since",
			check: func(t *testing.T, got time.Time) {
				expected := now.AddDate(0, -3, 0)
				if got.Sub(expected).Abs() > time.Second {
					t.Errorf("expected ~%v, got %v", expected, got)
				}
			},
		},
		{
			name:  "years",
			value: "1y",
			flag:  "--since",
			check: func(t *testing.T, got time.Time) {
				expected := now.AddDate(-1, 0, 0)
				if got.Sub(expected).Abs() > time.Second {
					t.Errorf("expected ~%v, got %v", expected, got)
				}
			},
		},
		{
			name:  "absolute date",
			value: "2024-06-15",
			flag:  "--since",
			check: func(t *testing.T, got time.Time) {
				expected := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
				if !got.Equal(expected) {
					t.Errorf("expected %v, got %v", expected, got)
				}
			},
		},
		{
			name:    "invalid suffix",
			value:   "5x",
			flag:    "--since",
			wantErr: `invalid --since suffix "x"`,
		},
		{
			name:    "invalid format",
			value:   "abc",
			flag:    "--until",
			wantErr: `invalid --until value "abc"`,
		},
		{
			name:    "zero value",
			value:   "0d",
			flag:    "--since",
			wantErr: `invalid --since value "0d"`,
		},
		{
			name:    "negative value",
			value:   "-1d",
			flag:    "--since",
			wantErr: `invalid --since value "-1d"`,
		},
		{
			name:    "single char",
			value:   "d",
			flag:    "--since",
			wantErr: `invalid --since value "d"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTimeValue(tt.value, tt.flag)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.check(t, got)
		})
	}
}

func TestParseTimeWindow(t *testing.T) {
	tests := []struct {
		name    string
		since   string
		until   string
		limit   int
		all     bool
		wantErr string
		check   func(t *testing.T, opts store.FetchOptions)
	}{
		{
			name: "defaults to 90 days",
			check: func(t *testing.T, opts store.FetchOptions) {
				if opts.Since.IsZero() {
					t.Error("expected Since to be set")
				}
				if opts.FetchAll {
					t.Error("expected FetchAll to be false")
				}
			},
		},
		{
			name: "all flag",
			all:  true,
			check: func(t *testing.T, opts store.FetchOptions) {
				if !opts.FetchAll {
					t.Error("expected FetchAll to be true")
				}
			},
		},
		{
			name:    "all with since is error",
			all:     true,
			since:   "7d",
			wantErr: "cannot use --since or --until with --all",
		},
		{
			name:    "all with until is error",
			all:     true,
			until:   "7d",
			wantErr: "cannot use --since or --until with --all",
		},
		{
			name:  "limit with all",
			all:   true,
			limit: 50,
			check: func(t *testing.T, opts store.FetchOptions) {
				if opts.MaxRevisions != 50 {
					t.Errorf("expected MaxRevisions=50, got %d", opts.MaxRevisions)
				}
			},
		},
		{
			name:  "since and until",
			since: "2024-01-01",
			until: "2024-06-30",
			check: func(t *testing.T, opts store.FetchOptions) {
				if opts.Since.IsZero() || opts.Until.IsZero() {
					t.Error("expected both Since and Until to be set")
				}
			},
		},
		{
			name:    "since after until is error",
			since:   "2024-06-30",
			until:   "2024-01-01",
			wantErr: "--since date must be before --until date",
		},
		{
			name:  "until only implies fetch all",
			until: "2024-06-30",
			check: func(t *testing.T, opts store.FetchOptions) {
				if !opts.FetchAll {
					t.Error("expected FetchAll to be true when only --until is set")
				}
				if opts.Until.IsZero() {
					t.Error("expected Until to be set")
				}
			},
		},
		{
			name:  "limit standalone implies fetch all",
			limit: 100,
			check: func(t *testing.T, opts store.FetchOptions) {
				if opts.MaxRevisions != 100 {
					t.Errorf("expected MaxRevisions=100, got %d", opts.MaxRevisions)
				}
				if !opts.FetchAll {
					t.Error("expected FetchAll to be true when --limit is set without time bounds")
				}
				if !opts.Since.IsZero() {
					t.Error("expected Since to be zero when --limit implies fetch all")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := parseTimeWindow(tt.since, tt.until, tt.limit, tt.all)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, opts)
			}
		})
	}
}

func TestResolveColumns(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int // expected number of columns
		wantErr string
	}{
		{
			name:  "default",
			input: defaultColumns,
			want:  5,
		},
		{
			name:  "single column",
			input: "revision",
			want:  1,
		},
		{
			name:  "all columns",
			input: "revision,version,arch,status,created,confinement,base,size",
			want:  8,
		},
		{
			name:  "with spaces",
			input: "revision , version , created",
			want:  3,
		},
		{
			name:  "case insensitive",
			input: "REVISION,Version,ARCH",
			want:  3,
		},
		{
			name:    "unknown column",
			input:   "revision,foo",
			wantErr: `unknown column "foo"`,
		},
		{
			name:  "empty string",
			input: "",
			want:  5, // falls back to default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cols, err := resolveColumns(tt.input)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(cols) != tt.want {
				t.Errorf("expected %d columns, got %d", tt.want, len(cols))
			}
		})
	}
}

func TestMatchesBuildType(t *testing.T) {
	tests := []struct {
		version   string
		buildType string
		want      bool
	}{
		// release
		{"2.75.2", "release", true},
		{"2.75.2+g307.abc", "release", false},
		{"2.50~pre1", "release", false},

		// git
		{"2.75.2+g307.abc", "git", true},
		{"2.75.2+git307.gabc", "git", true},
		{"2.75.2+g307.abc+fips", "git", false},
		{"2.38+git4.g7de2afe-dirty", "git", false},
		{"2.50~pre1+git10.g930660d", "git", false},
		{"2.49~rc1+git942.g47f5210", "git", false},
		{"2.75.2", "git", false},

		// fips
		{"2.75.2+g307.abc+fips", "fips", true},
		{"2.75.2+g307.abc", "fips", false},

		// pre (includes ~rc)
		{"2.50~pre1+git10.g930660d", "pre", true},
		{"2.34~pre1", "pre", true},
		{"2.54~rc1", "pre", true},
		{"2.49~rc1+git942.g47f5210", "pre", true},
		{"2.75.2", "pre", false},

		// dirty
		{"2.38+git4.g7de2afe-dirty", "dirty", true},
		{"2.75.2+g307.abc", "dirty", false},

		// unknown build type with no regex matches everything
		{"2.75.2", "unknown", true},

		// case insensitive
		{"2.75.2", "Release", true},
		{"2.75.2+G307.ABC+FIPS", "FIPS", true},
	}

	for _, tt := range tests {
		t.Run(tt.version+"/"+tt.buildType, func(t *testing.T) {
			got := matchesBuildType(tt.version, tt.buildType)
			if got != tt.want {
				t.Errorf("matchesBuildType(%q, %q) = %v, want %v",
					tt.version, tt.buildType, got, tt.want)
			}
		})
	}
}

func TestMatchesBuildTypeCustomRegex(t *testing.T) {
	// Set the package-level regex to simulate --build with a custom pattern.
	filterBuildRe = regexp.MustCompile(`(?i)\+g\d+\..*\+fips`)
	defer func() { filterBuildRe = nil }()

	tests := []struct {
		version string
		want    bool
	}{
		{"2.75.2+g307.abc+fips", true},
		{"2.75.2+g42.xyz+FIPS", true},
		{"2.75.2+g307.abc", false},
		{"2.75.2", false},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			// Use a non-preset build type to trigger regex path.
			got := matchesBuildType(tt.version, "custom-pattern")
			if got != tt.want {
				t.Errorf("matchesBuildType(%q, custom regex) = %v, want %v",
					tt.version, got, tt.want)
			}
		})
	}
}

func TestApplyFilters(t *testing.T) {
	revisions := []store.RevisionEntry{
		{Revision: 1, Version: "2.75.2", Architectures: []string{"amd64"}, Status: "Published"},
		{Revision: 2, Version: "2.75.2+g307.abc", Architectures: []string{"arm64"}, Status: "Published"},
		{Revision: 3, Version: "2.75.2+fips", Architectures: []string{"amd64"}, Status: "Rejected"},
	}

	// Reset global filter state.
	filterArch = ""
	filterStatus = ""
	filterVer = ""
	filterVerRe = nil
	filterBuild = ""

	// No filters: all revisions returned.
	got := applyFilters(revisions)
	if len(got) != 3 {
		t.Errorf("no filters: expected 3, got %d", len(got))
	}

	// Filter by arch.
	filterArch = "amd64"
	got = applyFilters(revisions)
	if len(got) != 2 {
		t.Errorf("arch=amd64: expected 2, got %d", len(got))
	}
	filterArch = ""

	// Filter by status.
	filterStatus = "Rejected"
	got = applyFilters(revisions)
	if len(got) != 1 {
		t.Errorf("status=Rejected: expected 1, got %d", len(got))
	}
	filterStatus = ""

	// Filter by build type.
	filterBuild = "release"
	got = applyFilters(revisions)
	if len(got) != 1 {
		t.Errorf("build=release: expected 1, got %d", len(got))
	}
	filterBuild = ""
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		s      string
		maxLen int
		want   string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 8, "hello..."},
		{"hello world", 3, "hel"},
		{"hello world", 2, "he"},
		{"hi", 1, "h"},
		{"", 5, ""},
	}

	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			got := truncate(tt.s, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestColumnValueExtractors(t *testing.T) {
	base := "core22"
	rev := store.RevisionEntry{
		Revision:      12345,
		Version:       "2.75.2",
		Architectures: []string{"amd64", "arm64"},
		Status:        "Published",
		CreatedAt:     "2024-06-15T10:30:00Z",
		Confinement:   "strict",
		Base:          &base,
		Size:          52428800, // 50 MB
	}

	tests := []struct {
		col  string
		want string
	}{
		{"revision", "12345"},
		{"version", "2.75.2"},
		{"arch", "amd64,arm64"},
		{"status", "Published"},
		{"created", "2024-06-15"},
		{"confinement", "strict"},
		{"base", "core22"},
		{"size", "50.0 MB"},
	}

	for _, tt := range tests {
		t.Run(tt.col, func(t *testing.T) {
			col, ok := allColumns[tt.col]
			if !ok {
				t.Fatalf("column %q not found", tt.col)
			}
			got := col.value(rev)
			if got != tt.want {
				t.Errorf("column %q value = %q, want %q", tt.col, got, tt.want)
			}
		})
	}

	// Test nil base.
	revNoBase := store.RevisionEntry{Base: nil}
	if got := allColumns["base"].value(revNoBase); got != "" {
		t.Errorf("nil base should return empty string, got %q", got)
	}

	// Test size units.
	small := store.RevisionEntry{Size: 500}
	if got := allColumns["size"].value(small); got != "500 B" {
		t.Errorf("500 bytes: expected %q, got %q", "500 B", got)
	}

	kb := store.RevisionEntry{Size: 2048}
	if got := allColumns["size"].value(kb); got != "2.0 KB" {
		t.Errorf("2048 bytes: expected %q, got %q", "2.0 KB", got)
	}
}
