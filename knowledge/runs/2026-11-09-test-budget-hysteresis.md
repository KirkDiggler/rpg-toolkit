# Run — 2026-11-09 — the test budget fails only over twice the target

Base: `rpg-toolkit` `main` at e4ac31f8, after [#2341](https://github.com/KirkDiggler/rpg-toolkit/pull/2341). Story: [rpg-project#907](https://github.com/KirkDiggler/rpg-project/issues/907), follow-up to slice [rpg-project#910](https://github.com/KirkDiggler/rpg-project/issues/910).

## Why

The 5s line flapped twice in one day on tests near it. `TestSingleRoomWallSuite` took 21s on CI and 8.5s locally. `TestRegionSetupSuite` took 6.7s on CI after passing the run before. One test's CI time varies about 2x between runs (the #2341 run record has two more), so a hard line at the target turns runner variance into red builds.

## Shipped

- `scripts/test-budget.sh check` keeps `-budget` (5s) as the target. A test over it that the allowlist does not name is reported "over target" with its seconds, in the summary table and the log. GitHub also gets a warning annotation, and the job stays green.
- `-hard` sets the line the job fails over: a multiple of the budget (`2x`, the default) or seconds (`10s`). A test over it that the allowlist does not name fails the job, as before.
- An allowlisted test never fails. Every allowlist line still needs a package-qualified name and a reason. The `.allow` files are unchanged.
- The workflow paragraph says why there are two lines. The sizing issues [#2342](https://github.com/KirkDiggler/rpg-toolkit/issues/2342) and [#2343](https://github.com/KirkDiggler/rpg-toolkit/issues/2343) stand.

## The three fixture cases, by hand

```
$ # a 6 s test
$ test-budget.sh check
### Slowest tests

| package | test | seconds |
|---|---|---|
| small | TestSix (over target) | 6 |

test-budget: over target: TestSix (github.com/KirkDiggler/rpg-toolkit/small) took 6s; the target is 5s, the job fails over 10s
test-budget: every top-level test is within the 10s hard line or allowed
exit 0

$ # an 11 s test
$ test-budget.sh check
### Slowest tests

| package | test | seconds |
|---|---|---|
| small | TestEleven **over the hard line** | 11 |

test-budget: over the 10s hard line (the target is 5s):
  TestEleven (github.com/KirkDiggler/rpg-toolkit/small) took 11s
The target is 5s per top-level test and the job fails over 10s; exceptions live in (none: pass -module or -allow).
A slow test is a finding: size the fixture to the cases, or name it here with a reason (rpg-project#907).
exit 1

$ # the same 11 s test, allowlisted
$ test-budget.sh check -allow h.allow
### Slowest tests

| package | test | seconds |
|---|---|---|
| small | TestEleven (allowed) | 11 |

test-budget: allowed: TestEleven (github.com/KirkDiggler/rpg-toolkit/small) took 11s
test-budget: every top-level test is within the 10s hard line or allowed
exit 0
```

## The script's cases

`./scripts/test-budget.test.sh`:

```
• an 11 s test over the 10 s hard line fails, naming it, the target, the hard line and the allowlist
• the same 11 s test allowlisted with a reason passes
• an allowlist line without a reason is an error
• a '#' with nothing after it is not a reason
• 'not yet sized' is not a reason
• an entry is package-qualified: a bare name is an error
• a same-named test in another package is not allowed by the first's entry
• -module reads .github/testbudget/<module>.allow from the repository root
• a 6 s test is over target: reported with its seconds, not red
• -hard moves the hard line: seconds or a multiple of the budget
• -budget raises the bar
• the table has ten rows at most, slowest first
• with GITHUB_STEP_SUMMARY set, the table goes there
• a failed test counts too, and non-JSON lines are ignored
• log prints a failing test's output and stays quiet on passing ones

✅ test-budget.sh cases pass
```
