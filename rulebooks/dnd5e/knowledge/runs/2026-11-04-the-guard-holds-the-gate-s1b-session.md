# 2026-11-04 · The guard holds the gate, S1b · Session

Story [rpg-project#836](https://github.com/KirkDiggler/rpg-project/issues/836), slice [rpg-project#852](https://github.com/KirkDiggler/rpg-project/issues/852), Task 12 of `ideas/the-guard-holds-the-gate/plan.md` (H4 at the Move, H5 at the sighting, H6 on the session's map; design R5). On encounter [rpg-toolkit#2263](https://github.com/KirkDiggler/rpg-toolkit/pull/2263).

## Shipped

- `go.mod` pins encounter at the pseudo-version of #2263's head 06567a22 while it is in review, after b727dad4 before R7, 61f09764 before `DoorHeldFor`, and 979466c0 before its reach check. The release tag replaces it once #2263 merges.
- `mind/machine` becomes a direct requirement. The turn-clock test builds a posted recipe with it.
- Launch hands the placement's posts to the join beside its blocking states at [`launch.go:242`](https://github.com/KirkDiggler/rpg-toolkit/blob/c6ccada9/rulebooks/dnd5e/session/launch.go#L242). Without it the castle's gate-1 holds his one cell, and the party steps through the opening beside him.
- [`TestCastleTheLaunchSeatsTheGatePost`](https://github.com/KirkDiggler/rpg-toolkit/blob/c6ccada9/rulebooks/dnd5e/session/castle_gate_hold_test.go#L67) asserts gate-1 is seated with the posts the author wrote, both cells and the qualified door, and gate-2 with none.
- [`TestCastleThePostHoldsTheGatesOpening`](https://github.com/KirkDiggler/rpg-toolkit/blob/c6ccada9/rulebooks/dnd5e/session/castle_gate_hold_test.go#L81) opens the gate door with alice on the opening's free cell. Her step through it answers `stopped` with "Guard bars the way" and takes no step. A walk through his own cell answers "Guard stands in the way", as before. After the captain stands aside she still sees him, cowed, holding nothing, and she crosses.
- [`TestAHeldCrossingOnTheTurnClockPaysNoMovement`](https://github.com/KirkDiggler/rpg-toolkit/blob/c6ccada9/rulebooks/dnd5e/session/move_held_turnclock_test.go#L28) posts a skeleton on an open edge door in a fight. The walked cell is paid and the held crossing is not.
- `Sighting.HoldsDoor` is filled from `encounter.ObservedHolds` at [`read.go:475`](https://github.com/KirkDiggler/rpg-toolkit/blob/c6ccada9/rulebooks/dnd5e/session/read.go#L475). Alice's sighting of posted gate-1 names `the-mad-king/gate-wall-door`. gate-2's sighting of him, as his ally, names none. A memory names none ([`TestCastleAMemoryOfTheGateGuardHoldsNoDoor`](https://github.com/KirkDiggler/rpg-toolkit/blob/c6ccada9/rulebooks/dnd5e/session/castle_gate_hold_test.go#L137)).
- `AtlasDoorway.Crossing` is copied in the atlas conversion at [`convert.go:183`](https://github.com/KirkDiggler/rpg-toolkit/blob/c6ccada9/rulebooks/dnd5e/session/convert.go#L183). It decodes from both reveal beats through its `crossing` tag, with no code in `act_family.go`.
- [`TestCastleTheGatesCrossingsAreDoorwaysOfTheMap`](https://github.com/KirkDiggler/rpg-toolkit/blob/c6ccada9/rulebooks/dnd5e/session/atlas_crossing_test.go#L20) asserts the gate's two crossings by value, each marked. [`TestAnEdgeDoorsDoorwayIsNoCrossing`](https://github.com/KirkDiggler/rpg-toolkit/blob/c6ccada9/rulebooks/dnd5e/session/atlas_crossing_test.go#L37) asserts an edge door's doorways read false.
- [`TestRevealedCrossingsKeepTheirMark`](https://github.com/KirkDiggler/rpg-toolkit/blob/c6ccada9/rulebooks/dnd5e/session/act_fold_internal_test.go#L699) folds a concealment reveal and a cells-learned beat, with and without the mark.
- The footprint leaf's reveal test now asserts the act carries the leaf's crossings, each marked, equal to the doorways her map lists. The veil's reveal asserts its edge doorway is unmarked.
- The door-revealed doc and `Atlas.Doorways` doc now say a footprint door's crossings arrive marked.

## Decided

- `HoldsDoor` sits beside `Passage` on `Sighting`, outside `Seen`. The hold is a fact about the viewer, because an ally is barred by nothing, and the encounter strips it from delivered testimony.
- The session asks `ObservedHolds` once per view and copies its answer. It decides nothing about allies, memories or unfound doors.
- The castle test teaches `gate-cowed` through alice's stand-aside line on a fixed die, as S1's test does.

## Found

- The castle has no edge doors. Every door is a footprint door, so the gate's opening is two crossings into the door's own cell. The edge-door rows use the session's own hex fixtures.
- At launch alice does not see gate-1. She first sees him from the gatehouse.

## Not walked

- A guard drawn off his post by a voice. At the session it needs the stranger's companion seated outside, which this castle launch does not seat. The encounter's castle test covers it, and the session copies the encounter's answer.
- A path from the courtyard side of the opening. Nobody reaches the courtyard while gate-1 holds the gate.

## R7: a held door refuses its door verbs

Ruled on lane #835, with the #855 design review's two additions.

- A held door's refusal crosses as `ErrDoorHeld`, with `DoorHeldError{Reason}` carrying the encounter's public reason as written. The arm sits at the top of [`translate`](https://github.com/KirkDiggler/rpg-toolkit/blob/d2251f22/rulebooks/dnd5e/session/read.go#L720). The encounter's error answers both its door-held and its body-in-the-way sentinels, so the arm comes before anything the second could match. It matches the held error alone, so `ErrNoDoor` and `ErrBadDoor` stay no connection.
- Unlock asks `DoorHeldFor` (held and its public reason) before it reads the lock at [`doors.go:298`](https://github.com/KirkDiggler/rpg-toolkit/blob/d2251f22/rulebooks/dnd5e/session/doors.go#L298). A held lock throws no die and poses no Guidance pause.
- A paused unlock whose door is held by the time it is answered is declined, committed, then refused ([`declineHeldUnlock`](https://github.com/KirkDiggler/rpg-toolkit/blob/d2251f22/rulebooks/dnd5e/session/react.go#L450)). A refusal that dropped the scope would leave the window in the ledger and the session frozen.
- [`TestCastleAHeldDoorRefusesItsDoorVerbs`](https://github.com/KirkDiggler/rpg-toolkit/blob/d2251f22/rulebooks/dnd5e/session/castle_gate_hold_test.go#L222): OpenDoor and CloseDoor are refused with "Guard bars the door" and the gate is unchanged. Once gate-1 is cowed she opens it.
- [`TestCastleAHeldLockedDoorIsAnsweredByTheGuard`](https://github.com/KirkDiggler/rpg-toolkit/blob/d2251f22/rulebooks/dnd5e/session/castle_gate_hold_test.go#L256): the gate is locked in the record before alice arrives, so she knows the lock. Unlock is refused by the guard, and the castle dice count no die thrown. Dropping the pre-check fails it.
- [`TestAHeldDoorClosesAPausedUnlock`](https://github.com/KirkDiggler/rpg-toolkit/blob/d2251f22/rulebooks/dnd5e/session/held_unlock_resume_test.go#L62): a skeleton behind the gate stands to its post while her Guidance answer waits. Her answer is refused with "someone bars the door", the window is closed, her die is not spent, and the next Unlock meets the hold rather than a frozen session. Dropping the check fails all three.
- [`TestTranslateCarriesAHeldDoorWithItsReason`](https://github.com/KirkDiggler/rpg-toolkit/blob/d2251f22/rulebooks/dnd5e/session/translate_internal_test.go#L373) covers the arm both ways.
- The castle tests now open and lock the gate in the stored record. The hold test keeps its shut-door half before the open one.

### Found in R7

- Unlock reads the lock through what the member knows of the door. A lock set in the record after alice saw the gate shut never rolls, so the first locked test passed without the pre-check. The gate is now locked before she arrives, with a precondition that she knows it.
- `DoorHeldFor` at 979466c0 did not check reach, while the encounter's door verbs check reach before the hold, so an Unlock from out of reach on a held door answered the hold. 06567a22 checks reach first. `TestCastleAHeldDoorOutOfReachIsOutOfReach` pins the order: red on 979466c0, green on 06567a22.

— cross-team agent, on behalf of KirkDiggler
