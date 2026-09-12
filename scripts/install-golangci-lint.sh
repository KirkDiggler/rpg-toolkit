#!/usr/bin/env bash
#
# install-golangci-lint.sh — install the one golangci-lint version this repo lints with.
#
# The version lives in .golangci-version and nowhere else. Before this script
# there were four pin sites carrying three different versions:
#
#   Makefile (lint, install-tools, lint-all)      v2.2.1
#   .github/workflows/ci-optimized.yml            v2.3.1
#   .github/workflows/ci-enhanced.yml (inert)     v2.2.1
#   whatever a contributor had on their machine   v2.13.2
#
# That is not a tidiness problem, it is a correctness one. CI's `test-all` job
# (main pushes) runs `make lint-all`, while `test-changed` (pull requests)
# installed its own pin — so the repository linted its main branch and its pull
# requests with two different binaries, and a local run was a third answer
# again. "It passed locally" and "it passed in CI" were statements about
# different tools, which is exactly how a lint gate stops meaning anything.
#
# A version is a contract, so it gets one home and every consumer reads it.
#
# Fail CLOSED: an unreadable, empty, or malformed .golangci-version is an error,
# never a silent fallback to "whatever is on PATH". Falling back is how the
# drift above became invisible in the first place.
#
# Usage: ./scripts/install-golangci-lint.sh [bindir]
#   bindir defaults to $(go env GOPATH)/bin

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
version_file="$repo_root/.golangci-version"

if [[ ! -r "$version_file" ]]; then
  echo "✗ $version_file is missing or unreadable." >&2
  echo "  It is the single source of truth for the golangci-lint version." >&2
  exit 1
fi

# Ignore blank lines and #-comments so the file can explain itself later.
want="$(grep -vE '^\s*(#|$)' "$version_file" | head -1 | tr -d '[:space:]')"

if [[ ! "$want" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "✗ $version_file does not contain a pinned version (got: '${want}')." >&2
  echo "  Expected an exact tag such as v2.3.1 — a range or 'latest' would" >&2
  echo "  reintroduce the drift this file exists to remove." >&2
  exit 1
fi

bindir="${1:-$(go env GOPATH)/bin}"

# Already correct? Do nothing. This is what makes the script safe to call from
# every Makefile target without paying a download each time.
#
# Check the binary that will actually be USED: the one in $bindir when it is
# already there, otherwise whatever PATH resolves. Checking only PATH would
# report "already installed" while leaving an explicitly requested $bindir
# empty — a silent no-op, which is the failure mode this whole file is about.
current=""
if [[ -x "$bindir/golangci-lint" ]]; then
  current="$bindir/golangci-lint"
elif command -v golangci-lint >/dev/null 2>&1; then
  current="$(command -v golangci-lint)"
fi

if [[ -n "$current" ]]; then
  have="v$("$current" version 2>/dev/null | sed -nE 's/.*has version ([0-9]+\.[0-9]+\.[0-9]+).*/\1/p')"
  if [[ "$have" == "$want" ]]; then
    echo "✓ golangci-lint $want already installed ($current)"
    exit 0
  fi
  echo "→ $current is $have, repo pins $want — installing $want into $bindir"
fi

echo "→ Installing golangci-lint $want into $bindir"
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh |
  sh -s -- -b "$bindir" "$want"

# Verify we got what we asked for rather than trusting the installer's exit code.
installed="v$("$bindir/golangci-lint" version 2>/dev/null | sed -nE 's/.*has version ([0-9]+\.[0-9]+\.[0-9]+).*/\1/p')"
if [[ "$installed" != "$want" ]]; then
  echo "✗ Installed $installed but .golangci-version pins $want" >&2
  exit 1
fi

echo "✓ golangci-lint $want installed"
