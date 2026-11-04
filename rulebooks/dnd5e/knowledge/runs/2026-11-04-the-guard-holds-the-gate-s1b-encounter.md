# 2026-11-04 · The guard holds the gate S1b · A post holds a passage

Story [rpg-project#836](https://github.com/KirkDiggler/rpg-project/issues/836), slice [rpg-project#852](https://github.com/KirkDiggler/rpg-project/issues/852), lane [rpg-project#835](https://github.com/KirkDiggler/rpg-project/issues/835) (R5). Plan Task 11 in `ideas/the-guard-holds-the-gate/plan.md`.

## Shipped

- `Post{Door, Cells}` and `MemberInput.Posts` / `JoinInput.Posts`: by state, the door whose opening that state holds and the cells a holder stands on. Empty cells mean the cell the member is placed at, resolved at admission. A state the recipe lacks, posts with no recipe, a door the field lacks or a cell that is not floor is `ErrBadMember` (H1).
- `MemberData.Posts` and `ReserveData.Posts`, `posts,omitempty`, beside `BlockingStates`. A stored post with no cells, or any of the refusals above, is `ErrInvalidData` at load (H5).
- `Encounter.memberHolds` reads the state, the cell and the participation on every call. Nothing is stored, so any leaving empties the hold and standing there again restores it (H1, H2).
- `heldAgainst` returns the non-allied holders of a crossing, nearest the mover first. It skips a door hidden to the mover. A read memoizes the holds once beside, not inside, the shared crossing memo (H2, H3).
- The step refuses a held crossing before the crossing fold, so a shut held door is named by its holder. `walkerEdge`, `stepInto` and `doorToOpen` close a held crossing before the door pass, so a route goes round it and a walk never opens it (H2, H4).
- The held reason is `"<name> bars the way"` for the nearest holder the mover sees, else `"someone bars the way"`, inside `ErrBodyInTheWay`. Driven walks stop silently (H4).
- Sight testimony stores `holds_door`, stripped on delivery like conditions. `ObservedHolds` answers per viewer and omits allies and hidden doors. A transition that changes the post a state names notes the cell for the look (H5).
- `Atlas()` lists a footprint door's crossings as `AtlasDoorway{Crossing: true}`. The door-revealed beat and the room reveal carry them marked `crossing: true` (H6).
- The dialect's `holds: { door, at }` on a state compiles to `MonsterPlacement.Posts`, the door qualified to `<key>/<id>`. An unknown door, a non-floor cell and `holds` on a terminal state are refused at their paths.
- The castle mirror: gate-1's `posted` and `alert` hold `gate-wall-door` at `{q: -3, r: 7}` and `{q: -4, r: 7}`.

## Decided

- The post code lives in `post.go`. The plan named `hold.go`, which is the Hold verb's file.
- `memberHolds` takes the caller's participation read, since every reader already holds one. A failed read holds, as a failed canvas read blocks.
- The step and the masquerade: a crossing the mover's own atlas draws as wall is answered by the masquerade, never by the hold, so a stop cannot name a holder behind a secret.
- `ObservedHolds` reads the stored view, since the hold is not delivered. Allies are left out by believed stance, as `ObservedPassages` does.
- Castle routes about something else open with `castleHost.gateUnheld`. `postedLike` stands gate-2 on the post for the group rule.

## Found

- The hold bars the opening both ways. A hero coming from the kitchen out through the gate to the captain is barred by gate-1, so those routes now open the gate unheld. Whether a post holds an opening inbound only is a design question for the keep's use case.
- A shut door hides a holder on its far side, so the stop says "someone bars the way". The castle's guard stands on the party's side, so his name is told.
- The voice that draws gate-1 is heard in the round it also walks him off, so no sound leaves him alert on his post. The castle's alert case stands him alert in a stored record.
- The session's launch copies `BlockingStates` from the placement by hand (`session/launch.go:237`). T12 must copy `Posts` beside it, or the api castle holds nothing. The plan's T12 does not list it.
- `Atlas.Doorways` gained a `Crossing` field with no JSON tag, so every compiled golden's edge doorways carry `"Crossing": false`.

## Mistaken

- `TestTheLeafIsNotADoorwayAndStillHasAState` pinned that a footprint door is never a doorway. H6 reverses it: its doorways are its crossings, each marked.

— cross-team agent, on behalf of KirkDiggler
