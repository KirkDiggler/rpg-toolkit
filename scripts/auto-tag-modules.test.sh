#!/usr/bin/env bash
#
# auto-tag-modules.test.sh — run scripts/auto-tag-modules.sh against throwaway
# repositories and check which tags it mints, where, and whether it fails.
#
# Each case builds a fresh repository with a bare `origin` beside it, so the
# script's real push and remote checks run. Modules in the fixture:
#
#   alpha/        a leaf module
#   beta/         a module with a nested module inside it
#   beta/inner/   the nested module (#917)
#
# Usage: ./scripts/auto-tag-modules.test.sh

set -euo pipefail

script="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/auto-tag-modules.sh"

# Isolate every git call from the caller's configuration (hooks, signing,
# default branch).
export GIT_CONFIG_GLOBAL=/dev/null
export GIT_CONFIG_NOSYSTEM=1
export GIT_AUTHOR_NAME=test GIT_AUTHOR_EMAIL=test@example.com
export GIT_COMMITTER_NAME=test GIT_COMMITTER_EMAIL=test@example.com

scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT

failures=0
current_case=""

fail() {
  echo "  ✗ ${current_case}: $*"
  failures=$((failures + 1))
}

# new_repo NAME — a repository with every module tagged v0.1.0 at its first
# commit, pushed to a bare origin. Leaves the working directory inside it.
new_repo() {
  local dir="$scratch/$1"
  git init -q --bare "$dir/origin.git"
  git init -q -b main "$dir/work"
  cd "$dir/work"
  git remote add origin "$dir/origin.git"
  local m
  for m in alpha beta beta/inner; do
    mkdir -p "$m"
    printf 'module example.com/%s\n' "$m" > "$m/go.mod"
    printf 'package x\n' > "$m/x.go"
  done
  git add -A
  git commit -q -m "initial"
  git push -q origin main
  for m in alpha beta beta/inner; do
    tag_at HEAD "$m/v0.1.0"
  done
}

# change PATH MESSAGE — commit a change to PATH on the current branch.
change() {
  echo "// $RANDOM $2" >> "$1"
  git add "$1"
  git commit -q -m "$2"
}

# tag_at REF TAG — an annotated tag, pushed, as an earlier run would leave it.
tag_at() {
  git tag -a "$2" -m "$2" "$1"
  git push -q origin "$2"
}

# run_tagger — run the script at the checked-out commit; sets $status and
# $output.
run_tagger() {
  set +e
  output=$(bash "$script" 2>&1)
  status=$?
  set -e
}

tag_count() {
  git tag -l | wc -l | tr -d ' '
}

expect_status() {
  [[ "$status" == "$1" ]] || fail "exit status $status, want $1; output:"$'\n'"$output"
}

# expect_tag TAG REF — TAG exists locally and on origin, at REF's commit.
expect_tag() {
  local want got
  want=$(git rev-parse "$2^{commit}")
  got=$(git rev-parse -q --verify "$1^{commit}" 2>/dev/null) || { fail "$1 was not minted"; return; }
  [[ "$got" == "$want" ]] || fail "$1 is at ${got:0:8}, want ${want:0:8}"
  git ls-remote --tags origin | grep -q "refs/tags/$1$" || fail "$1 was not pushed"
}

expect_no_tag() {
  if git rev-parse -q --verify "refs/tags/$1" >/dev/null; then
    fail "$1 was minted"
  fi
}

expect_listed() {
  grep -q "^$2:" "$1" || fail "$2 is not listed in $1"
}

# The normal case: one merge, one run, in order. A rerun at the same commit
# mints nothing.
test_in_order() {
  current_case="in order"
  new_repo in-order
  change alpha/x.go "feat(alpha): a feature"
  run_tagger
  expect_status 0
  expect_tag alpha/v0.2.0 HEAD
  expect_no_tag beta/v0.1.1
  expect_no_tag beta/inner/v0.1.1
  local before
  before=$(tag_count)
  run_tagger
  expect_status 0
  [[ "$(tag_count)" == "$before" ]] || fail "a rerun at the same commit minted a tag"
}

# The incident (#1941): commit A changes alpha, commit B on top changes beta.
# B's run finishes first and tags both at B; A's run comes second. A must mint
# nothing — B's tags already contain A — and the next merge must tag only
# what it changed.
test_out_of_order() {
  current_case="out of order"
  new_repo out-of-order
  change alpha/x.go "fix(alpha): a fix"
  local a
  a=$(git rev-parse HEAD)
  change beta/x.go "feat(beta): a feature"
  local b
  b=$(git rev-parse HEAD)

  run_tagger
  expect_status 0
  expect_tag alpha/v0.1.1 "$b"
  expect_tag beta/v0.2.0 "$b"

  local before
  before=$(tag_count)
  git checkout -q "$a"
  run_tagger
  expect_status 0
  [[ "$(tag_count)" == "$before" ]] || fail "the run at A minted a tag: $(git tag --points-at "$a" | tr '\n' ' ')"
  expect_no_tag alpha/v0.1.2
  expect_listed deferred_modules.txt alpha
  expect_listed deferred_modules.txt beta
  grep -q "::warning title=Module deferred::alpha: alpha/v0.1.1 is at ${b:0:7}" <<<"$output" ||
    fail "no deferral warning naming alpha, its tag and commit; output:"$'\n'"$output"

  git checkout -q main
  change alpha/x.go "fix(alpha): another fix"
  run_tagger
  expect_status 0
  expect_tag alpha/v0.1.2 HEAD
  expect_no_tag beta/v0.2.1
  expect_no_tag beta/v0.3.0
}

# A nested module's change tags the nested module and never its parent (#917).
test_nested_module() {
  current_case="nested module"
  new_repo nested
  change beta/inner/x.go "feat(inner): a feature"
  run_tagger
  expect_status 0
  expect_tag beta/inner/v0.2.0 HEAD
  expect_no_tag beta/v0.2.0
  expect_no_tag beta/v0.1.1
}

# Pending runs for A and B were replaced by C's run (GitHub keeps one pending
# run per concurrency group). C's run alone tags everything A, B and C
# changed, graded by all their commits.
test_superseded_runs() {
  current_case="superseded runs"
  new_repo superseded
  change alpha/x.go "fix(alpha): a fix"
  change beta/x.go "fix(beta): a fix"
  change alpha/x.go "feat(alpha): a feature"
  run_tagger
  expect_status 0
  expect_tag alpha/v0.2.0 HEAD
  expect_tag beta/v0.1.1 HEAD
}

# The repository as #1941 left it: a regressive tag (higher version, older
# commit) is in main's history, with correct tags above it. The run at the
# newest commit mints nothing when nothing changed, the next real change
# mints the next number, and a rerun of an old commit's run mints nothing.
test_regressive_tag_present() {
  current_case="regressive tag present"
  new_repo regressive
  change alpha/x.go "fix(alpha): a fix"
  local a
  a=$(git rev-parse HEAD)
  change alpha/x.go "feat(alpha): a feature"
  local b
  b=$(git rev-parse HEAD)
  git push -q origin main
  tag_at "$b" alpha/v0.199.0
  tag_at "$a" alpha/v0.199.1
  change beta/x.go "fix(beta): a fix"
  tag_at HEAD alpha/v0.200.0
  tag_at HEAD beta/v0.1.1
  change alpha/go.mod "fix(alpha): retract v0.199.1"
  tag_at HEAD alpha/v0.200.1

  local before
  before=$(tag_count)
  run_tagger
  expect_status 0
  [[ "$(tag_count)" == "$before" ]] || fail "the run at the newest tagged commit minted a tag"

  change beta/x.go "fix(beta): another fix"
  run_tagger
  expect_status 0
  expect_tag beta/v0.1.2 HEAD
  expect_no_tag alpha/v0.200.2

  change alpha/x.go "fix(alpha): a later fix"
  run_tagger
  expect_status 0
  expect_tag alpha/v0.200.2 HEAD

  before=$(tag_count)
  git checkout -q "$b"
  run_tagger
  expect_status 0
  [[ "$(tag_count)" == "$before" ]] || fail "a rerun at the regressive tag's child minted a tag"
  expect_listed deferred_modules.txt alpha
  git checkout -q main
}

# A module's highest tag sits on a side branch, off main. Nothing can be
# minted for it without breaking version order, so it is refused and the run
# fails — after tagging every other module that changed.
test_tag_off_main() {
  current_case="tag off main"
  new_repo off-main
  git checkout -q -b side
  change alpha/x.go "feat(alpha): side work"
  git push -q origin side
  tag_at HEAD alpha/v0.3.0
  git checkout -q main
  echo "// main" >> alpha/x.go
  echo "// main" >> beta/x.go
  git add alpha/x.go beta/x.go
  git commit -q -m "fix: alpha and beta"
  run_tagger
  expect_status 1
  expect_listed refused_modules.txt alpha
  expect_no_tag alpha/v0.3.1
  expect_no_tag alpha/v0.1.1
  expect_tag beta/v0.1.1 HEAD
  grep -q "::error title=Module refused::alpha: alpha/v0.3.0" <<<"$output" ||
    fail "no refusal error naming alpha and its tag; output:"$'\n'"$output"
}

for t in test_in_order test_out_of_order test_nested_module test_superseded_runs \
  test_regressive_tag_present test_tag_off_main; do
  before=$failures
  "$t"
  if [[ "$failures" == "$before" ]]; then
    echo "  ✓ ${current_case}"
  fi
done

if [[ "$failures" -gt 0 ]]; then
  echo "✗ ${failures} failure(s)"
  exit 1
fi
echo "✓ auto-tag-modules: all cases pass"
