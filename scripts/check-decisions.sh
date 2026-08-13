#!/usr/bin/env bash
#
# check-decisions.sh — every ADR must appear in docs/adr/DECISIONS.md.
#
# DECISIONS.md is the cliffnotes digest: the file you read INSTEAD of the ADR
# corpus, so a decision missing from it is invisible to everyone who follows the
# instruction to read it. That makes drift silent, which is the failure this
# guards.
#
# It is guarding against a real precedent, not a hypothetical one. The ADR
# directory's own README.md carries a hand-maintained "Current ADRs" list that
# drifted to naming 7 of 37 — nothing failed, nobody noticed, and the index
# quietly stopped being true. A digest maintained by memory rots the same way.
#
# Fail CLOSED: a new ADR is not listed by default, so it fails here until
# somebody writes its one-line summary. That is the point — the work of
# distilling a decision is the work this file exists to force.
#
# Usage: ./scripts/check-decisions.sh

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
adr_dir="$repo_root/docs/adr"
digest="$adr_dir/DECISIONS.md"

if [ ! -f "$digest" ]; then
	echo "✗ missing $digest" >&2
	exit 1
fi

missing=()
checked=0

for path in "$adr_dir"/*.md; do
	file="$(basename "$path")"

	case "$file" in
	README.md | template.md | DECISIONS.md) continue ;;
	esac

	checked=$((checked + 1))

	# Cited by filename is always sufficient, and is REQUIRED for a number
	# that more than one file claims. The corpus has two 0006s and two 0019s,
	# so a bare number cannot identify those unambiguously — and a digest
	# entry that points at the wrong ADR is worse than one that is absent.
	if grep -qF "$file" "$digest"; then
		continue
	fi

	num="$(printf '%s' "$file" | grep -oE '^[0-9]{4}' || true)"
	if [ -z "$num" ]; then
		missing+=("$file (no ADR number, and not cited by filename)")
		continue
	fi

	siblings="$(find "$adr_dir" -maxdepth 1 -name "${num}[-_]*.md" | wc -l | tr -d ' ')"
	if [ "$siblings" -gt 1 ]; then
		missing+=("$file (number $num is shared — cite it by filename)")
		continue
	fi

	if ! grep -qE "\*\*${num}\*\*" "$digest"; then
		missing+=("$file (no **$num** entry)")
	fi
done

if [ ${#missing[@]} -gt 0 ]; then
	echo "✗ ADRs missing from docs/adr/DECISIONS.md:" >&2
	printf '    %s\n' "${missing[@]}" >&2
	echo >&2
	echo "  Add a one-line entry: the decision, and the rule it generalises to." >&2
	echo "  Readers are told to read DECISIONS.md instead of the corpus, so an" >&2
	echo "  unlisted ADR is invisible to them." >&2
	exit 1
fi

echo "✓ all $checked ADRs are summarised in docs/adr/DECISIONS.md"
