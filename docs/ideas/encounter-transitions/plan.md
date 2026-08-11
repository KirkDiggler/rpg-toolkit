# Encounter transitions — plan (v0.2.0)

Five tasks, one in-flight PR for the module (house rule). Each task is
dispatched with the standing brief standards from wave 1
(`docs/ideas/encounter/plan.md`), which remain binding:

- Rejection fixtures are valid-except-exactly-one-defect with
  discriminating message fragments asserted.
- Golden JSON exercises every `omitempty` field (rich golden).
- Mutation pins use compiling mutants only (build before judging).
- Scene tests are ONE continuous story.
- Lint is judged by exit code, `gofmt -l` checked explicitly — pipes
  hide failures.
- Deltas returned, never published; no randomness; sorted iteration
  anywhere order is observable.

## Task 1 — connections grow endpoints

`ConnectionInput` gains `FromPosition`/`ToPosition`; Setup + Load
validation per the design's defect list; `FieldData` persists the new
fields; one-defect rejection table rows + fragments; rich golden grows
a connection with both endpoints; `deepCopyRoomInputs`-style aliasing
protection extended to connections; reload pin for connection survival.

## Task 1.5 — rooms choose their grid

`RoomInput.Grid spatial.GridShape` (zero value square) routed into
both grid-construction sites (Setup and load path); persisted in
`FieldData`; rich golden grows a hex room; in-bounds validation
(members, connection endpoints, move targets) refactored to defer to
the grid's `IsValidPosition` — delete the module's own rectangle math.
Tests: hex room constructs, validates a legal hex position, rejects an
illegal one at both seams; square defaults byte-stable (existing
goldens unchanged); reload preserves grid shape.

## Task 2 — the Traverse verb

`TraverseInput`/`TraverseOutput`, `ErrNoConnection` sentinel wired
multi-`%w`; T2 precondition; percept refresh in both rooms; `traversed`
beat; ending evaluation at arrival; `ErrClosed` sweep grows the verb.
Tests: threshold success, off-threshold `ErrBadPlacement`, unknown
connection, both-directions traversal (T1 bidirectionality), arrival
onto an ending cell closes with the declared key, T3 pin (observer at
an endpoint sees nothing in the far room), beat pinned via Story.

## Task 3 — deciders learn where they stand

`Snapshot{Room, Position, Holdings}`; `Decider.Decide(Snapshot)`;
migrate in-repo implementors (tests, workbench); `IntentTraverse`;
Pump phase-2 execution with atomic abort on failed precondition (R5
pin: clock not advanced, no beats, no partial moves — read the clock).
Integration pin: pursuit decider crosses a room boundary chasing a
ghost using only snapshot + holdings + construction-time map.

## Task 4 — the scene

New scene test (sibling to tombwatch, which stays untouched with its
archived transcript): a two-room chase told as one continuous story —
sight, the slip through the opening, the ghost at the threshold, the
pursuit traverse, save/reload mid-chase (decider re-attached, room
survives), the ending in the far room, the archive sweep including
`Traverse`.

## Task 5 — transcript, workbench, gate

Example with pinned `// Output:` showing a traversal beat (new sibling
Example; the wave-1 Example stays byte-identical). Workbench gains the
second room + `traverse` command. `./scripts/verify.sh
rulebooks/dnd5e/encounter` green; `gorelease` gate green; module tag
`rulebooks/dnd5e/encounter/v0.2.0` after merge.

## Review cadence

Per-task combined review (sonnet), Opus persistence/deep review on
Tasks 1 and 3 (the seam and the interface change), director checklist
before each commit. Execution addenda appended here as tasks land.

---

## Execution addenda

**Task 1 (commits ae803ce, 01eb760, 4194d4d) — COMPLETE.** Endpoints +
nine-defect validation both seams + sorted persistence. Sonnet review
found five mutation gaps (To-side bounds/occluder and From-room checks
unpinned at both seams; weak fragment on the pre-existing To-room row;
ErrBadConnection unasserted at Load) — closed with per-mutant kill
evidence. Opus deep pass found the fixtures symmetric two ways (rooms
same-sized → endpoint-vs-wrong-room cross-wiring invisible; reload
endpoints equal → Load-direction transposition invisible): one
deliberately asymmetric fixture (10x4 / 3x9, endpoints {7,1}/{1,7})
killed all four survivors. Also fixed: duplicate-member message
falsely read "empty member id" (pre-existing); orphaned NewEncounter
doc comment. New standard minted: **connection fixtures must be
asymmetric in room dimensions AND endpoints** so cross-wiring and
transposition mutants stay observable. 29 mutants accounted.

**Task 1.5 (commit baec468) — COMPLETE, added mid-wave.** Platform
pushback on rpg-api#793 (hex) answered by making the grid choice
per-room: `RoomInput.Grid spatial.GridShape` (zero value square, byte-
stable goldens), both construction sites routed, rectangle math
deleted in favor of grid-deferred validity, room-ID validation added
(empty/duplicate — the Opus-found hole). Honest-signal finding worth
keeping: spatial's bounded HexGrid validity formula is numerically
IDENTICAL to square's (offset coordinates), so hex bounds tests cannot
prove shape routing — the GRIDLESS inclusive upper bound is the
discriminating signal, and the build-square-always mutant is pinned by
the gridless tests. Review: APPROVED, 14 hostile-blob probes (no
panics; unknown-room checks all precede grid-map indexing).

**Task 2 (commits f7ff0ce, e9f2a95) — COMPLETE.** The Traverse verb:
own prechecks carry the contract (new ErrNoConnection sentinel;
ErrBadPlacement off-threshold), spatial's TransitionEntity+PlaceEntity
execute it (revised from RemoveEntity+PlaceEntity after research:
TransitionEntity applies the same registry-lesson cleanup AND its
indexed-room backstop independently catches wiring bugs the module's
own checks structurally cannot — proven by a FromRoom/ToRoom-swap
mutant failing 9 tests). One global refreshSight covers both rooms
(each observer scopes to their own room) — verified by three
exclusion mutants. Review: APPROVED. R5 analysis: the transition→place
limbo window is unreachable under four invariants (immutable rooms,
non-blocking members, grid-validated endpoints, permanently-passable
door connections), proven by chained-precondition argument AND a
forced-failure probe (consequence if ever opened: member recorded in a
room she is not spatially present in; graceful ErrBadPlacement, no
panic). Debt: an invariant comment at the shared seam (Task 3 item 0);
ending-evaluation loop now duplicated 3-4x file-wide (pre-existing,
refactor candidate); beat-audience narrowing mutant unpinned file-wide
(pre-existing).
