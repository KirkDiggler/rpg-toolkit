# Run — 2026-11-03 — nilnil #2220 — encounter first tranche

processed: 2026-11-05T201556Z

Part of [rpg-toolkit#2220](https://github.com/KirkDiggler/rpg-toolkit/issues/2220). Base: `rpg-toolkit` `main` at 8622a295. Ruling on the fix shape: [the issue's plan comment](https://github.com/KirkDiggler/rpg-toolkit/issues/2220#issuecomment-6612643483).

## Shipped

- Three seams answer sentinels: `bubbleFor` → `errNotInBubble` (14 call sites), `disengageBeforeWalk` → `errNoDisengage` (2 callers), `settleWalkOn` → `errNoSettle` (2 callers) — [rpg-toolkit#2220](https://github.com/KirkDiggler/rpg-toolkit/issues/2220).
- The two `behavior.Pick` consumers adapted to `ErrSilent`: `answer.go` consumes silence as an empty answer; `tabledriver.go` maps silence to `ErrBadAnswer`, which is what its nil-guard always meant — [rpg-toolkit#2249](https://github.com/KirkDiggler/rpg-toolkit/pull/2249).
- Internal pins for the two encounter-owned sentinels: `TestBubbleForAnswersNotInBubble`, `TestDisengageBeforeWalkAnswersNoDisengage` — neither signal crosses the API, so external tests cannot see them — [rpg-toolkit#2220](https://github.com/KirkDiggler/rpg-toolkit/issues/2220).

## Decided

- The bubble sentinel is `errNotInBubble`, not `errOnWorldClock`: a nil bubble means two states — the world clock, or NO clock at all after a half-failed Dissolve — and clock_of re-checks for exactly that; the sentinel carries what the function knows, the caller infers the rest — [clocks.go](https://github.com/KirkDiggler/rpg-toolkit/blob/main/rulebooks/dnd5e/encounter/clocks.go).
- The caller transform keeps each site's branch shape: `if err != nil && !errors.Is(err, …)` falls through with a nil bubble, so every `bubble == nil` branch below reads unchanged — the signal is the communication, the nil is the branch, and the 14-site diff stays one line each — [trigger.go](https://github.com/KirkDiggler/rpg-toolkit/blob/main/rulebooks/dnd5e/encounter/trigger.go).
- `tabledriver`'s `ErrSilent` branch is untestable through the public seam — `Pick` never silences for `AnswerTime` (an empty table holds) — so it stays the documented contract-guard: if the contract ever breaks, the driver aborts rather than crashes — [tabledriver.go](https://github.com/KirkDiggler/rpg-toolkit/blob/main/rulebooks/dnd5e/encounter/tabledriver.go).

## Found

- The true nilnil counts at `origin/main` are: encounter 23, session 22, resolution 2 (both test files), behavior/dnd5e/world 1 each — the wave's first two measurements said 5 and 11, both wrong — [#2220 counts](https://github.com/KirkDiggler/rpg-toolkit/issues/2220#issuecomment-6612643483).
- This tranche fixes 6 of encounter's 23; the remaining 17 are the follow-up tranches, listed per site in the issue — [#2220](https://github.com/KirkDiggler/rpg-toolkit/issues/2220).
- The review round's mutant proved nothing pinned that ONLY [behavior.ErrSilent] is swallowed in `answer`: the all-errors-as-silence mutant survived the full encounter suite at the tranche head. The named test `TestAZeroLoadedAnswerIsRefusedNotSilenced` kills it now — a zero-loaded flee answer is refused with `ErrBadTable`, and under the mutant it fails with "expected `behavior: table cannot be rolled` in chain but got nil" — [the review thread](https://github.com/KirkDiggler/rpg-toolkit/pull/2253#pullrequestreview-5758990296).

## Mistaken

- Two measurement faults stacked: the scan ran in a main checkout 4 commits behind `origin/main`, and `--enable-only` did not carry the config's `max-same-issues: 0`, so every count ≥3 read as exactly 3. The corrected scan runs from a detached worktree at `origin/main` with `--max-same-issues 0 --max-issues-per-linter 0` explicit — [#2220](https://github.com/KirkDiggler/rpg-toolkit/issues/2220).
- The two new pins were written into `clocks_internal_test.go` as a new file, overwriting 306 committed lines of four white-box tests — the file existed at base; it was never checked. Restored in the same PR, the pins appended to it; the four restored tests validate this tranche's form and clock_of seams — [clocks_internal_test.go](https://github.com/KirkDiggler/rpg-toolkit/blob/main/rulebooks/dnd5e/encounter/clocks_internal_test.go).
- Two mutant-discipline slips while answering the review finding: the first mutant did not compile (a build failure proves nothing), the second run's output was swallowed by a grep so the kill was unread. The kill line must be seen, not assumed — the third run printed it — [the review thread](https://github.com/KirkDiggler/rpg-toolkit/pull/2253#pullrequestreview-5758990296).