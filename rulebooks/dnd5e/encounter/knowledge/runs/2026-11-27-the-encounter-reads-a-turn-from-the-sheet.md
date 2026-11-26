# Run — 2026-11-27 — NPC turn 1d-3 the encounter reads a creature's turn from its sheet

Slice [rpg-project#1208](https://github.com/KirkDiggler/rpg-project/issues/1208) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`0db836af`](https://github.com/KirkDiggler/rpg-toolkit/commit/0db836af).

One line per entry. Every line ends with a link. The blocking gap was answered by Amendment 1 (way b) and the slice taken up again.

## Shipped

- `SheetFacts.Turn *TurnFacts` (nil is one action and one bonus action; negative is `ErrInvalidData`) and `turnNumbers`, the one function that assigns the view's two counts — [#1208](https://github.com/KirkDiggler/rpg-project/issues/1208).
- `Dasher` seam and `Actors.Dasher` (optional); `refuseUnpaidDash` asks the sheet, then the Dasher, then adds the speed, and refuses with `its action is spent` or `nobody can be paid for its run` — [#1208](https://github.com/KirkDiggler/rpg-project/issues/1208).
- The turn loop counts movement only: `attackRefusal` reads the view, nothing is zeroed after a strike, `disengageBeforeWalk` reads the sheet and decrements nothing; `turnPause` and `TurnBudgetData` save movement only — [#1208](https://github.com/KirkDiggler/rpg-project/issues/1208).
- Ten new tests (`turnfacts_test.go`) and the turn host (`testturns_test.go`, `testturns_internal_test.go`) — [#1208](https://github.com/KirkDiggler/rpg-project/issues/1208).

## Decided

- None beyond what the issue gave; the turn host is as the issue describes it (spends after a Strike that returned nil or `ErrStrikePaused`) — [#1208](https://github.com/KirkDiggler/rpg-project/issues/1208).
- `buildMonsterView` takes the clock as a fourth argument and sets `view.Clock` itself, because the world's zero-and-zero is decided where the counts are read — [#1208](https://github.com/KirkDiggler/rpg-project/issues/1208).
- `disengageBeforeWalk` takes the clock instead of a budget, and never reads the sheet on the world clock — [#1208](https://github.com/KirkDiggler/rpg-project/issues/1208).

## Found

- F1 to F4 and F6 hold as written; F5 is a map with drift: 8 files define a `Sheets` method on a named receiver (not 21), 13 `Strike` methods (not 19), 5 `Announce` (not 8) — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1208#issuecomment-7200439173).
- The suite passes at `0db836af` in 1:16 wall; at the production change the `encounter` package passes and two tests in other packages of this module fail — [list](https://github.com/KirkDiggler/rpg-project/issues/1208#issuecomment-7200596637).
- GAP, answered by [Amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1208#issuecomment-7200768904): `castletest` `TestMadKingTheReservesComeIntoTheGatehouse` and `dungeonspec` `TestThePursuitHallGuardRunsThenSwings` play a run on a turn and need the turn host, which is in package `encounter_test` and cannot be imported; no file in either package is in the issue's list — [gap](https://github.com/KirkDiggler/rpg-project/issues/1208#issuecomment-7200597012).
- Existing tests edited, by kind. A (wrapped in the turn host, assertions unchanged): `TestAParkedEndingFiresBeforeTheConsultDrivesAMonster/control` (internal host), `TestNimbleGoblinDisengagesBeforeWalkingOutOfReach`, `TestSpentBonusActionDisengagesNobody`, `TestARunWithTheSwingSpentIsNotOnTheTable`, `TestADownedTeammateDoesNotHandTheDrivenMonsterASecondTurn`, `TestAttackExecutesAndConsumesTheBudget`, `TestAReloadDescribesNoTableTheRecordHolds`, `TestARefusedAttackSaysWhyWhoeverDrove`, `TestTowardATargetAlreadyInReachStaysAndSaysSo`, `TestAnOpportunityAttackMidWalkRepointsAttacker`, `TestASpentCreatureIsNotAskedToSwingAgain`, `TestTheDefaultTableClosesAndThenSwings` (12) — [list](https://github.com/KirkDiggler/rpg-project/issues/1208#issuecomment-7200596637).
- D (a run on a turn, now with the turn host): `TestACarefulDasherRunsOnlyWhenNobodyElseCouldReachWhereItLands`, `TestADashRunsTwiceTheCellsAndDoesNotSwing`, `TestAHandRolledRunAfterTheSwingIsRefused`, `TestARunAddsTheSpeedToWhatIsLeft`, `TestAMovedBeatNamesItsKind/a_Dash` (5) — [list](https://github.com/KirkDiggler/rpg-project/issues/1208#issuecomment-7200596637).
- B (shape changed, same assertion): the `buildMonsterView` and `disengageBeforeWalk` call sites in 8 internal test files, `turnPause.movementFeet` in 2, `TurnBudgetData` in 1; `TestRebindBindsEveryCapability` and `TestRefusingCapabilitiesLeaveNoInterfaceSlotNil` (a new optional `Actors` field) fit B only as "updated to the new shape" — [list](https://github.com/KirkDiggler/rpg-project/issues/1208#issuecomment-7200531506).
- C: `TestPausedTurnKeepsItsBonusAction` rewritten as `TestAPausedTurnSavesItsMovementOnly` — [list](https://github.com/KirkDiggler/rpg-project/issues/1208#issuecomment-7200531506).
- Proof (a), `budget.AttacksLeft = 0` back after a strike with the view filled from it: Test 3 fails, expected views `[2 1 0]`, actual `[1 0]` — [#1208](https://github.com/KirkDiggler/rpg-project/issues/1208).
- Proof (b), a nil `Turn` meaning nothing left: 24 top-level tests fail in the module, one new (Test 1, expected 1 got 0) and 23 existing, among them `TestADrivenAttackTurnsBeforeTheSwing`, `TestActSuite`, `TestCannotMoveSuite` — [#1208](https://github.com/KirkDiggler/rpg-project/issues/1208).
- Proof (c), the speed added before the Dasher is asked, SURVIVES: a Dasher that fails aborts the verb, the turn's budget dies with it, and nothing observable differs; Test 6 stays green. The closest mutant that is observable, swallowing the Dasher's error, fails Test 6 with "Expected error ... in chain but got nil" — [#1208](https://github.com/KirkDiggler/rpg-project/issues/1208).
- Proof (d), the world clock's view filled from the sheet: Test 7 fails, expected 0 attacks and 0 bonus actions, actual 1 and 1 — [#1208](https://github.com/KirkDiggler/rpg-project/issues/1208).
- Amendment 1: the turn host moved to `internal/turntest`; `castletest` (wrapped once in `castleHost.capabilities`, about 35 tests across six files run on it) and `dungeonspec` (`TestThePursuitHallGuardRunsThenSwings`) became Kind D with no assertion changed and all pass; `replaytest` needed nothing — [Amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1208#issuecomment-7200768904).
- Gates at the head: `go build ./...`, `go vet ./...`, `golangci-lint run ./...` print pass; `go test ./... -count=1 -timeout 30m` passes in every package, 1:00.6 wall against 1:16.4 before — [#1208](https://github.com/KirkDiggler/rpg-project/issues/1208).

## Mistaken

- The second comment said 24 existing tests fail at runtime; the list under it has 21 (12 A, 5 D, 2 B, 2 in the gap) — [list](https://github.com/KirkDiggler/rpg-project/issues/1208#issuecomment-7200596637).
- `TestADownedTeammateDoesNotHandTheDrivenMonsterASecondTurn` builds its own capabilities and was not wrapped the first time, so it looked like a failure of the host rather than of the wrap; wrapping it passes with the host exactly as the issue describes it — [#1208](https://github.com/KirkDiggler/rpg-project/issues/1208).
