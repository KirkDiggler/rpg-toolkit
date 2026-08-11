#!/bin/bash

# Verify any module and print its verified transcripts.
#
#   ./scripts/verify.sh rulebooks/dnd5e/encounter
#   ./scripts/verify.sh events
#
# The transcript printed at the end is not decoration: it is the Output
# block of the module's Example_* functions, which go test just
# re-proved character for character. See docs/how-to/verified-transcripts.md.

set -e

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MODULE="${1:?usage: ./scripts/verify.sh <module-dir>   e.g. play/intel}"
cd "$REPO_ROOT/$MODULE"

pass() { echo "  ✓ $1"; }
fail() { echo "  ✗ $1"; exit 1; }

echo "$MODULE — verification"
echo ""

go build ./... >/dev/null 2>&1 && pass "build" || fail "build"

COUNT=$(go test ./... -count=1 -v 2>/dev/null | grep -cE "^=== RUN" || true)
go test ./... -count=1 >/dev/null 2>&1 && pass "tests ($COUNT run)" || fail "tests"

go vet ./... >/dev/null 2>&1 && pass "vet" || fail "vet"

if [ -z "$(gofmt -l .)" ]; then pass "gofmt"; else fail "gofmt"; fi

if command -v golangci-lint >/dev/null 2>&1; then
    golangci-lint run ./... >/dev/null 2>&1 && pass "lint" || fail "lint"
else
    echo "  - lint skipped (golangci-lint not installed)"
fi

if grep -lq "^func Example" ./*_test.go 2>/dev/null; then
    go test -run Example -count=1 >/dev/null 2>&1 && pass "transcripts re-proved" || fail "transcripts"
    echo ""
    echo "as verified:"
    awk '
        /^func Example/ { name=$2; sub(/\(\).*/, "", name) }
        /^\t\/\/ Output:$/ { printing=1; printf "\n== %s ==\n", name; next }
        printing && /^\t\/\// { line=$0; sub(/^\t\/\/ ?/, "", line); print "  " line; next }
        printing { printing=0 }
    ' ./*_test.go
else
    echo ""
    echo "  no transcript yet — add an Example_<scene> with a pinned Output"
    echo "  block (docs/how-to/verified-transcripts.md) so this module can"
    echo "  show itself working as designed."
fi
