# D&D 5e NPCs - Plan

**Executes:** `design.md`.
**Module:** `rulebooks/dnd5e/npcs`

## Task 1 - Package Census

- [x] Inspect D&D package style for `equipment`, `character`, `monster`, and
  nearby content constructors.
- [x] Confirm the exact D&D module dependency path for importing the newly
  published `npc` module.
- [x] Confirm first stock constants for longsword, greatsword, bow, and arrows.
- [x] Confirm whether a generic vendor world declaration helper belongs directly
  in `rulebooks/dnd5e/npcs` or in an example/content package.

Gate: record the chosen package/API shape before implementation.

## Task 2 - Vendor Role/Profile

- [x] Add `rulebooks/dnd5e/npcs`.
- [x] Add package docs explaining that D&D NPC roles compose generic `npc.NPC`.
- [x] Add `VendorConfig`.
- [x] Add `Vendor`.
- [x] Add `NewVendor`.
- [x] Add `LoadVendor`.
- [x] Add `ToData`.
- [x] Add copy-out accessors for base NPC and inventory.
- [x] Do not add public toolkit archetype constructors such as `NewBlacksmith`.

Tests:

- [x] vendor requires valid generic NPC data;
- [x] vendor preserves the generic `npc.NPC` ref, display name, capabilities,
  and policies;
- [x] vendor data round-trips.

## Task 3 - Vendor Inventory

- [x] Add `VendorInventory`.
- [x] Add `StockEntry`.
- [x] Add `Availability`.
- [x] Add `StockModeLimited`.
- [x] Add `StockModeUnlimited`.
- [x] Add persisted stock data with `shared.EquipmentType`, item ID, and
  availability.
- [x] Resolve persisted stock into runtime `equipment.Equipment`.
- [x] Validate limited stock quantity.
- [x] Validate unknown stock modes.
- [x] Ensure unlimited stock does not require or expose quantity.

Tests:

- [x] limited stock with quantity loads;
- [x] limited stock without positive quantity rejects;
- [x] unlimited stock loads without quantity;
- [x] unknown mode rejects;
- [x] unknown equipment rejects;
- [x] loaded stock exposes resolved equipment.

## Task 4 - Display View

- [x] Add `VendorView`.
- [x] Add `InventoryView`.
- [x] Add `StockEntryView`.
- [x] Include item type, ID, name, stock mode, and limited quantity.
- [x] Omit prices.
- [x] Return copy-out slices.

Tests:

- [x] view contains resolved display names;
- [x] view includes limited quantities only when meaningful;
- [x] view does not mutate the vendor when caller mutates returned slices.

## Task 5 - Configured Vendor Fixture

- [x] Add a test/example vendor configured as a blacksmith.
- [x] Use consumer-supplied NPC ref/display/capability/policy values.
- [x] Add fixed test/example stock:
  - [x] longsword, limited quantity 1;
  - [x] greatsword, limited quantity 1;
  - [x] one bow, limited quantity 1;
  - [x] arrows, unlimited.

Tests:

- [x] configured vendor has `npc.CapabilityVendor`;
- [x] configured vendor uses neutral, non-combatant, subject-only, blocking
  defaults;
- [x] configured vendor inventory matches the supplied stock.

## Task 6 - World Declaration Proof

- [x] Add a minimal scenario/declaration helper or example for declaring a
  configured vendor as a visible world entity.
- [x] Use `world.Scenario` and `world/graph` declarations rather than adding
  shop behavior to `world`.
- [x] Keep vendor inventory lookup in `npcs`, not in `world`.
- [x] Keep the helper small enough that it does not force premature placement
  semantics into `world`.

Tests:

- [x] scenario builds with existing `world.New` using test resolver/witness
  doubles;
- [x] truth view sees the configured vendor entity;
- [x] appropriate observer view sees the configured vendor entity;
- [x] no world API depends on vendor inventory.

## Task 7 - Verification

- [x] Run package tests for `rulebooks/dnd5e/npcs`.
- [x] Run affected D&D tests if dependency changes touch broader rulebook code.
- [x] Run root `git diff --check`.
- [x] Confirm no local `replace` or `go.work` file is committed.

Note: `go test -count=1 ./npcs ./refs` passes from `rulebooks/dnd5e`.
The user also verified `go test -v ./...` from `rulebooks/dnd5e/npcs`.
`go test -race ./...` from `rulebooks/dnd5e/npcs` is blocked locally because
the Go race detector needs cgo and no `gcc` is available on PATH.

`go test ./...` from `rulebooks/dnd5e` was attempted, and all reported packages
passed except `classes`, where Windows Application Control blocked Go's
generated `classes.test.exe` before package tests could run.

## Deferred Follow-Ups

- [ ] Character wallet/currency.
- [ ] Money arithmetic and price parsing.
- [ ] Quote and buy-only transaction flow.
- [ ] Stock decrement and inventory mutation.
- [ ] Generated stock through `tools/selectables`.
- [ ] Category expansion helpers.
- [ ] Encounter/session placement and adjacent interaction.
- [ ] Public toolkit vendor archetypes such as `NewBlacksmith`.
