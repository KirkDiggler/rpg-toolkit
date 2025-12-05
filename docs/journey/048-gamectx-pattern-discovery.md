# Journey 048: Discovering the gamectx Pattern

## The Problem

We needed to implement Dueling fighting style, which requires knowing:
> "Is the character wielding exactly one melee weapon in one hand, with no other weapon?"

The damage chain event only has `AttackerID` and damage components. Where does the weapon information come from?

## The Exploration

### Option 1: Frankenstein Events

The naive approach - stuff everything into the event:

```go
type DamageChainEvent struct {
    AttackerID      string
    Damage          int
    // And now...
    AttackerWeapons []Weapon      // For Dueling
    AttackerArmor   *Armor        // For Defense
    NearbyAllies    []string      // For Protection
    AttackerLevel   int           // For level-scaling
    // ... grows forever
}
```

**Why we rejected it:** Events become kitchen sinks. Every condition adds fields. Events lose their clean "what happened" nature.

### Option 2: Resolver Functions on Chain

Chain carries typed resolver functions:

```go
chain := events.NewStagedChainWithContext[DamageEvent, CombatContext](stages, CombatContext{
    GetWeapons: gameServer.GetEquippedWeapons,
    GetArmor:   gameServer.GetEquippedArmor,
})
```

**The concern:** What is `gameServer`? Where does it come from? The toolkit can't depend on the game server.

### Option 3: Conditions Subscribe to Equipment Changes

Conditions listen to equipment events and maintain their own state:

```go
type DuelingFeature struct {
    ownerID          string
    hasOneHandedOnly bool  // Updated by equipment subscription
}
```

**The concern:** Every feature tracks its own state. State could get stale. Lots of subscribers for simple queries.

### Option 4: Bus with Registries

The bus itself holds queryable registries:

```go
bus.Register("characters", characterRegistry)
weapons := bus.Get("characters").(CharacterRegistry).GetWeapons(id)
```

**The pivot:** This is close, but putting registries on the bus conflates pub/sub with data storage. These are separate concerns.

## The Breakthrough

We realized: **Go's `context.Context` is designed exactly for this.** It's request-scoped data that flows through the system.

But we needed to think about the layers:

```
┌─────────────────────────────────────────┐
│  rpg-api (game server)                  │
│  - Loads domain objects from DB         │
│  - Orchestrates calls                   │
│  - Provides real data                   │
├─────────────────────────────────────────┤
│  rpg-toolkit/rulebooks/dnd5e            │
│  - Defines what questions can be asked  │
│  - Implements Dueling, Rage, etc.       │
├─────────────────────────────────────────┤
│  rpg-toolkit (core, events)             │
│  - Generic infrastructure               │
│  - No game-specific knowledge           │
└─────────────────────────────────────────┘
```

Key insight: **The game server loads everything for a turn. The rulebook defines how to query it. Conditions ask questions.**

## The Pattern

### Game Server Sets Up Context

```go
// rpg-api loads domain objects (fully hydrated)
characters := repo.LoadCharacters(encounterID)
room := repo.LoadRoom(roomID)

// Wrap them in gamectx
gctx := gamectx.New()
for _, char := range characters {
    gctx.AddCharacter(char)
}
ctx = gctx.Wrap(ctx)

// Apply conditions - they can now query the context
for _, char := range characters {
    for _, cond := range char.Conditions {
        cond.Apply(ctx, bus)
    }
}
```

### Condition Queries Context

```go
func (d *DuelingCondition) Apply(ctx context.Context, bus EventBus) error {
    // Declare requirements - fail fast if missing
    if err := gamectx.RequireCharacters(ctx); err != nil {
        return err
    }

    // Query what we need
    chars := gamectx.Characters(ctx)
    mainHand := chars.GetMainHandWeapon(d.CharacterID)
    offHand := chars.GetOffHandWeapon(d.CharacterID)

    // Determine eligibility at Apply time
    d.eligible = mainHand != nil &&
                 !mainHand.IsTwoHanded() &&
                 (offHand == nil || offHand.IsShield())

    // Subscribe with captured eligibility
    DamageChain.On(bus).SubscribeWithChain(ctx, d.onDamageChain)
    return nil
}
```

### Naming Journey

We almost called the package `context` (shadows Go's context), `rpgctx`, or used a builder pattern. We landed on:

```go
gctx := gamectx.New()
gctx.AddCharacter(char)
ctx = gctx.Wrap(ctx)
```

- `gamectx` - clearly "game context", not Go context
- `New()` / `AddCharacter()` / `Wrap()` - simple, no builder baggage
- `gctx` as variable - short, clear

### Requirement Checking

Conditions declare what they need and fail fast:

```go
if err := gamectx.RequireCharacters(ctx); err != nil {
    return err  // "CharacterRegistry required but not in context"
}
```

The condition owns the decision. Helpers provide consistent error messages.

## Key Decisions Made

1. **Context carries registries, not raw data** - Registries know how to answer questions about loaded data

2. **Rulebook defines registry interfaces** - `CharacterRegistry` with `GetEquippedWeapons()`, etc.

3. **Game server provides implementations** - Wraps loaded domain objects in registries

4. **Conditions check requirements on Apply** - Fail fast with clear errors if context is missing

5. **Eligibility determined at Apply time** - Not on every event, since equipment doesn't change mid-turn

## Data Categories

We identified two kinds of data conditions might need:

| Type | Example | How to Get |
|------|---------|------------|
| **Self data** | My weapons, my armor | `CharacterRegistry` at Apply time |
| **World data** | Who's adjacent to target | `SpatialRegistry` at event time |

Dueling needs self data (captured at Apply). Sneak Attack needs world data (queried at event time).

## What This Enables

- **Dueling** - Query weapons at Apply
- **Two-Weapon Fighting** - Query off-hand weapon
- **Sneak Attack** - Query spatial for ally adjacency
- **Defense** - Query armor for AC bonus
- **Any future condition** - Pattern is extensible

## References

- Issue #382 - gamectx implementation
- Issue #360 - Context-Dependent Fighting Styles
- ADR-0025 - gamectx Pattern (companion decision record)
