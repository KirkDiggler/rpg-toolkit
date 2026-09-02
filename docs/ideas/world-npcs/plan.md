# World NPCs - Plan

**Executes:** `design.md`.
**Issue:** [rpg-toolkit#1280](https://github.com/KirkDiggler/rpg-toolkit/issues/1280)
**Related:** [rpg-toolkit#1275](https://github.com/KirkDiggler/rpg-toolkit/issues/1275)
**Modules:** `npc`, `rulebooks/dnd5e/encounter`, `rulebooks/dnd5e/session`

Follow the repository's normal idea lifecycle: approve `design.md` first, then
keep this plan current while implementation proceeds. If coding exposes a
different ownership boundary, stop and amend the design before continuing.

Family standards apply: black-box package tests where practical, one-defect
rejection fixtures, deterministic ordering, copy-out pins, mutation-proof tests
for every claimed invariant that can be falsified, and owning-module test/lint
evidence.

## Task 1 - Census and Placement Spike

- [x] Read `rulebooks/dnd5e/encounter/doc.go`, `field.go`, `encounter.go`,
  `step.go`, `trigger.go`, `clocks.go`, and the current `MemberData` load path.
- [x] Confirmed the implementation base: `origin/main`, encounter v0.43.0
  (well past the v0.37/v0.38 fork this bullet used to hedge on — moot).
- [x] **Corrected (2026-09-02, see `design.md`'s same-dated amendment):**
  traced how a monster's content actually flows and it never crosses into
  `encounter` at all — `session.Spawn` resolves the ref into a sheet, stores
  it in `SessionData.NPCs`, and calls `encounter.Join` with only bare facts
  (Name, Position, Speed, Actions, Targeting). A world NPC is placed the
  same way: `encounter` carries no `npc` import, no ref, no capabilities, no
  policy field anywhere in its types. That content lives in a new
  `session`-owned store, keyed by member ID, parallel to `SessionData.NPCs`.
  This retires most of the "living world" framing this task originally
  carried (movement-policy blocking, observation policy, location-learned
  semantics as a new mechanism) — see the struck items below.
- [x] ~~Determine whether the current canvas treats all placed members as
  movement-blocking and whether `MovementPolicyPassable` can be represented~~
  — checked directly: `memberEntity.BlocksMovement()` returns `false` for
  every member today, monsters included. Not part of this slice; a
  `KindWorld` member gets the same answer a monster already does.
- [x] ~~Implement the settled rule: every world NPC is a valid location-intel
  subject...~~ — not a new rule to implement. `e.clock.Join(...)` and
  `refreshSight`/`rebuildPercepts` already run unconditionally for every
  member kind (no kind filter) in both `NewEncounter` and `Join` — a
  `KindWorld` member is on the world clock and gets sight-refreshed exactly
  like anyone else, automatically, which is what makes it discoverable and
  later interactable. Nothing to build here.
- [x] ~~Decide the exact public name and default for the per-NPC observation
  policy: subject-only versus observer~~ — no such policy exists in this
  slice. A `KindWorld` member with nonzero `SightFeet` already gets its own
  percept through the existing mechanism, same as a monster.
- [x] Confirmed the package split still holds: toolkit-level `npc` (shipped,
  `npc/v0.1.0`) for reusable NPC data; `rulebooks/dnd5e/encounter` carries
  only `KindWorld` and bare member facts; `rulebooks/dnd5e/session` owns the
  actual `npc.NPC` content, keyed by member ID.
- [ ] **Settled (2026-09-02, fourth pass):** add `npcs.NewMerchant(config
  *MerchantConfig) (*Vendor, error)` to `rulebooks/dnd5e/npcs` — real,
  shipped toolkit code, not a test-only fixture, but NOT the
  `NewBlacksmith`-style fixed archetype `docs/ideas/dnd5e-npcs/design.md`
  already ruled out either. The distinction: the general function stays
  parameterized — a non-nil `config` is validated normally (same as
  `npc.New` today: a non-nil config with a missing `Ref` still errors, no
  silent defaulting *within* an explicit config) — and `config == nil`
  means "give me the toolkit's own default," an explicit signal a caller
  cannot produce by accident (unlike a zero-value struct, which could be a
  forgotten field). The default values themselves live inside this
  function, in the toolkit, because there is no host-side NPC builder yet
  to own them instead — when one exists, `NewMerchant(config)` with a real
  config becomes the normal path and the `nil` branch stays exactly what it
  always was: a demo convenience, not a stand-in for a repository or
  catalog.
- [ ] Draw the explicit boundary with #1275: #1280 owns world placement and
  the interaction descriptor; #1275 owns stock, quote, buy, and item
  transfer.
- [x] Disposition: unchanged from the 2026-09-01 correction — not required
  to ship this MVP, graph-relation mechanism is forward context only, not
  built here.

Gate: a short note in the implementation PR explaining the chosen placement
shape and package ownership — met by this task's own corrections above.

## Task 2 - Toolkit NPC Package

- [ ] Add top-level `npc` for reusable common-NPC definition/data.
- [ ] Name the primary generic struct `NPC`, not `Definition`.
- [ ] Define the common NPC facts known now: ref, display name, interaction
  capabilities, combat policy, observation policy, disposition policy, and
  movement policy.
- [ ] Use `*core.Ref` for NPC refs.
- [ ] Keep the package rule-agnostic: no D&D stats, no D&D combat rules, no D&D
  encounter/session imports.
- [ ] Model `npc` as the generic carrier that D&D vendor types can compose with,
  not as the owner of vendor stock or shop behavior.
- [ ] Treat package defaults as authoring convenience only; placed/loadable
  encounter state must carry explicit current values.
- [ ] Ship only the `vendor` built-in interaction capability for now, while
  keeping the capability shape extensible.
- [ ] Keep vendor stock, quote, buy, wallet, and inventory fields out of this
  package.
- [ ] Add `ToData`/load or simple data validation, matching nearby package
  conventions.
- [ ] Defer `npc/npcs` built-in/common profile registry until implementation or
  #1275 proves it is needed.
- [ ] Leave actual D&D vendor type/profile constructors to #1275 under
  `rulebooks/dnd5e/vendors` or the nearest existing D&D content package.

Tests:

- [ ] common NPC data round-trips and deep-copies capabilities;
- [ ] unknown capability strings remain carried as opaque capability data if the
  final type permits extension;
- [ ] NPC refs are `*core.Ref` and survive data round-trip;
- [ ] unknown combat, observation, or disposition policies reject by name;
- [ ] first vendor profile exposes vendor capability, neutral disposition,
  non-combatant combat policy, and `MovementPolicyBlocking`.

## Task 3 - Encounter Model and Validation

**Corrected (2026-09-02):** `encounter` carries no NPC content of any kind.
This task is now small — see `design.md`'s Encounter Model section.

- [ ] Add `KindWorld MemberKind = "world"` (`field.go`) — no other new type.
- [ ] Add the one new validation rule, mirroring the existing player-decider
  check exactly: `KindWorld` + `Decider != nil` → reject with the same
  sentinel the player check uses.
- [ ] Do NOT add `Ref`, `Capabilities`, `CombatPolicy`, `ObservationPolicy`,
  `MovementPolicy`, or `DispositionPolicy` to `MemberInput`, `JoinInput`,
  `Member`, `memberRecord`, or `MemberData`. Do NOT add an `npc` dependency
  to `rulebooks/dnd5e/encounter/go.mod`.
- [ ] Do NOT enforce zero `SpeedFeet`/empty `Actions`/empty `Targeting` for
  `KindWorld` — this package doesn't validate kind-appropriate field
  combinations beyond the one Decider rule above for any other kind either;
  trust the caller, matching house style.
- [ ] Do NOT touch `memberEntity.BlocksMovement()` — it stays `false`
  unconditionally, for every kind, exactly as before this feature.

Tests:

- [ ] setup accepts one player plus one `KindWorld` member on valid floor,
  with the same bare fields (`ID`, `Name`, `Position`) any member needs;
- [ ] `Join` (mid-scene) also accepts a `KindWorld` member — it is the only
  mid-scene placement path this package has, shared with monsters (no
  separate `AddMonster` verb exists);
- [ ] setup and Join both reject a `KindWorld` member carrying a `Decider`;
- [ ] a player or monster is untouched by the new check (no regression on
  the existing player-decider rejection).

## Task 4 - Combat Exclusion in Encounter

- [ ] Confirm (do not implement) that `sidesInContactOrder`'s switch already
  excludes `KindWorld` from both sides — no `default` case exists, so this is
  free. Add the regression test; do not add a graph/relation lookup here for
  this MVP.
- [ ] Confirm (do not implement) `Pump` already skips `KindWorld` — its
  monster loop is filtered to `KindMonster` explicitly.
- [ ] Confirm fight formation and straggler joining exclude `KindWorld` —
  both are fed exclusively from `sidesInContactOrder`, so this follows from
  the first bullet.
- [ ] Add a defense-in-depth guard on `Transfer`: reject moving a `KindWorld`
  member onto `ClockTurn` directly, since `Transfer` is a public verb with no
  other kind check today (unreachable via `classify()`'s own callers, but
  worth guarding since nothing else does).
- [ ] Confirm `ClockOf` for a `KindWorld` member reports the world clock —
  automatic, from `e.clock.Join(...)` running unconditionally for every kind
  (see Task 1). Not something to build.

Tests:

- [ ] first light with player + monster + `KindWorld` member forms only the
  player/monster fight (proven via `Join`/`Step`'s public `Formed` output,
  not by calling `sidesInContactOrder` directly);
- [ ] a monster's `Pump`-driven walk into contact with a `KindWorld` member
  forms no fight;
- [ ] `ClockOf` reports the world clock for a `KindWorld` member both before
  and after a real fight forms between a player and a monster elsewhere on
  the map;
- [ ] `Transfer` to `ClockTurn` refuses a `KindWorld` member by name;
- [ ] an encounter with a `KindWorld` member produces identical trigger
  behavior, for every other member, to one with no `KindWorld` member at all.

## Task 5 - Interaction Verb (encounter) + Descriptor (session)

**Corrected (2026-09-02):** the descriptor is a `session`-owned type, built
from `session`'s own NPC store. `encounter` only confirms identity/adjacency/
visibility — see `design.md`'s Read Shape / Interaction Verb sections.

- [ ] Add `encounter.Interact` (`InteractInput{Actor, Target, Range}` →
  `InteractOutput{Target, Seq}`) — no descriptor, no capabilities, no policy.
- [ ] Default range to one cell unless a caller supplies a positive range.
- [ ] Require actor to be a player member for MVP.
- [ ] Require target to be a `KindWorld` member.
- [ ] Require adjacency plus current visibility; do not require an unobstructed
  path or movement-reachable cell beyond the visibility check.
- [ ] Append a story beat confirming the interaction; no NPC-state mutation.
- [ ] Add `session.WorldNPCDescriptor{TargetID, Ref, DisplayName, Capabilities,
  CombatPolicy}` and a `session.Interact` verb: call `encounter.Interact`,
  look up `InteractOutput.Target` in session's NPC store (Task 7), build the
  descriptor, save if a beat was written, publish, return.

Tests (encounter-level, black-box):

- [ ] adjacent player can `Interact` and gets back the confirmed target ID;
- [ ] adjacent but not visible target is refused;
- [ ] visible but non-adjacent target is refused;
- [ ] distant player is refused;
- [ ] non-player actor is refused;
- [ ] monster target is refused;
- [ ] interaction with a closed encounter returns `ErrClosed`.

Tests (session-level, added alongside Task 7):

- [ ] `session.Interact` returns a `WorldNPCDescriptor` with ref/name/
  capabilities/policy sourced from session's store, for a target `encounter`
  confirmed reachable;
- [ ] descriptor capability slice is copy-out.

## Task 6 - Persistence

**Corrected (2026-09-02):** `EncounterData`/`MemberData` needs nothing new
beyond the `"world"` kind value itself. All NPC-content persistence moves to
Task 7 (session-owned store).

- [ ] Confirm a `KindWorld` blob round-trips through `ToData`/`LoadEncounter`
  using only the fields every member already persists (no new required
  fields, no new `ErrInvalidData` cases beyond the existing decider check
  applied at load).
- [ ] Confirm old player/monster blobs — with no `"world"` kind present at
  all — keep loading unchanged.

Tests:

- [ ] an encounter containing a `KindWorld` member round-trips through
  `ToData`/`LoadEncounter`;
- [ ] a loaded `KindWorld` member can still be `Interact`-ed with;
- [ ] a pre-existing player/monster-only blob loads exactly as it did before
  this feature.

## Task 7 - Session Seam

**Corrected (2026-09-02):** this is where an NPC's actual content lives — see
`design.md`'s Session Seam section.

- [ ] **Filled in (2026-09-02, third pass):** `npc.Data` carries no
  instance/member-ID field (by its own documented design — it is reusable
  content, not a placed record), so `SessionData.NPCs []monster.Data`'s
  shape (lookup by `.ID` on the stored struct itself) does not transfer
  directly. Add a session-owned wrapper:

  ```go
  type PlacedWorldNPC struct {
      MemberID string
      NPC      npc.Data
  }
  ```

  and `SessionData.WorldNPCs []PlacedWorldNPC` — the field name distinct
  from `NPCs`, which already means monster sheets (N1). Wire it through
  `SessionData.ToData()`/load the same way `NPCs` already is.
- [ ] **Corrected (2026-09-02, second pass):** the placement verb does NOT
  mirror `Spawn`'s ref-resolution shape — `instantiate()` resolves a
  monster's ref through `monsters.ByRef`, a real toolkit-shipped catalog,
  and no NPC equivalent exists or is planned (already decided against in
  `docs/ideas/dnd5e-npcs/design.md`). Instead: the verb takes an
  already-built `npc.Data` directly from the caller — closer to how `Join`
  accepts an already-loaded character than how `Spawn` resolves a ref. Name
  it `PlaceNPC`:

  ```go
  type PlaceNPCInput struct {
      Session  string
      Member   string
      Position spatial.Position
      NPC      *npc.Data // caller-resolved; nil-means-default lives one
                          // layer up in npcs.NewMerchant, not here — by the
                          // time this verb is called, the caller has already
                          // decided default or explicit.
  }
  ```

  Reject `NPC == nil` here explicitly (`ErrNoRef` or similar) — this verb
  does not itself interpret nil as "give me defaults"; that would duplicate
  the same decision `npcs.NewMerchant` already makes, in a second place.
  Record the content in `SessionData.WorldNPCs`, then call `place()` →
  `encounter.Join` with `KindWorld` and bare facts (same
  content-recorded-before-placement ordering `Spawn` already documents and
  for the same reason — a sight refresh mid-verb reads back what was just
  written).
- [ ] Add `session.Interact` per Task 5, returning:

  ```go
  type WorldNPCDescriptor struct {
      TargetID     string
      Ref          string // core.Ref.String() — S2, never *core.Ref itself,
                           // matching the existing convention (turndriver.go)
      DisplayName  string
      Capabilities []npc.Capability
      CombatPolicy npc.CombatPolicy
  }
  ```

- [ ] **Filled in (2026-09-02, third pass):** `TestNoInnerTypeCrossesTheBoundary`
  (`boundary_test.go`) statically checks type signatures in `session`'s own
  source, independent of runtime values — it will fail the moment
  `PlaceNPCInput.NPC` or `WorldNPCDescriptor.Capabilities`/`CombatPolicy`
  exist as exported fields, regardless of whether a nil-default or a real
  caller config ever flows through them. Add to `boundary_test.go`'s
  `contractTypes` map (not `persistenceShapes` — the caller constructs this
  content field-by-field, exactly `character.Data`'s reasoning, not
  `monster.Data`'s ref-resolved-by-the-toolkit one):
  - `"npc.Data"`: host/caller constructs this — same promise as
    `character.Data`.
  - `"npc.Capability"`, `"npc.CombatPolicy"`: reachable from
    `WorldNPCDescriptor`'s exported fields, same reasoning.
  (`npc.Data`'s own nested `*core.Ref` field does not need a separate entry
  — the test only parses `session`'s own source files, not into `npc`'s
  package definition.)
- [ ] Leave vendor stock, quotes, purchases, and character-inventory mutation to
  #1275; only carry `npc.NPC` identity/capabilities/policy here.
- [ ] Do not reuse `SpawnOutput.NPC` or `SessionData.NPCs` for world NPCs (N1).

Tests:

- [ ] session can `PlaceNPC` and later start/load an encounter containing it;
- [ ] `PlaceNPC` rejects a nil `NPC` field by name;
- [ ] session interaction returns a `WorldNPCDescriptor` and save/delivery
  reports;
- [ ] session attack-candidate projection excludes `KindWorld` members
  (fixing `buildTargetPreflight` in `offers.go`, which today has no kind gate
  at all — visibility/range only);
- [ ] session `Attack` rejects a `KindWorld` target before resolution;
- [ ] existing spawned-monster behavior is unchanged — sheets still land in
  `SessionData.NPCs`;
- [ ] `TestNoInnerTypeCrossesTheBoundary` passes with the new allowlist
  entries — and would fail without them (the meta-pin pattern
  `TestBoundaryTestCanActuallyFail` already establishes).

## Task 8 - Acceptance Scene and Verification

- [ ] Add one narrative test with a player, a monster, and a vendor-profile
  world NPC on the same map: the player stands adjacent and visible, interacts
  with the vendor through `session.Interact` and sees the vendor capability
  (from session's store), the player walks into monster contact, the fight
  forms without the vendor, the monster turn driver ignores the vendor,
  monster targeting ignores the vendor, and the vendor remains queryable
  afterward. No stock, quote, purchase, or inventory mutation appears in
  this test.
- [ ] Add or update an Example if the final API has a clean public interaction
  story worth pinning.
- [ ] Update package docs/README text only where current behavior changes.

Verification:

- [ ] `go test -race ./...` from `rulebooks/dnd5e/encounter`;
- [ ] `go test -race ./...` from `rulebooks/dnd5e/session`;
- [ ] `golangci-lint run ./...` from each changed module, if available;
- [ ] root `git diff --check`;
- [ ] no committed local `replace` directives or `go.work`.

## Post-Merge

- [ ] Add `implementation.md` with final commit/tag evidence, deviations, and
  observed behavior.
- [ ] Comment on #1280 with the implementation PR and verification evidence.
- [ ] Comment on #1275 with the world-NPC framework version or PR it should
  build against.
- [ ] Open follow-up issues for dialogue, quests, trainers, attackable NPC
  policy, hostile NPC policy, or NPC sheet persistence only after the MVP lands.
