# Journey 007: Enhanced Conditions for Discord Bot

## Context

The Discord bot currently has a basic condition system but needs significant enhancements to properly support D&D 5e gameplay. Based on analysis of the bot's codebase, we need to implement:

1. **Full mechanical effects** for all D&D 5e conditions
2. **Event-driven condition system** that modifies rolls and actions
3. **Condition relationships** (immunity, suppression, upgrades)
4. **Complex conditions** like exhaustion levels
5. **Better integration** with combat, saves, and movement

## Current State

### RPG Toolkit Conditions Module
- Basic `Condition` interface with Apply/Remove
- `SimpleCondition` implementation using effects.Core
- `RelationshipManager` for concentration, auras, etc.
- Event bus integration for subscriptions

### Discord Bot Needs
- 15 standard D&D conditions defined but only partially implemented
- Needs mechanical effects (advantage/disadvantage, auto-fails, etc.)
- Needs condition checking in combat, saves, movement
- Needs exhaustion levels (1-6) with progressive effects
- Needs condition immunity and suppression

## Design Goals

1. **Comprehensive Coverage**: All D&D 5e conditions with full mechanical effects
2. **Event Integration**: Conditions modify events (attacks, saves, checks, movement)
3. **Flexible Builder**: Easy creation of custom conditions for game-specific needs
4. **Type Safety**: Use Go's type system to prevent errors
5. **Performance**: Efficient condition checking during gameplay

## Implementation Plan

### Phase 1: Condition Types and Definitions

Create a condition type system with all D&D 5e conditions:

```go
// ConditionType represents a specific condition
type ConditionType string

const (
    ConditionBlinded       ConditionType = "blinded"
    ConditionCharmed       ConditionType = "charmed"
    ConditionDeafened      ConditionType = "deafened"
    ConditionExhaustion    ConditionType = "exhaustion"
    ConditionFrightened    ConditionType = "frightened"
    ConditionGrappled      ConditionType = "grappled"
    ConditionIncapacitated ConditionType = "incapacitated"
    ConditionInvisible     ConditionType = "invisible"
    ConditionParalyzed     ConditionType = "paralyzed"
    ConditionPetrified     ConditionType = "petrified"
    ConditionPoisoned      ConditionType = "poisoned"
    ConditionProne         ConditionType = "prone"
    ConditionRestrained    ConditionType = "restrained"
    ConditionStunned       ConditionType = "stunned"
    ConditionUnconscious   ConditionType = "unconscious"
)

// ConditionDefinition defines the mechanical effects of a condition
type ConditionDefinition struct {
    Type        ConditionType
    Name        string
    Description string
    Effects     []ConditionEffect
    Immunities  []ConditionType // Conditions this prevents
    Includes    []ConditionType // Other conditions this includes
}

// ConditionEffect represents a mechanical effect
type ConditionEffect struct {
    Type   EffectType
    Target string // "attacks", "saves", "checks", etc.
    Value  interface{}
}
```

### Phase 2: Mechanical Effects System

Implement the mechanical effects that conditions apply:

```go
type EffectType string

const (
    EffectAdvantage       EffectType = "advantage"
    EffectDisadvantage    EffectType = "disadvantage"
    EffectAutoFail        EffectType = "auto_fail"
    EffectImmunity        EffectType = "immunity"
    EffectSpeedReduction  EffectType = "speed_reduction"
    EffectIncapacitated   EffectType = "incapacitated"
    EffectNoReactions     EffectType = "no_reactions"
    EffectVulnerability   EffectType = "vulnerability"
    EffectResistance      EffectType = "resistance"
)

// Event handlers for each effect type
func applyAdvantageEffect(event events.Event) { /* ... */ }
func applyDisadvantageEffect(event events.Event) { /* ... */ }
func applyAutoFailEffect(event events.Event) { /* ... */ }
```

### Phase 3: Enhanced Condition Implementation

Build on SimpleCondition to create EnhancedCondition:

```go
type EnhancedCondition struct {
    *SimpleCondition
    conditionType ConditionType
    definition    *ConditionDefinition
    level         int // For exhaustion
    saveDC        int // For conditions that allow saves
    immunities    []ConditionType
}

// Builder pattern for easy creation
type ConditionBuilder struct {
    conditionType ConditionType
    target        core.Entity
    source        string
    duration      events.Duration
    saveDC        int
    level         int
}

func NewConditionBuilder(condType ConditionType) *ConditionBuilder {
    return &ConditionBuilder{
        conditionType: condType,
    }
}

func (b *ConditionBuilder) WithTarget(target core.Entity) *ConditionBuilder {
    b.target = target
    return b
}

func (b *ConditionBuilder) Build() *EnhancedCondition {
    // Create condition with all mechanical effects
}
```

### Phase 4: Condition Interactions

Implement immunity, suppression, and condition relationships:

```go
// ConditionManager tracks all conditions on entities
type ConditionManager struct {
    conditions map[string][]Condition // entity ID -> conditions
    immunities map[string][]ConditionType // entity ID -> immune to
}

func (cm *ConditionManager) CanApplyCondition(entity core.Entity, condType ConditionType) bool {
    // Check immunities
    if cm.IsImmune(entity, condType) {
        return false
    }
    
    // Check if a stronger version exists
    existing := cm.GetConditions(entity)
    for _, cond := range existing {
        if cm.suppresses(cond.Type(), condType) {
            return false
        }
    }
    
    return true
}

func (cm *ConditionManager) ApplyCondition(condition Condition) error {
    // Check if can apply
    // Remove weaker conditions
    // Apply new condition
    // Handle includes (e.g., Paralyzed includes Incapacitated)
}
```

### Phase 5: Exhaustion Levels

Special handling for the exhaustion condition:

```go
type ExhaustionCondition struct {
    *EnhancedCondition
    level int // 1-6
}

func (e *ExhaustionCondition) Apply(bus events.EventBus) error {
    // Apply effects based on level
    switch e.level {
    case 1:
        // Disadvantage on ability checks
    case 2:
        // Speed halved
    case 3:
        // Disadvantage on attacks and saves
    case 4:
        // Hit point maximum halved
    case 5:
        // Speed reduced to 0
    case 6:
        // Death
    }
}

func (e *ExhaustionCondition) IncreaseLevel() {
    if e.level < 6 {
        e.level++
        // Reapply effects
    }
}
```

### Phase 6: Event Integration

Integrate conditions with the event system:

```go
// Example: Blinded condition
func createBlindedEffects() []func(events.Event) {
    return []func(events.Event){
        // Attack rolls have disadvantage
        func(e events.Event) {
            if e.Type() == events.EventOnAttackRoll {
                if e.Source() == condition.Target() {
                    e.Context().AddModifier(events.NewModifier(
                        "blinded",
                        events.ModifierDisadvantage,
                        events.IntValue(1),
                        100,
                    ))
                }
            }
        },
        // Attacks against have advantage
        func(e events.Event) {
            if e.Type() == events.EventOnAttackRoll {
                if e.Target() == condition.Target() {
                    e.Context().AddModifier(events.NewModifier(
                        "blinded_target",
                        events.ModifierAdvantage,
                        events.IntValue(1),
                        100,
                    ))
                }
            }
        },
        // Auto-fail sight-based checks
        func(e events.Event) {
            if e.Type() == events.EventOnAbilityCheck {
                if requiresSight(e) {
                    e.Context().Set("auto_fail", true)
                }
            }
        },
    }
}
```

## Example Usage

```go
// Apply poisoned condition from a spell
poisoned := conditions.NewConditionBuilder(conditions.ConditionPoisoned).
    WithTarget(target).
    WithSource("ray_of_sickness").
    WithDuration(events.NewRoundsDuration(10)).
    WithSaveDC(15).
    Build()

err := conditionManager.ApplyCondition(poisoned)

// Apply exhaustion
exhaustion := conditions.NewExhaustionCondition(target, 1)
err := conditionManager.ApplyCondition(exhaustion)

// Check conditions in combat
func rollAttack(attacker, target core.Entity) {
    // Check if attacker is blinded, poisoned, etc.
    conditions := conditionManager.GetConditions(attacker)
    for _, cond := range conditions {
        // Conditions automatically modify the event
    }
}
```

## Benefits

1. **Complete D&D 5e Support**: All conditions with proper mechanical effects
2. **Event-Driven**: Conditions automatically apply their effects
3. **Extensible**: Easy to add custom conditions
4. **Type-Safe**: Compile-time checking prevents errors
5. **Discord Bot Ready**: Directly addresses all bot requirements

## Next Steps

1. Create condition_types.go with all definitions
2. Implement EnhancedCondition with builder
3. Create ConditionManager for tracking
4. Add exhaustion level support
5. Write comprehensive tests
6. Create examples for Discord bot integration

This enhanced condition system will provide the Discord bot with everything it needs for proper D&D 5e gameplay while maintaining the flexibility of the rpg-toolkit.