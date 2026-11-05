# 2026-11-03 · The guard holds the gate S1 · A body blocks by its state

processed: 2026-11-05T201556Z

Story [rpg-project#836](https://github.com/KirkDiggler/rpg-project/issues/836), slice [rpg-project#841](https://github.com/KirkDiggler/rpg-project/issues/841), lane [rpg-project#835](https://github.com/KirkDiggler/rpg-project/issues/835) (R1 A, R3 C, R4 A). Plan Task 1 in `ideas/the-guard-holds-the-gate/plan.md`.

## Shipped

- `MemberInput.BlockingStates` and `JoinInput.BlockingStates`: the states of the member's recipe in which its body blocks. Empty keeps `BlocksMovement`. Refused at every door without a recipe or for a state the recipe lacks (B1, B4).
- `Encounter.memberBlocks` reads the current machine state on every call. The canvas entity holds a read through the encounter, not a flag. The sighting, the board, the roster row and the fold all ask the same read (B2, B5).
- A transition that enters or leaves a blocking state notes the body's cell for the pass's look, so observers see the change in the round it fires (B5).
- `MemberData.BlockingStates` and `ReserveData.BlockingStates` persist beside the recipe. A record without them loads as before; a stored name the recipe lacks is `ErrInvalidData` (B4).
- `ErrBodyInTheWay`, not wrapping `ErrBadPlacement`. A step refused only by a body returns it inside `StepObstructedError`. `PublicReason` says "<name> stands in the way" for a body the mover sees and "someone stands in the way" otherwise. The driven walk stops at one silently (B6).
- The dialect's `blocks: true` on a state compiles into `MonsterPlacement.BlockingStates`, and is refused on a terminal state (C1).
- A `toward` an authored cell the creature already stands on passes with "already there", so a posted guard's stayed beat says why (found by the session goldens).
- The castle mirror: gate-1 posted on the crossing, `cowed` and `turned`, the post key with a voice draw, and the troupe's admission fact. Castle and replay tests re-aimed (Q1).

## Decided

- The canvas asks who is down only for a blocking monster on a cell somebody is placed on. Otherwise a downed posted guard is standable to the fold and refused by the canvas.
- `ValidateWalkEnd` is unchanged: no walk in this story aims at his cell. The bump is reached by walking past him where he is the only way, so the last cell is beyond him. A goal on his cell comes back with the use case that needs it.
- The post's draw is a `voice` heard this round. The captain stands beside the crossing, so his own lines draw nobody off it. The stranger's answer to a companion outside draws gate-1 for one round.
- The troupe's success teaches `troupe-admitted`, which stands the post aside. Without it the troupe route of the QA script stops at gate-1.

## Found

- Session's launch copies `placement.Machine` into the join by hand (`session/launch.go:233`). The session task must copy `BlockingStates` beside it, or the api castle never blocks.
- A downed `BlocksMovement: true` monster was already refused by the canvas while the fold called it standable. Nothing declared one, so it never showed.
- A companion's word spends a round it had banked and raises nothing. The post answers the voice on the world's next round.

## Mistaken

- Three tests pinned `ErrBadPlacement` and an unnamed reason for a body. They now pin `ErrBodyInTheWay` and the seen body's name, as B6 rules.

— cross-team agent, on behalf of KirkDiggler
