# Selectables Tool

Universal weighted random selection system for the RPG Toolkit. Provides flexible, context-aware "grabbag" functionality that works with any content type through Go generics.

## Overview

The selectables tool enables procedural content generation across all RPG domains:

- **Monster selection** by challenge rating and environment
- **Treasure generation** with rarity tiers and level-appropriate loot  
- **Encounter activities**, plot hooks, and environmental effects
- **Generic weighted random choices** for any game content

## Key Features

- **Generic Type Safety**: Works with any type T through Go generics
- **Context-Aware Selection**: Weights can be modified based on game state
- **Multiple Selection Modes**: Single, multiple, and unique selection
- **Hierarchical Tables**: Nested category-then-item selection patterns  
- **Event Integration**: Full integration with RPG Toolkit's event system
- **Performance Optimized**: Caching and efficient algorithms
- **Thread Safe**: Concurrent access supported

## Quick Start

```go
package main

import (
    "github.com/KirkDiggler/rpg-toolkit/tools/selectables"
    "github.com/KirkDiggler/rpg-toolkit/dice"
)

// Define your content types
type Monster struct {
    Name string
    CR   int
}

type Item struct {
    Name   string
    Rarity string
}

func main() {
    // Create a monster table
    monsterTable := selectables.NewBasicTable[Monster](selectables.BasicTableConfig{
        ID: "forest_monsters",
    })

    // Add monsters with weights
    monsterTable.
        Add(Monster{Name: "Goblin", CR: 1}, 50).
        Add(Monster{Name: "Orc", CR: 2}, 30).
        Add(Monster{Name: "Troll", CR: 5}, 10).
        Add(Monster{Name: "Dragon", CR: 15}, 1)

    // Create selection context
    ctx := selectables.NewBasicSelectionContext().
        Set("player_level", 3).
        Set("environment", "forest")

    // Select a single monster
    monster, err := monsterTable.Select(ctx)
    if err != nil {
        panic(err)
    }
    
    // Select multiple monsters
    encounters, err := monsterTable.SelectMany(ctx, 3)
    if err != nil {
        panic(err)
    }

    // Select with dice-driven quantity
    randomEncounter, err := monsterTable.SelectVariable(ctx, "1d4+1")
    if err != nil {
        panic(err)
    }
}
```

## Selection Modes

### Single Selection
```go
monster, err := table.Select(ctx)
```

### Multiple Selection (with replacement)
```go
// Can select the same item multiple times
monsters, err := table.SelectMany(ctx, 3)
```

### Unique Selection (without replacement)
```go
// Each item can only be selected once
uniqueMonsters, err := table.SelectUnique(ctx, 3)
```

### Variable Quantity Selection
```go
// Quantity determined by dice expression
loot, err := treasureTable.SelectVariable(ctx, "2d6")
```

## Context-Aware Selection

Selection context allows dynamic weight modification based on game state:

```go
// Create context with game state
ctx := selectables.NewContextBuilder().
    SetInt("player_level", 5).
    SetString("biome", "forest").
    SetBool("is_night", true).
    Build()

// Different contexts can affect selection probability
dayCtx := ctx.Set("is_night", false)
nightCtx := ctx.Set("is_night", true)

// Same table, different outcomes based on context
dayEncounter, _ := encounterTable.Select(dayCtx)
nightEncounter, _ := encounterTable.Select(nightCtx)
```

## Event Integration

Automatic integration with RPG Toolkit's event system:

```go
// Subscribe to selection events
eventBus.SubscribeFunc("selectables.selection.completed", 0, func(ctx context.Context, event events.Event) error {
    tableID := event.Context().Get("table_id")
    selectedItems := event.Context().Get("selected_items")
    
    // Handle selection results
    return nil
})

// Create table with event publishing
table := selectables.NewBasicTable[Item](selectables.BasicTableConfig{
    ID: "treasure_chest_1",
    EventBus: eventBus,
    Configuration: selectables.TableConfiguration{
        EnableEvents: true,
        EnableDebugging: true,
    },
})
```

## Game Integration Patterns

### Treasure Chests
```go
type TreasureChest struct {
    id       string
    lootTable selectables.SelectionTable[Item]
    isOpened bool
}

func (t *TreasureChest) OnOpen(player core.Entity) ([]Item, error) {
    if t.isOpened {
        return nil, nil
    }
    
    ctx := selectables.NewBasicSelectionContext().
        Set("player_level", getPlayerLevel(player)).
        Set("chest_type", "common")
    
    loot, err := t.lootTable.SelectVariable(ctx, "1d4+1")
    if err != nil {
        return nil, err
    }
    
    t.isOpened = true
    return loot, nil
}
```

### Dynamic Spawning
```go
type SpawnerEntity struct {
    id         string
    spawnTable selectables.SelectionTable[Monster]
    biome      string
}

func (s *SpawnerEntity) OnPlayerEnter(eventBus events.EventBus) error {
    ctx := selectables.NewBasicSelectionContext().
        Set("biome", s.biome).
        Set("time_of_day", getCurrentTime())
    
    monsters, err := s.spawnTable.SelectMany(ctx, 2)
    if err != nil {
        return err
    }
    
    // Publish spawn event
    event := events.NewGameEvent("monsters.spawned", s, nil)
    event.Context().Set("monsters", monsters)
    return eventBus.Publish(context.Background(), event)
}
```

### Vendor Inventory
```go
type VendorEntity struct {
    id        string
    inventory selectables.SelectionTable[Item]
    location  string
}

func (v *VendorEntity) GetAvailableItems(player core.Entity) ([]Item, error) {
    ctx := selectables.NewBasicSelectionContext().
        Set("player_level", getPlayerLevel(player)).
        Set("location", v.location).
        Set("reputation", getPlayerReputation(player))
    
    return v.inventory.SelectMany(ctx, 10)
}
```

## Advanced Patterns

### Hierarchical Selection
```go
// Category selection first, then item from category
categoryTable := selectables.NewBasicTable[string](config).
    Add("weapons", 40).
    Add("armor", 30).
    Add("consumables", 20).
    Add("magic_items", 10)

itemTables := map[string]selectables.SelectionTable[Item]{
    "weapons": weaponTable,
    "armor": armorTable,
    "consumables": potionTable,
    "magic_items": magicTable,
}

// Roll category, then roll item from that category
category, _ := categoryTable.Select(ctx)
item, _ := itemTables[category].Select(ctx)
```

### Conditional Exclusions
```go
// Use SelectUnique with filtering logic
filter := func(selected []Item, candidate Item) bool {
    // Exclude duplicate item types
    for _, item := range selected {
        if item.Type == candidate.Type {
            return true // Exclude this candidate
        }
    }
    return false // Allow this candidate
}

// This pattern would be implemented in future versions
// uniqueItems, err := table.SelectUniqueWithFilter(ctx, 3, filter)
```

## Error Handling

The selectables tool provides specific error types for different failure scenarios:

```go
items, err := table.SelectMany(ctx, 5)
if err != nil {
    switch {
    case errors.Is(err, selectables.ErrEmptyTable):
        // Handle empty table
    case errors.Is(err, selectables.ErrInvalidCount):
        // Handle invalid count parameter
    case errors.Is(err, selectables.ErrInsufficientItems):
        // Handle unique selection with too few items
    default:
        // Handle other errors
    }
}
```

## Performance Considerations

- **Weight Caching**: Enable for repeated selections with same context
- **Thread Safety**: All operations are thread-safe by default
- **Memory Usage**: Tables store references, not copies of items
- **Event Overhead**: Disable events in production if not needed

```go
config := selectables.TableConfiguration{
    CacheWeights: true,    // Enable weight caching
    EnableEvents: false,   // Disable events for performance
    EnableDebugging: false, // Disable debug data
}
```

## Testing

The module follows the RPG Toolkit's testify suite patterns:

```go
func TestMonsterSelection(t *testing.T) {
    suite.Run(t, new(MonsterSelectionTestSuite))
}

type MonsterSelectionTestSuite struct {
    suite.Suite
    table selectables.SelectionTable[Monster]
    ctx   selectables.SelectionContext
}

func (s *MonsterSelectionTestSuite) SetupTest() {
    s.table = selectables.NewBasicTable[Monster](selectables.BasicTableConfig{
        ID: "test_table",
    })
    s.ctx = selectables.NewBasicSelectionContext()
}
```

## Integration with Other Modules

- **Core**: Uses `core.Entity` interface for event integration
- **Events**: Full event bus integration for analytics and debugging  
- **Dice**: Integrated with `dice.Roller` for randomization
- **Spatial**: Tables can be attached to spatial entities
- **Conditions**: Selection can trigger condition applications

## Architecture

The selectables tool follows the RPG Toolkit's design principles:

- **Infrastructure, not Implementation**: Provides tools, games define content
- **Event-Driven**: Communicates through events, not direct calls
- **Entity-Based**: Works with any entity implementing `core.Entity`
- **Generic Design**: Type-safe reusability across all content types

## Related Documentation

- [ADR-0012: Selectables Tool Architecture](../../docs/adr/0012-selectables-tool-architecture.md)
- [Issue #70: Core Implementation](https://github.com/KirkDiggler/rpg-toolkit/issues/70)
- [RPG Toolkit Architecture Guidelines](../../CLAUDE.md)

## License

This module is part of the RPG Toolkit and follows the same licensing terms.