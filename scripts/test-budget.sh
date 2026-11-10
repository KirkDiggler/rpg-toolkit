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
#   test-budget.sh check [-budget 5s] [-hard 2x] [-module PATH | -allow FILE] [-label NAME]
#       Fold the elapsed time of every top-level test (no "/" in its name),
#       per package. Write the ten slowest as a Markdown table to
#       $GITHUB_STEP_SUMMARY when it is set, else to stdout. A test over the
#       budget (the target) that FILE does not list is reported "over
#       target" with its seconds, in the table and the log, and does not
#       fail. Exit 1 naming each test over the hard line that FILE does not
#       list. -hard is a multiple of the budget (2x, the default) or seconds
#       (10s). Exit 2 when FILE has a line without a reason.
#
# Why two lines: CI run times for one test vary about 2x between runs, so a
# hard line at the 5s target flaps on tests near it. The target stays 5s; a
# test over it is still a finding, and the sizing issues hold them.
#
# -module PATH reads the module's allowlist from
# .github/testbudget/PATH.allow at the repository root (PATH as the module's
# directory, slashes kept, e.g. rulebooks/dnd5e/encounter) and labels the
# table with PATH. The allowlists live there, not in the module, so editing
# one mints no module tag. -allow FILE reads FILE instead. An absent file
# means the module has no exceptions.
#
# The allowlist names one test per line, as its package's import path (the
# "Package" field of go test -json), a slash and the top-level test name, with
# its reason on the same line:
#
#   github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/TestObserversOf_EqualsFullRefresh  # the castle row needs ... (rpg-project#909)
#
# A reason says why the test is over budget (what it builds or sweeps and why
# that cannot shrink), or names the issue that will size it ("over budget, to
# be sized: rpg-toolkit#NNNN"); "not yet sized" alone is no reason. The budget
# is judged on CI's timing. An entry matches that one package's test only; a
# line that is not package-qualified is an error too. Blank
# lines and lines starting with "#" are ignored. A test run more than once
# (-count) is judged by its slowest run.
#
# Cases: scripts/test-budget.test.sh. Why: rpg-project#907.

set -euo pipefail

usage() {
  awk 'NR > 2 && !/^#/ { exit } NR > 2 { sub(/^# ?/, ""); print }' "$0" >&2
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
  local budget=5 hard=2x allow="" label="" root
  root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
  while [[ $# -gt 0 ]]; do
    case $1 in
      -budget) budget=$(parse_seconds "${2:?-budget needs a value}"); shift 2 ;;
      -hard) hard=${2:?-hard needs a value}; shift 2 ;;
      -allow) allow=${2:?-allow needs a path}; shift 2 ;;
      -module)
        allow="$root/.github/testbudget/${2:?-module needs a path}.allow"
        label=${label:-$2}
        shift 2
        ;;
      -label) label=${2:?-label needs a value}; shift 2 ;;
      *) usage ;;
    esac
  done

  # The hard line: a multiple of the budget, or seconds.
  if [[ $hard =~ ^([0-9]+(\.[0-9]+)?)x$ ]]; then
    hard=$(awk -v b="$budget" -v m="${BASH_REMATCH[1]}" 'BEGIN { print b * m }')
  else
    hard=$(parse_seconds "$hard")
  fi

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
      # "not yet sized" parks a test without saying why it is slow.
      if [[ $name != ?*/?* ]]; then
        echo "test-budget: $allow:$n: '$name' is not package-qualified; write it as '<import path>/$name', the Package field of go test -json" >&2
        bad=1
        continue
      fi
      if [[ -z $reason || ${reason,,} == notyetsized* ]]; then
        echo "test-budget: $allow:$n: '$name' has no reason; write why it is over budget (what it builds or sweeps and why that cannot shrink), or 'over budget, to be sized: <issue>'" >&2
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
      | map(. + {allowed: ((.pkg + "/" + .test) as $k | $allowed | index($k) != null)})
      | sort_by(-.s, .pkg, .test)')

  # fd 3 is the summary; stdout itself when there is none (reopening
  # /dev/stdout would write over the log lines that follow).
  if [[ -n ${GITHUB_STEP_SUMMARY:-} ]]; then
    exec 3>>"$GITHUB_STEP_SUMMARY"
  else
    exec 3>&1
  fi
  {
    echo "### Slowest tests${label:+ — $label}"
    echo
    if [[ $(jq length <<<"$rows") -eq 0 ]]; then
      echo "No tests ran."
    else
      echo "| package | test | seconds |"
      echo "|---|---|---|"
      jq -r --argjson b "$budget" --argjson h "$hard" '.[:10][]
          | "| \(.pkg | sub("^github\\.com/[^/]+/[^/]+/"; "")) | \(.test)\(if .s > $b then (if .allowed then " (allowed)" elif .s > $h then " **over the hard line**" else " (over target)" end) else "" end) | \(.s * 100 | round / 100) |"' <<<"$rows"
    fi
    echo
  } >&3

  # Allowed tests, with this run's time, so an entry that no longer needs its
  # line shows up in the log.
  jq -r --argjson b "$budget" '.[] | select(.allowed)
      | "test-budget: allowed: \(.test) (\(.pkg)) took \(.s * 100 | round / 100)s\(if .s > $b then "" else ", within the budget this run" end)"' <<<"$rows"

  local allow_path=${allow:-"(none: pass -module or -allow)"}
  allow_path=${allow_path#"$root/"}

  # Over the target, under the hard line: reported, not red.
  local warn pkg test s
  warn=$(jq -r --argjson b "$budget" --argjson h "$hard" '.[] | select(.s > $b and .s <= $h and (.allowed | not))
      | "\(.pkg) \(.test) \(.s * 100 | round / 100)"' <<<"$rows")
  if [[ -n $warn ]]; then
    while read -r pkg test s; do
      echo "test-budget: over target: $test ($pkg) took ${s}s; the target is ${budget}s, the job fails over ${hard}s"
      if [[ -n ${GITHUB_ACTIONS:-} ]]; then
        echo "::warning title=Test over target::$test ($pkg) took ${s}s; the target is ${budget}s (allowlist $allow_path)"
      fi
    done <<<"$warn"
  fi

  local over
  over=$(jq -r --argjson h "$hard" '.[] | select(.s > $h and (.allowed | not))
      | "\(.pkg) \(.test) \(.s * 100 | round / 100)"' <<<"$rows")
  if [[ -z $over ]]; then
    echo "test-budget: every top-level test${label:+ in $label} is within the ${hard}s hard line or allowed"
    return 0
  fi

  echo "test-budget: over the ${hard}s hard line (the target is ${budget}s)${label:+ in $label}:" >&2
  while read -r pkg test s; do
    echo "  $test ($pkg) took ${s}s" >&2
    if [[ -n ${GITHUB_ACTIONS:-} ]]; then
      echo "::error title=Test over the hard line::$test ($pkg) took ${s}s; the target is ${budget}s, the hard line ${hard}s (allowlist $allow_path)"
    fi
  done <<<"$over"
  echo "The target is ${budget}s per top-level test and the job fails over ${hard}s; exceptions live in $allow_path." >&2
  echo "A slow test is a finding: size the fixture to the cases, or name it here with a reason (rpg-project#907)." >&2
  return 1
}

case ${1:-} in
  log) shift; cmd_log "$@" ;;
  check) shift; cmd_check "$@" ;;
  *) usage ;;
esac
