# 2026-11-04 · The guard holds the gate, S1c · Session

Story [rpg-project#836](https://github.com/KirkDiggler/rpg-project/issues/836), slice [rpg-project#857](https://github.com/KirkDiggler/rpg-project/issues/857), Task 18 of `ideas/the-guard-holds-the-gate/plan.md` (design R6). Session [rpg-toolkit#2269](https://github.com/KirkDiggler/rpg-toolkit/pull/2269), on encounter [rpg-toolkit#2268](https://github.com/KirkDiggler/rpg-toolkit/pull/2268).

## Shipped

- `go.mod` pins encounter at #2268's pushed head ca4d2e3c, pseudo-version `v0.176.1-0.20261104210954-ca4d2e3c21b0` ([4413bbd1](https://github.com/KirkDiggler/rpg-toolkit/commit/4413bbd1)). The release tag re-pin follows #2268's merge.
- The launch needed no change. The dungeonspec compiles `against` and `pass` into the post itself, and [`launch.go:242`](https://github.com/KirkDiggler/rpg-toolkit/blob/4413bbd1/rulebooks/dnd5e/session/launch.go#L242) carries the post by value. [`TestCastleTheLaunchSeatsTheGatePost`](https://github.com/KirkDiggler/rpg-toolkit/blob/4413bbd1/rulebooks/dnd5e/session/castle_gate_hold_test.go#L117) asserts gate-1's posts with `against: entering` and `pass: troupe-admitted`.
- `Sighting` gains [`HoldsAgainst`](https://github.com/KirkDiggler/rpg-toolkit/blob/4413bbd1/rulebooks/dnd5e/session/types.go#L1050) (`holds_against`) and [`HoldsSide`](https://github.com/KirkDiggler/rpg-toolkit/blob/4413bbd1/rulebooks/dnd5e/session/types.go#L1057) (`holds_side`) beside `HoldsDoor`. They are filled from the same single [`ObservedHolds` call](https://github.com/KirkDiggler/rpg-toolkit/blob/4413bbd1/rulebooks/dnd5e/session/read.go#L469), copied as given, and empty when no hold is told.
- [`TestCastleTheGateGuardIsSeenToCheckArrivals`](https://github.com/KirkDiggler/rpg-toolkit/blob/4413bbd1/rulebooks/dnd5e/session/castle_gate_direction_test.go#L96): alice beside the gate sees gate-1 hold the gate door, `entering`, with the cells (-4,7) and (-3,7) as its side. The JSON carries `holds_against` and `holds_side`. His ally gate-2 is told none of the three. Dropping the side copy fails it.
- [`TestCastleTheTroupeIsToldNoHold`](https://github.com/KirkDiggler/rpg-toolkit/blob/4413bbd1/rulebooks/dnd5e/session/castle_gate_direction_test.go#L128): after the stranger's line and the captain's troupe line, alice sees gate-1 still on his post and is told no hold. She opens the gate and crosses while he stays posted.
- [`TestCastleTheGateLetsThePartyOut`](https://github.com/KirkDiggler/rpg-toolkit/blob/4413bbd1/rulebooks/dnd5e/session/castle_gate_direction_test.go#L163): from the courtyard past the open gate, alice walks out unchallenged. The walk back in stops with "Guard bars the way".
- [`TestCastleABothWaysPostBarsTheWayOut`](https://github.com/KirkDiggler/rpg-toolkit/blob/4413bbd1/rulebooks/dnd5e/session/castle_gate_direction_test.go#L180): with gate-1's posts set both ways in the stored world, her sighting of him from the courtyard says `both`, and the same walk out is barred.
- [`TestCastleTheGateDoorWorksFromInside`](https://github.com/KirkDiggler/rpg-toolkit/blob/4413bbd1/rulebooks/dnd5e/session/castle_gate_direction_test.go#L198): from the courtyard, OpenDoor opens the shut gate, and Unlock of the locked gate faces the lock and throws a die.
- The arrival from the gatehouse stays barred. [`TestCastleThePostHoldsTheGatesOpening`](https://github.com/KirkDiggler/rpg-toolkit/blob/4413bbd1/rulebooks/dnd5e/session/castle_gate_hold_test.go#L137) and [`TestCastleAHeldDoorRefusesItsDoorVerbs`](https://github.com/KirkDiggler/rpg-toolkit/blob/4413bbd1/rulebooks/dnd5e/session/castle_gate_hold_test.go#L229) pass unchanged on the entering post.

## Decided

- The plan's test of a viewer at the party start is not written ([rpg-project#857](https://github.com/KirkDiggler/rpg-project/issues/857)). From the start alice never sees gate-1, so every sighting test stands her in sight of the gate.
- The plan's "viewer on the open side sees no hold" line is stale ([rpg-toolkit#2268](https://github.com/KirkDiggler/rpg-toolkit/pull/2268)). A hold is told in any region, filtered by pass and alliance, and the session copies that answer without filtering.
- The courtyard tests seat alice past the gate in the stored record, then step her a cell inward and back ([`insideTheGate`](https://github.com/KirkDiggler/rpg-toolkit/blob/4413bbd1/rulebooks/dnd5e/session/castle_gate_direction_test.go#L78)). Without the step she has not seen the gate's lock from inside, and Unlock answers no such connection.
- No session test covers an `against: leaving` post or a hold with no one side ([rpg-toolkit#2268](https://github.com/KirkDiggler/rpg-toolkit/pull/2268)). The castle has neither, the encounter's tests cover both, and the session copies the answer as given.

## Found

- The troupe line no longer cows gate-1 ([rpg-toolkit#2268](https://github.com/KirkDiggler/rpg-toolkit/pull/2268)). [`playTheTroupe`](https://github.com/KirkDiggler/rpg-toolkit/blob/4413bbd1/rulebooks/dnd5e/session/castle_acts_test.go#L232) and [`TestOpenDoorLoadsOnce`](https://github.com/KirkDiggler/rpg-toolkit/blob/4413bbd1/rulebooks/dnd5e/session/one_load_test.go#L102) walked onto his cell beneath the gate, which only the cowing had freed. Both now work the gate from the opening's other cell, (-4,7).
- [`Example_theTroupeActs`](https://github.com/KirkDiggler/rpg-toolkit/blob/4413bbd1/rulebooks/dnd5e/session/castle_acts_test.go#L851) is regenerated. On the troupe line gate-1 goes `posted -> alert (heard something)` instead of `posted -> cowed`, and stays at his post.
- In that transcript, alert gate-1 answers `post` with "toward" while already on his post, and the act folds as a `move` stay with an empty reason (`gate-1 stays:`). Posted, the same answer folds as a `pass` with "already there". This is the encounter's beat ([`Example_theTroupeActs`](https://github.com/KirkDiggler/rpg-toolkit/blob/4413bbd1/rulebooks/dnd5e/session/castle_acts_test.go#L851)).
- Opening the gate from (-4,7) folds a sight perception with no gained, lost or changed members, carrying the knowledge change alone ([`Example_theTroupeActs`](https://github.com/KirkDiggler/rpg-toolkit/blob/4413bbd1/rulebooks/dnd5e/session/castle_acts_test.go#L851)).

## Mistaken

- The first push left encounter v0.176.0's two `go.sum` lines in place. `go get` keeps them and `go mod tidy` drops them, so CI's "Check for modifications" failed at 9f7d08ce ([#2269 review](https://github.com/KirkDiggler/rpg-toolkit/pull/2269)). Fixed by `go mod tidy` in [4413bbd1](https://github.com/KirkDiggler/rpg-toolkit/commit/4413bbd1). A re-pin runs `go mod tidy` before the commit.
- The both-ways test's doc promised a sighting it did not assert ([#2269 review](https://github.com/KirkDiggler/rpg-toolkit/pull/2269)). It now asserts `both` on her sighting ([4413bbd1](https://github.com/KirkDiggler/rpg-toolkit/commit/4413bbd1)).

— cross-team agent, on behalf of KirkDiggler
