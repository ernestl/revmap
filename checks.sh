#!/bin/bash
set -euo pipefail

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

# 1. gofmt - formatting
unformatted=$(gofmt -l . 2>&1)
if [ -n "$unformatted" ]; then
    fail "gofmt"
    echo "$unformatted" | sed 's/^/    /'
else
    pass "gofmt"
fi

# 2. go vet - built-in static analysis
if go vet ./... >/dev/null 2>&1; then
    pass "go vet"
else
    fail "go vet"
    go vet ./... 2>&1 | sed 's/^/    /'
fi

# 3. staticcheck - the standard Go linter
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

# 4. go mod tidy - module hygiene
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

# 5. go build - verify the binary compiles
if go build ./... 2>/dev/null; then
    pass "go build"
else
    fail "go build"
    go build ./... 2>&1 | sed 's/^/    /'
fi

# 6. go test -race - tests with race detector
if go test -race ./... >/dev/null 2>&1; then
    pass "go test -race"
else
    fail "go test -race"
    go test -race ./... 2>&1 | sed 's/^/    /'
fi

echo ""
if [ "$failed" -ne 0 ]; then
    echo "FAILED"
    exit 1
else
    echo "All checks passed."
fi
