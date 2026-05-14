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

To export credentials for use in CI or with
SNAPCRAFT_STORE_CREDENTIALS:

    revmap login --export credentials.txt
    export SNAPCRAFT_STORE_CREDENTIALS=$(cat credentials.txt)

If credentials already exist on disk (from a prior login),
--export writes them without re-authenticating. If credentials
are only available via SNAPCRAFT_STORE_CREDENTIALS, --export
forces an interactive login to produce fresh credentials.

When installed as a snap, relative paths are resolved under
~/snap/revmap/common/ (the snap's writable user directory)
since strict confinement prevents writing elsewhere. Absolute
paths are used as-is. Example from within the snap:

    revmap login --export credentials.txt
    # writes to ~/snap/revmap/common/credentials.txt

When running from source or `go install`, relative paths are
resolved from the current working directory as usual.

Alternatively, set the SNAPCRAFT_STORE_CREDENTIALS environment
variable to skip interactive login. This accepts two formats:

**Snapcraft export (recommended for CI):**

Export credentials with snapcraft and set the variable to the
file contents:

    snapcraft export-login --snaps <snap> \
      --acls package_access credentials.txt
    export SNAPCRAFT_STORE_CREDENTIALS=$(cat credentials.txt)

The exported file uses the INI-style format:

    [login.ubuntu.com]
    macaroon = <root macaroon>
    unbound_discharge = <discharge macaroon>

**Base64-encoded JSON:**

Encode the credentials file that revmap login creates:

    export SNAPCRAFT_STORE_CREDENTIALS=$(base64 -w0 \
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
    revmap list snapd -b release,fips   # multiple types (OR)

Build types: release, fips (comma-separated).

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

### whoami

Show the currently authenticated account:

    revmap whoami

Displays email, username, and registered snap names.

### demo

Run an interactive demo showcasing revmap's features:

    revmap demo
    revmap demo --no-pause

The demo uses the snapd snap as an example and walks through
list/show commands with various filters, including cache
fallback.

### readme / design

Display documentation directly in the terminal, rendered with
styled markdown (headers, code blocks, syntax highlighting):

    revmap readme
    revmap design

The `design` command is particularly useful as context for
coding agents before making contributions.

### cache-build

`cache-build` is a standalone binary (not a revmap subcommand) that
builds the offline cache for all snaps in `cache-snaps.json`:

    ./cache-build
    ./cache-build -workers 50

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
       make cache

   This fetches the complete revision list and all individual
   revision details for each configured snap, writing
   compressed files to `cache/<snap>.json.gz`.

   Options:

       ./cache-build -workers 50

3. Build the snap (cache is bundled automatically):

       snapcraft

   The `override-pull` step copies the pre-built `cache/`
   directory into the build tree. Since `cache/` is gitignored,
   it must exist locally before running `snapcraft`.

### Automated builds (CI / Launchpad)

For automated builds where interactive login is not possible,
set environment variables instead:

    export REVMAP_EMAIL="user@example.com"
    export REVMAP_PASSWORD="secret"
    ./cache-build

The account **must not** have two-factor authentication (2FA)
enabled. A 2FA-enabled account will fail with "two-factor
authentication required". Use a dedicated service account with
only `package_access` permission and no 2FA.

Launchpad does not support injecting secrets into snap builds.
To ship cache in an LP-built snap, either:

- Run `make cache` locally before `snapcraft` (the
  `override-pull` stage copies it into the build tree via
  `$CRAFT_PROJECT_DIR`), or
- Use a CI system with secrets support (e.g. GitHub Actions)
  to run `cache-build` and then invoke `snapcraft`.

If no cache is present, the snap still builds and works — it
just requires network access and store authentication for all
queries.

### Cache location

At runtime, revmap searches for cache files in order:

1. `$SNAP/cache/<snap>.json.gz` (inside the snap)
2. Next to the executable: `<exe-dir>/cache/<snap>.json.gz`
3. Current working directory: `cache/<snap>.json.gz`
4. Current working directory directly: `<snap>.json.gz`

### Cache contents

Each `.json.gz` file contains gzip-compressed JSON with:

- `snap` -- snap name
- `cached_at` -- timestamp of when the cache was built
- `revisions` -- full revision list (same as store API)
- `details` -- map of revision number to full revision detail

## Building

    make              # build both binaries (revmap + cache-build)
    make test         # run unit tests (default)
    make test FLAGS=--static   # run static analysis
    make test FLAGS=--all      # run both
    make cache        # build offline cache (requires login)
    make clean        # remove binaries and cache

`make build` produces two binaries:

- `revmap` -- the main CLI tool
- `cache-build` -- standalone cache generation tool

Both binaries share the same version, set automatically from the
latest git tag. Check with:

    ./cache-build -version

The `cache` target runs `go run ./cmd/cache-build`.
You must be logged in first:

    revmap login
    make cache

The full local snap build workflow:

    revmap login          # one-time
    make cache            # fetch and compress revision data
    snapcraft             # builds snap with cache bundled

## Testing

    go test ./...

## License

See LICENSE file.
