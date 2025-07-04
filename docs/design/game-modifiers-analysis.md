# Game Modifiers Analysis

## Overview

This document analyzes the common patterns across various game systems in rpg-toolkit to identify opportunities for shared abstractions.

## System Comparison

| System | Purpose | Duration | Target/Owner | Key Features |
|--------|---------|----------|--------------|--------------|
| **Conditions** | Status effects (poisoned, blessed) | Temporary | Target (who is affected) | Relationships, concentration, auras |
| **Proficiencies** | Capabilities (weapon, skill) | Permanent* | Owner + Subject | Subject tracking, source attribution |
| **Resources** | Consumables (HP, spell slots) | Variable | Owner | Current/max values, restoration rules |
| **Features** | Abilities (class features, racial traits) | Permanent | Owner | Usage limits, prerequisites |
| **Equipment** | Item-granted effects | While equipped | Wearer/wielder | Tied to item state |

*Some proficiencies could be conditional/temporary

## Common Core Pattern

All systems share these fundamental needs:

```go
// Pseudo-interface showing common pattern
type GameModifier {
    // Identity
    ID string
    Type string
    
    // Lifecycle
    Active bool
    Apply(EventBus) error
    Remove(EventBus) error
    
    // Event Management
    Subscriptions []string
    Subscribe(EventBus, EventType, Priority, Handler)
    UnsubscribeAll(EventBus)
    
    // Association
    Target/Owner Entity  // Who this affects/belongs to
    Source string        // What created/granted this
}
```

## Unique Requirements by System

### Conditions
- Complex relationships (concentration breaks others)
- Duration tracking
- Stacking rules
- Save DCs

### Proficiencies
- Subject identification (what you're proficient with)
- Conditional activation (e.g., only in certain armor)
- Proficiency bonus application

### Resources
- Numerical tracking (current/maximum)
- Restoration mechanics (short rest, long rest)
- Depletion consequences
- Resource pools (spell slots by level)

### Features
- Usage tracking (3/day, recharge on 5-6)
- Prerequisites and dependencies
- Level-based scaling
- Activation conditions

### Equipment
- Item association
- Equipped/unequipped state
- Attunement requirements
- Set bonuses

## Proposed Abstraction Hierarchy

```
core.Entity
    └── GameModifier (interface)
            ├── Effect (base implementation with subscription management)
            │   ├── SimpleCondition
            │   ├── SimpleProficiency
            │   ├── SimpleFeature
            │   └── SimpleEquipmentEffect
            └── Resource (different pattern - numerical tracking)
```

## Key Design Decisions

1. **Event-Driven**: All modifiers work through the event system
2. **Entity-Based**: Modifiers are entities for persistence/querying
3. **Composition**: Use embedding and interfaces over inheritance
4. **Lifecycle Management**: Consistent Apply/Remove pattern
5. **Subscription Tracking**: Automatic cleanup of event handlers

## Benefits of Shared Abstraction

1. **Code Reuse**: Eliminate duplicate subscription management
2. **Consistency**: Same patterns across all game systems
3. **Extensibility**: Easy to add new modifier types
4. **Testability**: Shared test utilities
5. **Documentation**: One pattern to learn

## Potential Names for Base Abstraction

- `Effect` - Simple, game-agnostic
- `GameModifier` - Descriptive but verbose
- `Modifier` - Clean but might conflict
- `StatusEffect` - Traditional but limiting
- `GameEffect` - Balanced clarity

## Next Steps

1. Create ADR for this architectural decision
2. Implement base `Effect` type in mechanics package
3. Refactor existing systems to use shared base
4. Update documentation with unified pattern