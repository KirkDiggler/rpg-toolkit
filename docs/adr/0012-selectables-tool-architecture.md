# ADR-0012: Selectables Tool Architecture

## Status
Proposed

## Context

The RPG Toolkit needs a universal weighted random selection system that can work with any content type. This "selectables" or "grabbag" tool would enable procedural content generation across multiple domains:

- Monster selection by challenge rating and environment
- Treasure generation with rarity tiers and level-appropriate loot
- Encounter activities, plot hooks, and environmental effects
- Generic weighted random choices for any game content

Current gap: No standardized way to perform weighted random selection with support for:
- Generic type safety
- Nested/hierarchical selection tables
- Context-based selection modifications
- Multiple selection modes (single, multiple, unique)
- Integration with existing dice systems

## Decision

We will implement a generic selectables tool as `tools/selectables` that provides:

### Core Architecture

1. **Generic Selection Interface**
   ```go
   type SelectionTable[T any] interface {
       Add(item T, weight int) SelectionTable[T]
       AddTable(name string, table SelectionTable[T], weight int) SelectionTable[T]
       Select(ctx context.Context) (T, error)
       SelectMany(ctx context.Context, count int) ([]T, error)
       SelectUnique(ctx context.Context, count int) ([]T, error)
   }
   ```

2. **Selection Context**
   ```go
   type SelectionContext struct {
       Conditions map[string]interface{}
       Modifiers  []SelectionModifier
       Dice       dice.Roller
   }
   ```

3. **Hierarchical Support**
   - Nested tables for category-then-item selection
   - Weighted table references within tables
   - Dynamic weight modification based on context

### Implementation Types

1. **BasicTable[T]**: Simple weighted random selection
2. **ConditionalTable[T]**: Context-aware selection with conditional weights
3. **HierarchicalTable[T]**: Nested category-based selection
4. **CompositeTable[T]**: Multiple selection strategies combined

### Integration Points

- **Dice Package**: Use existing dice.Roller for randomization
- **Events Package**: Selection events for debugging/analytics
- **Context Package**: Standard Go context for cancellation/deadlines

### Selection Modes

1. **Single Selection**: `Select()` - one item with replacement
2. **Multiple Selection**: `SelectMany()` - multiple items with replacement
3. **Unique Selection**: `SelectUnique()` - multiple items without replacement

## Rationale

### Why Generic Types?
- Type safety at compile time
- Reusable across all content types (monsters, items, encounters, etc.)
- Clear API contracts and better IDE support

### Why Hierarchical Tables?
- Real-world use cases often need category-then-item selection
- Enables complex procedural generation patterns
- Supports nested probability distributions

### Why Context-Based Selection?
- Game state affects selection probability (player level, environment, etc.)
- Conditional selection based on game rules
- Dynamic weight modification without table reconstruction

### Why Multiple Selection Modes?
- Different use cases need different selection behaviors
- Unique selection prevents duplicate results when inappropriate
- Flexible API supports various procedural generation needs

## Consequences

### Positive
- Universal tool for all weighted random selection needs
- Type-safe and reusable across the entire toolkit
- Supports complex procedural generation patterns
- Integrates cleanly with existing dice and events systems
- Follows established RPG Toolkit patterns

### Negative
- Additional complexity in the tools layer
- Generic interface may have learning curve
- Need to maintain backwards compatibility as use cases evolve

### Neutral
- New module increases toolkit surface area
- Will need comprehensive documentation and examples

## Implementation Plan

### Phase 1: Core Foundation
1. Define generic interfaces and basic types
2. Implement BasicTable[T] with weighted selection
3. Integration with dice package
4. Basic test coverage

### Phase 2: Advanced Features
1. Conditional selection with context
2. Hierarchical/nested table support
3. Multiple selection modes
4. Events integration

### Phase 3: Optimization & Polish
1. Performance optimization for large tables
2. Memory efficiency improvements
3. Comprehensive examples and documentation
4. Integration examples with other modules

## Advanced Features Discussion

### Conditional Exclusions
Provide optional hooks for games to implement exclusion logic without forcing it:
```go
table.SelectUnique(ctx, count, WithExclusionFilter(func(selected []T, candidate T) bool {
    return gameRules.IsIncompatible(selected, candidate)
}))
```

### Quantity Rolling
Essential for spawn mechanics and variable loot:
```go
// Roll quantity first, then items
table.SelectQuantity(ctx, dice.Parse("1d4+1")) // roll 2-5 items

// Or integrated approach
table.SelectVariable(ctx, "1d4+1") // quantity determined by dice expression
```

### Meta-Table Patterns
Support "roll to determine which table to roll" patterns while keeping it open-ended:
```go
// Encounter type determines monster table
encounterTypeTable := NewBasicTable[string]().
    Add("patrol", 60).
    Add("ambush", 30).
    Add("boss", 10)

monsterTables := map[string]SelectionTable[Monster]{
    "patrol": guardTable,
    "ambush": banditTable,
    "boss": bossTable,
}

// Games compose however they want
encounterType, _ := encounterTypeTable.Select(ctx)
monsters, _ := monsterTables[encounterType].SelectMany(ctx, 3)
```

### Depletion Mechanics
Provide hooks for games to handle inventory depletion:
```go
table.SelectWithCallback(ctx, func(selected T) {
    vendor.RemoveFromInventory(selected) // Game handles depletion
})
```

### Theme-Based Composition
Since tables are generic, they can contain other tables for unlimited nesting:
```go
// Pirates theme - tables containing tables
pirateWeapons := NewBasicTable[Item]().Add(cutlass, 50).Add(flintlock, 30)
pirateTreasure := NewBasicTable[Item]().Add(goldCoins, 70).Add(ruby, 20)

pirateTheme := NewBasicTable[SelectionTable[Item]]().
    Add(pirateWeapons, 60).
    Add(pirateTreasure, 40)

// Arbitrary composition patterns
dungeonTheme := map[string]interface{}{
    "monsters": undeadTable,
    "traps": tombTraps,
    "loot": ancientTreasure,
    "atmosphere": spookyEvents,
}
```

## Event Bus Integration

The selectables tool integrates with RPG Toolkit's event-driven architecture for reactive content generation.

### Event Patterns
Based on existing codebase patterns (`events.EventBus.SubscribeFunc`, `events.EventBus.Publish`):

```go
// Treasure chest interaction triggers loot generation
eventBus.SubscribeFunc("treasure_chest.opened", 0, func(ctx context.Context, event events.Event) error {
    chestID := event.Source().GetID()
    playerLevel := event.Context().Get("player_level")
    
    // Select appropriate loot table and generate contents
    lootCtx := selectables.NewContext().Set("player_level", playerLevel)
    items, err := treasureTable.SelectMany(lootCtx, dice.D4(1).GetValue())
    if err != nil {
        return err
    }
    
    // Publish loot generated event
    lootEvent := events.NewGameEvent("loot.generated", event.Source(), nil)
    lootEvent.Context().Set("items", items)
    lootEvent.Context().Set("source_chest", chestID)
    return eventBus.Publish(ctx, lootEvent)
})

// Monster spawn system reacts to player movement
eventBus.SubscribeFunc("spatial.entity.moved", 0, func(ctx context.Context, event events.Event) error {
    roomID, ok := event.Context().Get("room_id")
    if !ok {
        return nil
    }
    
    // Check if spawn should occur based on room
    if shouldSpawnInRoom(roomID) {
        spawnCtx := selectables.NewContext().
            Set("room_type", getRoomType(roomID)).
            Set("party_level", getPartyLevel())
            
        monsters, err := encounterTable.SelectMany(spawnCtx, 2)
        if err != nil {
            return err
        }
        
        // Publish spawn event
        spawnEvent := events.NewGameEvent("monsters.spawned", event.Source(), nil)
        spawnEvent.Context().Set("monsters", monsters)
        spawnEvent.Context().Set("room_id", roomID)
        return eventBus.Publish(ctx, spawnEvent)
    }
    return nil
})
```

### Event-Driven Use Cases

1. **Reactive Loot Generation**: Treasure chests, boss defeats, quest completion
2. **Dynamic Spawning**: Room entry, time-based events, player actions  
3. **Vendor Restocking**: Time passage, player transactions, market events
4. **Environmental Effects**: Weather changes, day/night cycles, seasonal events
5. **Narrative Content**: Plot hooks, random encounters, NPC interactions

### Selection Events
The tool can also publish its own events for debugging and analytics:
```go
// Selection debugging events
type SelectionEvent struct {
    TableID     string
    Context     SelectionContext
    Selected    interface{}
    Alternatives []interface{} // What could have been selected
    Weights     map[interface{}]int
}

eventBus.Publish(ctx, events.NewGameEvent("selectables.selection_made", table, nil))
```

## Entity Patterns for Integration

The selectables tool integrates with the RPG Toolkit's entity system (`core.Entity`) to provide seamless content generation across all game objects.

### Core Entity Interface Reference
Based on `/home/frank/projects/rpg-toolkit/core/entity.go`:
```go
type Entity interface {
    GetID() string   // Unique identifier within entity type scope
    GetType() string // Entity category (e.g., "character", "item", "location")
}
```

### Entity Patterns for Selectables

#### 1. Rollable Entity Interface
Entities that can hold and trigger selection tables:
```go
type RollableEntity interface {
    core.Entity
    GetSelectionTable() SelectionTable[interface{}]
    TriggerRoll(ctx SelectionContext) ([]interface{}, error)
    SetSelectionTable(table SelectionTable[interface{}])
}
```

#### 2. Loot Container Entities
Based on existing patterns like `FeatureEntity` in `/home/frank/projects/rpg-toolkit/tools/environments/room_builder.go`:
```go
// Treasure chests, reward caches, etc.
type TreasureChestEntity struct {
    id           string
    featureType  string // "treasure_chest"
    isOpened     bool
    lootTable    SelectionTable[Item]
    minimumLevel int
    properties   map[string]interface{}
}

func (t *TreasureChestEntity) GetID() string { return t.id }
func (t *TreasureChestEntity) GetType() string { return t.featureType }

// Event-driven interaction
func (t *TreasureChestEntity) OnInteract(actor core.Entity, eventBus events.EventBus) error {
    if t.isOpened {
        return nil // Already looted
    }
    
    ctx := selectables.NewContext().
        Set("player_level", getActorLevel(actor)).
        Set("chest_type", t.featureType)
    
    loot, err := t.lootTable.SelectMany(ctx, dice.D4(1).GetValue())
    if err != nil {
        return err
    }
    
    t.isOpened = true
    
    // Publish loot event
    event := events.NewGameEvent("treasure_chest.opened", t, actor)
    event.Context().Set("loot", loot)
    return eventBus.Publish(context.Background(), event)
}

// Vendors with restocking inventory
type VendorEntity struct {
    id        string
    name      string
    inventory SelectionTable[Item]
    location  string
    schedule  RestockSchedule
}

func (v *VendorEntity) GetID() string { return v.id }
func (v *VendorEntity) GetType() string { return "vendor" }
```

#### 3. Spawner Entities
Following patterns from spatial module entity tracking:
```go
// Monster spawn points
type SpawnerEntity struct {
    id           string
    spawnTable   SelectionTable[Monster]
    maxActive    int
    currentCount int
    respawnTimer time.Duration
    biome        string
    roomID       string
}

func (s *SpawnerEntity) GetID() string { return s.id }
func (s *SpawnerEntity) GetType() string { return "spawner" }

// Event-driven spawning based on spatial.entity.moved patterns
func (s *SpawnerEntity) OnPlayerEnterRoom(eventBus events.EventBus) error {
    if s.currentCount >= s.maxActive {
        return nil
    }
    
    ctx := selectables.NewContext().
        Set("biome", s.biome).
        Set("room_id", s.roomID).
        Set("current_spawns", s.currentCount)
    
    monsters, err := s.spawnTable.SelectMany(ctx, 2)
    if err != nil {
        return err
    }
    
    // Publish spawn event
    event := events.NewGameEvent("monsters.spawned", s, nil)
    event.Context().Set("monsters", monsters)
    event.Context().Set("room_id", s.roomID)
    return eventBus.Publish(context.Background(), event)
}

// Event trigger entities for narrative content
type EventTriggerEntity struct {
    id         string
    eventTable SelectionTable[GameEvent]
    conditions map[string]interface{}
    triggered  bool
}

func (e *EventTriggerEntity) GetID() string { return e.id }
func (e *EventTriggerEntity) GetType() string { return "event_trigger" }
```

#### 4. Selection Result Entities
Wrapper entities for generated content with provenance tracking:
```go
type GeneratedContentEntity struct {
    id           string
    contentType  string    // "loot", "monster", "event", etc.
    sourceTable  string    // Which table generated this
    sourceEntity string    // Which entity triggered the generation
    generatedAt  time.Time
    content      interface{}
}

func (g *GeneratedContentEntity) GetID() string { return g.id }
func (g *GeneratedContentEntity) GetType() string { return g.contentType }
```

#### 5. Context Provider Interface
Entities that provide context for selection decisions:
```go
type ContextProvider interface {
    GetSelectionContext() map[string]interface{}
}

// Rooms provide environmental context
type EnvironmentalRoom struct {
    // Embed existing room functionality
    spatial.Room
    biome       string
    dangerLevel int
    theme       string
    timeOfDay   string
}

func (r *EnvironmentalRoom) GetSelectionContext() map[string]interface{} {
    return map[string]interface{}{
        "biome":        r.biome,
        "danger_level": r.dangerLevel,
        "theme":        r.theme,
        "time_of_day":  r.timeOfDay,
        "room_size":    r.GetDimensions(),
    }
}

// Characters provide party context
type PartyMember struct {
    id    string
    level int
    class string
}

func (p *PartyMember) GetID() string { return p.id }
func (p *PartyMember) GetType() string { return "character" }

func (p *PartyMember) GetSelectionContext() map[string]interface{} {
    return map[string]interface{}{
        "level": p.level,
        "class": p.class,
    }
}
```

#### 6. Interactive Entity Patterns
Following existing interaction patterns from the toolkit:
```go
type InteractableEntity interface {
    core.Entity
    OnInteract(actor core.Entity, eventBus events.EventBus) error
    CanInteract(actor core.Entity) bool
}

// Quest givers with dynamic quest tables
type QuestGiverEntity struct {
    id         string
    name       string
    questTable SelectionTable[Quest]
    reputation map[string]int // Player reputation affects available quests
    location   string
}

func (q *QuestGiverEntity) GetID() string { return q.id }
func (q *QuestGiverEntity) GetType() string { return "quest_giver" }

func (q *QuestGiverEntity) OnInteract(actor core.Entity, eventBus events.EventBus) error {
    playerRep := q.reputation[actor.GetID()]
    
    ctx := selectables.NewContext().
        Set("player_reputation", playerRep).
        Set("location", q.location).
        Set("giver_type", "town_elder")
    
    availableQuests, err := q.questTable.SelectMany(ctx, 3)
    if err != nil {
        return err
    }
    
    // Publish quest offer event
    event := events.NewGameEvent("quest.offered", q, actor)
    event.Context().Set("quests", availableQuests)
    return eventBus.Publish(context.Background(), event)
}
```

### Integration with Existing Toolkit Components

#### Spatial Module Integration
```go
// Treasure chests as spatial entities (following FeatureEntity pattern)
treasureChest := &TreasureChestEntity{
    id:          "chest_1",
    featureType: "treasure_chest",
    lootTable:   treasureTable,
}

// Place in room (spatial module handles positioning)
room.PlaceEntity(treasureChest, spatial.Position{X: 5, Y: 5})

// Subscribe to spatial events
eventBus.SubscribeFunc("spatial.entity.interacted", 0, func(ctx context.Context, event events.Event) error {
    if chest, ok := event.Source().(*TreasureChestEntity); ok {
        return chest.OnInteract(event.Target().(core.Entity), eventBus)
    }
    return nil
})
```

#### Event Bus Integration
```go
// Subscribe to movement for spawner triggers
eventBus.SubscribeFunc("spatial.entity.moved", 0, func(ctx context.Context, event events.Event) error {
    roomID, ok := event.Context().Get("room_id")
    if !ok {
        return nil
    }
    
    // Find spawners in this room and check if they should activate
    spawners := getSpawnersForRoom(roomID)
    for _, spawner := range spawners {
        if spawner.ShouldSpawn() {
            return spawner.OnPlayerEnterRoom(eventBus)
        }
    }
    return nil
})
```

#### Dice Module Integration
```go
// Variable quantity selection using existing dice infrastructure
quantityRoll := dice.Parse("1d4+1") // 2-5 items
quantity := quantityRoll.Roll().GetValue()
loot, err := treasureTable.SelectMany(ctx, quantity)
```

## Example Usage

```go
// Monster selection by challenge rating
monsterTable := selectables.NewBasicTable[Monster]().
    Add(goblin, 50).
    Add(orc, 30).
    Add(troll, 10).
    Add(dragon, 1)

// Treasure with rarity tiers
treasureTable := selectables.NewHierarchicalTable[Item]().
    AddTable("common", commonItems, 70).
    AddTable("uncommon", uncommonItems, 25).
    AddTable("rare", rareItems, 5)

// Context-based selection
ctx := selectables.NewContext().
    Set("player_level", 5).
    Set("environment", "forest")

selectedMonster, err := monsterTable.Select(ctx)

// Event-driven treasure chest
eventBus.SubscribeFunc("treasure_chest.opened", 0, func(ctx context.Context, event events.Event) error {
    playerLevel := event.Context().Get("player_level")
    selectionCtx := selectables.NewContext().Set("player_level", playerLevel)
    
    loot, err := treasureTable.SelectMany(selectionCtx, dice.D4(1).GetValue())
    if err != nil {
        return err
    }
    
    lootEvent := events.NewGameEvent("loot.generated", event.Source(), nil)
    lootEvent.Context().Set("items", loot)
    return eventBus.Publish(ctx, lootEvent)
})
```

## Related ADRs
- ADR-0002: Hybrid Architecture (tools layer placement)
- ADR-0011: Environment Generation (procedural content integration)

## References
- GitHub Issue #59: Selectables Tool Requirements
- RPG Toolkit Architecture Guidelines
- Go Generics Best Practices