# 2026-11-04 · The guard holds the gate S1c · A post's direction and pass fact

Story [rpg-project#836](https://github.com/KirkDiggler/rpg-project/issues/836), slice [rpg-project#857](https://github.com/KirkDiggler/rpg-project/issues/857), lane [rpg-project#835](https://github.com/KirkDiggler/rpg-project/issues/835) (R6). Plan Task 17 in `ideas/the-guard-holds-the-gate/plan.md`.

## Shipped

- `Post` gains `Against PostDirection` (`both`, `entering`, `leaving`) and `Pass FactID`. An empty direction reads as both ways. `PostData` stores both `omitempty`, so a post with no settings stores what it stored before, and a record without them loads as both ways with no pass.
- A post's side is the region its cells stand in. A directed post is refused at the door (`ErrBadMember`, at `NewEncounter` and `Join`) and at load (`ErrInvalidData`) unless its cells stand in one region and every crossing of its door has exactly one cell in it. An unknown direction word is refused the same way.
- `heldAgainst` keeps a holder only when its post bars this mover this way: the direction matches the side the crossing starts on, then the mover does not know the pass. The direction is asked first, so a crossing it lets through reads no journal. All three route readers (`walkerEdge`, `stepInto`, `doorToOpen`) and the step's `heldOnTheWay` pass the crossing in walking order, so the direction is decided per crossing and never enters the undirected crossing memo.
- A read memoizes each mover's pass answer per fact (`cellQuery.passes`, made with `q.held`). The step memoizes it across its ray.
- `refuseHeldDoor`, and so `DoorHeldFor` and the three door verbs, read the same two knobs, with the actor's cell as the side it acts from.
- `ObservedHolds` now answers `map[MemberID]HeldDoor`, where `HeldDoor{Door, Against, Side}`. `Against` is always one of the three words. `Side` is the door's crossing cells on the post's side, in cell order, a cell two crossings share named once; nil when the post has no one side. The settings come from the holder's live post for that door at read time, never from the testimony. The hold is told in any region, and omitted for a viewer who knows the pass or believes the holder allied.
- dungeonspec: `holds: { door, at, against, pass }`. An unknown `against` word is refused at `.holds.against`. A `pass` naming no fact the file sets is refused at `.holds.pass`.
- The castle mirror: gate-1's `posted` and `alert` hold `against: entering` with `pass: troupe-admitted`. The two `troupe-admitted` transitions to `cowed` are removed.
- `castleHost.gateUnheld` and its five call sites are gone. The routes from the kitchen out through the gate walk out unchallenged. `castleHost.gateAgainst` sets a test post's direction on the gate.

## Decided

- `barsFrom` reads the side from the post's first cell at read time, not a stored region. The door admitted the post only if all its cells share it.
- `HeldDoor.Side` names each side cell once. The castle gate's three crossings meet two gatehouse cells, `(-4, 7)` and `(-3, 7)`. A consumer finds each crossing's side end by membership.
- `ObservedHolds` tells a hold to a viewer on the side a one-way post lets through, as the slice issue and the lead's brief say. The plan's test line ("a viewer on the open side of a one-way post sees no holds_door") predates that and was not followed: a viewer inside the gate may still plan a path out and back in.
- `TestMadKingThePostHoldsTheGatesPassage`'s two-holder case uses a test post set both ways, because its hero comes from the courtyard, the way the castle's post lets through.
- The troupe routes open the gate from the opening's free cell, `(-1, 7)` offset, because gate-1 now stays posted on the other.

## Found

- The castle's gate-1 is often `alert` rather than `posted` after the troupe line: the captain's voice turns him alert. Both hold, so the castle tests assert a holding state (`onHisPost`), not the word `posted`.
- From the party start, the castle host's hero holds no current sighting of gate-1, ten hexes off, even after two steps, so `ObservedHolds` names nothing there. T18's test that a viewer at the party's start sees `HoldsDoor` depends on the session's sight reaching him; it may need a viewer nearer the gate. From the gatehouse the answer is `{gate-wall-door, entering, [(-4, 7), (-3, 7)]}`.
- From the far side of a shut door the holder is unseen, so the door verbs say "someone bars the door", as the step already did.
- The replay test's troupe route stepped onto gate-1's cell, which he now keeps. It paces through the opening's other cell instead, in by the pass and back out.

— cross-team agent, on behalf of KirkDiggler
