# Run — 2026-11-09 — CI module test budget and the encounter root package inventory

processed: 2026-11-12T165555Z

Base: `rpg-toolkit` `main` at d7c8ea06. Blocked PRs: [#2320](https://github.com/KirkDiggler/rpg-toolkit/pull/2320), [#2331](https://github.com/KirkDiggler/rpg-toolkit/pull/2331). Follow-up: [#2335](https://github.com/KirkDiggler/rpg-toolkit/issues/2335).

## Shipped

- The "Check module" step runs `go test` with `-timeout 25m` per test binary, and `test-changed` has `timeout-minutes: 40`. This buys time, not speed.

## Decided

- No sharding here: [#2236](https://github.com/KirkDiggler/rpg-toolkit/issues/2236) (shard by package) is open and unbuilt, so there is no shape to extend. The root package alone now crosses 10m, so package shards are not enough; the root-package partition is filed as [#2335](https://github.com/KirkDiggler/rpg-toolkit/issues/2335).
- The Makefile `test-all` (main pushes, race without cover) is left at the default 10m: it still passes on main, and the brief's budget is the module step.

## Found

- CI trend for the root package under race + cover: 319 s, 406 s, 505 s across S7–S9 of rpg-project#879, then past 600 s on #2320 and #2331. CI runners are about 3x the dev box.
- Local inventory: package 188 s, 242 s wall with build, 526 top-level tests, all pass. Machine: 32 cores, load 23–25 during the run.
- The head dominates: top 3 tests 84 s (45%), top 15 138 s; 498 tests under 1 s sum to 26 s.
- Every branch of "Detect changed modules" that set `should_run_all=true` left `modules` unset, so `test-changed`'s matrix evaluated `fromJson('')` even with its `if` false; the job graph failed and the required `ci-status` never reported (main's push run 42670064127 shows "failure" with every job green). Those branches now also write `modules=[]`.

Commands:

```
cd rulebooks/dnd5e/encounter
go test -race -timeout 40m -json . > root.json
# top-level pass/fail events, no "/" in Test, sorted by Elapsed
```

| # | test | s |
|---|------|---|
| 1 | `TestObserversOf_EqualsFullRefresh` | 34.7 |
| 2 | `TestTheGarrisonsDoorOpeningsAreTheWholeFloorScans` | 29.7 |
| 3 | `TestPlacedReach_AgreesWithTheWalk` | 19.6 |
| 4 | `TestDiscoverCells_EqualsFullSweep` | 6.6 |
| 5 | `TestRouteTo_EqualsFlood` | 5.3 |
| 6 | `TestApproaches_CastleHoldsTheOracle` | 5.0 |
| 7 | `TestACastsShovesLandInsideItsLandingOnTheWorldClock` | 5.0 |
| 8 | `TestRouteTo_Deterministic` | 4.9 |
| 9 | `TestShownTo_MatchesAtlasFor` | 4.5 |
| 10 | `TestPropWithinReach_AgreesWithWithinReach` | 4.3 |
| 11 | `TestAPushOnSomebodyTheCastDidNotReachIsRefusedBeforeAnyBeat` | 4.2 |
| 12 | `TestAThunderwavedGuardKeepsItsFacingOnTheCastle` | 4.0 |
| 13 | `TestAWhisperedGuardTurnsToFaceItsFlightOnTheCastle` | 4.0 |
| 14 | `TestPlacedReach_ADroppedBlockerIsAskedWhereItLies` | 3.2 |
| 15 | `TestPlacementIndex_AgreesWithTheWalk` | 3.2 |
