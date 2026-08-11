# dnd5e encounter composition Implementation Plan (the HOW), wave 1: free-roam

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship `rulebooks/dnd5e/encounter` wave 1 exactly as pinned by `design.md`: the free-roam ambient encounter — Setup/Join/Exit/Move/Pump/End, declared endings, per-member carry-forward, one aggregate persistence pair.

**Architecture:** One `Encounter` aggregate composing published `play/clock` + `play/intel` + `play/record` + `tools/spatial` (managed seam) behind composition laws C1–C8. Courier cycle (surveil → decide → execute → record → evaluate endings) as a private helper every mutating verb shares.

**Tech Stack:** Go 1.24, testify, repo `make` CI, `compat.yml` gorelease gate. Deps pinned at published tags: `play/clock v0.1.0`, `play/intel v0.1.0`, `play/record v0.1.0`, `tools/spatial v0.8.0` (or current latest at execution — check tags), `core` latest, `events` ONLY as spatial-loading plumbing (law C4).

**Normative source:** `docs/ideas/encounter/design.md` — on any disagreement, STOP and reconcile the design (with Kirk sign-off), never code around it.

**Docs-PR flow:** this triplet's PR stays OPEN through implementation as the review surface; amendments land on its branch; merges as ratification alongside the module PR (family precedent).

**Working setup** (module isolation: only `rulebooks/dnd5e/encounter/` plus the one `compat.yml` edit in Task 7):

```bash
git -C /home/kirk/game-dev/rpg-toolkit fetch origin main
git -C /home/kirk/game-dev/rpg-toolkit worktree add /home/kirk/game-dev/.worktrees/toolkit-dnd5e-encounter -b feat/dnd5e-encounter origin/main
```

**Standing dispatch-brief standards (non-negotiable, proven over four modules):** worktree-discipline header (ONE worktree, verify `pwd`, absolute `git -C`); no unplanned scope; gate = ZERO issues with golangci-lint checked by EXIT CODE (lll fires at 120 chars); `nolint` FORBIDDEN (shared test consts for repeated literals); license header (2 lines) on every Go file; black-box test package; **mutation-proof pins from Task 1** — every aliasing/copy/isolation pin is shown to FAIL under the exact breakage it guards, evidence in the task report; commit per task with the `Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>` trailer, NO `--no-verify`, do NOT push until the final task. Review tiers: cheap implementers, combined sonnet review per task, **high-rigor (Opus) review reserved for Task 6 (persistence)**.

---

### Task 1: Scaffold — `go.mod`, `doc.go`, `errors.go`

Create `rulebooks/dnd5e/encounter/`: `go mod init github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter`; `go get` the pinned deps above; `go 1.24`, no `toolchain` line. `doc.go`: package comment stating the composition's one-liner (the ambient free-roam encounter — courier between clock/intel/record/spatial; an encounter is a composition with an outcome) + design-contract path + laws C1–C8 summary. `errors.go`: sentinels per design (`ErrNilInput`, `ErrNoMember`, `ErrNotMember`, `ErrNoEnding`, `ErrClosed`, `ErrNoField`, `ErrBadPlacement`, `ErrInvalidData`), doc-commented, `TestSentinelsAreDistinct` black-box. Full gate; commit `feat(dnd5e/encounter): module scaffold — sentinel vocabulary`.

---

### Task 2: Setup — field, members, first light

**Files:** `encounter.go`, `field.go`, `encounter_test.go`.

TDD the aggregate's birth:
- `SetupInput{Field FieldInput{Rooms, Connections}, Members []MemberInput{ID, Kind, Room, Position, Decider}, Endings []EndingInput{Key, Trigger}}` → `NewEncounter` builds spatial rooms + orchestrator through the **managed seam** (`PlaceEntityInput` etc. — see the refreshed spatial usage in the repo CLAUDE.md; ConnectToEventBus only where loading demands it, law C4), places every member, runs the initial surveil cycle (each sighted member's percept lands in intel — LoS via `IsLineOfSightBlocked`), appends the opening record beat.
- Validation order (family style, first failure wins, R5): nil → no field/rooms → no endings (`ErrNoEnding` — an encounter that cannot end) → empty/duplicate member IDs → bad placements (`ErrBadPlacement` wrapping spatial's rejection).
- Percept construction is the composition's own helper: for observer O, the percept = every OTHER member placed in O's room with unblocked LoS, subject = the observed member's ID, payload = a compact position encoding (document the payload shape — it is composition-owned vocabulary).
- Tests: two members see each other at Setup (both `View`s hold Current holdings); a pillar boundary blocks sight (no holding); validation order; R5 from failed Setup (no partial aggregate). Mutation-proof the LoS gate (remove the blocked-check → the pillar test must fail).
- Minimal `View`/`Members`/`Status` so tests compile (hardened Task 5).

Commit `feat(dnd5e/encounter): Setup — the field lights up`.

---

### Task 3: Move — the courier cycle for players

**Files:** modify `encounter.go`; tests.

- `Move(MoveInput{Member, To})`: managed spatial move (same-room wave 1) → **surveil cycle for affected observers** (the mover's own percept from the new position; every member whose sight of the mover changed) → record beat ("X moved") → `ReachedPosition` ending evaluation → `MoveOutput{Spatial delta, IntelDeltas per observer, Seqs, Outcome?}`.
- The courier cycle lands here as the shared private helper (`refreshSight(observers…)`) that Setup/Join/Pump reuse.
- Tests: mover's percept updates; observer watching the mover cross behind a pillar → observer's holding FADES to ghost at last-seen (the ghost-goblin, player-side); mover reaching the declared stairs position → Outcome in the Output, encounter closed, `ErrClosed` on further mutation; `ErrNotMember`; R5 on invalid destination. Mutation-proof the fade (skip the absent-subject surveil contract → ghost test fails).

Commit `feat(dnd5e/encounter): Move — the courier cycle, the ghost forms`.

---

### Task 4: Pump — the world acts back

**Files:** `decider.go`, modify `encounter.go`; tests.

- `Decider` interface + `Intent` (MoveTo/Hold) per design. Fixture wanderer in tests (deterministic patrol route — NO randomness, C8).
- `Pump(PumpInput{})`: `clock.Tick` advance → per monster in deterministic member order: `Decide(HeldBy(monster))` → execute intent through the SAME managed-move path as players → shared courier cycle → record beats → ending evaluation. `PumpOutput` carries the tick delta + everything the monsters' moves produced.
- Tests: goblin patrols on pumps; the goblin's OWN intel updates as it moves (symmetric — it sees the players back); decider isolation pin (a spy decider records exactly what it was shown: its own holdings, nothing else — mutation-proof by handing it another member's view → pin fails); no-monsters Pump still ticks; closed → `ErrClosed`.

Commit `feat(dnd5e/encounter): Pump — monsters move on their own intel`.

---

### Task 5: Membership + endings — Join, Exit, End, queries hardened

**Files:** modify `encounter.go`; tests.

- `Join` (place + first light + beat + deltas), `Exit` (remove from field, return `MemberOutcome` + the member's holdings as carry-forward; holdings REMAIN in the aggregate archive), `End` (external declared key only → Outcome; undeclared → `ErrNoEnding`), membership-empties → auto-close with reserved ending `abandoned` (document the reserved key; Setup rejects declaring it).
- Queries hardened: `View` copy-out (mutation-proof), `Story` = record `SliceFor` pass-through with audience, `Status` open/closed+Outcome, `Members` stable order.
- Tests: late joiner is seen by (and sees) incumbents; Exit carry-forward matches the member's final state; exactly-one-Outcome (End after close → `ErrClosed`); the two-kinds-of-leave law as executable doc (Exit ≠ pause: a comment-annotated test that snapshots, "pauses," reloads, and continues — foreshadows Task 6).

Commit `feat(dnd5e/encounter): membership flows, endings close — members exit, encounters close`.

---

### Task 6: Persistence — `ToData` / `LoadEncounter` *(high-rigor review: Opus tier)*

**Files:** `data.go`; tests.

- `EncounterData` per design: leaves' Data embedded VERBATIM (C3); composition-owned `FieldData` (rooms via `spatial.RoomData` + own `ConnectionData`); `MemberData`; `EndingData`; `OutcomeData?`. Deciders NOT persisted — re-attached at load (reconcile the load signature per the design note: behavior re-attaches, state never contains it).
- `LoadEncounter`: validate composition-level R9 list (design: Persistence) FIRST, delegate leaf validation to leaf loaders (wrap their `ErrInvalidData`), rebuild orchestrator + rooms through spatial loading (the C4 bus-plumbing quarantine — internal bus, nothing subscribes), re-place members, behavior-identical continuation.
- Golden exact-string pins: one open shape, one closed shape. `EncounterData{}` rejected.
- **MUTATION-PROOF checklist (report evidence per mutation):** ToData aliasing (every embedded slice/map family — including inside the embedded leaf Data); wire-tag renames; stowaway fields; leaf-Data substitution (swap the intel blob of observer A into observer B → a pin must object via behavior differences); orchestrator rebuild broken (member placed in the wrong room after load); deciders-reattachment law (load without a monster's decider → documented behavior, test-pinned).
- The `all states round-trip` walk: snapshot at post-Setup, mid-fade (ghost present), post-Exit, closed — behavior-identical continuation asserted for each.

Commit `feat(dnd5e/encounter): persistence — one aggregate at the seam, mutation-proven`.

---

### Task 7: AC1 — the tomb watch + compat gate

**Files:** `tombwatch_test.go`; modify `.github/workflows/compat.yml`.

- Design AC1 verbatim as one plain-function narrative test with beat-labeled failure messages: crypt room with pillar boundaries → two players + goblin (fixture patrol) → goblin slips behind the pillar (A's ghost) → A moves + Pumps (goblin steps; percepts refresh BOTH ways) → mid-scene `ToData`/`LoadEncounter` (the pause law: the suspended scene continues identically) → A reaches the stairs → ending fires → Outcome carries both players → the closed encounter still answers `View`/`Story` (archive law) → the `Story` transcript asserted as the scene's own narration. Expected to PASS; failures are regressions — report, never bend.
- `compat.yml`: add `rulebooks/dnd5e/encounter/**` to paths + `gorelease-dnd5e-encounter` job cloned from the play jobs (pinned gorelease; tag pattern `rulebooks/dnd5e/encounter/v*`). Explicit adds.

Commit `test(dnd5e/encounter): AC1 tomb watch; ci: compat gate`.

---

### Task 8: Full gate + PR

- `go mod tidy && go test ./... -count=1 && golangci-lint run ./... && go vet ./... && gofmt -l .` — lint by EXIT CODE; toolkit#904 version-skew note stands (CI pins golangci-lint v2.3.1; prefer `make([]T, 0, n)` for append-only results).
- Push `feat/dnd5e-encounter`; PR ready-for-review: `feat(rulebooks/dnd5e/encounter): the free-roam encounter — a composition with an outcome`, body citing this design, AC evidence, laws C1–C8 held, review trail, signature footnote + attribution. Drive CI green.

---

## Execution notes

- Task order: field-first (Setup) so every later verb has a world to
  act in; the courier cycle lands with Move and is REUSED (not
  reimplemented) by Pump/Join.
- The percept payload encoding decided in Task 2 is composition-owned
  vocabulary — document it in doc.go; intel never interprets it.
- Deviation forced by implementation → STOP, design first (the
  ownership-transfer precedent: amend, don't code around).

## Execution addenda (logged during subagent-driven execution)

- **With Task 2**: review found two CI-breakers the implementer's gate
  missed (stale go.mod marking direct deps indirect; gofmt drift), a
  dead sentinel (`ErrBadPlacement` declared and distinctness-tested but
  never wrapped — fixed with multi-`%w` at all three placement sites,
  `errors.Is` pinned), and two unpinned behaviors: the complete-percept
  contract (no observer ever saw 2+ members — a break-after-first
  mutant survived; pinned with a three-member mutual-visibility test,
  mutation-proven) and the opening beat (deleting the Append survived —
  resolved by a **plan deviation, director-approved: `Story` pulled
  forward from Task 5** as the beat's observation surface, since a pin
  needs designed surface and leaving a known-unpinned behavior standing
  violates the family standard). Also landed: exported `SightPayload`
  vocabulary type (replacing an anonymous struct Tasks 3/4 would have
  had to reverse-engineer), `Status` copy-out, boundary-blocks-LoS
  test, test-const hygiene. The managed-seam regression pin was
  deliberately deferred to Task 3, where movement gives it a behavioral
  surface instead of new API. Reviewer confirmed placements correctly
  ride the managed orchestrator seam (no #909-style bypass).
- **With Task 3**: review found the commit's tagline test could not
  fail — `TestMoveGhostForms` asserted `len >= 0` and its pillar sat
  five rows off the sightline, so the ghost never formed; rewritten
  with occluding geometry and the true ghost assertion (bob's holding
  of alice pinned at her PRE-move position — he never saw her arrive).
  Move's record beat was unpinned (Append deletion survived) — pinned
  via Story with Seq matched to the Output. The deferred managed-seam
  pin turned out UNFALSIFIABLE for same-room moves (spatial's managed
  MoveEntity is observationally identical to a raw room call there) —
  per the mutation-proof law the test was renamed to what it honestly
  pins (sequential-move consistency); seam enforcement stays convention
  (C2) until cross-room Transition makes it falsifiable. Also landed:
  closed-before-not-member combo pin, populated-state spatial-rejection
  atomicity pin, and a gofmt fix on the committed file (the
  implementer's gate had been read through a pipe that hid it — gates
  now check `gofmt -l` output and lint exit codes explicitly).
  Process note: two director-run mutants initially "passed" because
  they broke the build — a build-broken mutant proves nothing; the
  compiling-mutant rule is now explicit in every brief.
- **With Task 4 (a live R5 bug)**: review proved the clock advanced
  BEFORE the decider loop — a decider error permanently ate a tick
  (repro'd live; the "clock NOT advanced" test never read the clock) —
  and the same shape implied partial mutation with multiple monsters.
  Restructured to decide-then-execute: phase 1 consults every decider
  (any error aborts with zero state touched), phase 2 advances and
  executes. Pinning the partial-abort took three honest attempts at an
  observable: Views can't see it (no refresh ran), occupancy can't
  (members don't block movement) — the next pump's From-position can.
  The redundant decider-view copy was REMOVED (intel.HeldBy's
  documented copy-out is the protection; a vandal-decider test pins the
  composed guarantee); endings became an ordered slice (deterministic
  first-declared-wins — a latent C8 hazard on map iteration).
- **With Task 5**: review killed 6 of 8 mutants; the two survivors were
  an External-only guard never reached (the test used an undeclared key,
  dying at the earlier branch) and Exit's RemoveEntity omission —
  invisible to every query but permanently leaking the ID in the room
  registry, which would have locked out a RETURNING player (the sequel
  model's premise). Pinned via exit-then-rejoin-same-ID. Also pinned:
  the exited beat, and exited-monster deciders never consulted again.
- **With Task 6 (persistence; implementer died mid-stream twice, the
  director completed directly; design amendment: FieldData persists
  construction inputs, construct-path reload — the C4 events wart never
  entered the module)**: the director's own seven-mutant checklist
  caught two theatrical pins in the drafted tests (tick-continuation
  snapshotted at tick 0; no-surveil-on-load geometry agreed with the
  belief — rewritten with a DIVERGENT belief: a ghost in clear LoS,
  legal under C2, that a re-surveilling load would resurrect). The
  Opus review then found SIX majors including in the director's work:
  ToData was nondeterministic via map iteration (the round-trip suite
  was ~16% flaky ON THE CLEAN TREE); a reachable SIGSEGV at the trust
  boundary (reached_position ending without position); the rejection
  tests were theatrical — 7 of 8 checks individually deletable because
  every fixture was invalid in MULTIPLE ways and the last-run check
  absorbed all deletions (fixed: a table where each case starts VALID,
  breaks exactly one thing, and asserts the discriminating fragment);
  NewEncounter aliased the caller's SetupInput (post-construction edits
  corrupted snapshots into unsavable states); three forgery classes
  loaded; construct-path rejections escaped ErrInvalidData. All fixed,
  all re-proven with compiling mutants; a rich golden was added because
  eleven tag renames had survived behind omitempty in the two small
  goldens. Standing lesson, now family law shape: **rejection fixtures
  must be valid except for exactly one defect, and every golden set
  must exercise every omitempty field at least once.**
- **With Task 7**: the drafted tomb watch was nine disconnected
  vignettes — nine separate encounters, one per subtest — where AC1's
  entire point is state FLOWING through one story (the mid-scene ghost
  surviving the reload; the transcript accumulating; the outcome
  seeding the sequel). The director rewrote it as one continuous scene
  with beat-labeled assertions ending in the full ten-beat transcript
  pin and the sequel-seed epilogue; it passed end-to-end on the first
  run — the model held. compat.yml gained its fifth gorelease job.
- **With Task 8**: 172 tests, full gate (lint by exit code, gofmt
  verified), PR #921 opened ready-for-review.
