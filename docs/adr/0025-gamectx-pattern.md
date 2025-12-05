# ADR-0025: gamectx Pattern for Condition Context Requirements

Date: 2024-12-03

## Status

Accepted

## Context

Conditions like Dueling, Sneak Attack, and other context-dependent features need access to game state that isn't present in events. For example:

- **Dueling** needs to know if the character has exactly one one-handed weapon equipped
- **Sneak Attack** needs to know if an ally is adjacent to the target
- **Defense** needs to know if the character is wearing armor

Events are intentionally slim data structs representing "what happened." We don't want to bloat events with every possible piece of data a condition might need (the "Frankenstein event" problem).

The toolkit cannot depend on the game server (rpg-api), so conditions can't directly query repositories or services.

We need a pattern where:
1. Game server loads and provides game state
2. Rulebook defines what questions can be asked
3. Conditions query what they need
4. Missing context fails fast with clear errors

## Decision

Implement a `gamectx` package in `rulebooks/dnd5e/` that uses Go's `context.Context` to carry typed registries of game state.

### Core Pattern

```go
// Game server creates and populates game context
gctx := gamectx.New()
gctx.AddCharacter(char1)
gctx.AddCharacter(char2)
ctx = gctx.Wrap(ctx)

// Condition declares requirements and queries context
func (d *DuelingCondition) Apply(ctx context.Context, bus EventBus) error {
    if err := gamectx.RequireCharacters(ctx); err != nil {
        return err
    }

    chars := gamectx.Characters(ctx)
    weapons := chars.GetEquippedWeapons(d.CharacterID)
    // ... use weapons to determine eligibility
}
```

### Package Structure

```
rulebooks/dnd5e/gamectx/
├── gamectx.go      # GameContext struct, New(), Wrap()
├── characters.go   # CharacterRegistry interface and implementation
├── require.go      # RequireCharacters(), RequireSpatial(), etc.
└── doc.go          # Package documentation
```

### Registries

Registries are thin query interfaces over loaded domain objects:

```go
type CharacterRegistry interface {
    Get(id string) *character.Character
    GetEquippedWeapons(id string) []Weapon
    GetMainHandWeapon(id string) *Weapon
    GetOffHandWeapon(id string) *Weapon
}
```

### Requirement Checking

Conditions check requirements at Apply time:

```go
func RequireCharacters(ctx context.Context) error {
    if Characters(ctx) == nil {
        return rpgerr.New(rpgerr.CodeFailedPrecondition,
            "CharacterRegistry required but not in context")
    }
    return nil
}
```

## Consequences

### Positive

- **Clean separation of concerns** - Bus handles pub/sub, context carries data
- **Type-safe queries** - Registries have typed methods, not string keys
- **Fail-fast behavior** - Missing context caught immediately at Apply
- **Idiomatic Go** - Uses standard `context.Context` pattern
- **Extensible** - Add new registries (Spatial, Encounter) as needed
- **Testable** - Easy to mock registries in tests
- **No Frankenstein events** - Events stay slim

### Negative

- **Context must be set up correctly** - Game server must populate context before applying conditions
- **Runtime errors for missing context** - Not compile-time checked (mitigated by RequireX helpers)
- **Registry interfaces in rulebook** - Ties registry shape to dnd5e rulebook

### Neutral

- **Conditions query at Apply vs event time** - Self data (weapons) queried at Apply, world data (positions) may need querying at event time
- **Game server responsible for hydration** - Must load all needed data before wrapping in context

## Example

Complete flow for Dueling:

```go
// rpg-api: Load and setup context
characters := repo.LoadCharacters(encounterID)
gctx := gamectx.New()
for _, char := range characters {
    gctx.AddCharacter(char)
}
ctx = gctx.Wrap(ctx)

// rpg-api: Apply conditions
for _, char := range characters {
    for _, cond := range char.Conditions {
        cond.Apply(ctx, bus)  // gamectx available here
    }
}

// rulebooks/dnd5e: Dueling checks eligibility
func (d *DuelingCondition) Apply(ctx context.Context, bus EventBus) error {
    if err := gamectx.RequireCharacters(ctx); err != nil {
        return err
    }

    chars := gamectx.Characters(ctx)
    mainHand := chars.GetMainHandWeapon(d.CharacterID)
    offHand := chars.GetOffHandWeapon(d.CharacterID)

    d.eligible = mainHand != nil &&
                 !mainHand.IsTwoHanded() &&
                 mainHand.IsMelee() &&
                 (offHand == nil || offHand.IsShield())

    return DamageChain.On(bus).SubscribeWithChain(ctx, d.onDamageChain)
}
```

## References

- Journey 048 - gamectx Pattern Discovery
- Issue #382 - Implementation issue
- Issue #360 - Context-Dependent Fighting Styles
- `docs/design/effect-query-system/003-context-pattern.md` - Earlier exploration
