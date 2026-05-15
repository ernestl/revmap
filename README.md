# snaprev

A CLI tool for inspecting the revision and version history
of snaps published in the Snap Store (https://snapcraft.io).

## Requirements

- Go 1.22+
- An Ubuntu One account (https://login.ubuntu.com/) with
  access to the Snap Store dashboard

## Installation

### Snap

    sudo snap install snaprev

### Go

    go install github.com/ernestl/snaprev@latest

### From source

    git clone https://github.com/ernestl/snaprev.git
    cd snaprev
    make

The version is set automatically from the latest git tag.
Check the current version with:

    snaprev --version

## Authentication

snaprev uses macaroon-based authentication against the Snap
Store. Log in once with your Ubuntu One credentials:

    snaprev login

Credentials are stored at:

    ~/.local/share/snaprev/credentials.json

When installed as a snap:

    ~/snap/snaprev/common/credentials.json

This respects XDG_DATA_HOME. Expired discharge macaroons are
refreshed automatically.

Alternatively, set the SNAPREV_STORE_CREDENTIALS environment
variable to skip interactive login. This accepts two formats:

**Snapcraft export (recommended for CI):**

Export credentials with snapcraft and set the variable to the
file contents:

    snapcraft export-login --snaps <snap> \
      --acls package_access credentials.txt
    export SNAPREV_STORE_CREDENTIALS=$(cat credentials.txt)

The exported file uses the INI-style format:

    [login.ubuntu.com]
    macaroon = <root macaroon>
    unbound_discharge = <discharge macaroon>

**Base64-encoded JSON:**

Encode the credentials file that snaprev login creates:

    export SNAPREV_STORE_CREDENTIALS=$(base64 -w0 \
      ~/.local/share/snaprev/credentials.json)

When set, the environment variable takes priority over the
credentials file.

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
