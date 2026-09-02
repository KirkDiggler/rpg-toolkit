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

- [ ] Read `rulebooks/dnd5e/encounter/doc.go`, `field.go`, `encounter.go`,
  `step.go`, `trigger.go`, `clocks.go`, and the current `MemberData` load path.
- [ ] Confirm the implementation base: `origin/main` at encounter v0.37.0 or
  the v0.38.0 location/intel-delta branch. If v0.38.0 is used, build NPC sight
  testimony through `LocationKnowledge`, `EncodeLocationPayload`, and
  `IntelDelta`.
- [ ] Determine whether the current canvas treats all placed members as
  movement-blocking and whether `MovementPolicyPassable` can be represented
  without changing `tools/spatial`.
- [ ] Implement the settled rule: every world NPC is a valid location-intel
  subject, but its location is not known until authored/loaded intel or actual
  perception creates that knowledge.
- [ ] Decide the exact public name and default for the per-NPC observation
  policy: subject-only versus observer.
- [ ] Confirm observer-capable NPCs can hold intel without requiring any MVP
  behavior to consume it.
- [ ] Define the first vendor profile as generic world NPC data: known at start
  through authored/loaded intel, vendor capability, non-combatant, non-hostile
  to players and monsters.
- [ ] Draw the explicit boundary with #1275: #1280 owns world placement,
  interaction descriptor, and policies; #1275 owns stock, quote, buy, and item
  transfer.
- [ ] Confirm the package split: toolkit-level `npc` for reusable NPC
  data, deferred `npc/npcs` only if a built-in/common profile registry becomes
  necessary, `rulebooks/dnd5e/vendors` or the nearest existing D&D content
  package for actual vendor types, `rulebooks/dnd5e/encounter` for placed D&D
  runtime behavior, and
  `rulebooks/dnd5e/session` for the host seam.
- [ ] Record any design amendment needed before implementation.
- [ ] **Recorded (2026-09-01, see `design.md` amendment):** disposition is
  modeled as `world/graph` `Relation` edges (`HostileTo`/`AlliedWith`, proven
  in `examples/world/scenarios/banditcamp/camp.go`), folded per-question the
  same way `rulebooks/dnd5e/encounter/conceal.go`'s `encounterWorld` already
  folds concealment — not a widened `NPCDispositionPolicy` enum.
- [ ] **Correction (2026-09-01):** `KindWorld` needs no change to
  `sidesInContactOrder`/`classify` to be neutral — that switch has no
  `default` case, so a `KindWorld` member already falls into neither the
  `players` nor `monsters` slice and never enters `engaged`. Confirm this
  with a test in Task 4; do not write a `case KindWorld:` that "excludes" it,
  since it is already excluded by construction.
- [ ] The graph-relation mechanism in the amendment above is **not required
  to ship this idea's MVP.** It only matters once something wants a
  `KindWorld` NPC that isn't neutral, or a `KindMonster`/`KindPlayer` that
  isn't unconditionally opposing — both of which are this doc's own listed
  non-goals (hostile NPCs, attackable neutral NPCs, faction behavior).
  Building the graph now, before that consumer exists, is scope beyond what
  Task 3/4 need. Keep the amendment as forward-context for whichever later
  idea adds graded disposition; do not gate this plan's Task 3 on it.

Gate: a short note in the implementation PR explaining the chosen placement
shape, package ownership, sight-subject path, and observation-policy
default.

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

- [ ] Add `KindWorld` for placed world members, with NPC content identified by
  `npc` refs such as `dnd5e:npcs:merchant`.
- [ ] Consume or mirror the `npc` definition/data needed to place a world
  NPC instance.
- [ ] Add interaction capability and combat-policy vocabulary at the encounter
  boundary as needed.
- [ ] Add `NPCDispositionPolicy` as an authoring-default field only
  (`neutral` is the only shipped value, per Task 2). It does not drive
  runtime behavior in this MVP — `KindWorld`'s exclusion from combat is
  structural (see Task 1's correction), not disposition-driven. Do not build
  the `world/graph` relation mechanism from the amendment as part of this
  task; it has no consumer yet.
- [ ] Extend member construction/input validation for world NPC facts.
- [ ] Forbid world NPC deciders, actions, targeting, and non-non-combatant
  policy in the MVP.
- [ ] Require subject-only NPCs to omit sight reach, or document and test why
  the implementation keeps an inert value.
- [ ] Allow observer-capable NPCs to carry sight reach and receive their own
  intel refreshes.
- [ ] Ensure setup placement accepts valid world NPCs and rejects invalid cells,
  duplicate IDs, movement-blocking overlaps, and malformed NPC policy.

Tests:

- [ ] setup accepts one player plus one world NPC on valid floor;
- [ ] player sight of a world NPC produces location testimony about that NPC;
- [ ] a player with no authored/loaded intel and no sightline does not learn
  the NPC's location;
- [ ] an authored/loaded vendor-profile NPC can be known to the party at
  encounter start;
- [ ] v0.38.0-base implementation uses tagged `LocationKnowledge` for NPC
  sightings;
- [ ] setup rejects a world NPC outside the authored floor;
- [ ] setup rejects a world NPC on blocking terrain/prop;
- [ ] setup rejects a world NPC overlapping another blocking entity;
- [ ] setup rejects a world NPC with a decider or actions;
- [ ] setup rejects malformed observation policy;
- [ ] setup rejects malformed disposition policy;
- [ ] setup does not form a fight when player and world NPC see each other.

## Task 4 - Combat Exclusion in Encounter

- [ ] Confirm (do not implement) that `sidesInContactOrder`'s switch already
  excludes `KindWorld` from both sides — no `default` case exists, so this is
  free. Add the regression test; do not add a graph/relation lookup here for
  this MVP (see Task 1/3's correction).
- [ ] Ensure `Pump` skips world NPCs.
- [ ] Ensure fight formation and straggler joining exclude world NPCs.
- [ ] Ensure `ClockOf` and turn reads never put world NPCs on a turn clock.
- [ ] Ensure NPC sight deltas, including first contact, are not treated as
  player-monster combat contact.
- [ ] Add tests that would fail if a world NPC entered initiative or received a
  driven turn.

Tests:

- [ ] first light with player + monster + world NPC forms only the
  player/monster fight;
- [ ] world NPC never appears in bubble order;
- [ ] `Pump` does not consult any world NPC behavior hook;
- [ ] observer-capable NPC first-contacting a player does not form a fight;
- [ ] subject-only NPC does not receive an intel delta;
- [ ] observer-capable NPC may receive an intel delta, but no MVP behavior
  consumes it;
- [ ] vendor-profile NPC is ignored as a target by both player and monster
  combat logic;
- [ ] neutral/non-hostile disposition does not make the NPC an ally or enemy for
  either side;
- [ ] a monster and player separated only by a world NPC still obey normal line
  of sight and combat rules according to the final blocking rule;
- [ ] an encounter with a `KindWorld` NPC produces byte-identical trigger
  behavior, for every other member, to one with no `KindWorld` NPC at all.

## Task 5 - Interaction Descriptor and Verb

- [ ] Add encounter-owned `InteractionDescriptor`.
- [ ] Ensure the descriptor carries enough stable identity/capability data for
  #1275 to route vendor inventory without reading encounter internals.
- [ ] Add `Interact` or split `Interaction` read plus `Interact` write if the
  existing event/save shape calls for it.
- [ ] Default range to one cell unless a caller supplies a positive range.
- [ ] Require actor to be a player member for MVP.
- [ ] Require target to be a `KindWorld` NPC member.
- [ ] Require adjacency plus current visibility; do not require an unobstructed
  path or movement-reachable cell beyond the visibility check.
- [ ] Append a story beat if `Interact` is a mutating verb.
- [ ] Return descriptor with target ID, display name, and copied capabilities.

Tests:

- [ ] adjacent player can interact and receives ID/name/capabilities;
- [ ] adjacent but not visible target is refused;
- [ ] visible but non-adjacent target is refused;
- [ ] adjacent across a counter/half-wall-style setup works when sight is not
  blocked;
- [ ] distant player is refused;
- [ ] non-player actor is refused;
- [ ] monster target is refused;
- [ ] descriptor capability slice is copy-out;
- [ ] interaction with a closed encounter returns `ErrClosed`.

## Task 6 - Persistence

- [ ] Extend `MemberData` or add a nested world-NPC data block with explicit
  `ref`, `movement_policy`, `combat_policy`, `observation_policy`,
  `disposition_policy`, and capabilities.
- [ ] Keep old player/monster blobs loadable.
- [ ] Require explicit NPC fields when `kind == "world"`.
- [ ] Reject unknown combat policies and malformed persisted NPCs with
  `ErrInvalidData`.
- [ ] Reject unknown observation policies with `ErrInvalidData`.
- [ ] Reject unknown disposition policies with `ErrInvalidData`.
- [ ] Preserve persisted intel about world NPC subjects.
- [ ] Preserve instance state separately from reusable `npc` profile refs.
- [ ] Ensure `ToData` output is deterministic.

Tests:

- [ ] open encounter with world NPC round-trips byte-identically;
- [ ] loaded world NPC can still be interacted with;
- [ ] loaded player intel can still name a world NPC's known or unknown
  location;
- [ ] loaded world NPC still blocks movement;
- [ ] missing `movement_policy` on a world NPC blob rejects by name;
- [ ] missing or unknown `combat_policy` rejects by name;
- [ ] missing or unknown `observation_policy` rejects by name;
- [ ] missing or unknown `disposition_policy` rejects by name;
- [ ] profile reload does not overwrite per-instance world state;
- [ ] mutating the returned data/capability slices does not mutate the
  encounter.

## Task 7 - Session Seam

- [ ] Add session-owned world NPC input/output types.
- [ ] Add a session interaction verb that loads, delegates, saves if needed,
  publishes, and returns a host-shaped descriptor.
- [ ] Add the minimum host-shaped input needed to seed a vendor-profile NPC's
  known starting location, if that authored/loaded intel enters through
  `session`.
- [ ] Update start/world authoring input if world NPCs can be authored at
  session start in this issue.
- [ ] Leave vendor stock, quotes, purchases, and character-inventory mutation to
  #1275; only carry the vendor capability and world-NPC identity here.
- [ ] Do not reuse `SpawnOutput.NPC` or `SessionData.NPCs` for world NPCs.

Tests:

- [ ] session can start or load an encounter containing a world NPC;
- [ ] session interaction returns descriptor and save/delivery reports;
- [ ] session attack candidate projection excludes world NPCs;
- [ ] session `Attack` rejects a world NPC target before resolution;
- [ ] existing spawned monster behavior still stores monster sheets in
  `SessionData.NPCs`.

## Task 8 - Acceptance Scene and Verification

- [ ] Add one narrative test with a player, a monster, and a vendor-profile
  world NPC on the same map: the vendor's location is known at start, the player
  stands adjacent and visible, interacts with the vendor and sees the vendor
  capability, the player walks into monster contact, the fight forms without the
  vendor, the monster turn driver ignores the vendor, monster targeting ignores
  the vendor, and the vendor remains queryable afterward. No stock, quote,
  purchase, or inventory mutation appears in this test.
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
