# NPC - Plan

**Executes:** `design.md`.
**Issue:** [rpg-toolkit#1280](https://github.com/KirkDiggler/rpg-toolkit/issues/1280)
**Module:** `npc`

## Task 1 - Package Census

- [x] Inspect top-level package conventions in `items`, `core`, `world`, and
  nearby data packages.
- [x] Confirm whether the repo wants `npc` as one module with `go.mod`, matching
  top-level package/module style.
- [x] Confirm current `*core.Ref` constructors and validation helpers.
- [x] Confirm local error style before adding package-specific errors.

Gate: note the exact package/module shape chosen before implementation.

## Task 2 - Core Type And Data

- [x] Add top-level `npc`.
- [x] Add public `NPC` type.
- [x] Add public `Config` type and `New(Config) (*NPC, error)`.
- [x] Add public `Data` type.
- [x] Use `*core.Ref` for refs, matching existing toolkit conventions.
- [x] Add `DisplayName`.
- [x] Add `Capabilities []Capability`.
- [x] Add `CombatPolicy`, `ObservationPolicy`, and `DispositionPolicy`.
- [x] Add `MovementPolicy`.
- [x] Keep runtime placement, location, team, visibility, encounter member ID,
  and living-world state out of `NPC`.
- [x] Add `Load(*Data) (*NPC, error)`.
- [x] Add `ToData() *Data`.

Tests:

- [x] `NPC` loads with all required fields.
- [x] `Data` round-trips through `ToData`.
- [x] returned capability slices are copy-out.

## Task 3 - Capabilities

- [x] Add `Capability` string type.
- [x] Add `CapabilityVendor = "vendor"` as the only built-in capability.
- [x] Allow unknown capability strings to load and round-trip.
- [x] Preserve capability order.

Tests:

- [x] vendor capability round-trips.
- [x] unknown capability string round-trips unchanged.
- [x] caller mutation of returned capabilities does not mutate `NPC`.

## Task 4 - Policies

- [x] Add `CombatPolicy` with `CombatPolicyNonCombatant`.
- [x] Add `ObservationPolicy` with `SubjectOnly` and `Observer`.
- [x] Add `DispositionPolicy` with `Neutral`.
- [x] Validate policy values at load.
- [x] Do not model pairwise hostility, factions, or teams here.

Tests:

- [x] known policy values load.
- [x] missing/unknown combat policy rejects by name.
- [x] missing/unknown observation policy rejects by name.
- [x] missing/unknown disposition policy rejects by name.

## Task 5 - Movement Policy And Runtime Boundary

- [x] Add `MovementPolicyBlocking = "blocking"`.
- [x] Add `MovementPolicyPassable = "passable"`.
- [x] Add `MovementPolicy.BlocksMovement() (bool, error)` as the sanctioned
  adapter helper for today's binary spatial occupancy seam.
- [x] Constructor defaults first-vendor-style NPCs to
  `MovementPolicyBlocking`, `CombatPolicyNonCombatant`,
  `ObservationPolicySubjectOnly` unless an explicit observer is supplied, and
  `DispositionPolicyNeutral`.
- [x] Document why movement is a named policy rather than a bool.
- [x] Document that current spatial cell occupancy maps movement policy to
  `BlocksMovement() bool`, but richer mover-vs-occupant checks are a future
  spatial/encounter concern.
- [x] Document that defaults are authoring convenience only.
- [x] Document that placed runtime systems own current mutable blocking state.

Tests:

- [x] constructor default sets `MovementPolicyBlocking`.
- [x] `MovementPolicyPassable` survives load/round-trip.
- [x] movement-policy adapter maps `blocking` to true and `passable` to false.
- [x] movement-policy adapter rejects missing/unknown policies.

## Task 6 - Package Docs

- [x] Add `doc.go` explaining that `npc` is generic content/data.
- [x] State that `npc` does not import `world`, `encounter`, `session`, or D&D.
- [x] State that world/scenario and encounter placement adapters live outside
  `npc`.
- [x] State that vendor stock/pricing/quote/buy behavior belongs outside `npc`.

## Task 7 - Verification

- [x] `go test ./...` from `npc`.
- [ ] `go test -race ./...` from `npc`, if local package runtime permits.
- [x] Root `git diff --check`.
- [x] No local `replace` directives or workspace-only dependency hacks.

Note: `go test ./...` passed from `npc` in the user's VS Code terminal. This
Codex session still does not have `go` or `gofmt` available on `PATH`, so
format/race verification remains outside this session.

## Follow-Up Integration

- [ ] Update [World NPCs](../world-npcs/) after `npc` implementation lands with
  the final exported shape.
- [ ] Let D&D vendor work compose with `npc.NPC` rather than redefining generic
  NPC identity.
- [ ] Let living-world adapters project `npc.NPC` into `world.Scenario` when a
  real consumer needs that path.
- [ ] Let encounter/session integration own placed state and current
  movement-blocking changes.
