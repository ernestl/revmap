# Design

This document describes the architecture and key design decisions behind revmap.

## Overview

revmap is a read-only CLI tool that queries the Snap Store's dashboard API to display revision and version history for published snaps. It authenticates using the same macaroon-based scheme as snapcraft.

## Project Structure

```
revmap/
  main.go                 Entry point; embeds README, sets version via ldflags
  cmd/
    root.go               Root Cobra command
    version.go            Version resolution (ldflags or VCS fallback)
    login.go              Interactive login flow
    logout.go             Credential removal
    list.go               Revision listing with filters and table output
    show.go               Single revision detail view
    readme.go             Embedded README display
    list_test.go           Tests for list logic
    show_test.go           Tests for show logic
    version_test.go        Tests for version logic
  store/
    constants.go          API URLs and app-wide constants
    auth.go               Macaroon serialization, SSO discharge, login flow
    credentials.go        File-based credential storage with env var override
    client.go             Authenticated HTTP client with auto-refresh
    revisions.go          Store API calls (revisions, releases with pagination)
    auth_test.go          Tests for macaroon serialization and caveat extraction
    credentials_test.go   Tests for credential storage
    client_test.go        Tests for refresh detection
```

## Authentication

### Version

The binary version is set via one of two mechanisms:

1. **ldflags (release builds)** -- `go build -ldflags "-X main.version=1.0.0"` sets a package-level `version` variable in `main.go`, which is passed to `cmd.SetVersion()`. Output: `revmap 1.0.0`.

2. **VCS build info (dev builds)** -- When `version` is empty, `runtime/debug.ReadBuildInfo()` extracts the git commit hash (`vcs.revision`) and dirty flag (`vcs.modified`). Output: `revmap dev (abc1234)` or `revmap dev (abc1234, dirty)`.

Cobra's built-in `Version` field provides the `--version` flag automatically.

### Macaroon Scheme

The Snap Store uses a two-macaroon authentication model:

1. **Root macaroon** -- Obtained from the store's ACL endpoint (`POST /dev/api/acl/`) with `package_access` permission. Contains a third-party caveat that must be discharged by Ubuntu One SSO.

2. **Discharge macaroon** -- Obtained from Ubuntu One SSO (`POST /api/v2/tokens/discharge`) using email, password, and optional OTP. Bound to the root macaroon's signature before use.

The authorization header sent with every request:

```
Macaroon root="<root>", discharge="<bound-discharge>"
```

Macaroons are serialized with `base64.RawURLEncoding` (URL-safe, no padding) and backed by `gopkg.in/macaroon.v1`, matching snapd's implementation.

### Credential Storage

The credentials file path is resolved with the following priority:

1. `$SNAP_USER_COMMON/credentials.json` -- When running as a snap (strict confinement). This directory persists across snap refreshes, unlike `$SNAP_USER_DATA` which is versioned.
2. `$XDG_DATA_HOME/revmap/credentials.json` -- When `XDG_DATA_HOME` is set.
3. `~/.local/share/revmap/credentials.json` -- Default.

The file contains JSON with the serialized root and discharge macaroons (`{"r":"...","d":"..."}`), written with `0600` permissions.

The `REVMAP_STORE_CREDENTIALS` environment variable overrides file-based storage. It auto-detects two formats:

1. **Snapcraft export format** -- The INI-style output from `snapcraft export-login`, containing `macaroon` and `unbound_discharge` fields under `[login.ubuntu.com]`. This is the recommended approach for CI pipelines.

2. **Base64-encoded JSON** -- Standard base64 encoding of the credentials JSON file (`{"r":"...","d":"..."}`). Useful for encoding the file revmap itself creates.

### Auto-Refresh

When the store returns a `401` response with the error code `macaroon-needs-refresh`, the client automatically:

1. Reads the current discharge macaroon from storage
2. Posts it to the SSO refresh endpoint (`POST /api/v2/tokens/refresh`)
3. Saves the new discharge macaroon
4. Replays the original request with fresh credentials

Request bodies are buffered to support replay.

## Store API

### Endpoints Used

| Endpoint | Method | Purpose |
|---|---|---|
| `/dev/api/acl/` | POST | Request root macaroon |
| `login.ubuntu.com/api/v2/tokens/discharge` | POST | Discharge SSO caveat |
| `login.ubuntu.com/api/v2/tokens/refresh` | POST | Refresh expired discharge |
| `/api/v2/snaps/{name}/revisions/{rev}` | GET | Single revision details |
| `/api/v2/snaps/{name}/releases?page=N&size=500` | GET | Paginated revision listing |

### Pagination

The releases endpoint returns pages of up to 500 revisions, ordered newest-first. Each page includes a `_links.next` URL for the next page.

Pagination stops early based on `FetchOptions`:

- **`Since`** -- When all revisions on a page are older than the cutoff, remaining pages are skipped (valid because pages are newest-first).
- **`Until`** -- Cannot enable early exit alone because newer revisions must be paged through to reach the target window. Sets `FetchAll` internally.
- **`MaxRevisions`** -- Stops after collecting enough unique revisions.

Revisions are deduplicated by revision number across pages.

## Commands

### list

Fetches paginated revision data and displays it as a fixed-width table.

**Time window parsing** (`parseTimeWindow`): Combines `--since`, `--until`, `--limit`, and `--all` flags into a `FetchOptions` struct. Validates mutual exclusivity (`--all` vs `--since`/`--until`) and ensures `--since` is before `--until`. Defaults to 90 days when no time flags are given.

**Relative time values** (`parseTimeValue`): Accepts `Nd`, `Nw`, `Nm`, `Ny` for relative durations and `yyyy-mm-dd` for absolute dates. `--until` dates are made inclusive by adding `24h - 1s`.

**Row filtering** (`applyFilters`): Applied after fetching, before display. Filters are combined with AND logic:

- `--arch` / `-a` -- Case-insensitive architecture match
- `--status` / `-s` -- Case-insensitive status match
- `--version` -- Case-insensitive regex match against version string
- `--build` / `-b` -- Build type classification:
  - `release` -- No `+` or `~` suffix (e.g. `2.75.2`)
  - `git` -- Has `+g`/`+git` suffix, excluding fips/dirty/pre/rc
  - `fips` -- Contains `+fips`
  - `pre` -- Contains `~pre`
  - `rc` -- Contains `~rc`
  - `dirty` -- Contains `-dirty`

**Column system** (`resolveColumns`): A registry of column definitions (`allColumns` map), each with a header string, a value extractor function, and a fixed/shrinkable flag. The `--columns` / `-c` flag selects and orders columns.

Default columns: `revision,version,arch,status,created`. Additional: `confinement`, `base`, `size`.

**Table rendering** (`printTable`): Computes natural column widths from data, then iteratively shrinks the widest non-fixed column until total width fits within 80 characters. Overflowing cell values are truncated with `...`. The last column is not right-padded.

### show

Fetches a single revision by number and outputs the JSON response. The `--fields` / `-f` flag filters to specific fields from the nested `revision` object.

## Testing Strategy

Tests focus on pure logic functions that don't require network access or interactive I/O:

- **`cmd/list_test.go`** -- Time parsing, column resolution, build type matching, row filtering, string truncation, column value extractors
- **`cmd/show_test.go`** -- Field list parsing, JSON field filtering
- **`cmd/version_test.go`** -- Explicit version setting, dev fallback, build info extraction
- **`store/auth_test.go`** -- Macaroon serialize/deserialize roundtrip, URL-safe encoding, caveat ID extraction
- **`store/credentials_test.go`** -- Save/load/clear lifecycle, file permissions, env var override, error cases
- **`store/client_test.go`** -- Refresh detection across JSON variants (underscore vs hyphen keys, multiple errors, empty/invalid bodies)

Not tested (require integration/real API): `store/revisions.go` (HTTP client methods), `cmd/login.go`/`cmd/logout.go` (interactive I/O), `main.go`.

## Dependencies

| Package | Purpose |
|---|---|
| `github.com/spf13/cobra` | CLI framework |
| `golang.org/x/term` | Secure password input (no echo) |
| `gopkg.in/macaroon.v1` | Macaroon creation, serialization, binding (matches snapd) |
