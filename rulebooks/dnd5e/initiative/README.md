# Initiative Package

Simple turn order tracking for D&D 5e encounters.

## What This Is

A minimal package that:
1. Tracks whose turn it is
2. Manages rounds
3. Handles removing participants

That's it. No combat logic, no events, no complex patterns.

## Usage from Game Service

```go
// In your game service
func StartEncounter(characters []Character, monsters []Monster) {
    // Collect DEX modifiers
    entities := make(map[string]int)
    for _, char := range characters {
        entities[char.ID] = char.DexModifier
    }
    for _, monster := range monsters {
        entities[monster.ID] = monster.DexModifier
    }
    
    // Roll and create tracker
    order := initiative.RollForOrder(entities)
    tracker := initiative.New(order)
    
    // Now you can ask: whose turn is it?
    currentTurn := tracker.Current()
    
    // When turn ends
    nextTurn := tracker.Next()
    
    // When someone is defeated
    tracker.Remove(defeatedID)
}
```

## Why This Design?

- **Simple**: Just tracks turns, nothing else
- **Flexible**: Works for any kind of encounter
- **Testable**: Pure functions, no dependencies
- **Game-agnostic**: Doesn't know about characters, monsters, or combat

The game service handles everything else:
- What actions are available
- How attacks work  
- Victory conditions
- AI behavior
- Events and notifications