# Conditions System

The conditions system provides infrastructure for status effects and conditions in RPG games. Following our data-driven architecture, conditions are loaded from data at runtime and modify game behavior through event handlers.

## Design Philosophy

**Everything is data-driven at runtime.** The toolkit provides:
- Simple `Condition` interface with essential methods
- `SimpleCondition` that embeds `effects.Core` for common functionality
- Options pattern for flexible condition application
- `ConditionData` loader for extracting ref + JSON for routing
- Well-defined error types for clear communication

Games implement their specific conditions using these building blocks.

## Basic Usage

### 1. Implement Your Condition

```go
// Embed SimpleCondition for common functionality
type PoisonedCondition struct {
    *conditions.SimpleCondition
}

func NewPoisonedCondition() (*PoisonedCondition, error) {
    ref := &core.Ref{
        Category: "dnd5e",
        Type:     "condition",
        ID:       "poisoned",
    }
    
    simple, err := conditions.NewSimpleCondition(ref)
    if err != nil {
        return nil, err
    }
    
    simple.SetName("Poisoned")
    simple.SetDescription("Disadvantage on attack rolls and ability checks")
    
    return &PoisonedCondition{
        SimpleCondition: simple,
    }, nil
}

// Override Apply to register event handlers
func (p *PoisonedCondition) Apply(target core.Entity, bus events.EventBus, opts ...conditions.ApplyOption) error {
    // First apply using SimpleCondition
    if err := p.SimpleCondition.Apply(target, bus, opts...); err != nil {
        return err
    }
    
    // Register our handlers for disadvantage
    p.Subscribe(bus, events.EventOnAttackRoll, 100, func(ctx context.Context, e events.Event) error {
        if e.Source() == p.Target() {
            e.Context().AddModifier(events.NewModifier(
                "poisoned_disadvantage",
                events.ModifierDisadvantage,
                events.IntValue(1),
                100,
            ))
        }
        return nil
    })
    
    return nil
}
```

### 2. Load Conditions from Data

```go
// Load condition data (from database, file, etc)
jsonData := []byte(`{
    "ref": {"category": "dnd5e", "type": "condition", "id": "poisoned"},
    "name": "Poisoned",
    "description": "Disadvantage on attacks and checks",
    "metadata": {"save_dc": 13}
}`)

// Extract ref and JSON for routing
condData, err := conditions.Load(jsonData)
if err != nil {
    log.Fatal(err)
}

// Route to the right implementation based on ref
var cond conditions.Condition
switch condData.Ref().ID {
case "poisoned":
    cond, err = NewPoisonedCondition()
    // Load any saved state if needed
case "stunned":
    cond, err = NewStunnedCondition()
default:
    return fmt.Errorf("unknown condition: %s", condData.Ref())
}
```

### 3. Apply Conditions with Options

```go
bus := events.NewBus()

// Create and apply a condition
poisoned, _ := NewPoisonedCondition()

err := poisoned.Apply(character, bus,
    conditions.WithSource("spider_bite"),
    conditions.WithSaveDC(13),
    conditions.WithDuration(10 * time.Minute),
    conditions.WithMetadata("poison_type", "venom"),
)

if err != nil {
    // Handle errors appropriately
    switch {
    case errors.Is(err, conditions.ErrAlreadyActive):
        fmt.Println("Already poisoned")
    case errors.Is(err, conditions.ErrConditionImmune):
        fmt.Println("Target is immune to poison")
    default:
        log.Fatal(err)
    }
}
```

### 4. Manage Multiple Conditions

```go
// Track conditions on an entity
type Character struct {
    conditions []conditions.Condition
}

func (c *Character) AddCondition(cond conditions.Condition, bus events.EventBus) error {
    // Check for duplicates
    for _, existing := range c.conditions {
        if existing.Ref().Equals(cond.Ref()) {
            return conditions.ErrAlreadyActive
        }
    }
    
    // Apply the condition
    if err := cond.Apply(c, bus); err != nil {
        return err
    }
    
    c.conditions = append(c.conditions, cond)
    return nil
}

func (c *Character) RemoveCondition(ref *core.Ref, bus events.EventBus) error {
    for i, cond := range c.conditions {
        if cond.Ref().Equals(ref) {
            if err := cond.Remove(bus); err != nil {
                return err
            }
            c.conditions = append(c.conditions[:i], c.conditions[i+1:]...)
            return nil
        }
    }
    return conditions.ErrNotActive
}
```

## Available Effect Types

The toolkit provides these generic effect types that games can use:

- `EffectAdvantage` / `EffectDisadvantage` - Modify dice rolls
- `EffectAutoFail` / `EffectAutoSucceed` - Force roll outcomes
- `EffectSpeedReduction` / `EffectSpeedZero` - Movement effects
- `EffectIncapacitated` / `EffectNoReactions` - Action restrictions
- `EffectResistance` / `EffectVulnerability` - Damage modifications
- `EffectCantSpeak` / `EffectCantHear` / `EffectCantSee` - Sensory effects

## Available Effect Targets

Effects can target different game mechanics:

- `TargetAttackRolls` / `TargetAttacksAgainst` - Combat
- `TargetSavingThrows` / `TargetAbilityChecks` - Dice rolling
- `TargetMovement` / `TargetActions` / `TargetReactions` - Actions
- `TargetSight` / `TargetHearing` / `TargetSpeech` - Senses
- `TargetDamage` - Damage calculations

## Condition Relationships

Conditions can have complex relationships:

```go
// Paralyzed includes Incapacitated
paralyzedDef := &conditions.ConditionDefinition{
    Type: conditions.ConditionType("paralyzed"),
    // ...
    Includes: []conditions.ConditionType{
        conditions.ConditionType("incapacitated"),
    },
}

// Immunity relationships
eldrichKnightDef := &conditions.ConditionDefinition{
    Type: conditions.ConditionType("eldritch_ward"),
    // ...
    Immunities: []conditions.ConditionType{
        conditions.ConditionType("charmed"),
        conditions.ConditionType("frightened"),
    },
}
```

## Event Integration

Conditions automatically integrate with the event system:

```go
// The condition system listens for events and applies effects
bus.Subscribe(events.EventOnAttackRoll, func(ctx context.Context, event events.Event) error {
    // Poisoned conditions automatically add disadvantage
    // Blessed conditions automatically add bonuses
    // etc.
    return nil
})
```

## Examples

For complete examples of how to implement specific game systems:

- **D&D 5e**: See `/examples/dnd5e/conditions/`
- **Custom Systems**: The tests show various usage patterns

## Simplified Architecture

The conditions system follows our simplified patterns:

### Core Interface
```go
type Condition interface {
    Ref() *core.Ref
    Name() string
    Description() string
    Target() core.Entity
    Source() string
    Apply(target core.Entity, bus events.EventBus, opts ...ApplyOption) error
    IsActive() bool
    Remove(bus events.EventBus) error
    ToJSON() (json.RawMessage, error)
    IsDirty() bool
    MarkClean()
}
```

### Key Patterns

1. **SimpleCondition with effects.Core**: Common functionality for all conditions
2. **Options Pattern**: Flexible condition application without builder accumulation
3. **Data-Driven Loading**: Extract ref + JSON for routing to implementations
4. **Well-Defined Errors**: Clear communication for different failure modes
5. **Event-Driven Effects**: Conditions modify behavior through event subscriptions

### Benefits

- **Simpler Interface**: Only 11 essential methods (was more complex)
- **No Builder Accumulation**: Options pattern with immediate validation
- **Clear Error Handling**: Typed errors for different scenarios
- **Consistent with Features**: Same patterns across the toolkit
- **Runtime Flexibility**: Everything loads from data, no compile-time coupling

This infrastructure supports any RPG system while keeping implementations simple and maintainable.