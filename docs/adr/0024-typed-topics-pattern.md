# ADR-0024: Typed Topics Pattern for Event Bus

## Status
Accepted

## Context

The events package had evolved to include three patterns:
1. Legacy ref-based event bus with runtime type checking
2. TypedTopic for pure notifications  
3. ChainedTopic for events with modifier chains

This created unnecessary complexity:
- Events were forced to implement interfaces
- Runtime type assertions everywhere
- Three ways to do the same thing
- Approximately 2000+ lines of code

We needed a simpler, type-safe approach that would serve rulebook authors well.

## Decision

We will implement a typed topics pattern with these key components:

### 1. Events Are Pure Data
```go
// No interface required - just structs
type AttackEvent struct {
    AttackerID string
    TargetID   string
    Damage     int
}
```

### 2. Topics Are Explicitly Defined
```go
// Define topic constants - explicit and reusable
const (
    TopicAttack events.Topic = "combat.attack"
    TopicDamage events.Topic = "combat.damage"
)

// Define typed topics using the constants
var (
    AttackTopic = events.DefineTypedTopic[AttackEvent](TopicAttack)
    DamageTopic = events.DefineTypedTopic[DamageEvent](TopicDamage)
)
```

### 3. The `.On(bus)` Connection Pattern
```go
// Clear, explicit connection to bus
attacks := combat.AttackTopic.On(bus)

// Type-safe subscription
attacks.Subscribe(ctx, func(ctx context.Context, e AttackEvent) error {
    // Handle event
    return nil
})
```

### 4. Two Topic Types for Different Needs

**TypedTopic[T]** for pure notifications:
- Events that notify without transformation
- Simple publish/subscribe pattern

**ChainedTopic[T]** for modifier collection:
- Events that need staged processing
- Collect modifiers into chains
- Execute chains to transform data

### 5. Bus Uses Topic Type
```go
type Topic string  // Local type, not raw string

type EventBus interface {
    Subscribe(topic Topic, handler any) (string, error)
    Unsubscribe(id string) error
    Publish(topic Topic, event any) error
}
```

## Consequences

### Benefits
1. **Type Safety** - Compile-time checking, no runtime assertions
2. **Simple Events** - Just data structs, no interfaces required
3. **Clear API** - The `.On(bus)` pattern is self-documenting
4. **Explicit Topics** - Rulebook owns topic uniqueness
5. **Reduced Complexity** - From 2000+ lines to ~450 lines
6. **IDE-Friendly** - Autocomplete shows all available topics

### Trade-offs
1. **Topic Uniqueness** - Rulebook's responsibility to ensure unique topic IDs
2. **Static Topics** - Topics must be defined at compile time (features remain dynamic)
3. **Explicit IDs** - Must provide topic strings (but only in one place)

### Key Insight
**Features are dynamic, topics are static.** This design separates concerns perfectly:
- Topics are defined statically by the rulebook (compile time)
- Features are loaded dynamically from JSON (runtime)
- Features' Apply() methods explicitly subscribe to known topics
- No dynamic string resolution needed

## Implementation Details

### ChainedEvent Wrapper Solution
For ChainedTopic, we wrap the event and chain together to pass through the bus:
```go
type chainedEvent[T any] struct {
    ctx   context.Context
    event T
    chain chain.Chain[T]
}
```
Handlers unwrap it, modify the chain, and the modified chain is returned to the publisher.

### Shared Data Structures
Events can share common structs without sharing the event type:
```go
// Shared struct
type Damage struct {
    Amount int
    Type   DamageType
}

// Different events using same struct
type AttackEvent struct {
    Damage Damage
}

type SpellEvent struct {
    Damage Damage
}
```

### Testing Strategy
- Testify suite pattern for organization
- SetupTest() for fresh state each test
- Real scenarios (attack damage, conditions)
- 93.5% test coverage achieved

### Migration from Legacy System
1. Remove Event interface requirement
2. Replace ref-based routing with Topic constants
3. Update subscribers to use typed topic APIs
4. Remove legacy bus implementation (~2000 lines → ~500 lines)

## Examples

### Pure Notification - Character Condition Tracking
```go
// Topic definition
const TopicConditionApplied events.Topic = "character.condition.applied"
var ConditionAppliedTopic = events.DefineTypedTopic[ConditionAppliedEvent](TopicConditionApplied)

// Event is just data
type ConditionAppliedEvent struct {
    CharacterID string
    Condition   json.RawMessage  // The condition as JSON
    ExpiresAt   *time.Time
}

// Character subscribes to track its conditions
type Character struct {
    ID         string
    Conditions []json.RawMessage
    dirty      bool
}

func (c *Character) OnConditionApplied(bus events.EventBus) {
    // Beautiful connection pattern
    conditions := ConditionAppliedTopic.On(bus)
    
    conditions.Subscribe(ctx, func(ctx context.Context, e ConditionAppliedEvent) error {
        if e.CharacterID != c.ID {
            return nil
        }
        
        // Add condition JSON to character's conditions
        c.Conditions = append(c.Conditions, e.Condition)
        c.dirty = true  // Mark for persistence
        
        // Could emit another event here for UI updates
        return nil
    })
}
```

### Chained Processing - Rage Damage Modifier
```go
// Topic definition
const TopicAttack events.Topic = "combat.attack"
var AttackChain = events.DefineChainedTopic[AttackEvent](TopicAttack)

// Event carries the data
type AttackEvent struct {
    AttackerID string
    TargetID   string
    Damage     int
    Critical   bool
}

// Rage feature modifies attacks
type RageFeature struct {
    characterID string
    damageBonus int
}

func (r *RageFeature) Apply(bus events.EventBus) error {
    // Connect to the attack chain
    attacks := AttackChain.On(bus)
    
    // Subscribe to modify attacks
    _, err := attacks.SubscribeWithChain(ctx, 
        func(ctx context.Context, e AttackEvent, chain chain.Chain[AttackEvent]) (chain.Chain[AttackEvent], error) {
            // Only modify our character's attacks
            if e.AttackerID != r.characterID {
                return chain, nil
            }
            
            // Add rage modifier at the conditions stage
            err := chain.Add(combat.StageConditions, "rage", 
                func(ctx context.Context, event AttackEvent) (AttackEvent, error) {
                    event.Damage += r.damageBonus
                    return event, nil
                })
            
            return chain, err
        })
    
    return err
}

// Combat system uses the chain
func ResolveAttack(bus events.EventBus, attack AttackEvent) (AttackEvent, error) {
    // Define processing stages
    stages := []chain.Stage{
        combat.StageBase,       // Base damage calculation
        combat.StageFeatures,   // Class features (sneak attack, etc)
        combat.StageConditions, // Rage, bless, etc
        combat.StageEquipment,  // Magic weapons
        combat.StageFinal,      // Critical multipliers
    }
    
    // Create chain and connect to bus
    attackChain := events.NewStagedChain[AttackEvent](stages)
    attacks := AttackChain.On(bus)
    
    // Publish to collect modifiers from all subscribers
    modifiedChain, err := attacks.PublishWithChain(ctx, attack, attackChain)
    if err != nil {
        return attack, err
    }
    
    // Execute chain to get final result - super clean!
    result, err := modifiedChain.Execute(ctx, attack)
    // result.Damage now includes rage bonus and any other modifiers
    
    return result, err
}
```

### The Power of chain.Execute()
The pattern of `chain.Execute(ctx, input) -> (output, error)` is incredibly clean:
- Input goes in, modified output comes out
- All modifiers are applied in stage order
- Each modifier transforms the event
- No mutation of shared state
- Pure functional transformation

## Future Research & Development

While out of scope for this ADR, there are interesting areas to explore:

### Nested Pipelines
The ability to have chains that spawn sub-chains for complex multi-step processing:
- Attack chain could spawn a damage resistance chain
- Saving throw chain could spawn an advantage/disadvantage chain

### Pausable Chains
The ability to pause chain execution and resume later:
- Save chain state to JSON
- Resume from saved state
- Useful for turn-based mechanics or async processing

### Chain Introspection
The ability to inspect what modifiers are in a chain:
- List all active modifiers
- Debug why damage is calculated a certain way
- UI could show "Damage: 10 + 2 (rage) + 4 (bless) = 16"

These would be tracked as separate R&D issues for future exploration.

## References
- Journey 007: The Evolution of Typed Topics
- Original events package (pre-refactor)
- Toolkit philosophy: "Pick ONE way", "Optimize for simplicity"