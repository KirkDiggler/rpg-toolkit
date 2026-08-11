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
