# 2026-11-04 · The guard holds the gate S1b · A post holds a passage

processed: 2026-11-12T165555Z

Story [rpg-project#836](https://github.com/KirkDiggler/rpg-project/issues/836), slice [rpg-project#852](https://github.com/KirkDiggler/rpg-project/issues/852), lane [rpg-project#835](https://github.com/KirkDiggler/rpg-project/issues/835) (R5). Plan Task 11 in `ideas/the-guard-holds-the-gate/plan.md`, and R7 (lane #835: a held door refuses its verbs).

## Shipped

- `Post{Door, Cells}` and `MemberInput.Posts` / `JoinInput.Posts`: by state, the door whose opening that state holds and the cells a holder stands on. Empty cells mean the cell the member is placed at, resolved at admission. A state the recipe lacks, posts with no recipe, a door the field lacks or a cell that is not floor is `ErrBadMember` (H1).
- `MemberData.Posts` and `ReserveData.Posts`, `posts,omitempty`, beside `BlockingStates`. A stored post with no cells, or any of the refusals above, is `ErrInvalidData` at load (H5).
- `Encounter.memberHolds` reads the state, the cell and the participation on every call. Nothing is stored, so any leaving empties the hold and standing there again restores it (H1, H2).
- `heldAgainst` returns the non-allied holders of a crossing, nearest the mover first. It skips a door hidden to the mover. A read memoizes the holds once beside, not inside, the shared crossing memo (H2, H3).
- The step refuses a held crossing before the crossing fold, so a shut held door is named by its holder. `walkerEdge`, `stepInto` and `doorToOpen` close a held crossing before the door pass, so a route goes round it and a walk never opens it (H2, H4).
- The held reason is `"<name> bars the way"` for the nearest holder the mover sees, else `"someone bars the way"`, inside `ErrBodyInTheWay`. Driven walks stop silently (H4).
- Sight testimony stores `holds_door`, stripped on delivery like conditions. `ObservedHolds` answers per viewer and omits allies and hidden doors. A transition that changes the post a state names notes the cell for the look (H5).
- `Atlas()` lists a footprint door's crossings as `AtlasDoorway{Crossing: true}`. The door-revealed beat and the room reveal carry them marked `crossing: true` (H6).
- A held door refuses its verbs (R7). `OpenDoor`, `CloseDoor` and `Unlock` on a door a post holds are refused to any actor not allied with a holder, by the same hold read the crossing makes. The order is probe, actor, reach, held, then the lock and state. The refusal is a `*DoorHeldError`, which is `ErrDoorHeld` only: a door verb is not a step, so the step's `ErrBodyInTheWay` stays the step's. Its `PublicReason` is "<name> bars the door" for the nearest holder the actor sees, else "someone bars the door". The holder himself and his allies work the door as before. An empty actor, nobody's hands, is never held against.
- `Encounter.DoorHeldFor(actor, door) (held, reason, err)` answers the same hold read without acting: true with the public reason the verb would give, so a host can ask it before it rolls a lock's check or poses a pause. An unknown or hidden door answers `ErrNoDoor`, as every door verb does under the probe law. The refusal does not wrap `ErrBadDoor`.
- The dialect's `holds: { door, at }` on a state compiles to `MonsterPlacement.Posts`, the door qualified to `<key>/<id>`. An unknown door, a non-floor cell and `holds` on a terminal state are refused at their paths.
- The castle mirror: gate-1's `posted` and `alert` hold `gate-wall-door` at `{q: -3, r: 7}` and `{q: -4, r: 7}`.

## Decided

- The post code lives in `post.go`. The plan named `hold.go`, which is the Hold verb's file.
- `memberHolds` takes the caller's participation read, since every reader already holds one. A failed read holds, as a failed canvas read blocks.
- The step and the masquerade: a crossing the mover's own atlas draws as wall is answered by the masquerade, never by the hold, so a stop cannot name a holder behind a secret.
- `ObservedHolds` reads the stored view, since the hold is not delivered. Allies are left out by believed stance, as `ObservedPassages` does.
- `castleHost.gateUnheld` is opt-in by name, and only on the ten routes that walk from inside the castle out through the gate, pending R6 (rpg-project#835, open: does a post hold both ways). They are `TestMadKingTroupe` and `TestMadKingTroupeBlown` (the cook's gossip), `TestMadKingTheCaptainsWordOutranksRecognition`, `TestMadKingACaptainWithNoFollowRowStaysBehind`, `TestMadKingTurnTheGarrison`, `TestMadKingDeposed`, `TestMadKingSlainAndDeposedEndsSlain`, `TestMadKingTheOrdersTurnTheGarrison`, `TestMadKingTheGuardHoldsTheGate`/turned and, under R7, `TestMadKingScout`. Nothing else sets it.
- The two inbound routes keep the hold, as R5 rules. `TestMadKingEveryRoomHasOneWayIn` starts its room routes in the yard, so its shut-door half is not vacuous. `TestMadKingPickingTheCellLockEndsTheRunFreed` takes the gate by a cowed gate-1, stood so in the record.
- `postedLike` stands gate-2 on the post for the group rule.
- R7 put `TestMadKingScout` among the outbound routes, because it opens the gate from the courtyard on its way out. Two inbound openings now take the gate by the ruled paths. In `TestMadKingEveryRoomHasOneWayIn`/the gatehouse, the record stands gate-1 cowed. In `TestReserves_MarchOnFightRounds`, gate-1 falls before the hero opens the gate mid-fight.

## Found

- The hold bars the opening both ways. A hero coming from the kitchen out through the gate to the captain is barred by gate-1. Whether a post holds an opening inbound only is R6 (rpg-project#835), open; the ten outbound routes run unheld until it is ruled.
- A shut door hides a holder on its far side, so the stop says "someone bars the way". The castle's guard stands on the party's side, so his name is told.
- The voice that draws gate-1 is heard in the round it also walks him off, so no sound leaves him alert on his post. The castle's alert case stands him alert in a stored record.
- Under main's `nilnil`, `postsFromData` answers an empty map for a record with no posts.
- The session rolls a lock's check before it calls `Unlock` (`session/doors.go`), so `Unlock`'s own refusal comes after the roll. T12 must ask `DoorHeldFor` before it stages the check, or a held lock is still rolled against.
- R7 also stops the operator's walk step "open the gate door first from beside the crossing" while gate-1 is posted. The walk opens it only once the post is emptied.
- The session's launch copies `BlockingStates` from the placement by hand (`session/launch.go:237`). T12 must copy `Posts` beside it, or the api castle holds nothing. The plan's T12 does not list it.
- `Atlas.Doorways` gained a `Crossing` field with no JSON tag, so every compiled golden's edge doorways carry `"Crossing": false`.

## Mistaken

- `TestTheLeafIsNotADoorwayAndStillHasAState` pinned that a footprint door is never a doorway. H6 reverses it: its doorways are its crossings, each marked. Renamed `TestTheLeafsDoorwaysAreItsMarkedCrossingsAndItStillHasAState`.

— cross-team agent, on behalf of KirkDiggler
