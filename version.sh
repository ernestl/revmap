#!/bin/sh
# version.sh -- single source of truth for the project version.
# Outputs the version string to stdout.
#
# Usage:
#   ./version.sh              # e.g. "1.0.6" or "1.0.6+abc1234"
#   ./version.sh --with-date  # e.g. "1.0.6+abc1234+20-05-2026"

set -e

tag="$(git describe --tags --abbrev=0 2>/dev/null || echo 0.0.0)"

if git describe --tags --exact-match HEAD >/dev/null 2>&1; then
  version="${tag}"
else
  commit="$(git rev-parse --short HEAD)"
  version="${tag}+${commit}"
fi

# Append build date if requested (used by daily CI builds).
if [ "$1" = "--with-date" ]; then
  version="${version}+$(date +%d-%m-%Y)"
fi

printf '%s' "$version"
