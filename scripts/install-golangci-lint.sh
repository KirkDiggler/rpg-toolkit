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
# WHY THIS DOES NOT PIPE UPSTREAM'S install.sh
#
# The obvious implementation — `curl .../install.sh | sh -s -- -b DIR VERSION` —
# cannot install v2.13.2, which is the version this repo wants to move to.
# Upstream's installer verifies the download with an unanchored match:
#
#   want=$(grep "${BASENAME}" "${checksums}" | ... | cut -d ' ' -f 1)
#
# Releases from v2.13.x also publish an SBOM whose filename CONTAINS the
# tarball's filename:
#
#   ...  golangci-lint-2.13.2-linux-amd64.tar.gz
#   ...  golangci-lint-2.13.2-linux-amd64.tar.gz.sbom.json
#
# Both lines match, `want` becomes two concatenated checksums, and verification
# fails every time on a perfectly good download. So we fetch the release asset
# and verify it here, matching the checksum line exactly.
#
# Fail CLOSED throughout: an unreadable, empty, or malformed .golangci-version
# is an error, never a silent fallback to "whatever is on PATH". A download
# that does not match its published checksum is an error, never a warning.
# Falling back is how the drift above became invisible in the first place.
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

ver="${want#v}"

case "$(uname -s)" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) echo "✗ Unsupported OS: $(uname -s)" >&2; exit 1 ;;
esac

case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) echo "✗ Unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

asset="golangci-lint-${ver}-${os}-${arch}.tar.gz"
base="https://github.com/golangci/golangci-lint/releases/download/${want}"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "→ Downloading $asset"
if ! curl -sSfL -o "$tmp/$asset" "$base/$asset"; then
  echo "✗ Could not download $base/$asset" >&2
  echo "  Check that $want is a real golangci-lint release." >&2
  exit 1
fi

if ! curl -sSfL -o "$tmp/checksums.txt" "$base/golangci-lint-${ver}-checksums.txt"; then
  echo "✗ Could not download checksums for $want" >&2
  exit 1
fi

# Exact match on the filename: two spaces then the name, anchored at end of
# line. This is the bug in upstream's installer — see the header.
expected="$(awk -v name="$asset" '$2 == name {print $1}' "$tmp/checksums.txt")"

if [[ -z "$expected" ]]; then
  echo "✗ No checksum published for $asset in $want" >&2
  exit 1
fi

if [[ "$(echo "$expected" | wc -l)" -ne 1 ]]; then
  echo "✗ Ambiguous checksum for $asset — refusing to guess." >&2
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "$tmp/$asset" | awk '{print $1}')"
else
  actual="$(shasum -a 256 "$tmp/$asset" | awk '{print $1}')"
fi

if [[ "$actual" != "$expected" ]]; then
  echo "✗ Checksum mismatch for $asset" >&2
  echo "  expected: $expected" >&2
  echo "  actual:   $actual" >&2
  exit 1
fi

tar -xzf "$tmp/$asset" -C "$tmp"

mkdir -p "$bindir"
install -m 0755 "$tmp/golangci-lint-${ver}-${os}-${arch}/golangci-lint" "$bindir/golangci-lint"

# Verify we got what we asked for rather than trusting the steps above.
installed="v$("$bindir/golangci-lint" version 2>/dev/null | sed -nE 's/.*has version ([0-9]+\.[0-9]+\.[0-9]+).*/\1/p')"
if [[ "$installed" != "$want" ]]; then
  echo "✗ Installed $installed but .golangci-version pins $want" >&2
  exit 1
fi

echo "✓ golangci-lint $want installed ($bindir/golangci-lint)"
