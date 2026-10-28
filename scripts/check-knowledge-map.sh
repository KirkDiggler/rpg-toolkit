#!/usr/bin/env bash
#
# check-knowledge-map.sh — every Go module appears in exactly one knowledge map,
# and that map is the nearest one above it.
#
# Each level that has a knowledge/ directory carries map.md: a seam document
# and one mermaid diagram of exactly the modules directly beneath that level.
# The repo root composes the level maps and never redraws them. A diagram that
# is maintained by memory drifts, and a drifted diagram is worse than none
# because it is trusted. This check makes a module added, moved or retired
# without its map a red build.
#
# What counts as a module node: inside a ```mermaid block of a
# knowledge/map.md, a rectangle node written ID["<module path>..."] — the
# label's text up to the first `<` or `"` is the module path, relative to the
# repo root. Every other node shape is free: [[...]] for a level box that links
# to its own map, (...) for something outside the repo. Subgraph titles are
# never nodes.
#
# It fails when:
#   - a directory holding a go.mod appears in no map;
#   - a module appears in more than one map;
#   - a module appears in a map other than the nearest map above it;
#   - a map names a module path that has no go.mod.
#
# Usage: ./scripts/check-knowledge-map.sh [repo-root]

set -euo pipefail

repo_root="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
repo_root="$(cd "$repo_root" && pwd)"

# Directories holding a go.mod, relative to the root; "." for a root module.
mapfile -t modules < <(
	find "$repo_root" -path "$repo_root/.worktrees" -prune -o -path '*/.git' -prune \
		-o -name go.mod -type f -print |
		sed -e "s#^$repo_root/##" -e 's#/\?go\.mod$##' -e 's#^$#.#' | sort
)

# Every knowledge map, relative to the root.
mapfile -t maps < <(
	find "$repo_root" -path "$repo_root/.worktrees" -prune -o -path '*/.git' -prune \
		-o -path '*/knowledge/map.md' -type f -print |
		sed "s#^$repo_root/##" | sort
)

if [ ${#maps[@]} -eq 0 ]; then
	echo "✗ no knowledge/map.md found under $repo_root" >&2
	exit 1
fi

# map level: the directory that holds knowledge/ ("." for the root).
level_of() {
	local level="${1%/knowledge/map.md}"
	if [ "$level" = "knowledge/map.md" ]; then level="."; fi
	printf '%s' "$level"
}

# The nearest map at or above a module directory.
nearest_map_for() {
	local dir="$1"
	while :; do
		if [ "$dir" = "." ]; then
			[ -f "$repo_root/knowledge/map.md" ] && printf '%s' "knowledge/map.md"
			return
		fi
		if [ -f "$repo_root/$dir/knowledge/map.md" ]; then
			printf '%s' "$dir/knowledge/map.md"
			return
		fi
		dir="$(dirname "$dir")"
	done
}

# Module paths a map's mermaid blocks name, one per line, in file order.
nodes_of() {
	awk '/^[[:space:]]*```mermaid/{inb=1;next} inb&&/^[[:space:]]*```/{inb=0} inb&&!/^[[:space:]]*subgraph/{print}' "$repo_root/$1" |
		grep -oE '[A-Za-z0-9_]+\["[^"<]+' |
		sed -e 's#^[A-Za-z0-9_]*\["##' -e 's#[[:space:]]*$##' || true
}

failures=()
pairs=()

for map in "${maps[@]}"; do
	while IFS= read -r node; do
		[ -n "$node" ] || continue
		pairs+=("$node"$'\t'"$map")
		if [ ! -f "$repo_root/$node/go.mod" ]; then
			failures+=("$node — named in $map, but $node/go.mod does not exist")
		fi
	done < <(nodes_of "$map")
done

for module in "${modules[@]}"; do
	mapfile -t found_in < <(printf '%s\n' "${pairs[@]}" | awk -F'\t' -v m="$module" '$1==m{print $2}' | sort -u)
	want="$(nearest_map_for "$module")"
	if [ ${#found_in[@]} -eq 0 ]; then
		failures+=("$module — has a go.mod but appears in no map; draw it in ${want:-a knowledge/map.md}")
		continue
	fi
	if [ ${#found_in[@]} -gt 1 ]; then
		failures+=("$module — appears in ${#found_in[@]} maps (${found_in[*]}); draw it only in $want")
		continue
	fi
	if [ "${found_in[0]}" != "$want" ]; then
		failures+=("$module — drawn in ${found_in[0]}, but its nearest map is $want")
	fi
done

if [ ${#failures[@]} -gt 0 ]; then
	echo "✗ knowledge maps disagree with the module tree:" >&2
	printf '    %s\n' "${failures[@]}" >&2
	echo >&2
	echo "  Each go.mod directory is drawn as ID[\"<module path>...\"] in exactly one" >&2
	echo "  knowledge/map.md: the nearest one above it. Fix the map in the same PR" >&2
	echo "  that adds, moves or retires the module." >&2
	exit 1
fi

levels=()
for map in "${maps[@]}"; do levels+=("$(level_of "$map")"); done
echo "✓ all ${#modules[@]} modules appear exactly once, in the nearest of ${#maps[@]} knowledge maps (${levels[*]})"
