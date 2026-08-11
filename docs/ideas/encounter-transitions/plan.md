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

**Task 3 (commits 4236390, f869e81) — COMPLETE.** Snapshot{Room,
Position, Holdings} replaces the bare holdings slice (the wave-1
own-placement gap, resolved); IntentTraverse executes through a helper
shared with the Traverse verb. REAL latent bug found during the
restructure: Pump's planned list stored member VALUE COPIES — harmless
while only same-room moves existed, state corruption once traverses
mutate member.Room — fixed with live-pointer lookups and pinned by
reintroduction-mutant. Design correction ratified: Pump's shipped
failure contract is decider-error-aborts (phase 1) / execution-
failure-skips (phase 2) — the triplet's original "aborts atomically"
claim was wrong about v0.1 and is amended. Sonnet round: APPROVED, two
minors (mid-pump closure coherent-but-unpinned → TestPumpFullTickThen
EvaluateAcrossTraverse, whose falsifiable signal is the second
monster's POST-move outcome position; sibling-rigor gap on the
unknown-connection skip test → phantom-beat mutant now dies).

**Task 4 (commit 736db19) — COMPLETE.** The vault chase: corridor +
vault + "gate", one continuous scene — sight, the slip, the ghost
pinned symmetrically at the true last-seen threshold, mid-chase
save/reload with a fresh decider re-attached, pursuit walk + the
goblin's OWN traverse decision, Current reacquired same-pump, ending
with full-position Outcome, archive sweep grown to Traverse, exact
nine-beat transcript pin (predicted from code before first run;
liveness proven by beat-rename mutant in review).

**Opus deep pass (T3+T4 combined; first attempt stalled mid-stream on
an API error and was replaced) — APPROVED, three minors, all closed in
fefbbcb:** (1) reentrant contract-violating decider could nil-panic
phase 2 → nil-guard + TestPumpSurvivesReentrantSelfExitingDecider
(reject-never-crash upheld); (2) the endings field comment overstated
first-declared-wins — the true, now-PINNED law: decision order
dominates across actions in one pump, declaration order tiebreaks
within one action (loop-nesting mutant flips only the cross-ordering
subtest); (3) chase sweep's Traverse entry made valid-if-open (goblin,
who ends on the vault endpoint, replaces alice). Also proven: percept
simultaneity (refreshSight once, end-of-tick, both rooms final
positions); phase-1 purity on the early-error path (byte-identical
ToData); member.Room-vs-spatial divergence UNREPRESENTABLE (both
derive from the one persisted room field); PumpOutput aliasing
impossible. Observation recorded, pre-existing, wave-1 follow-up:
intel Refreshed delta ORDERING is nondeterministic across runs (4
orderings / 200 runs) but never reaches blobs or transcripts (1 blob,
1 transcript / 200 runs).

**Task 5 (commits fefbbcb, 44371c4) — COMPLETE.** Example_theTraverse
with pinned transcript; workbench gains the ossuary + a traverse
command with room-aware world/belief grids; verify.sh green (259
tests); `gorelease -base v0.1.0` verdict matches the design exactly —
one intentional incompatible change (Decider.Decide signature), all
else additive, suggested version v0.2.0.

**Wave complete: PR #924, eleven commits, 259 subtests.** Awaiting
merge + tag `rulebooks/dnd5e/encounter/v0.2.0` (Platform's dependency
signal on rpg-api#793).

**Copilot round (commit 095e6f5) — COMPLETE.** Both findings verified
real and fixed pre-tag while the wire shape was still free: (1) grid
persisted as the iota INTEGER — brittle against constant reordering
and inconsistent with spatial's own string serialization → RoomData.
Grid is now a string reusing spatial's GridType* constants (empty =
square; goldens byte-stable; unknown strings rejected, mutant-
verified); (2) missing connection endpoints silently unmarshaled to
the LEGAL cell (0,0) — invented topology, and the design's "v0.1
blobs fail v0.2 load" claim was unenforced → endpoints are now
*PositionData with presence required (two one-defect rows, mutant-
verified), making the compat claim true. 261 tests; gorelease verdict
unchanged (v0.2.0, one ratified incompatible change). Copilot replies
posted on both threads.

**Axial round (commit 41412f5, Kirk-directed) — COMPLETE.** Kirk
caught the coordinate-model confusion at its root: T1.5 had wired
spatial's bounded OFFSET HexGrid; the wire and Platform's pathing
speak CUBE. Hex rooms now construct AxialHexGrid — X/Y are axial Q/R,
true cube math for distance/LoS, origin-centered spans with legal
negative coordinates, identity wire projection. Test matrix rewritten
both seams (negative-Q accepted — the flip; ±Width/2 boundary
semantics); golden's hex fixture corrected (its old endpoint was
invalid under centered spans — the correction doubles as a negative-
axial proof); the build-square-always mutant now dies via hex tests
directly, superseding the "only gridless discriminates" finding. 269
tests; gorelease verdict unchanged (v0.2.0).
