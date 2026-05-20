# Design

This document describes the architecture and key design decisions behind revmap.

## Overview

revmap is a read-only CLI tool that queries the Snap Store's dashboard API to display revision and version history for published snaps. It authenticates using the same macaroon-based scheme as snapcraft. When authentication is unavailable or insufficient, it falls back to pre-built compressed cache files bundled in the snap.

## Project Structure

```
revmap/
  main.go                 Entry point; embeds README, sets version via ldflags
  cache-snaps.json        Configuration: list of snaps to pre-cache
  cache/                  Generated cache files (gitignored)
  demo.sh                 Interactive demo script (invoked by demo command)
  cmd/
    root.go               Root Cobra command
    version.go            Version resolution (ldflags or VCS fallback)
    login.go              Interactive login flow with credential export
    logout.go             Credential removal
    list.go               Revision listing with filters and table output
    show.go               Single revision detail view
    whoami.go             Account information display
    cache.go              Cache-build subcommand (pre-build cache generation)
    demo.go               Demo subcommand (runs demo.sh)
    readme.go             Embedded README display
    list_test.go           Tests for list logic
    show_test.go           Tests for show logic
    version_test.go        Tests for version logic
  cmd/cache-build/
    main.go               Standalone cache-build binary (separate main package)
  store/
    constants.go          API URLs and app-wide constants
    auth.go               Macaroon serialization, SSO discharge, login flow
    credentials.go        File-based credential storage with env var override
    account.go            Account info retrieval (whoami endpoint)
    client.go             Authenticated HTTP client with auto-refresh
    revisions.go          Store API calls (revisions, releases with pagination)
    cache.go              Cache data structures, gzip read/write, file lookup
    auth_test.go          Tests for macaroon serialization and caveat extraction
    credentials_test.go   Tests for credential storage
    client_test.go        Tests for refresh detection
```

## Authentication

### Version

The project produces two binaries (`revmap` and `cache-build`), both receiving the same version via ldflags at build time.

1. **ldflags (release builds)** -- `go build -ldflags "-X main.version=1.0.0"` sets a package-level `version` variable in each binary's `main.go`. For `revmap`, this is passed to `cmd.SetVersion()`. Output: `revmap 1.0.0` / `cache-build 1.0.0`.

2. **VCS build info (dev builds)** -- When `version` is empty, `revmap` uses `runtime/debug.ReadBuildInfo()` to extract the git commit hash (`vcs.revision`) and dirty flag (`vcs.modified`). Output: `revmap dev (abc1234)` or `revmap dev (abc1234, dirty)`. The `cache-build` binary displays `cache-build dev` when version is unset.

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

The `SNAPCRAFT_STORE_CREDENTIALS` environment variable overrides file-based storage. It auto-detects two formats:

1. **Snapcraft export format** -- The INI-style output from `snapcraft export-login`, containing `macaroon` and `unbound_discharge` fields under `[login.ubuntu.com]`. This is the recommended approach for CI pipelines.

2. **Base64-encoded JSON** -- Standard base64 encoding of the credentials JSON file (`{"r":"...","d":"..."}`). Useful for encoding the file revmap itself creates.

### Credential Export

The `login --export <file>` flag writes stored credentials to a file in the snapcraft INI format (`[login.ubuntu.com]\nmacaroon = ...\nunbound_discharge = ...`), compatible with `SNAPCRAFT_STORE_CREDENTIALS`. If the user is already logged in, the existing credentials are exported without re-authenticating. If not yet logged in, the interactive login flow runs first, then the credentials are exported.

**Path resolution** (`resolveExportPath`):

- **Absolute paths** -- Used as-is regardless of environment.
- **Relative paths (snap)** -- When `$SNAP_USER_COMMON` is set (running inside the snap), relative paths are resolved under `$SNAP_USER_COMMON` (e.g. `credentials.txt` becomes `~/snap/revmap/common/credentials.txt`). This is necessary because strict confinement prevents writing to arbitrary directories.
- **Relative paths (non-snap)** -- Resolved from the current working directory as usual.

The resolved path is displayed to the user after export so they know exactly where the file was written.

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

**Time window parsing** (`parseTimeWindow`): Combines `--since`, `--until`, `--limit`, and `--all` flags into a `FetchOptions` struct. Validates mutual exclusivity (`--all` vs `--since`/`--until`) and ensures `--since` is before `--until`. Defaults to 90 days when no scope flags are given. When `--limit` is set without explicit time bounds, the 90-day default is bypassed and all pages are fetched until the count limit is reached.

**Relative time values** (`parseTimeValue`): Accepts `Nd`, `Nw`, `Nm`, `Ny` for relative durations and `yyyy-mm-dd` for absolute dates. `--until` dates are made inclusive by adding `24h - 1s`.

**Row filtering** (`applyFilters`): Applied after fetching, before display. Filters are combined with AND logic:

- `--arch` / `-a` -- Case-insensitive architecture match
- `--status` / `-s` -- Case-insensitive status match
- `--version` -- Case-insensitive regex match against version string
- `--build` / `-b` -- Build type classification (preset or custom regex):
  - `release` -- No `+` or `~` suffix (e.g. `2.75.2`)
  - `git` -- Has `+g`/`+git` suffix, excluding fips/dirty/pre
  - `fips` -- Contains `+fips`
  - `pre` -- Contains `~pre` or `~rc`
  - `dirty` -- Contains `-dirty`
  - Any other value -- Treated as a case-insensitive regex matched against the version string

**Column system** (`resolveColumns`): A registry of column definitions (`allColumns` map), each with a header string, a value extractor function, and a fixed/shrinkable flag. The `--columns` / `-c` flag selects and orders columns.

Default columns: `revision,version,arch,status,created`. Additional: `confinement`, `base`, `size`.

**Table rendering** (`printTable`): Computes natural column widths from data, then iteratively shrinks the widest non-fixed column until total width fits within 80 characters. Overflowing cell values are truncated with `...`. The last column is not right-padded.

### show

Fetches a single revision by number and outputs the JSON response. The `--fields` / `-f` flag filters to specific fields from the nested `revision` object.

### whoami

Queries the store's account endpoint (`GET /dev/api/account`) to display the authenticated user's email, username, and registered snap names. Snaps are filtered to `status == "Approved"` in the default series (`16`), sorted alphabetically, and displayed in a 3-column grid truncated to fit within 80 characters.

If no credentials are available, prints an error directing the user to `revmap login` or the `SNAPCRAFT_STORE_CREDENTIALS` environment variable.

### cache-build

A standalone binary (`cmd/cache-build/main.go`) that fetches the complete revision history and individual revision details for all snaps listed in `cache-snaps.json`, writing compressed cache files to `cache/`. It is a separate `main` package so it can be built independently and is not included in the revmap snap.

Built by `make build` alongside the main binary, with the same version injected via ldflags. Supports `-version` to print its version.

**Authentication:** If credentials already exist (user ran `revmap login` or `SNAPCRAFT_STORE_CREDENTIALS` is set), they are used directly. Otherwise, `cache-build` checks for `REVMAP_EMAIL` and `REVMAP_PASSWORD` environment variables and performs a non-interactive login via `store.Login(email, password, "")`. The OTP parameter is always empty — the account must not have two-factor authentication enabled. A 2FA-enabled account will return `ErrTwoFactorRequired`, surfaced as `"automatic login failed: two-factor authentication required"`.

**Workflow:**
1. Authenticates (existing credentials or env-var login)
2. Reads `cache-snaps.json` (searched in cwd, `$SNAP/`, or next to executable)
3. For each snap: fetches all releases (paginating to completion with `FetchAll: true`)
4. Concurrently fetches each revision's detail via the revisions endpoint (`--workers` controls parallelism, default 10)
5. Skips revisions that return 404 (some entries in the releases list may have been deleted)
6. Writes `cache/<snap>.json.gz` — gzip-compressed JSON containing the full `CacheData` struct

**Cache data structure** (`store.CacheData`):
```go
type CacheData struct {
    Snap      string                            // snap name
    CachedAt  time.Time                         // build timestamp
    Revisions []RevisionEntry                   // full revision list
    Details   map[string]map[string]interface{} // revision number → detail JSON
}
```

### demo

Locates and executes `demo.sh` with the current binary path set as `REVMAP`. Searches for the script in `$SNAP/bin/`, next to the executable, or the current working directory. Supports `--no-pause` for non-interactive execution.

## Cache Fallback

Both `list` and `show` commands implement a two-tier fallback to cached data:

1. **No credentials** -- If `CredentialsExist()` returns false, attempt to load from cache immediately. Notice: `"Using cached data from <date> (run 'revmap login' for live results)"`.

2. **Permission error** -- If the store returns 401, 403, or 404 after authentication, fall back to cache. Notice: `"Using cached data from <date> (insufficient permissions for live data)"`. This handles users who are logged in but lack access to a particular snap.

3. **No cache available** -- If neither credentials nor cache exist, return an error: `"no cache available for <snap> (<reason>)"`.

**Cache file resolution** (`store.FindCacheFile`): Searches in order:
- `$SNAP/cache/<snap>.json.gz` (inside snap at runtime)
- `<executable-dir>/cache/<snap>.json.gz`
- `./cache/<snap>.json.gz` (development, running from project root)
- `./<snap>.json.gz` (running from inside the cache directory)

**Local filtering on cached data** (`applyCacheTimeWindow`): When serving from cache, the same time window and limit flags (`--since`, `--until`, `--limit`, `--all`) are applied locally against the cached revision list. The default 90-day window is applied when no scope flags are given. Row filters (`--arch`, `--build`, `--version`, `--status`) work identically on cached data.

**Error classification** (`isCacheFallbackErr`): Matches error strings containing "status 401", "status 403", or "status 404" — the patterns produced by `store/revisions.go` and `store/client.go`.

## Testing Strategy

Tests focus on pure logic functions that don't require network access or interactive I/O:

- **`cmd/list_test.go`** -- Time parsing, column resolution, build type matching, row filtering, string truncation, column value extractors
- **`cmd/show_test.go`** -- Field list parsing, JSON field filtering
- **`cmd/version_test.go`** -- Explicit version setting, dev fallback, build info extraction
- **`store/auth_test.go`** -- Macaroon serialize/deserialize roundtrip, URL-safe encoding, caveat ID extraction
- **`store/credentials_test.go`** -- Save/load/clear lifecycle, file permissions, env var override, error cases
- **`store/client_test.go`** -- Refresh detection across JSON variants (underscore vs hyphen keys, multiple errors, empty/invalid bodies)

Not tested (require integration/real API): `store/revisions.go` (HTTP client methods), `cmd/login.go`/`cmd/logout.go` (interactive I/O), `main.go`.

## User Messages

All user-facing messages follow consistent conventions:

| Type | Style | Example |
|---|---|---|
| Informational (stdout) | Sentence case, ends with period | `"Credentials cleared."` |
| Errors (stderr) | Prefixed with `error: `, lowercase, no period | `"error: not logged in (use 'revmap login'...)"` |
| Notices/banners | No period (followed by output) | `"Using cached data from %s (%s)\n\n"` |
| Progress | Ends with ellipsis | `"Authenticating via environment credentials..."` |

## Dependencies

| Package | Purpose |
|---|---|
| `github.com/spf13/cobra` | CLI framework |
| `golang.org/x/term` | Secure password input (no echo) |
| `gopkg.in/macaroon.v1` | Macaroon creation, serialization, binding (matches snapd) |

## Snap Packaging

The snap is built with `snapcraft` using `base: core24` (Ubuntu 24.04 runtime) and `confinement: strict`. The build process:

**`override-pull`:**
1. Clones from git (LP) or copies local source
2. Sets version from `git describe`
3. Copies pre-built `cache/` from `$CRAFT_PROJECT_DIR` into the build tree (local builds only — this directory is gitignored, so it only exists when `make cache` was run beforehand)

**`override-build`:**
1. Compiles the Go binary with version from `git describe`
2. Installs `demo.sh` to `$SNAP/bin/`
3. Copies `cache/*.json.gz` to `$SNAP/cache/` (if present)

The snap only requires the `network` plug for store API access. When running from cache, no network access is needed (though the plug is still declared).

**Build workflow (local snapcraft or LP):**

```
revmap login            # one-time interactive login
make cache              # builds binary + fetches all revision data
snapcraft               # override-pull copies cache/, produces .snap
```

Launchpad builds clone from git and will not have the `cache/` directory (it is gitignored). To ship cache in an LP-built snap, commit the cache files or use a CI pipeline that runs `make cache` before `snapcraft`.
