# D&D Discord Bot Integration Guide

This guide shows how to integrate rpg-toolkit into the existing D&D Discord bot without a major rewrite.

## Quick Start

The toolkit provides infrastructure that can replace or enhance existing bot systems:

- **Event Bus** → Replace bot's EventBus with toolkit's events.Bus
- **Conditions** → Replace status effect system with toolkit conditions
- **Resources** → Manage spell slots, rage uses, ki points, etc.
- **Dice** → Use toolkit's dice roller (when implemented)
- **Proficiency** → Base system for proficiency tracking

## Integration Examples

### 1. Entity Wrappers

The bot's Character and Monster types can implement `core.Entity`:

```go
// Minimal wrapper - bot keeps its existing Character struct
type CharacterEntity struct {
    *character.Character // Embed existing character
}

func (c *CharacterEntity) ID() string   { return c.Character.ID }
func (c *CharacterEntity) Type() string { return "character" }

// Same for monsters
type MonsterEntity struct {
    *combat.Monster
}

func (m *MonsterEntity) ID() string   { return m.Monster.ID }
func (m *MonsterEntity) Type() string { return "monster" }
```

### 2. Event Bus Migration

Replace the bot's event system gradually:

```go
// Old bot code:
botEventBus.Publish(EventTypeOnAttackRoll, attackData)

// New code using toolkit:
toolkitBus.Publish(ctx, events.NewEvent("attack.roll", attackData))
```

### 3. Spell Slot Management

Replace manual spell slot tracking with resources:

```go
// Old: Manual tracking in Character struct
// New: Use resource pool
pool := resources.NewSimplePool(characterEntity)

// Add spell slots based on class/level
for level, count := range getSpellSlots(class, characterLevel) {
    resource := &resources.SimpleResource{
        ResourceID:   fmt.Sprintf("spell-slots-%d", level),
        ResourceName: fmt.Sprintf("Level %d Spell Slots", level),
        Current:      count,
        Max:          count,
        OwnerEntity:  characterEntity,
        Restorations: []resources.RestorationTrigger{
            {Trigger: "long_rest", Amount: -1},
        },
    }
    pool.Add(resource)
}
```

### 4. Condition Management

Replace status effects with toolkit conditions:

```go
// Create condition manager per combat/session
manager := conditions.NewSimpleManager()

// Apply conditions
poisoned := conditions.NewSimpleCondition(conditions.ConditionConfig{
    ID:          fmt.Sprintf("poisoned-%s", target.ID),
    Name:        "Poisoned",
    Description: "Disadvantage on attack rolls and ability checks",
    Duration:    conditions.DurationUntilRemoved,
    Target:      targetEntity,
})

manager.ApplyCondition(poisoned, bus)

// Check conditions during combat
activeConditions := manager.GetActiveConditions(character.ID)
for _, cond := range activeConditions {
    // Apply mechanical effects based on condition
}
```

### 5. Proficiency Bonus Calculation

Simple helper that matches D&D 5e rules:

```go
func CalculateProficiencyBonus(level int) int {
    return 2 + ((level - 1) / 4)
}
```

## Migration Strategy

### Phase 1: Parallel Systems (Current State)
- Keep existing bot systems
- Add toolkit as enhancement
- Test with non-critical features first

### Phase 2: Gradual Replacement
1. **Event Bus** - Route some events through toolkit
2. **Resources** - Start with spell slots
3. **Conditions** - Replace one condition at a time
4. **Combat** - Integrate attack/damage events

### Phase 3: Full Integration
- Remove duplicate systems
- Bot uses toolkit as primary infrastructure
- Custom D&D logic built on toolkit foundation

## Benefits

1. **Cleaner Architecture** - Separation of infrastructure from game rules
2. **Reusability** - Other game systems can use same infrastructure  
3. **Testing** - Toolkit components are independently testable
4. **Extensibility** - Easy to add new mechanics

## Example: Combat Turn

```go
// Subscribe to turn events
bus.Subscribe("turn.start", func(ctx context.Context, e events.Event) error {
    data := e.Data().(map[string]interface{})
    character := data["character"].(core.Entity)
    
    // Reset per-turn resources (like sneak attack)
    if pool := getCharacterPool(character.ID()); pool != nil {
        pool.ProcessRestoration("turn_start", bus)
    }
    
    // Check condition durations
    manager.ProcessDurations("turn", bus)
    
    return nil
})

// Character attacks
bus.Publish(ctx, events.NewEvent("attack.before", map[string]interface{}{
    "attacker": attackerEntity,
    "target":   targetEntity,
    "weapon":   weapon,
}))
```

## Next Steps

1. Start with spell slots - easiest to integrate
2. Add conditions one at a time
3. Route combat events through toolkit bus
4. Gradually replace bot's event system

The key is that the bot doesn't need to change its core structure - just wrap entities and use toolkit services where beneficial.