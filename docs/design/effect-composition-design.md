# Effect Composition Design

## Overview

Keep domain-specific types (Proficiency, Condition, Feature) but compose them from shared effect building blocks.

## Core Effect Behaviors

### 1. Basic Effect Application
```go
// Core behavior all effects need
type EffectCore struct {
    id     string
    typ    string
    source string
    active bool
    subscriptions SubscriptionTracker
}

func (e *EffectCore) Apply(bus EventBus) error
func (e *EffectCore) Remove(bus EventBus) error
```

### 2. Conditional Effects
```go
// Effects that only apply under certain conditions
type ConditionalEffect interface {
    CheckCondition(ctx Context) bool
}

// Example: Proficiency with longsword only while not wearing heavy armor
// Example: Rage bonus only while raging
```

### 3. Resource Consuming Effects
```go
// Effects that consume resources when used
type ResourceConsumer interface {
    ConsumeResource(ctx Context) error
    HasResources(ctx Context) bool
}

// Example: Spell consumes spell slot
// Example: Ability uses X times per rest
```

### 4. Duration-Based Effects
```go
// Effects with time limits
type TemporaryEffect interface {
    GetDuration() Duration
    Tick(elapsed time.Duration) bool // returns false when expired
}

// Example: Bless lasts 1 minute
// Example: Rage lasts 10 rounds
```

### 5. Stacking Effects
```go
// Effects with stacking rules
type StackableEffect interface {
    GetStackingRule() StackingRule
    CanStackWith(other Effect) bool
}

// Example: Multiple bleeds stack
// Example: Bless doesn't stack with itself
```

### 6. Property Modifiers
```go
// Effects that modify entity properties
type PropertyModifier interface {
    GetProperty() string        // "strength", "inventory_slots", "speed"
    GetModificationType() ModType  // Add, Multiply, Set, Max, Min
    GetModifierValue() interface{} // Could be int, dice, or other
}

// Example: Gauntlets of Ogre Power set Strength to 19
// Example: Bag of Holding adds +500 inventory slots
// Example: Belt of Giant Strength sets Strength to 21 (or higher)
```

### 7. Capacity Modifiers
```go
// Effects that modify capacities/limits
type CapacityModifier interface {
    GetCapacityType() string    // "inventory", "attunement", "prepared_spells"
    GetModification() int       // +10 slots, +1 attunement
}

// Example: Bag of Holding adds inventory capacity
// Example: Artificer feature adds attunement slots
// Example: Wizard ability adds prepared spell slots
```

## Composition Examples

### Proficiency (Permanent Conditional Effect)
```go
type SimpleProficiency struct {
    EffectCore
    owner   Entity
    subject string
    
    // Optional conditional behavior
    condition ConditionalEffect // e.g., only with certain armor
}
```

### Condition (Temporary Effect)
```go
type SimpleCondition struct {
    EffectCore
    target Entity
    
    // Optional behaviors
    duration  TemporaryEffect
    stacking  StackableEffect
}
```

### Spell (Resource Consuming + Temporary)
```go
type SpellEffect struct {
    EffectCore
    caster Entity
    
    resource ResourceConsumer // spell slots
    duration TemporaryEffect  // concentration
}
```

### Class Feature (Conditional + Resource Limited)
```go
type ClassFeature struct {
    EffectCore
    owner Entity
    
    condition ConditionalEffect  // level requirements
    resource  ResourceConsumer   // uses per rest
}
```

### Magic Item (Property + Capacity Modifiers)
```go
type MagicItem struct {
    EffectCore
    wearer Entity
    
    property PropertyModifier    // Set STR to 19
    capacity CapacityModifier   // +500 inventory slots
    condition ConditionalEffect  // Only while worn/attuned
}

// Example: Item effects activate on equip events
func (m *MagicItem) Apply(bus EventBus) error {
    // Listen for equip/unequip events
    m.Subscribe(bus, EventItemEquipped, Priority, func(ctx Context, e Event) error {
        if equipEvent := e.(*ItemEquippedEvent); equipEvent.Item.ID == m.itemID {
            // Apply property modifiers
            m.property.Apply(equipEvent.Wearer)
            m.active = true
        }
        return nil
    })
    
    m.Subscribe(bus, EventItemUnequipped, Priority, func(ctx Context, e Event) error {
        if unequipEvent := e.(*ItemUnequippedEvent); unequipEvent.Item.ID == m.itemID {
            // Remove property modifiers
            m.property.Remove(unequipEvent.Wearer)
            m.active = false
        }
        return nil
    })
    
    return m.EffectCore.Apply(bus)
}
```

### Ability Score Improvement (Property Modifier)
```go
type AbilityBoost struct {
    EffectCore
    owner Entity
    
    property PropertyModifier   // +2 to ability score
    stacking StackableEffect    // How multiple boosts interact
}
```

## Benefits of This Approach

1. **Reusable Components**: Each behavior (conditional, resource, duration) is reusable
2. **Clear Composition**: Easy to see what behaviors an effect has
3. **Flexible Combinations**: Can create new effect types by mixing behaviors
4. **Domain Clarity**: Proficiency is still Proficiency, not generic Effect
5. **Testable**: Each behavior can be tested independently

## Implementation Strategy

1. Start with core EffectCore and SubscriptionTracker
2. Implement behavior interfaces one at a time
3. Refactor existing SimpleCondition and SimpleProficiency to use them
4. Add new behaviors as needed for resources, features, etc.

## Example: How They Work Together

```go
// Barbarian Rage: Conditional + Resource + Duration
type RageEffect struct {
    EffectCore
    
    // Only while not wearing heavy armor
    conditional ConditionalEffect
    
    // Uses per long rest
    resource ResourceConsumer
    
    // Lasts 1 minute
    duration TemporaryEffect
}

func (r *RageEffect) Apply(bus EventBus) error {
    // Check conditions
    if !r.conditional.CheckCondition(ctx) {
        return ErrConditionNotMet
    }
    
    // Consume resource
    if err := r.resource.ConsumeResource(ctx); err != nil {
        return err
    }
    
    // Apply core effect
    return r.EffectCore.Apply(bus)
}
```