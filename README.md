# snaprev

A CLI tool for inspecting the revision and version history
of snaps published in the Snap Store (https://snapcraft.io).

## Requirements

- Go 1.22+
- An Ubuntu One account (https://login.ubuntu.com/) with
  access to the Snap Store dashboard

## Installation

    go install github.com/ernestl/snaprev@latest

Or build from source:

    git clone https://github.com/ernestl/snaprev.git
    cd snaprev
    go build -o snaprev .

For release builds, set the version at build time:

    go build -ldflags "-X main.version=1.0.0" -o snaprev .

Dev builds show the git commit hash automatically. Check
the current version with:

    snaprev --version

## Authentication

snaprev uses macaroon-based authentication against the Snap
Store. Log in once with your Ubuntu One credentials:

    snaprev login

Credentials are stored at:

    ~/.local/share/snaprev/credentials.json

This respects XDG_DATA_HOME. Expired discharge macaroons are
refreshed automatically.

Alternatively, set the SNAPREV_STORE_CREDENTIALS environment
variable with base64-encoded JSON credentials to skip
interactive login.

To remove stored credentials:

    snaprev logout

## Usage

### list

List the revision history of a snap:

    snaprev list <snap>

By default, only revisions from the last 90 days are shown.

Time window:

    snaprev list snapd --since 7d
    snaprev list snapd --since 6m --until 3m
    snaprev list snapd --since 2024-01-01 --until 2024-06-30
    snaprev list snapd --all

The --since and --until flags accept relative durations
(Nd, Nw, Nm, Ny) or absolute dates (yyyy-mm-dd).

Row filters:

    snaprev list snapd -a amd64         # architecture
    snaprev list snapd -s Published      # status
    snaprev list snapd --version '2\.75' # version regex
    snaprev list snapd -b release        # build type

Build types: release, git, fips, pre, rc, dirty.

Display options:

    snaprev list snapd -n 50             # limit results
    snaprev list snapd -c revision,version,arch,size

Available columns: revision, version, arch, status, created,
confinement, base, size.

### show

Show full details of a specific revision:

    snaprev show <snap> <revision>

Optionally filter to specific fields:

    snaprev show snapd 17339 -f version,status,architectures

## Testing

    go test ./...

## License

See LICENSE file.
