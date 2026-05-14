#!/bin/bash
# revmap demo — showcasing capabilities with the snapd snap
#
# Usage: ./demo.sh [--no-pause]
#
# Set REVMAP env var to override the binary path.

set -e

PAUSE=true
if [[ "$1" == "--no-pause" ]]; then
    PAUSE=false
fi

# Use REVMAP env if set, otherwise auto-detect.
if [[ -z "$REVMAP" ]]; then
    if [[ -x "./revmap" ]]; then
        REVMAP="./revmap"
    else
        REVMAP="revmap"
    fi
fi

# Check authentication
if ! $REVMAP list snapd -n 1 >/dev/null 2>&1; then
    echo "Error: not logged in. Run 'revmap login' first."
    exit 1
fi

run() {
    local desc="$1"
    shift
    # Display a clean command (replace full binary path with "revmap").
    local display_cmd="${*//$REVMAP/revmap}"
    echo
    echo "  # $desc"
    echo "  \$ $display_cmd"
    echo
    "$@"
    if $PAUSE; then
        echo
        read -rp "  [enter] "
        clear
    else
        echo
        echo "  ---"
    fi
}

if $PAUSE; then
    clear
    echo "revmap — Snap Store revision history inspector"
    echo "================================================"
    echo
    echo "Demo snap: snapd"
    read -rp "  [enter to start] "
    clear
fi

# 1. Quick overview — most recent 10 revisions
run "Most recent revisions (default: last 90 days, limited to 10)" \
    $REVMAP list snapd -n 10

# 2. Filter by architecture
run "Filter by architecture: arm64 only, last 2 weeks" \
    $REVMAP list snapd --since 2w -a arm64

# 3. Release builds only (clean, no suffixes)
run "Build type filter: release builds only (last 6 months, amd64)" \
    $REVMAP list snapd --since 6m -b release -a amd64

# 4. FIPS builds (demonstrates niche filter)
run "Build type filter: FIPS-certified builds (last 2 weeks)" \
    $REVMAP list snapd -b fips --since 2w

# 5. Custom regex version filter
run "Version regex: match 2.66 branch builds" \
    $REVMAP list snapd --version '2\.66' --since 3m

# 6. Combined filters — the power combo
run "Combined: amd64 release builds, last 3 months" \
    $REVMAP list snapd --since 3m -a amd64 -b release

# 7. Custom columns with size info
run "Custom columns: show snap sizes" \
    $REVMAP list snapd -n 10 -c revision,version,arch,size

# 8. Date range (single day)
run "Absolute date range: a single day" \
    $REVMAP list snapd --since 2026-05-19 --until 2026-05-19

# 9. Show full revision details
run "Inspect a specific revision (JSON)" \
    $REVMAP show snapd 17339

# 10. Show specific fields
run "Select specific fields from a revision" \
    $REVMAP show snapd 17339 -f version,status,architectures

# 11. Cache fallback — demonstrate offline/unauthenticated usage
# Temporarily hide credentials to show cache behavior.
echo
echo "  # Cache fallback demo: simulating unauthenticated access"
echo "  # (temporarily hiding credentials)"
echo

CREDS_PATH="${XDG_DATA_HOME:-$HOME/.local/share}/revmap/credentials.json"
if [[ -n "$SNAP_USER_COMMON" ]]; then
    CREDS_PATH="$SNAP_USER_COMMON/credentials.json"
fi

CREDS_HIDDEN=false
if [[ -f "$CREDS_PATH" ]]; then
    mv "$CREDS_PATH" "${CREDS_PATH}.demo-bak"
    CREDS_HIDDEN=true
fi

# Ensure credentials are restored even if something fails.
restore_creds() {
    if [[ "$CREDS_HIDDEN" == "true" && -f "${CREDS_PATH}.demo-bak" ]]; then
        mv "${CREDS_PATH}.demo-bak" "$CREDS_PATH"
    fi
}
trap restore_creds EXIT

run "List from cache (no credentials)" \
    $REVMAP list snapd -n 5

run "Show from cache (no credentials)" \
    $REVMAP show snapd 27239 -f version,architectures

# Restore credentials (trap will also handle unexpected exits).
restore_creds
CREDS_HIDDEN=false
trap - EXIT

echo
echo "Demo complete."
