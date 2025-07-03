# Simple Combat Example

This example demonstrates the event-driven architecture of the RPG Toolkit through a simple combat scenario.

## Overview

The example shows:
- How events flow through the system
- Integration between the dice and events packages
- How modifiers from conditions affect calculations
- The flexibility of the architecture for RPG mechanics

## Architecture Flow

```
1. Attack Event (EventBeforeAttack)
   ├─ Blessed Handler (priority 50) → Adds d4 modifier
   └─ Combat Handler (priority 100) → Rolls attack
       │
       ├─ On Hit → Publishes Damage Event
       └─ On Miss → End

2. Damage Event (EventCalculateDamage)  
   ├─ Rage Handler (priority 50) → Adds +2 modifier
   └─ Combat Handler (priority 100) → Calculates damage
       │
       └─ Publishes Damage Applied Event

3. Damage Applied Event (EventAfterDamage)
   └─ Combat Handler → Displays result
```

## Key Concepts

### Event Bus
The central hub that manages all game events. Handlers subscribe to specific event types and are called in priority order.

### Modifiers
Conditions and effects add modifiers to events. These use the `ModifierValue` interface, allowing both:
- Flat values: `+2 (rage bonus)`
- Dice rolls: `+d4[3]=3 (blessed)`

### Handler Priority
Lower numbers execute first. This allows conditions to add modifiers before combat calculations use them:
- Priority 50: Condition handlers (rage, blessed)
- Priority 100: Core combat logic

### Event Context
Each event carries context data that can be:
- Read by handlers (`Get()`)
- Modified by handlers (`Set()`, `AddModifier()`)
- Passed to subsequent events

## Running the Example

```bash
cd examples/simple_combat
go run main.go
```

## Example Output

```
=== Simple Combat Example ===

Ragnar the Bold attacks Sneaky Goblin with a Greatsword!

1. Attack Roll Phase
   Attacker: Ragnar the Bold
   Target: Sneaky Goblin
   Base Roll: +d20[11]=11
   + blessed: +d4[4]=4
   Total: 15 vs AC 15
   Result: HIT!

2. Damage Calculation Phase
   Base Damage: +2d6[6,1]=7
   + rage: +2 (rage bonus)
   Total Damage: 9 slashing

3. Damage Applied Phase
   Sneaky Goblin takes 9 slashing damage!

=== Combat Complete ===
```

## Extending the Example

To add new features:

### Add a New Condition
```go
func registerSneakAttackHandler(bus *events.Bus) {
    bus.SubscribeFunc(events.EventCalculateDamage, 50, func(ctx context.Context, e events.Event) error {
        if isSneaking, ok := e.Context().Get("is_sneaking"); ok && isSneaking.(bool) {
            e.Context().AddModifier(events.NewModifier(
                "sneak_attack",
                events.ModifierDamageBonus,
                dice.D6(3), // 3d6 sneak attack
                100,
            ))
        }
        return nil
    })
}
```

### Add Critical Hits
```go
// In the attack handler, after rolling
if attackRoll.GetValue() == 20 {
    damageEvent.Context().Set("is_critical", true)
}

// In damage calculation
if isCrit, ok := e.Context().Get("is_critical"); ok && isCrit.(bool) {
    // Double the dice
}
```

### Add Saving Throws
```go
// After damage calculation
saveEvent := events.NewGameEvent(events.EventSavingThrow, target, nil)
saveEvent.Context().Set("save_type", "dexterity")
saveEvent.Context().Set("dc", 15)
bus.Publish(ctx, saveEvent)
```

## Design Benefits

1. **Extensible**: New conditions and effects can be added without modifying core combat
2. **Testable**: Each handler can be tested in isolation
3. **Flexible**: The same architecture supports any RPG system
4. **Clear**: Event flow is explicit and traceable
5. **Decoupled**: Combat logic doesn't know about specific conditions