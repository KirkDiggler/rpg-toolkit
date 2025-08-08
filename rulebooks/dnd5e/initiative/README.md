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
    // Collect participants with DEX modifiers
    entities := make(map[core.Entity]int)
    for _, char := range characters {
        participant := initiative.NewParticipant(char.ID, "character")
        entities[participant] = char.DexModifier
    }
    for _, monster := range monsters {
        participant := initiative.NewParticipant(monster.ID, "monster")
        entities[participant] = monster.DexModifier
    }
    
    // Roll and create tracker (pass nil to use default roller)
    order := initiative.RollForOrder(entities, nil)
    tracker := initiative.New(order)
    
    // Save to database
    data := tracker.ToData()
    db.SaveEncounter(encounterID, data)
    
    // Load from database
    data = db.LoadEncounter(encounterID)
    tracker = initiative.LoadFromData(data)
    
    // Now you can ask: whose turn is it?
    current := tracker.Current()
    
    // When turn ends
    next := tracker.Next()
    
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