# 2026-11-04 · The guard holds the gate, S1c · Session

Story [rpg-project#836](https://github.com/KirkDiggler/rpg-project/issues/836), slice [rpg-project#857](https://github.com/KirkDiggler/rpg-project/issues/857), Task 18 of `ideas/the-guard-holds-the-gate/plan.md` (design R6). On encounter [rpg-toolkit#2268](https://github.com/KirkDiggler/rpg-toolkit/pull/2268).

## Shipped

- `go.mod` pins encounter at the pushed commit a0a25b0c, pseudo-version `v0.176.1-0.20261104204649-a0a25b0cc5ce`. The release tag re-pin follows #2268's merge.
- The launch needed no change. The dungeonspec compiles `against` and `pass` into the post itself, and [`launch.go:242`](https://github.com/KirkDiggler/rpg-toolkit/blob/86008a65/rulebooks/dnd5e/session/launch.go#L242) carries the post by value. [`TestCastleTheLaunchSeatsTheGatePost`](https://github.com/KirkDiggler/rpg-toolkit/blob/86008a65/rulebooks/dnd5e/session/castle_gate_hold_test.go#L117) now asserts gate-1's posts with `against: entering` and `pass: troupe-admitted`.
- `Sighting` gains `HoldsAgainst` (`holds_against`) and `HoldsSide` (`holds_side`) beside `HoldsDoor` ([`types.go:1050`](https://github.com/KirkDiggler/rpg-toolkit/blob/86008a65/rulebooks/dnd5e/session/types.go#L1050)). They are filled from the same single `ObservedHolds` call at [`read.go:481`](https://github.com/KirkDiggler/rpg-toolkit/blob/86008a65/rulebooks/dnd5e/session/read.go#L481), copied as given, and empty when no hold is told.
- [`TestCastleTheGateGuardIsSeenToCheckArrivals`](https://github.com/KirkDiggler/rpg-toolkit/blob/86008a65/rulebooks/dnd5e/session/castle_gate_direction_test.go#L96): alice beside the gate sees gate-1 hold the gate door, `entering`, with the two gatehouse cells (-4,7) and (-3,7) as its side. The JSON carries `holds_against` and `holds_side`. His ally gate-2 is told none of the three. Dropping the side copy fails it.
- [`TestCastleTheTroupeIsToldNoHold`](https://github.com/KirkDiggler/rpg-toolkit/blob/86008a65/rulebooks/dnd5e/session/castle_gate_direction_test.go#L128): after the stranger's line and the captain's troupe line, alice sees gate-1 still on his post and is told no hold. She opens the gate and crosses into the courtyard while he stays posted.
- [`TestCastleTheGateLetsThePartyOut`](https://github.com/KirkDiggler/rpg-toolkit/blob/86008a65/rulebooks/dnd5e/session/castle_gate_direction_test.go#L163): from the courtyard cell past the open gate, alice walks out unchallenged. The walk back in stops with "Guard bars the way".
- [`TestCastleABothWaysPostBarsTheWayOut`](https://github.com/KirkDiggler/rpg-toolkit/blob/86008a65/rulebooks/dnd5e/session/castle_gate_direction_test.go#L180): gate-1's posts set both ways in the stored world bar the same walk out.
- [`TestCastleTheGateDoorWorksFromInside`](https://github.com/KirkDiggler/rpg-toolkit/blob/86008a65/rulebooks/dnd5e/session/castle_gate_direction_test.go#L193): from the courtyard, OpenDoor opens the shut gate, and Unlock of the locked gate faces the lock and throws a die.
- The arrival from the gatehouse stays barred. The S1b tests `TestCastleThePostHoldsTheGatesOpening` ("Guard bars the way") and `TestCastleAHeldDoorRefusesItsDoorVerbs` ("Guard bars the door") pass unchanged on the entering post.

## Decided

- The plan's test of "a viewer at the party's start" is not written. From the start alice never sees gate-1, so she is told no hold of him there. Every sighting test stands her in sight of the gate.
- The plan's "viewer on the open side sees no hold" line is stale. The ruled behaviour tells a hold in any region, filtered by pass and alliance, and the session copies that answer without filtering.
- The courtyard tests seat alice past the gate in the stored record, then step her a cell inward and back. Without the step she has not seen the gate's lock from inside, and Unlock answers no such connection.

## Changed by the content

- The troupe line no longer cows gate-1. The troupe route (`playTheTroupe`) and `TestOpenDoorLoadsOnce` walked onto his cell beneath the gate, which only the cowing had freed. Both now work the gate from the opening's other cell, (-4,7).
- `Example_theTroupeActs` is regenerated. On the troupe line gate-1 goes `posted -> alert (heard something)` instead of `posted -> cowed`, and stays at his post. The wait walks between (-5,8) and (-4,7), so the sight gains and losses of the courtyard guards are gone from it.

## Found

- On the troupe line, alert gate-1 answers `post` with "toward" while already on his post, and the act folds as a `move` stay with an empty reason (`gate-1 stays:`). The posted state answers the same way as a `pass` with "already there". This is the encounter's beat, not the session's.
- Opening the gate from (-4,7) folds a sight perception with no gained, lost or changed members. It carries the knowledge change alone.

## Not walked

- An `against: leaving` post. The castle sets none and the plan's leaving test is covered at the encounter. The session copies the word as given.
- A post with no one side, where `HoldsSide` is empty beside a door. The castle gate always has one.

— cross-team agent, on behalf of KirkDiggler
