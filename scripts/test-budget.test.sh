#!/usr/bin/env bash
#
# test-budget.test.sh — run scripts/test-budget.sh against fixture
# `go test -json` streams and check its exit status, table and messages.
#
# Usage: ./scripts/test-budget.test.sh

set -euo pipefail

script="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/test-budget.sh"

scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT
unset GITHUB_STEP_SUMMARY GITHUB_ACTIONS

failures=0
current_case=""

fail() {
  echo "  ✗ ${current_case}: $*"
  failures=$((failures + 1))
}

run_case() {
  current_case=$1
  echo "• $1"
}

# event ACTION PACKAGE TEST ELAPSED — one test2json line.
event() {
  printf '{"Action":"%s","Package":"%s","Test":"%s","Elapsed":%s}\n' "$1" "$2" "$3" "$4"
}

pkg=github.com/KirkDiggler/rpg-toolkit/small

# A package with one 6 s test, a 6 s subtest under a fast parent (the parent
# is what is judged), and a quick test.
slow_stream() {
  event run "$pkg" TestSlow 0
  event pass "$pkg" TestSlow 6.02
  event run "$pkg" TestParent 0
  event pass "$pkg" TestParent/sub 6.5
  event pass "$pkg" TestParent 1.0
  event pass "$pkg" TestQuick 0.01
  printf '{"Action":"pass","Package":"%s","Elapsed":7.1}\n' "$pkg"
}

# check STREAM-FN [ARGS...] — run check with the stream on stdin; sets
# $status, $out (stdout) and $err (stderr).
check() {
  local fn=$1
  shift
  set +e
  "$fn" | "$script" check "$@" >"$scratch/out" 2>"$scratch/err"
  status=$?
  set -e
  out=$(cat "$scratch/out")
  err=$(cat "$scratch/err")
}

run_case "a 6 s test over the 5 s budget fails, naming it, the budget and the allowlist"
check slow_stream -allow "$scratch/none.allow"
[[ $status -eq 1 ]] || fail "exit $status, want 1"
[[ $err == *"TestSlow"* ]] || fail "stderr does not name TestSlow: $err"
[[ $err == *"5s budget"* ]] || fail "stderr does not name the budget: $err"
[[ $err == *"$scratch/none.allow"* ]] || fail "stderr does not name the allowlist: $err"
[[ $err == *"slow test is a finding"* ]] || fail "stderr does not carry the finding line: $err"
[[ $err != *"TestParent"* ]] || fail "a 1 s parent with a slow subtest was judged: $err"

run_case "the same test allowlisted with a reason passes"
printf '# header comment\n\n%s/TestSlow  # sleeps on purpose for this case\n' "$pkg" >"$scratch/ok.allow"
check slow_stream -allow "$scratch/ok.allow"
[[ $status -eq 0 ]] || fail "exit $status, want 0: $err"
[[ $out == *"TestSlow (allowed)"* ]] || fail "table does not mark TestSlow allowed: $out"
[[ $out == *"allowed: TestSlow ($pkg) took 6.02s"* ]] || fail "log does not show the allowed test's time: $out"
printf '%s/TestQuick  # was slow once\n' "$pkg" >"$scratch/stale.allow"
check slow_stream -allow "$scratch/stale.allow" -budget 7s
[[ $out == *"allowed: TestQuick ($pkg) took 0.01s, within the budget this run"* ]] || fail "a stale entry is not named: $out"

run_case "an allowlist line without a reason is an error"
printf '%s/TestSlow\n' "$pkg" >"$scratch/bare.allow"
check slow_stream -allow "$scratch/bare.allow"
[[ $status -eq 2 ]] || fail "exit $status, want 2"
[[ $err == *"bare.allow:1"* && $err == *"no reason"* ]] || fail "stderr does not point at the line: $err"

run_case "a '#' with nothing after it is not a reason"
printf '%s/TestSlow   #   \n' "$pkg" >"$scratch/empty.allow"
check slow_stream -allow "$scratch/empty.allow"
[[ $status -eq 2 ]] || fail "exit $status, want 2"

run_case "'not yet sized' is not a reason"
printf '%s/TestSlow  # Not yet sized\n' "$pkg" >"$scratch/parked.allow"
check slow_stream -allow "$scratch/parked.allow"
[[ $status -eq 2 ]] || fail "exit $status, want 2"
printf '%s/TestSlow  # over budget, to be sized: rpg-toolkit#1\n' "$pkg" >"$scratch/issue.allow"
check slow_stream -allow "$scratch/issue.allow"
[[ $status -eq 0 ]] || fail "an issue reason: exit $status, want 0: $err"

run_case "an entry is package-qualified: a bare name is an error"
printf 'TestSlow  # sleeps on purpose\n' >"$scratch/unqualified.allow"
check slow_stream -allow "$scratch/unqualified.allow"
[[ $status -eq 2 ]] || fail "exit $status, want 2"
[[ $err == *"not package-qualified"* ]] || fail "stderr does not say why: $err"

run_case "a same-named test in another package is not allowed by the first's entry"
two_packages() {
  slow_stream
  event pass "$pkg/other" TestSlow 6.4
}
check two_packages -allow "$scratch/ok.allow"
[[ $status -eq 1 ]] || fail "exit $status, want 1"
[[ $err == *"TestSlow ($pkg/other) took 6.4s"* ]] || fail "the other package's TestSlow is not named: $err"
[[ $err != *"TestSlow ($pkg) took"* ]] || fail "the allowed package's TestSlow was named: $err"

run_case "-module reads .github/testbudget/<module>.allow from the repository root"
fake=$scratch/repo
mkdir -p "$fake/scripts" "$fake/.github/testbudget/rulebooks"
cp "$script" "$fake/scripts/test-budget.sh"
printf '%s/TestSlow  # sleeps on purpose for this case\n' "$pkg" >"$fake/.github/testbudget/rulebooks/small.allow"
set +e
slow_stream | (cd "$scratch" && "$fake/scripts/test-budget.sh" check -module rulebooks/small) >"$scratch/out" 2>"$scratch/err"
status=$?
set -e
[[ $status -eq 0 ]] || fail "exit $status, want 0: $(cat "$scratch/err")"
grep -q '^### Slowest tests — rulebooks/small' "$scratch/out" || fail "table not labelled with the module: $(cat "$scratch/out")"
set +e
slow_stream | "$fake/scripts/test-budget.sh" check -module rulebooks/absent >"$scratch/out" 2>"$scratch/err"
status=$?
set -e
[[ $status -eq 1 ]] || fail "absent allowlist: exit $status, want 1 (no exceptions)"
grep -q '.github/testbudget/rulebooks/absent.allow' "$scratch/err" || fail "the message does not name where the allowlist goes: $(cat "$scratch/err")"

run_case "-budget raises the bar"
check slow_stream -budget 7s
[[ $status -eq 0 ]] || fail "exit $status, want 0: $err"
check slow_stream -budget 1m
[[ $status -eq 0 ]] || fail "1m: exit $status, want 0: $err"
check slow_stream -budget fast
[[ $status -eq 2 ]] || fail "a bad budget: exit $status, want 2"

run_case "the table has ten rows at most, slowest first"
twelve() {
  local i
  for i in $(seq 1 12); do event pass "$pkg" "Test$i" "$(printf '0.%02d' "$i")"; done
}
check twelve
rows=$(grep -c '^| small | Test' <<<"$out" || true)
[[ $rows -eq 10 ]] || fail "$rows rows, want 10: $out"
first=$(grep -m1 '^| small | Test' <<<"$out")
[[ $first == "| small | Test12 | 0.12 |" ]] || fail "first row is '$first', want Test12"

run_case "with GITHUB_STEP_SUMMARY set, the table goes there"
: >"$scratch/summary.md"
GITHUB_STEP_SUMMARY="$scratch/summary.md" check slow_stream -label "small (all)" -allow "$scratch/ok.allow"
grep -q '^### Slowest tests — small (all)' "$scratch/summary.md" || fail "no heading in the summary"
grep -q 'TestSlow' "$scratch/summary.md" || fail "no TestSlow row in the summary"
[[ $out != *"| package |"* ]] || fail "table also went to stdout"

run_case "a failed test counts too, and non-JSON lines are ignored"
mixed() {
  echo "go: downloading something"
  event fail "$pkg" TestBroken 9.0
}
check mixed
[[ $status -eq 1 && $err == *"TestBroken"* ]] || fail "exit $status, err $err"

run_case "log prints a failing test's output and stays quiet on passing ones"
log_stream() {
  printf '{"Action":"output","Package":"%s","Test":"TestGood","Output":"=== RUN   TestGood\\n"}\n' "$pkg"
  printf '{"Action":"output","Package":"%s","Test":"TestGood","Output":"chatter from a passing test\\n"}\n' "$pkg"
  event pass "$pkg" TestGood 0.1
  printf '{"Action":"output","Package":"%s","Test":"TestBad","Output":"=== RUN   TestBad\\n"}\n' "$pkg"
  printf '{"Action":"output","Package":"%s","Test":"TestBad","Output":"    bad_test.go:9: want 1, got 2\\n"}\n' "$pkg"
  printf '{"Action":"output","Package":"%s","Test":"TestBad","Output":"--- FAIL: TestBad (0.00s)\\n"}\n' "$pkg"
  event fail "$pkg" TestBad 0.0
  printf '{"Action":"output","Package":"%s","Test":"TestHung","Output":"last words of a hung test\\n"}\n' "$pkg"
  printf '{"Action":"output","Package":"%s","Output":"FAIL\\n"}\n' "$pkg"
  printf '{"Action":"output","Package":"%s","Output":"FAIL\\t%s\\t0.2s\\n"}\n' "$pkg" "$pkg"
  printf '{"ImportPath":"%s/broken","Action":"build-output","Output":"broken/x.go:3:1: syntax error\\n"}\n' "$pkg"
}
logged=$(log_stream | "$script" log)
[[ $logged == *"want 1, got 2"* ]] || fail "failing test output missing: $logged"
[[ $logged == *"--- FAIL: TestBad"* ]] || fail "FAIL line missing: $logged"
[[ $logged != *"chatter"* ]] || fail "passing test output printed: $logged"
[[ $logged != *"=== RUN"* ]] || fail "=== RUN lines printed: $logged"
[[ $logged == *"last words of a hung test"* ]] || fail "unfinished test output missing: $logged"
[[ $logged == *"FAIL	$pkg"* ]] || fail "package result line missing: $logged"
[[ $logged == *"syntax error"* ]] || fail "build output missing: $logged"

echo
if [[ $failures -ne 0 ]]; then
  echo "❌ $failures check(s) failed"
  exit 1
fi
echo "✅ test-budget.sh cases pass"
