# Run — 2026-11-09 — a test budget CI enforces

Base: `rpg-toolkit` `main` at f61e4379, after [#2336](https://github.com/KirkDiggler/rpg-toolkit/pull/2336) and [#2338](https://github.com/KirkDiggler/rpg-toolkit/pull/2338). Story: [rpg-project#907](https://github.com/KirkDiggler/rpg-project/issues/907), slice [rpg-project#910](https://github.com/KirkDiggler/rpg-project/issues/910). PR: [#2341](https://github.com/KirkDiggler/rpg-toolkit/pull/2341).

## Shipped

- `scripts/test-budget.sh` reads `go test -json`. Its `log` mode prints what plain `go test` prints: package lines, build errors, and the output of failing or unfinished tests. Its `check` mode writes the ten slowest top-level tests to the job summary. It fails naming each test over `TEST_BUDGET` (5s) that the module's allowlist does not name. `-module <dir>` reads that allowlist from `.github/testbudget/<dir>.allow`, outside the module, so editing one mints no module tag. A module without a file has no exceptions. The change detector maps an allowlist edit to its module.
- An allowlist entry is `<import path>/<test>  # reason`. The import path is go test -json's `Package` field, so one entry exempts one package's test. A bare name, a missing reason, or "not yet sized" alone is an error.
- Every allowed test is logged with its time. A line that a run did not need says "within the budget this run".
- The module step, including each encounter shard, and the test-all job run tests with `-json` through the log filter. A separate step runs the check. `scripts/test-budget.test.sh` holds the cases and runs in its own cheap job.
- Both piping steps, "Check module" and "Run tests on all modules", carry `shell: bash`, which runs them as `bash --noprofile --norc -eo pipefail`. That is the load-bearing guarantee. A `run:` step without `shell:` is `bash -e`, where the log filter's exit 0 hid a failing test. Each step also sets `set -o pipefail`, so the commands keep the guarantee when copied out.

## The pipe keeps go test's exit

The "Check module" step body ran locally against a scratch module. Its root package holds `TestHeavy` and `TestRest`, and its subpackage holds `TestSub`. Each test calls `t.Fatal("probe")` when `PROBE` names it, and `ENCOUNTER_HEAVY_TESTS='^(TestHeavy)$'`. The lint tools were stubbed.

| shard | probe | shell: bash + set | shell: bash only | old: bash -e, no set |
|---|---|---|---|---|
| all | sub | 1 | 1 | 0 |
| all | none | 0 | 0 | 0 |
| heavy | heavy | 1 | 1 | 0 |
| heavy | rest | 0 | 0 | 0 |
| rest | rest | 1 | 1 | 0 |
| rest | heavy | 0 | 0 | 0 |
| subpackages | sub | 1 | 1 | 0 |
| subpackages | none | 0 | 0 | 0 |

The test-all loop ran the same way on a module with a failing test. It exits 1 with pipefail and 0 without it.

## Proof

| run | head | result |
|---|---|---|
| [42687404397](https://github.com/KirkDiggler/rpg-toolkit/actions/runs/42687404397) | c056746a | red: first CI inventory, 19 encounter tests and 1 rulebooks/dnd5e test over 5s that the local run kept under it |
| [42694712241](https://github.com/KirkDiggler/rpg-toolkit/actions/runs/42694712241) | b21f515d | red: names only `TestBudgetProofSleepsSixSeconds (dice) took 6.01s`, the temporary probe |
| [42695321108](https://github.com/KirkDiggler/rpg-toolkit/actions/runs/42695321108) | 82a5970d | green: probe removed, package-qualified allowlists |
| [42698617914](https://github.com/KirkDiggler/rpg-toolkit/actions/runs/42698617914) | 96c05291 | red: the test-all job fails at "Run tests on all modules" with `TestFailingProbe` (`t.Fatal("probe")` in dice), through the `-json` pipe, after the pipefail fix |

The slow probe proves the budget. The failing probe proves the pipe keeps go test's exit. The run at the final head, after both probes are removed, is green.

## Seeded allowlists

| file | entries | why it cannot shrink | near budget | to be sized |
|---|---|---|---|---|
| `.github/testbudget/rulebooks/dnd5e.allow` | 1 | 1 | 0 | 0 |
| `.github/testbudget/rulebooks/dnd5e/encounter.allow` | 56 | 3 | 17 | 36 ([#2342](https://github.com/KirkDiggler/rpg-toolkit/issues/2342); one points at [#2340](https://github.com/KirkDiggler/rpg-toolkit/issues/2340)) |
| `.github/testbudget/rulebooks/dnd5e/session.allow` | 32 | 0 | 15 | 17 ([#2343](https://github.com/KirkDiggler/rpg-toolkit/issues/2343)) |

Other modules had no test over 5s on CI.

## Found

- CI timing varies a lot between runs. `TestPropWithinReach_AgreesWithWithinReach` took 9.99s in run 42687404397 and 4.78s in run 42695321108. `TestAThunderwavedGuardKeepsItsFacingOnTheCastle` took 9.48s, then 4.6s. In the green run, 14 encounter and 18 session lines were not needed. The budget is judged on CI's timing, so the near-budget lines stay until the sizing issues decide them.
- The heaviest tests generate the benchmark keep up to 30k hexes (keepgen), play the mad-king castle script (castletest), or open the castle through the session. The sizing issues hold the per-test tables.

## Commands

```
cd rulebooks/dnd5e/<module>
go test -race -count=1 -timeout 40m -json ./... > out.json
"$(git rev-parse --show-toplevel)/scripts/test-budget.sh" check -module rulebooks/dnd5e/<module> < out.json
./scripts/test-budget.test.sh
```
