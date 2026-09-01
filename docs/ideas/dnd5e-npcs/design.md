# D&D 5e NPC Roles - Vendor First

**Status:** PROPOSED
**Related:** [NPC](../npc/), [World NPCs](../world-npcs/),
[rpg-toolkit#1275](https://github.com/KirkDiggler/rpg-toolkit/issues/1275)

## Purpose

Define D&D-specific NPC roles on top of the generic `npc.NPC` content record.
The first exercised role is a configurable vendor with display stock supplied by
the consumer.

This slice is not the D&D economy system. It proves that a rulebook-defined NPC
can compose generic NPC identity, carry rulebook-owned role data, persist and
load display inventory, declare itself into `world`, and expose a display
inventory that a host can render after an interaction.

## Boundary

Generic `npc` owns:

- reusable NPC identity;
- display name;
- opaque capabilities;
- combat, observation, disposition, and movement policies.

`rulebooks/dnd5e/npcs` owns:

- D&D-specific NPC role/profile constructors;
- D&D vendor inventory shape;
- D&D equipment references and resolved equipment values;
- view data suitable for rendering a vendor inventory.

`world` owns:

- declared graph entities, relations, slots, verbs, quests, and goals;
- folding what an observer currently knows.

`world` is not a shop engine. A D&D content package may declare that a
vendor exists in a scenario, but the D&D NPC package owns how D&D vendor
inventory is validated, resolved, persisted, and viewed.

## Package Shape

Add a rulebook package:

```text
rulebooks/dnd5e/npcs
```

The plural package name is deliberate: it collects D&D NPC roles and authored
profiles. The generic package remains singular `npc`, so the call sites read as
generic `npc.NPC` composed by D&D `npcs.Vendor`.

Expected files for the first slice:

```text
rulebooks/dnd5e/npcs/doc.go
rulebooks/dnd5e/npcs/vendor.go
rulebooks/dnd5e/npcs/vendor_inventory.go
```

Exact file boundaries may change if implementation discovers a cleaner local
shape.

## Core Types

`Vendor` is a D&D role/profile, not a generic toolkit concept.

```go
type Vendor struct {
    npc       *npc.NPC
    inventory VendorInventory
}
```

The package should expose copy-out accessors rather than public mutable fields:

```go
func NewVendor(config VendorConfig) (*Vendor, error)

func (v *Vendor) NPC() *npc.NPC
func (v *Vendor) Inventory() VendorInventory
func (v *Vendor) View() VendorView
func (v *Vendor) ToData() *VendorData
func LoadVendor(data *VendorData) (*Vendor, error)
```

Different inventories must not require different Go vendor types. A blacksmith,
a general merchant, and a quartermaster can all use `Vendor`; they differ by
consumer-supplied data and inventory, not by one-off type proliferation.

The toolkit should not expose public archetype constructors such as
`NewBlacksmith` in this slice. A blacksmith may appear in tests and examples as
the first configured vendor, but it is not a toolkit-owned vendor type. The
web/content authoring layer supplies the vendor's name, ref, and inventory.

## Inventory Shape

Use the existing D&D equipment model at runtime and a durable D&D item identity
in data.

Character inventory already has this split:

- runtime inventory carries `equipment.Equipment` plus quantity;
- persisted inventory carries `shared.EquipmentType`, item ID, and quantity.

Vendor inventory should mirror that pattern without reusing character inventory
rows directly:

```go
type VendorInventory struct {
    entries []StockEntry
}

type StockEntry struct {
    equipment    equipment.Equipment
    availability Availability
}
```

Persisted stock uses the same item identity fields as character inventory:

```go
type StockEntryData struct {
    Type         shared.EquipmentType `json:"type"`
    ID           string               `json:"id"`
    Availability Availability        `json:"availability"`
}
```

Do not use `character.InventoryItemData` for vendor stock. Its `Quantity` means
"the character owns this many." Vendor stock needs limited or unlimited display
availability, which is a different concept.

## Persistence

`VendorData` is the serializable D&D vendor role/profile.

```go
type VendorData struct {
    NPC       *npc.Data           `json:"npc"`
    Inventory *VendorInventoryData `json:"inventory"`
}

type VendorInventoryData struct {
    Entries []StockEntryData `json:"entries"`
}
```

`VendorData` includes inventory because display stock is part of this
implementation slice. The inventory data is a pointer so load can distinguish a
missing inventory payload from a present-but-empty inventory payload. It does
not include world entity ID, map location, current visibility, UI state, current
interaction session, wallet, price overrides, or transaction history.

Limited quantities in `VendorData` are current display availability for now, but
this package does not mutate them until a later buy/sell slice defines stock
decrement rules.

Loading a vendor must validate and resolve inventory entries. Saving a vendor
must emit stable data with the same D&D item identity shape used by character
inventory: `shared.EquipmentType` plus item ID.

## Availability

Vendor inventory supports limited and unlimited display stock.

```go
type StockMode string

const (
    StockModeLimited   StockMode = "limited"
    StockModeUnlimited StockMode = "unlimited"
)

type Availability struct {
    Mode     StockMode `json:"mode"`
    Quantity int       `json:"quantity,omitempty"`
}
```

Rules:

- limited stock requires a positive quantity;
- unlimited stock ignores quantity and should serialize with no quantity;
- unknown stock modes reject at load;
- this slice does not decrement stock, because buying is deferred.

## Display View

Toolkit should not return UI components. It should return structured data that
the host or web client can render as a popup, panel, list, or other interface.

```go
type VendorView struct {
    Ref         *core.Ref        `json:"ref"`
    DisplayName string           `json:"display_name"`
    Inventory   InventoryView    `json:"inventory"`
}

type InventoryView struct {
    Entries []StockEntryView `json:"entries"`
}

type StockEntryView struct {
    Type     shared.EquipmentType `json:"type"`
    ID       string               `json:"id"`
    Name     string               `json:"name"`
    Mode     StockMode            `json:"mode"`
    Quantity int                  `json:"quantity,omitempty"`
}
```

Prices are intentionally absent. The first interaction answers "what does this
vendor display?" not "what can the character afford?"

The view should resolve names through `equipment.Equipment`. If richer display
detail is already cheap to include through `equipment.Equipment` or existing
detail helpers, implementation may add fields such as weight or description,
but price/cost stays out of this slice.

## First Configured Vendor

The first acceptance fixture is a blacksmith-like vendor configured by the
consumer. It proves the D&D vendor role without adding public toolkit
archetypes.

Generic NPC defaults:

- `Ref`: consumer-supplied D&D NPC ref for the vendor profile;
- `DisplayName`: `Blacksmith`;
- `Capabilities`: includes `npc.CapabilityVendor`;
- `CombatPolicy`: `npc.CombatPolicyNonCombatant`;
- `ObservationPolicy`: `npc.ObservationPolicySubjectOnly` unless a stronger
  reason appears during implementation;
- `DispositionPolicy`: `npc.DispositionPolicyNeutral`;
- `MovementPolicy`: `npc.MovementPolicyBlocking`.

Fixed display stock:

- one longsword, limited;
- one greatsword, limited;
- one bow, limited;
- arrows, unlimited.

Use existing registry constants when available. Prefer a single existing bow
entry, such as `weapons.Longbow` if that is the cleanest available registry
constant.

## World Declaration

This slice should prove declaration into `world`, not full encounter/session
placement.

The D&D NPC package may expose a helper that turns a consumer-supplied vendor
into declared world content:

```go
func VendorScenario(config VendorScenarioConfig) (world.Scenario, error)
```

The exact API should follow what the implementation discovers in existing
`examples/world/scenarios/...`: scenario builders return `world.Scenario` or
`graph.Config`, declare `graph.Entity` values, and use `graph.Kind`,
`graph.Relation`, and `journal.EntityID` strings owned by the content package.

Minimum world proof:

- the configured vendor is declared as a visible world graph entity;
- the scenario includes only enough verb data to satisfy `world.New`;
- it has a stable entity ID supplied by config or authored default;
- it can belong to, occupy, or be related to a location/group using ordinary
  `world/graph` declarations if the scenario needs that fact;
- building a `world.World` from the scenario succeeds with injected resolver and
  witness test doubles;
- `World.Truth()` and an appropriate observer view can see the blacksmith.

Do not teach `world` about vendor inventory. World declaration says "this vendor
exists here." The `npcs.Vendor` value answers "what does this vendor display?"

If implementation shows that current `world` has no useful way to represent a
location beyond generic graph relationships, this leg may keep placement as a
scenario declaration with a named relation rather than inventing a spatial
location model.

## Leveraging Existing Toolkit Pieces

Use what already exists:

- `npc.NPC` for generic identity, capabilities, and policies;
- `core.Ref` for the reusable NPC profile ref;
- `rulebooks/dnd5e/equipment.Equipment` for runtime stock items;
- `equipment.GetByID` and/or existing equipment registries to resolve authored
  stock;
- `shared.EquipmentType` and item IDs for persisted vendor stock, matching
  character inventory identity;
- `world.Scenario`, `graph.Config`, `graph.Entity`, `graph.Edge`, and
  `graph.Slot` for declared world presence;
- testify suites for tests, matching repo guidance.

Do not introduce:

- a generic toolkit vendor package;
- a top-level inventory/economy module;
- wallet/currency on character data;
- quote or buy operations;
- stock decrement;
- `tools/selectables` integration.

`tools/selectables` is the likely future engine for generated/dynamic stock,
but this PR should use fixed authored stock only.

## First Implementation Slice

The original #1275 issue describes a larger vendor/economy foundation: money,
wallet, finite/infinite stock, quotes, buy-only transactions, inventory mutation,
category expansion, and generated stock. This design narrows the first PR to
the part that proves the NPC architecture.

This leg includes:

- D&D `npcs` package;
- `Vendor` role/profile type;
- `VendorInventory` display stock;
- limited and unlimited stock availability;
- consumer-supplied vendor inventory;
- blacksmith-like test/example fixture with fixed stock;
- vendor view suitable for UI rendering;
- vendor persistence through `VendorData`;
- world scenario declaration proof, if it can stay small and data-only;
- tests for construction, load/save, stock validation, view generation, and
  world declaration.

This leg defers:

- character wallet/currency;
- money arithmetic;
- price display;
- quote flow;
- buy/sell/barter;
- inventory mutation;
- stock decrement;
- haggling, reputation, scarcity, or restocking;
- generated/dynamic inventory;
- public toolkit vendor archetypes such as `NewBlacksmith`;
- encounter/session placement;
- attackability, hostility changes, teams, or AI.

## Acceptance Criteria

- `rulebooks/dnd5e/npcs` exists and documents that it owns D&D-specific NPC
  roles.
- `Vendor` composes `npc.NPC`; it does not redefine generic NPC identity.
- `NewVendor` accepts configurable NPC and inventory data so different
  inventories do not require different vendor types.
- The configured vendor exposes `npc.CapabilityVendor`.
- The configured vendor is neutral, non-combatant, subject-only by default, and
  movement-blocking.
- `VendorData` persists generic NPC data plus vendor inventory data.
- `VendorData` does not persist world placement, UI state, or economy state.
- Vendor runtime stock carries resolved `equipment.Equipment`.
- Vendor persisted stock carries `shared.EquipmentType`, item ID, and
  availability.
- Limited stock validates positive quantity.
- Unlimited stock round-trips without requiring quantity.
- Vendor view includes item identity, display name, stock mode, and limited
  quantity when present.
- Vendor view does not include prices.
- The test/example inventory includes one longsword, one greatsword, one bow,
  and unlimited arrows.
- A world scenario/declaration helper can place or declare the vendor as a
  visible world entity.
- The world proof does not require `world` to know vendor inventory.
- Tests cover the above behavior.
