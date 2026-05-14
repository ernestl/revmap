#!/bin/bash
set -euo pipefail

# test.sh -- unified test runner for revmap.
#
# Usage:
#   ./test.sh              Run unit tests (default)
#   ./test.sh --unit       Run unit tests
#   ./test.sh --static     Run static analysis checks
#   ./test.sh --all        Run both unit tests and static analysis

mode="${1:---unit}"

failed=0

fail() {
    echo "[✗] $1"
    failed=1
}

pass() {
    echo "[✓] $1"
}

skip() {
    echo "[-] $1 (skipped)"
}

run_unit() {
    echo "Running unit tests..."
    echo ""
    if go test -race ./... 2>&1; then
        pass "go test -race"
    else
        fail "go test -race"
    fi
}

run_static() {
    echo "Running static analysis..."
    echo ""

    # gofmt
    unformatted=$(gofmt -l . 2>&1)
    if [ -n "$unformatted" ]; then
        fail "gofmt"
        echo "$unformatted" | sed 's/^/    /'
    else
        pass "gofmt"
    fi

    # go vet
    if go vet ./... >/dev/null 2>&1; then
        pass "go vet"
    else
        fail "go vet"
        go vet ./... 2>&1 | sed 's/^/    /'
    fi

    # staticcheck
    if command -v staticcheck >/dev/null 2>&1; then
        if staticcheck ./... >/dev/null 2>&1; then
            pass "staticcheck"
        else
            fail "staticcheck"
            staticcheck ./... 2>&1 | sed 's/^/    /'
        fi
    else
        skip "staticcheck"
        echo "    install: go install honnef.co/go/tools/cmd/staticcheck@latest"
    fi

    # go mod tidy
    cp go.mod go.mod.bak
    cp go.sum go.sum.bak 2>/dev/null || true
    go mod tidy
    if diff -q go.mod go.mod.bak >/dev/null 2>&1 && \
       diff -q go.sum go.sum.bak >/dev/null 2>&1; then
        rm -f go.mod.bak go.sum.bak
        pass "go mod tidy"
    else
        fail "go mod tidy"
        echo "    go.mod or go.sum is not tidy (run 'go mod tidy')"
        mv go.mod.bak go.mod
        mv go.sum.bak go.sum 2>/dev/null || true
    fi

    # go build
    if go build ./... 2>/dev/null; then
        pass "go build"
    else
        fail "go build"
        go build ./... 2>&1 | sed 's/^/    /'
    fi
}

case "$mode" in
    --unit)
        run_unit
        ;;
    --static)
        run_static
        ;;
    --all)
        run_static
        echo ""
        run_unit
        ;;
    *)
        echo "Usage: ./test.sh [--unit|--static|--all]"
        exit 1
        ;;
esac

echo ""
if [ "$failed" -ne 0 ]; then
    echo "FAILED"
    exit 1
else
    echo "All checks passed."
fi
