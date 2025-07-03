# Journey 004: The Conditions System

## Date: 2025-01-03

## The Problem

We needed a flexible system for handling status effects, buffs, debuffs, and other temporary conditions that affect entities in the game. The challenge was creating infrastructure that could support diverse rulebook implementations without being too specific to any one system.

## Key Requirements

1. **Persistence**: Conditions need to survive across sessions
2. **Flexibility**: Support diverse mechanics (D&D concentration, Pathfinder conditions, custom systems)
3. **Event Integration**: Conditions modify game mechanics through the event system
4. **Relationships**: Complex interactions between conditions and their sources

## The Journey

### Initial Design: Concentration Focus

Started with a specific focus on D&D-style concentration mechanics, creating a dedicated `ConcentrationManager`. This felt too rulebook-specific.

### Realization: Conditions as Entities

Made the key decision that conditions themselves should be entities (ADR-0003). This allows:
- Persistence through standard entity storage
- Querying conditions like any other game object
- Consistent ID and type management

### The Relationship Abstraction

The breakthrough came when we realized concentration is just one type of relationship between conditions and their sources. We created a generic `RelationshipManager` supporting:

- **Concentration**: One effect per caster, broken by damage
- **Aura**: Range-based effects that move with source
- **Channeled**: Require continuous actions
- **Maintained**: Cost resources each turn
- **Linked**: Conditions that must be removed together
- **Dependent**: Conditions that require others to exist

### Self-Referencing Pattern

Implemented a pattern where condition handlers receive the condition itself, allowing proper subscription tracking:

```go
ApplyFunc: func(c *SimpleCondition, bus events.EventBus) error {
    c.Subscribe(bus, events.EventBeforeAttack, 50, handler)
    return nil
}
```

## Final Design

1. **Core Interface**: Minimal `Condition` interface extending `Entity`
2. **SimpleCondition**: Config-based implementation for common cases
3. **RelationshipManager**: Handles all inter-condition relationships generically
4. **Event Integration**: Conditions modify game state through event subscriptions

## Lessons Learned

1. **Generic Over Specific**: The relationship system is more powerful than just concentration
2. **Infrastructure, Not Implementation**: We provide the tools, rulebooks provide the specifics
3. **Config Pattern**: Using config structs for constructors keeps APIs clean and extensible

## What's Next

- Duration/expiration handling (turns, time, triggers)
- Condition registry for managing active conditions
- Integration with combat and other systems