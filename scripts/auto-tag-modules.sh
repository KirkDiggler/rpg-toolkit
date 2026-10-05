#!/usr/bin/env bash
#
# auto-tag-modules.sh — mint a version tag for every module that changed since
# its last tag, at the commit checked out (HEAD), and push the new tags.
#
# Called by .github/workflows/auto-tag-modules-safe.yml on every push to main.
# Tested by scripts/auto-tag-modules.test.sh, which builds throwaway
# repositories and runs this script against them.
#
# A module's version may only be minted at a commit that descends from that
# module's last tag. Version order must agree with history: a higher version
# at an older commit hands every `go get @latest` older content (#1941), and
# the Go proxy caches it, so the tag cannot be taken back. Before tagging a
# module this script requires its highest version tag to be an ancestor of
# HEAD:
#
#   - HEAD is an ancestor of that tag: this run is behind. A run for a later
#     commit already tagged the module, and its tag contains everything HEAD
#     contains. Nothing is owed; the module is deferred, loudly, and the run
#     still succeeds.
#   - Neither is an ancestor of the other: the tag sits off main's history
#     (cut by hand or from a side branch). No version can be minted here
#     without breaking version order, so the module is refused and the run
#     fails.
#
# Writes tagged_modules.txt, skipped_modules.txt, deferred_modules.txt and
# refused_modules.txt in the working directory for the workflow's summary.
#
# Usage: ./scripts/auto-tag-modules.sh   (from the repository root)

set -eo pipefail

# Pathspecs excluding any nested module living inside this one, so a
# child module's changes never mark or grade the parent. Without this,
# every resolution-only merge minted a phantom parent dnd5e version
# (#917: v0.85.0, v0.91.0, v0.92.0 all point at child-only merges).
# vendor/ go.mod files are vendored dependencies, not module
# boundaries, and stay included in the parent's diff.
nested_module_excludes() {
  local module=$1
  find "$module" -mindepth 2 -name "go.mod" -type f -not -path "*/vendor/*" | \
    while read -r nested; do
      printf ':(exclude)%s\n' "$(dirname "$nested")"
    done
}

# Markdown neither marks nor grades a module. A README, an AGENTS.md or
# a design note changes nothing a consumer compiles against, so minting a
# version for one hands whoever runs `go get` a tag whose whole content is
# prose (Kirk, 2026-09-10).
#
# Nothing is lost by not tagging it. The diff below is taken from the last
# TAG rather than the last commit, so a doc-only commit is simply carried
# into the next real version — documentation that lands beside code still
# ships in that code's tag.
#
# .go files are deliberately NOT excluded: a doc-comment-only change is a
# godoc change, and godoc IS part of what a consumer receives.
DOC_EXCLUDES=(':(exclude)*.md')

# Function to determine version bump from commit messages
# Looks for conventional commit patterns:
# - feat: minor bump
# - fix: patch bump
# - BREAKING CHANGE: major bump — but see increment_version: on a v0.x
#   module this lands as a MINOR bump, per semver §4. v1.0.0 is never
#   reached automatically.
# - chore/docs/style/refactor/test: patch bump
determine_bump() {
  local module=$1
  local last_tag=$2

  if [[ -z "$last_tag" ]]; then
    compare_from="HEAD~10"  # Look at last 10 commits for new modules
  else
    compare_from="$last_tag"
  fi

  # Check commit messages for this module — excluding nested modules,
  # whose subjects must not grade the parent's bump (#917). local, so
  # this can never shadow a caller's excludes — belt and braces: the
  # function is only ever invoked via $(...) command substitution,
  # whose subshell already isolates it.
  local -a excludes
  mapfile -t excludes < <(nested_module_excludes "$module")
  commits=$(git log --format="%s" "$compare_from..HEAD" -- "$module" "${excludes[@]}" "${DOC_EXCLUDES[@]}" 2>/dev/null || echo "")

  # Default to patch
  bump="patch"

  # Check for breaking changes (major)
  if echo "$commits" | grep -q "BREAKING CHANGE\|!:"; then
    bump="major"
  # Check for features (minor)
  elif echo "$commits" | grep -q "^feat"; then
    bump="minor"
  fi

  echo "$bump"
}

# Function to increment version
increment_version() {
  local version=$1
  local bump=$2

  # Remove 'v' prefix
  version=${version#v}

  # Split into major.minor.patch
  IFS='.' read -r major minor patch <<< "$version"

  case $bump in
    major)
      # Semver §4: "Major version zero (0.y.z) is for initial development.
      # Anything MAY change at any time." On a v0.x module a breaking change
      # is therefore a MINOR bump, not a major one — and reaching v1.0.0 is a
      # deliberate act of declaring an API stable, never a side effect of a
      # commit message containing "!:".
      #
      # Without this, a single `refactor(x)!:` promotes a mid-development
      # module straight to v1.0.0. Every module in this repo is currently
      # v0.x, so that promotion would always be wrong and is not something a
      # tag can cleanly take back once a consumer has fetched it.
      if [[ "$major" == "0" ]]; then
        minor=$((minor + 1))
        patch=0
      else
        major=$((major + 1))
        minor=0
        patch=0
      fi
      ;;
    minor)
      minor=$((minor + 1))
      patch=0
      ;;
    patch)
      patch=$((patch + 1))
      ;;
  esac

  echo "v${major}.${minor}.${patch}"
}

head_sha=$(git rev-parse --short HEAD)

# Track what we tag for the summary
TAGGED_MODULES=()
SKIPPED_MODULES=()
DEFERRED_MODULES=()
REFUSED_MODULES=()

# Process each module
for modfile in $(find . -name "go.mod" -type f -not -path "./vendor/*" -not -path "./examples/*"); do
  module_path=$(dirname "$modfile" | sed 's|^\./||')

  # Skip root module
  if [[ "$module_path" == "." ]]; then
    continue
  fi

  # The module's last tag is its highest version in HEAD's history. The
  # highest version overall must be that same tag: a higher one that HEAD
  # does not contain means version order and history disagree here, and
  # minting on top of either would release older content under a higher
  # number or re-mint a number that already exists.
  top_tag=$(git tag -l "${module_path}/v*" | sort -V | tail -1)
  last_tag=$(git tag -l --merged HEAD "${module_path}/v*" | sort -V | tail -1)

  if [[ -n "$top_tag" && "$top_tag" != "$last_tag" ]]; then
    top_sha=$(git rev-parse --short "${top_tag}^{commit}")
    if git merge-base --is-ancestor HEAD "$top_tag"; then
      echo "::warning title=Module deferred::${module_path}: ${top_tag} is at ${top_sha}, a descendant of ${head_sha}; a later run already tagged this module, not tagging at ${head_sha}"
      DEFERRED_MODULES+=("${module_path}:${top_tag}:${top_sha}")
    else
      echo "::error title=Module refused::${module_path}: ${top_tag} is at ${top_sha}, which is not in the history of ${head_sha}; refusing to tag a module whose highest version sits off main"
      REFUSED_MODULES+=("${module_path}:${top_tag}:${top_sha}")
    fi
    continue
  fi

  # Check if module has changes since last tag — a nested module's
  # files are a different module's changes and do not count (#917)
  mapfile -t excludes < <(nested_module_excludes "$module_path")
  if [[ -z "$last_tag" ]]; then
    has_changes=true  # New module
  else
    changes=$(git diff --name-only "$last_tag" HEAD -- "$module_path" "${excludes[@]}" "${DOC_EXCLUDES[@]}" | head -1)
    has_changes=$([[ -n "$changes" ]] && echo true || echo false)
  fi

  if [[ "$has_changes" == "true" ]]; then
    echo "Module $module_path has changes"

    # Determine version bump type
    bump=$(determine_bump "$module_path" "$last_tag")
    echo "Detected bump type: $bump"

    # Calculate new version
    if [[ -z "$last_tag" ]]; then
      new_version="v0.1.0"
    else
      current_version=${last_tag#${module_path}/}
      new_version=$(increment_version "$current_version" "$bump")
    fi

    # Create tag
    tag_name="${module_path}/${new_version}"

    # Check if tag already exists
    if git tag -l "$tag_name" | grep -q "^${tag_name}$"; then
      echo "Tag ${tag_name} already exists locally, skipping"
      SKIPPED_MODULES+=("${module_path}:${new_version}:exists")
      continue
    fi

    # Check if tag exists on remote
    if git ls-remote --tags origin | grep -q "refs/tags/${tag_name}$"; then
      echo "Tag ${tag_name} already exists on remote, skipping"
      SKIPPED_MODULES+=("${module_path}:${new_version}:remote_exists")
      continue
    fi

    # Generate release notes
    release_notes="## ${module_path} ${new_version}\n\n"

    if [[ -n "$last_tag" ]]; then
      # Include commit summary
      release_notes="${release_notes}### Changes\n"
      # Captured, not piped into `while read`: the pipe's subshell
      # discarded every appended line, so annotated tags have carried
      # an empty change list since this workflow was written. The
      # capture has real newlines; echo -e passes them through.
      commit_lines=$(git log --format="- %s (%h)" "$last_tag..HEAD" -- "$module_path" "${excludes[@]}")
      if [[ -n "$commit_lines" ]]; then
        release_notes="${release_notes}${commit_lines}\n"
      fi
    else
      release_notes="${release_notes}Initial release\n"
    fi

    # Create annotated tag
    echo -e "$release_notes" | git tag -a "$tag_name" -F -
    echo "Created tag: $tag_name"

    TAGGED_MODULES+=("${module_path}:${new_version}")
  fi
done

# Push only the new tags
PUSH_FAILED=()
if [[ ${#TAGGED_MODULES[@]} -gt 0 ]]; then
  echo "Pushing ${#TAGGED_MODULES[@]} new tags..."

  # Push each tag individually so one failure does not hold back the rest
  for item in "${TAGGED_MODULES[@]}"; do
    IFS=':' read -r module version <<< "$item"
    tag_name="${module}/${version}"

    if git push origin "$tag_name"; then
      echo "✅ Pushed tag: $tag_name"
    else
      echo "::error title=Tag push failed::${tag_name} was created locally but not pushed"
      PUSH_FAILED+=("$tag_name")
    fi
  done

  # Save for summary
  printf '%s\n' "${TAGGED_MODULES[@]}" > tagged_modules.txt
else
  echo "No modules needed tagging"
  : > tagged_modules.txt
fi

# Save skipped, deferred and refused modules for summary
write_list() {
  local file=$1
  shift
  if [[ $# -gt 0 ]]; then
    printf '%s\n' "$@" > "$file"
  else
    : > "$file"
  fi
}
write_list skipped_modules.txt "${SKIPPED_MODULES[@]}"
write_list deferred_modules.txt "${DEFERRED_MODULES[@]}"
write_list refused_modules.txt "${REFUSED_MODULES[@]}"

if [[ ${#REFUSED_MODULES[@]} -gt 0 || ${#PUSH_FAILED[@]} -gt 0 ]]; then
  echo "Failing: ${#REFUSED_MODULES[@]} module(s) refused, ${#PUSH_FAILED[@]} tag push(es) failed"
  exit 1
fi
