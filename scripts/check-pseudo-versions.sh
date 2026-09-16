#!/usr/bin/env bash
#
# check-pseudo-versions.sh — no committed go.mod may pin a sibling toolkit
# module at a pseudo-version, and none may carry a `replace` for one.
#
# Developing outside-in on pseudo-versions is CORRECT. `go get <module>@<sha>`
# against a pushed branch commit is how a wave gets built and walked before
# anything is tagged, and CLAUDE.md says so in as many words.
#
# It stops being correct the instant the pin reaches main. The branch that
# commit lived on is deleted when its pull request merges, so the version names
# a commit nobody can reach. That is a DEAD PIN — the same failure as a leaked
# `replace` directive, wearing a version number instead of a path. Both say
# "build this against something only the author has."
#
# PR #1761 merged rulebooks/dnd5e carrying exactly that, and PR #1773 had to
# repair it. Nothing caught it, and nothing would have: the pre-commit hook
# never ran, because a go.mod-only commit has no files the hook inspects. That
# is why this check lives in CI and not in the hook.
#
# The scan is whole-repo on purpose. A pull request that touches one module can
# still be the one that makes a rotten pin somewhere else start mattering, and a
# pin nobody is currently looking at is precisely the one that survives.
#
# THE ONE EXEMPTION, and why it is not a loophole.
#
# Every tagging workflow in .github/workflows/ enumerates modules with
# `-not -path "./examples/*"`. Modules under examples/ are therefore never
# tagged, have never had a version minted, and never will. A dependency on one
# CANNOT be written as a release version — a pseudo-version is the only form it
# can take, so failing on it would demand something impossible.
#
# So a pin is exempt when its PROVIDER is a module CI never tags. The exempt set
# is derived below from the repository's own layout, never from a hand-written
# allowlist, so it cannot drift away from what the tagging workflows actually
# skip. Exempt pins are PRINTED, not hidden: an unfixable pin is still a fact
# about the build, and a silent exemption is how a rule stops being read.
#
# Usage: ./scripts/check-pseudo-versions.sh

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

module_prefix='github.com/KirkDiggler/rpg-toolkit'

# A Go pseudo-version always ends in <14-digit UTC timestamp>-<12 hex chars>.
# Three separators reach that suffix, and only matching all three catches every
# form Go mints:
#
#   v0.0.0-20260912092832-d3fdee18fbaa        no base tag       -> '-' before
#   v0.5.2-0.20260804104806-a8a4dd3fc92a      base is a release -> '.' before
#   v1.2.3-rc1.0.20260804104806-a8a4dd3fc92a  base is a pre-rel -> '.' before
#
# An earlier draft of this check matched only the first shape and reported main
# clean while two of the three pins below it sat in the tree. Hence [-.].
pseudo_suffix='[-.][0-9]{14}-[0-9a-f]{12}'

# Requirement lines: optionally inside a require block, optionally // indirect.
require_re="^[[:space:]]*(require[[:space:]]+)?${module_prefix}/[^[:space:]]+[[:space:]]+v[^[:space:]]*${pseudo_suffix}"

# Any replace, block-form or single-line: both spell the arrow.
replace_re='=>'

modfiles="$(find . -name go.mod -type f -not -path './vendor/*' | sort)"

# Providers CI never tags, as module paths. Mirrors the `-not -path
# "./examples/*"` exclusion the tagging workflows use; keep the two in step.
untagged_providers="$(
	find ./examples -name go.mod -type f -not -path './vendor/*' 2>/dev/null |
		sed -e 's|^\./||' -e 's|/go\.mod$||' -e "s|^|${module_prefix}/|" |
		sort || true
)"

is_untagged_provider() {
	[ -n "$untagged_providers" ] || return 1
	printf '%s\n' "$untagged_providers" | grep -qxF "$1"
}

offenders=()
exempt=()
checked=0

for modfile in $modfiles; do
	path="${modfile#./}"
	checked=$((checked + 1))

	while IFS= read -r numbered; do
		[ -n "$numbered" ] || continue
		lineno="${numbered%%:*}"
		line="${numbered#*:}"
		# Strip the comment so `// indirect` never becomes the provider.
		provider="$(printf '%s' "${line%%//*}" | tr -s '[:space:]' ' ' |
			sed -e 's/^ //' -e 's/^require //' | cut -d' ' -f1)"

		if is_untagged_provider "$provider"; then
			exempt+=("$path:$lineno: $provider")
		else
			offenders+=("$path:$lineno:$(printf '%s' "$line" | sed 's/^[[:space:]]*/ /')")
		fi
	done < <(grep -nE "$require_re" "$modfile" || true)

	while IFS= read -r numbered; do
		[ -n "$numbered" ] || continue
		case "$numbered" in
		*"$module_prefix"*)
			offenders+=("${path}:${numbered%%:*}: replace directive ->${numbered#*:}")
			;;
		esac
	done < <(grep -nE "$replace_re" "$modfile" || true)
done

if [ ${#exempt[@]} -gt 0 ]; then
	echo "note: ${#exempt[@]} pin(s) on modules CI never tags — a pseudo-version is their only form:"
	printf '    %s\n' "${exempt[@]}"
	echo
fi

if [ ${#offenders[@]} -gt 0 ]; then
	echo "✗ committed pseudo-versions / replaces of rpg-toolkit modules:" >&2
	printf '    %s\n' "${offenders[@]}" >&2
	echo >&2
	echo "  These name commits on branches that are deleted when their PR merges." >&2
	echo "  Publish the provider first, let CI mint its tag, then in the consumer:" >&2
	echo >&2
	echo "      go get <module>@<minted tag> && go mod tidy" >&2
	echo >&2
	echo "  Developing on pseudo-versions is fine. Merging one is not." >&2
	exit 1
fi

echo "✓ no rpg-toolkit pseudo-versions or replaces in $checked go.mod files"
