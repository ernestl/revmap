# revmap

A CLI tool for inspecting the revision and version history
of snaps published in the Snap Store (https://snapcraft.io).

## Requirements

- Go 1.25+
- An Ubuntu One account (https://login.ubuntu.com/) with
  access to the Snap Store dashboard (optional if using
  cached data)

## Installation

### Snap

    sudo snap install revmap

### Go

    go install github.com/ernestl/revmap@latest

### From source

    git clone https://github.com/ernestl/revmap.git
    cd revmap
    make

The version is set automatically from the latest git tag.
Check the current version with:

    revmap --version

## Authentication

revmap uses macaroon-based authentication against the Snap
Store. Log in once with your Ubuntu One credentials:

    revmap login

Credentials are stored at:

    ~/.local/share/revmap/credentials.json

When installed as a snap:

    ~/snap/revmap/common/credentials.json

This respects XDG_DATA_HOME. Expired discharge macaroons are
refreshed automatically.

Alternatively, set the REVMAP_STORE_CREDENTIALS environment
variable to skip interactive login. This accepts two formats:

**Snapcraft export (recommended for CI):**

Export credentials with snapcraft and set the variable to the
file contents:

    snapcraft export-login --snaps <snap> \
      --acls package_access credentials.txt
    export REVMAP_STORE_CREDENTIALS=$(cat credentials.txt)

The exported file uses the INI-style format:

    [login.ubuntu.com]
    macaroon = <root macaroon>
    unbound_discharge = <discharge macaroon>

**Base64-encoded JSON:**

Encode the credentials file that revmap login creates:

    export REVMAP_STORE_CREDENTIALS=$(base64 -w0 \
      ~/.local/share/revmap/credentials.json)

When set, the environment variable takes priority over the
credentials file.

To remove stored credentials:

    revmap logout

## Usage

### list

List the revision history of a snap:

    revmap list <snap>

By default, only revisions from the last 90 days are shown.
Use --all to fetch complete history, or --limit/-n to fetch
up to a specific number of revisions across all pages.

Time window:

    revmap list snapd --since 7d
    revmap list snapd --since 6m --until 3m
    revmap list snapd --since 2024-01-01 --until 2024-06-30
    revmap list snapd --all

The --since and --until flags accept relative durations
(Nd, Nw, Nm, Ny) or absolute dates (yyyy-mm-dd).

Row filters:

    revmap list snapd -a amd64         # architecture
    revmap list snapd -s Published      # status
    revmap list snapd --version '2\.75' # version regex
    revmap list snapd -b release        # build type

Build types: release, git, fips, pre, dirty.
Any other value is treated as a custom regex matched against
the version string (e.g. -b '^\d+\.\d+$').

Display options:

    revmap list snapd -n 50             # limit to 50 (fetches all pages)
    revmap list snapd -c revision,version,arch,size

Available columns: revision, version, arch, status, created,
confinement, base, size.

### show

Show full details of a specific revision:

    revmap show <snap> <revision>

Optionally filter to specific fields:

    revmap show snapd 17339 -f version,status,architectures

### demo

Run an interactive demo showcasing revmap's features:

    revmap demo
    revmap demo --no-pause

The demo uses the snapd snap as an example and walks through
list/show commands with various filters, including cache
fallback.

### cache-build

Build the offline cache for all snaps in `cache-snaps.json`:

    revmap cache-build
    revmap cache-build --workers 20

Requires authentication. See the Offline Cache section below
for the full workflow.

## Offline Cache

revmap can bundle a pre-built cache of revision history so
that users without store permissions can still browse data.

### How it works

When a user runs `revmap list` or `revmap show` and either:
- is not logged in, or
- receives a permission error (401/403/404) from the store

revmap automatically falls back to cached data if available,
displaying a notice:

    Using cached data from 2026-05-20 (run 'revmap login' for live results)

or:

    Using cached data from 2026-05-20 (insufficient permissions for live data)

### Building the cache

The cache is built before creating the snap. It requires
authentication with an account that has access to the target
snaps.

1. Configure which snaps to cache in `cache-snaps.json`:

       ["snapd"]

2. Build the cache:

       revmap login
       revmap cache-build

   This fetches the complete revision list and all individual
   revision details for each configured snap, writing
   compressed files to `cache/<snap>.json.gz`.

   Options:

       --workers N    concurrent detail fetches (default 10)

3. Build the snap (cache is bundled automatically):

       snapcraft

### Automated builds (CI / Launchpad)

For automated builds where interactive login is not possible,
set environment variables instead:

    export REVMAP_EMAIL="user@example.com"
    export REVMAP_PASSWORD="secret"
    revmap cache-build

The account **must not** have two-factor authentication (2FA)
enabled. A 2FA-enabled account will fail with "two-factor
authentication required". Use a dedicated service account with
only `package_access` permission and no 2FA.

For Launchpad snap builds, store the credentials as build
secrets (`store-email` and `store-password`). The
`snapcraft.yaml` override-build step exports these as
`REVMAP_EMAIL` and `REVMAP_PASSWORD` automatically.

If the build secrets are not configured, the snap will still
build — it will just ship without a cache.

### Cache location

At runtime, revmap searches for cache files in order:

1. `$SNAP/cache/<snap>.json.gz` (inside the snap)
2. Next to the executable: `<exe-dir>/cache/<snap>.json.gz`
3. Current working directory: `cache/<snap>.json.gz`

### Cache contents

Each `.json.gz` file contains gzip-compressed JSON with:

- `snap` -- snap name
- `cached_at` -- timestamp of when the cache was built
- `revisions` -- full revision list (same as store API)
- `details` -- map of revision number to full revision detail

## Building

    make              # build the binary
    make test         # run tests with race detector
    make cache        # build offline cache (requires login)
    make clean        # remove binary and cache
    make check        # run checks.sh

The `cache` target depends on `build` and runs
`revmap cache-build`. You must be logged in first:

    revmap login
    make cache

## Testing

    go test ./...

## License

See LICENSE file.
