# Journey 025: Actions and Effects Architecture

## Date: 2025-08-16

## The Revelation

Everything in an RPG boils down to two fundamental concepts:
1. **Actions** - Things that activate (rage, cast spell, attack, use item)
2. **Effects** - Things that modify game state via the event bus

This isn't just a pattern we noticed - it's THE pattern. Every feature, spell, ability, and item follows this flow:

```
Action → Execute → Create Effects → Apply to Bus → Modify Events
```

## The Journey to This Insight

### Starting Point: Character Loading

We began by trying to design how characters load from data. The game server has character data it doesn't understand - just passes it to the rulebook which interprets it.

Key realizations:
- Character structure is fixed (D&D 5e character always looks the same)
- Features/effects need ref-based loading (they vary by module)
- The rulebook is data-driven, interpreting data the game server doesn't understand

### The Ref Breakthrough

The ref system (`module:type:value`) is like a simplified DLL/class loader:
- Modules register capabilities at init time
- Runtime routing based on ref.Module
- Clean boundaries - toolkit never knows specifics

This prevents namespace collisions and allows extensibility without modifying core.

### Constants Everywhere

A critical insight: **Every string should be a constant.**

```go
// Not just events
type EventType string
type ResourceKey string
type ModifierSource string
type DamageType string

// Rulebook defines values using toolkit types
const (
    RageUses resources.ResourceKey = "rage_uses"
    AttackRoll events.EventType = "combat.attack.roll"
)
```

Benefits:
- Type safety - can't typo
- IDE support - autocomplete knows all constants
- Future compatibility - if toolkit adds features, constants already work
- Teaching through types - even doing nothing, types guide best practices

### The Concentration Test

We tested the architecture with concentration - a perfect example because it has:
- Multiple targets (up to 3)
- Sustained effect requiring focus
- Triggers (damage causes saves)
- Cascading removal (fail save → remove from all targets)

This revealed that concentration is a **core RPG pattern**, not rulebook-specific:
- Toolkit provides the mechanism
- Rulebook provides the rules (DC calculation, what breaks it)

### The Generic Action Pattern

The breakthrough: Use generics to avoid type assertions!

```go
// Generic action interface
type Action[T ActionInput] interface {
    Execute(actor Entity, input T, bus EventBus) error
}

// Rage implements Action[EmptyInput]
func (r *RageAction) Execute(actor Entity, input EmptyInput, bus EventBus) error {
    // input is typed - no assertion needed!
}

// Bless implements Action[CastInput]
func (b *BlessAction) Execute(actor Entity, input CastInput, bus EventBus) error {
    // input is CastInput - compile-time safe!
}
```

## The Complete Architecture

### Layer 1: Toolkit Infrastructure

Provides types and patterns:
```go
// Types for constants
type EventType string
type ResourceKey string
type ModifierSource string

// Generic action pattern
type Action[T ActionInput] interface {
    Execute(actor Entity, input T, bus EventBus) error
}

// Effect pattern
type Effect interface {
    Apply(bus EventBus) error
    Remove(bus EventBus) error
}

// Core patterns
- Event Bus
- Resources
- Concentration
- Duration
- Targeting
```

### Layer 2: Rulebook Implementation

Uses toolkit types to define game rules:
```go
// Constants using toolkit types
const (
    RageUses resources.ResourceKey = "rage_uses"
    AttackRoll events.EventType = "combat.attack.roll"
)

// Concrete actions
type RageAction struct{}
func (r *RageAction) Execute(actor Entity, input EmptyInput, bus EventBus) error

// Concrete effects
type RageDamageBonus struct{}
func (e *RageDamageBonus) Apply(bus EventBus) error
```

### Layer 3: Runtime Orchestration

Data-driven loading and execution:
```go
// Load character from data
character := rulebook.LoadCharacter(data)

// Features provide actions
rage := character.GetAction("rage")

// Execute action (typed input)
rage.Execute(character, EmptyInput{}, bus)

// Creates effects that wire to bus
// Effects automatically participate in events
```

## Key Design Principles

1. **Actions and Effects** - Everything is one or the other
2. **Typed Constants** - No magic strings anywhere
3. **Generic Safety** - Compile-time type checking for inputs
4. **Data-Driven** - Load from data, execute via interfaces
5. **Event-Driven** - All runtime behavior through event bus

## What Makes This Architecture Solid

### Extensibility
- New modules can add features without changing toolkit
- Toolkit can add patterns that work with existing constants
- Clean boundaries between layers

### Type Safety
- Constants prevent typos
- Generics prevent runtime type assertions
- Compiler catches errors early

### Discoverability
- IDE knows all constants
- Types document the architecture
- Clear patterns guide implementation

### Future-Proof
- Unknown features can be added (new modules)
- Unknown patterns can be supported (toolkit additions)
- Prevents painting ourselves into corners

## Examples Validating the Design

### Rage (Simple Feature)
```go
// Action with no input
type RageAction struct{}
implements Action[EmptyInput]

// Creates effects
- RageDamageBonus
- RageResistance

// Effects modify events
- Subscribe to DamageRoll
- Add modifiers
```

### Bless (Complex Spell)
```go
// Action with typed input
type BlessAction struct{}
implements Action[CastInput]

// Input specifies targets
CastInput{Targets: [3]Entity}

// Creates effects per target
- BlessEffect (+1d4 to attacks/saves)

// Concentration tracking
- StartConcentrating()
- Subscribe to damage for saves
```

### Attack (Core Action)
```go
// Action with attack input
type AttackAction struct{}
implements Action[AttackInput]

// Input specifies target/weapon
AttackInput{Target: goblin, Weapon: "longsword"}

// Triggers event chain
- AttackRoll event
- DamageRoll event
- DamageTaken event
```

## Open Questions and Holes

1. **Action Registry** - How do we store different `Action[T]` types?
   - Type erasure at storage?
   - Separate registries per input type?

2. **Effect Lifecycle** - How long do effects last?
   - Duration tracking
   - Cleanup on removal

3. **State Persistence** - How to save activated features/effects?
   - Serialize effect state
   - Reload and reapply to bus

4. **Resource Management** - Should resources be effects too?
   - HP as an effect?
   - Or keep resources separate?

## Follow-Up Issues to Create

1. **Implement Generic Action Pattern** - Create the base Action[T] interface
2. **Define Toolkit Types** - Add ResourceKey, ModifierSource, etc. to toolkit
3. **Create Concentration System** - Core pattern for sustained effects
4. **Design Effect Lifecycle** - Duration, removal, cleanup
5. **Implement Bless as Test Case** - Validates concentration + multi-target
6. **Action Registry Pattern** - Solve the type storage problem
7. **State Persistence** - Serialize/deserialize active effects

## The Big Picture

We're building a system where:
- The toolkit knows nothing about specific games
- Games implement rules using toolkit patterns
- Everything is data-driven and type-safe
- New content can be added without code changes

The Actions and Effects pattern is the core abstraction that makes this possible. Every player choice becomes an Action, every game modifier becomes an Effect, and the event bus orchestrates it all.

## Next Steps

1. Formalize the Action[T] interface in the toolkit
2. Add missing types (ResourceKey, etc.) to toolkit
3. Implement a complete feature (Rage) using the pattern
4. Implement a spell (Bless) to validate concentration
5. Test that different modules can coexist

This architecture feels right. It's simple, extensible, and type-safe. The "everything is actions and effects" insight unifies the entire system.