#!/usr/bin/env bash
#
# test-budget.sh — read `go test -json` output on stdin.
#
#   test-budget.sh log
#       Stream the JSON as a readable log, the way plain `go test` prints:
#       package result lines, build errors, and the output of each test that
#       fails (or never finishes). Passing tests stay quiet. Always exits 0;
#       go test's own exit status is the caller's (use `set -o pipefail`).
#
#   test-budget.sh check [-budget 5s] [-allow FILE] [-label NAME]
#       Fold the elapsed time of every top-level test (no "/" in its name),
#       per package. Write the ten slowest as a Markdown table to
#       $GITHUB_STEP_SUMMARY when it is set, else to stdout. Exit 1 naming
#       each test over the budget that FILE does not list. Exit 2 when FILE
#       has a line without a reason.
#
# The allowlist names one test per line with its reason on the same line:
#
#   TestObserversOf_EqualsFullRefresh  # the castle row needs ... (rpg-project#909)
#
# A name matches that top-level test in any package of the module. Blank
# lines and lines starting with "#" are ignored. A test run more than once
# (-count) is judged by its slowest run.
#
# Cases: scripts/test-budget.test.sh. Why: rpg-project#907.

set -euo pipefail

usage() {
  sed -n '3,22p' "$0" | sed 's/^# \{0,1\}//' >&2
  exit 2
}

# The log filter. jq holds each running test's output and prints it only when
# that test fails, or when the stream ends with the test still running (a
# timeout panic kills the binary before test2json reports the test).
cmd_log() {
  { cat; printf '{"Action":"__end"}\n'; } | jq -Rrj --unbuffered '
    def key: (.Package // "") + " " + (.Test // "");
    def shown: map(select(test("^=== (RUN|PAUSE|CONT|NAME) ") | not)) | join("");
    foreach (inputs | fromjson? // empty) as $e ({buf: {}, emit: ""};
      .emit = ""
      | if $e.Action == "__end" then
          .emit = ([.buf | to_entries[] | .value | shown] | join(""))
          | .buf = {}
        elif $e.Action == "build-output" then
          .emit = $e.Output
        elif $e.Test == null then
          # Package level: result lines, "[no test files]", panics. Drop the
          # bare PASS that precedes "ok".
          if $e.Action == "output" and $e.Output != "PASS\n" then .emit = $e.Output else . end
        elif $e.Action == "output" then
          .buf[$e | key] += [$e.Output]
        elif $e.Action == "fail" then
          .emit = ((.buf[$e | key] // []) | shown)
          | del(.buf[$e | key])
        elif $e.Action == "pass" or $e.Action == "skip" then
          del(.buf[$e | key])
        else . end;
      .emit)'
}

# parse_seconds 5s|5|2.5s|1m → seconds as a number jq accepts.
parse_seconds() {
  local v=$1
  if [[ $v =~ ^([0-9]+(\.[0-9]+)?)s?$ ]]; then
    echo "${BASH_REMATCH[1]}"
  elif [[ $v =~ ^([0-9]+(\.[0-9]+)?)m$ ]]; then
    awk -v m="${BASH_REMATCH[1]}" 'BEGIN { print m * 60 }'
  else
    echo "test-budget: bad budget '$v' (want e.g. 5s, 2.5s, 1m)" >&2
    exit 2
  fi
}

cmd_check() {
  local budget=5 allow="" label=""
  while [[ $# -gt 0 ]]; do
    case $1 in
      -budget) budget=$(parse_seconds "${2:?-budget needs a value}"); shift 2 ;;
      -allow) allow=${2:?-allow needs a path}; shift 2 ;;
      -label) label=${2:?-label needs a value}; shift 2 ;;
      *) usage ;;
    esac
  done

  # Read the allowlist: every entry must carry a reason.
  local names=() bad=0 n=0 line name reason
  if [[ -n $allow && -f $allow ]]; then
    while IFS= read -r line || [[ -n $line ]]; do
      n=$((n + 1))
      [[ $line =~ ^[[:space:]]*(#|$) ]] && continue
      name=${line%%#*}
      name=${name//[[:space:]]/}
      reason=""
      [[ $line == *"#"* ]] && reason=${line#*#}
      reason=${reason//[[:space:]]/}
      if [[ -z $reason ]]; then
        echo "test-budget: $allow:$n: '$name' has no reason; write it as '$name  # why it may be slow'" >&2
        bad=1
        continue
      fi
      names+=("$name")
    done <"$allow"
  fi
  if [[ $bad -ne 0 ]]; then
    exit 2
  fi
  local allowed
  allowed=$(printf '%s\n' "${names[@]+"${names[@]}"}" | jq -R . | jq -sc 'map(select(. != ""))')

  # One row per top-level test, slowest first: [package, test, seconds, allowed].
  local rows
  rows=$(jq -Rc 'fromjson? // empty
      | select((.Action == "pass" or .Action == "fail") and .Test != null and (.Test | contains("/") | not))' |
    jq -sc --argjson allowed "$allowed" '
      group_by([.Package, .Test])
      | map({pkg: .[0].Package, test: .[0].Test, s: (map(.Elapsed // 0) | max)})
      | map(. + {allowed: (.test as $t | $allowed | index($t) != null)})
      | sort_by(-.s, .pkg, .test)')

  local out=${GITHUB_STEP_SUMMARY:-/dev/stdout}
  {
    echo "### Slowest tests${label:+ — $label}"
    echo
    if [[ $(jq length <<<"$rows") -eq 0 ]]; then
      echo "No tests ran."
    else
      echo "| package | test | seconds |"
      echo "|---|---|---|"
      jq -r --argjson b "$budget" '.[:10][]
          | "| \(.pkg | sub("^github\\.com/[^/]+/[^/]+/"; "")) | \(.test)\(if .s > $b then (if .allowed then " (allowed)" else " **over budget**" end) else "" end) | \(.s * 100 | round / 100) |"' <<<"$rows"
    fi
    echo
  } >>"$out"

  local over
  over=$(jq -r --argjson b "$budget" '.[] | select(.s > $b and (.allowed | not))
      | "\(.pkg) \(.test) \(.s * 100 | round / 100)"' <<<"$rows")
  if [[ -z $over ]]; then
    echo "test-budget: every top-level test${label:+ in $label} is within ${budget}s or allowed"
    return 0
  fi

  local allow_path=${allow:-testbudget.allow}
  echo "test-budget: over the ${budget}s budget${label:+ in $label}:" >&2
  while read -r pkg test s; do
    echo "  $test ($pkg) took ${s}s" >&2
    if [[ -n ${GITHUB_ACTIONS:-} ]]; then
      echo "::error title=Test over budget::$test ($pkg) took ${s}s; the budget is ${budget}s (allowlist $allow_path)"
    fi
  done <<<"$over"
  echo "The budget is ${budget}s per top-level test; exceptions live in $allow_path." >&2
  echo "A slow test is a finding: size the fixture to the cases, or name it here with a reason (rpg-project#907)." >&2
  return 1
}

case ${1:-} in
  log) shift; cmd_log "$@" ;;
  check) shift; cmd_check "$@" ;;
  *) usage ;;
esac
