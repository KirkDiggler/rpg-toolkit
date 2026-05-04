---
name: rpg-toolkit data model
description: Entity types, serialization structures, the chain/breakdown pattern, and known type-system gaps
updated: 2026-05-02
confidence: high — verified by reading core/entity.go, rulebooks/dnd5e/character/, tools/spatial/data.go, tools/environments/environment_data.go
---

# rpg-toolkit data model

## Core types

### Entity and EntityType

Every game object is an `Entity` (`core/entity.go:15`):

```go
type EntityType string

type Entity interface {
    GetID() string
    GetType() EntityType
}
```

`EntityType` is a distinct named type (not `string`). This is the source of the compile failure in `items/validation/basic_validator_test.go:27` where the mock returns `string` instead of `EntityType`.

Domain packages define `EntityType` constants:
- `core` defines `EntityTypeCharacter`, `EntityTypeItem`, etc.
- `rulebooks/dnd5e` defines `EntityTypeMonster`, `EntityTypeRoom`, `EntityTypeDoor`, etc.

### Ref and TypedRef

```go
// core/ref.go
type Ref struct {
    Module string  // "dnd5e"
    Type   string  // "features", "conditions", "actions"
    Value  string  // "rage", "dodging", "strike"
}

// TypedRef adds domain-typed ID
type TypedRef[T any] struct {
    Ref
    ID T
}
```

`Ref` is the routing key that flows between rpg-api and rpg-toolkit. rpg-api passes `Ref` values; toolkit routes them to the correct implementation. The `refs/` package in `rulebooks/dnd5e` provides named constructors:
```go
refs.Features.Rage()           // *core.Ref{Module:"dnd5e", Type:"features", Value:"rage"}
refs.Conditions.Dodging()      // *core.Ref
refs.CombatAbilities.Attack()  // *core.Ref
```

### Source and SourcedRef

```go
type SourceCategory string  // "class", "race", "background", "feat", "item", "manual"

type Source struct {
    Category SourceCategory
    Name     string
}

type SourcedRef struct {
    Ref
    Source *Source
    Label  string  // Display name for breakdown rendering
}
```

`SourcedRef` carries provenance through the modifier chain so the UI knows that a +2 bonus came from "Barbarian (class) — Rage" rather than an anonymous integer.

---

## Serializable data structs

Every stateful component has a `Data` struct — a plain Go struct with JSON tags, no methods, no embedded event bus. The `Data` struct is what rpg-api serializes to Redis.

### CharacterData (`rulebooks/dnd5e/character/`)

The canonical D&D 5e character state. Fields (non-exhaustive):

```go
type Data struct {
    ID               string
    Name             string
    Level            int
    Race             Race
    Class            Class
    AbilityScores    AbilityScores
    HitPoints        HitPoints
    Conditions       []ConditionJSON      // opaque JSON blobs, routed by Ref
    Features         []FeatureJSON        // opaque JSON blobs, routed by Ref
    Resources        map[string]ResourceData
    Equipment        EquipmentData
    ActionEconomy    ActionEconomyData
    // ... position, initiative, saves, skills
}
```

Roundtrip:
```go
data := char.ToData()               // serialize to Data struct
// rpg-api stores data as JSON in Redis
char, err := LoadFromData(ctx, data, bus)  // reconstitute live character
```

### EnvironmentData (`tools/environments/environment_data.go`)

```go
type EnvironmentData struct {
    ID       string
    Rooms    []RoomData
    Passages []PassageData
    Origins  map[string]OriginData  // room ID → absolute position
}
```

### RoomData (`tools/spatial/data.go`)

```go
type RoomData struct {
    RoomID      string
    RoomType    string
    GridType    string
    Width       int
    Height      int
    Orientation string       // "pointy" or "flat" for hex
    Entities    []PlaceableData
}
```

### ConditionJSON / FeatureJSON

Conditions and features serialize to `json.RawMessage` blobs. The blob contains a `{"ref": {...}}` field that the loader uses for routing:

```go
type RagingData struct {
    Ref         core.Ref `json:"ref"`
    CharacterID string   `json:"character_id"`
    DamageBonus int      `json:"damage_bonus"`
}
```

`LoadJSON(data json.RawMessage)` peeks at `ref.Value`, switches to the correct constructor, unmarshals the full struct. This keeps rpg-api's stored JSON opaque — it never needs to parse condition internals.

### ActionEconomyData

Introduced in PR #597. Tracks per-turn action spending:

```go
type ActionEconomyData struct {
    TurnNumber             int
    ActionsRemaining       int
    BonusActionsRemaining  int
    ReactionsRemaining     int
    AttacksRemaining       int
    MovementRemaining      int
    OffHandAttacksRemaining int
    FlurryStrikesRemaining int
}
```

Two-level model:
- **Action economy** — what you spend (action, bonus action, reaction)
- **Capacity** — what you get to do (attacks, movement, off-hand attacks, flurry strikes)

Taking the Attack ability spends an action and grants attack capacity. Each Strike action consumes one attack from that capacity.

---

## The chain / breakdown pattern

Modifiers flow through a `ChainedTopic` rather than direct function calls. A chain is a sequence of handlers that each contribute a modifier to a running `Breakdown`:

```go
// Illustrative, not exact code
type Breakdown struct {
    Base      int
    Modifiers []Modifier
    Total     int
}

type Modifier struct {
    Source SourcedRef
    Delta  int
}
```

Each subscribed handler receives the current chain event, adds its modifier, and publishes the updated value. The final handler collects the `Breakdown`. This is why `BusEffect.Apply()` subscribes a handler and `BusEffect.Remove()` unsubscribes — the feature's modifier is active only while it is applied.

The chain pattern is defined in `events/chain.go` and `events/chained_topic.go`. D&D-specific chains (AC, attack roll, damage) are wired in `rulebooks/dnd5e/combat/`.

---

## Entity relationships

```
Character
  ├─ AbilityScores (STR, DEX, CON, INT, WIS, CHA)
  ├─ HitPoints (current, max, temporary)
  ├─ Conditions[] → ConditionBehavior (implements BusEffect)
  ├─ Features[]   → FeatureBehavior  (implements BusEffect)
  ├─ Resources{}  → ResourcePool (spell slots, ki, rage uses)
  ├─ Equipment    → EquipmentSlots → Item refs
  └─ ActionEconomy

Environment (multi-room dungeon graph)
  ├─ Room[]   → BasicRoom (spatial.Room)
  │   ├─ GridType (Hex | Square | Gridless)
  │   └─ Entities[] → Placeable (extends core.Entity)
  └─ Passage[] → Connection (BasicConnection)

Combat
  ├─ InitiativeTracker → ordered combatant list
  ├─ CombatantState   → per-entity HP, action economy snapshot
  └─ ActionResolution → Attack, Damage, Save, Check breakdowns
```

---

## Known data model gaps

### `items` module: no implementation types
`items/item.go` defines `Item`, `EquippableItem`, `WeaponItem`, `ArmorItem`, `ConsumableItem` as interfaces. There are no implementing structs in the base `items` module — implementations live in `rulebooks/dnd5e/weapons`, `rulebooks/dnd5e/armor`, etc. This is intentional (the base module is infrastructure), but the `items` module's test layer is broken because the mock uses `GetType() string` where `core.Entity.GetType()` returns `core.EntityType`. See `items/validation/basic_validator_test.go:27`.

### `game` module: version spread
`game` is pinned at `v0.1.0` across all consumers but carries no replace directive. However the module's dependency on `events v0.1.1` means it receives older event types than the spatial or dnd5e modules. This has not caused a runtime issue but creates a version spread that makes it hard to know "which events interface does game.Context use." Watch for this when upgrading events past v0.6.x.

### ActionEconomyData.TurnNumber is an integer, not a turn ID
The `TurnNumber` field increments across the whole encounter, not per-round. If a round has 4 combatants and the encounter goes 3 rounds, TurnNumber might be 12. There is no separate `Round` field. Callers that want per-round tracking must track separately. This is not a bug — it was an explicit design choice in PR #597 — but it is non-obvious.
