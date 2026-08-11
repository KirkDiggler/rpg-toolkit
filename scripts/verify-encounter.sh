#!/bin/bash

# Run the free-roam encounter's verification steps and print a simple
# report — then show the VERIFIED tomb-watch transcript (the narration
# is pinned by Example_theTombWatch's Output block, so what prints
# below is exactly what the composition proved it does).

set -e

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT/rulebooks/dnd5e/encounter"

pass() { echo "  ✓ $1"; }
fail() { echo "  ✗ $1"; exit 1; }

echo "free-roam encounter — verification"
echo ""

go build ./... >/dev/null 2>&1 && pass "build" || fail "build"

COUNT=$(go test ./... -count=1 -v 2>/dev/null | grep -cE "^=== RUN" || true)
go test ./... -count=1 >/dev/null 2>&1 && pass "tests ($COUNT run, incl. the tomb-watch scene + persistence round-trips)" || fail "tests"

go vet ./... >/dev/null 2>&1 && pass "vet" || fail "vet"

if [ -z "$(gofmt -l .)" ]; then pass "gofmt"; else fail "gofmt"; fi

if command -v golangci-lint >/dev/null 2>&1; then
    golangci-lint run ./... >/dev/null 2>&1 && pass "lint" || fail "lint"
else
    echo "  - lint skipped (golangci-lint not installed)"
fi

go test -run Example_theTombWatch -count=1 >/dev/null 2>&1 && pass "the transcript below is pinned and just re-proved" || fail "example transcript"

echo ""
echo "the tomb watch, as verified:"
echo ""
sed -n '/\/\/ Output:/,$p' example_tombwatch_test.go | sed -n 's|^\t// \(.*\)$|  \1|p' | sed '/^  Output:$/d'
