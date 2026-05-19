# revmap

A CLI tool for inspecting the revision and version history
of snaps published in the Snap Store (https://snapcraft.io).

## Requirements

- Go 1.22+
- An Ubuntu One account (https://login.ubuntu.com/) with
  access to the Snap Store dashboard

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

Build types: release, git, fips, pre, rc, dirty.

Display options:

    revmap list snapd -n 50             # limit results
    revmap list snapd -c revision,version,arch,size

Available columns: revision, version, arch, status, created,
confinement, base, size.

### show

Show full details of a specific revision:

    revmap show <snap> <revision>

Optionally filter to specific fields:

    revmap show snapd 17339 -f version,status,architectures

## Testing

    go test ./...

## License

See LICENSE file.
