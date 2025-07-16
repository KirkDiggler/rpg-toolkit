# Spatial Module

The spatial module provides 2D spatial positioning and movement capabilities for the RPG Toolkit. It supports multiple grid systems, entity placement, movement tracking, and event-driven spatial queries.

## Table of Contents

- [Overview](#overview)
- [Key Concepts](#key-concepts)
- [Quick Start](#quick-start)
- [Grid Systems](#grid-systems)
- [Room Management](#room-management)
- [Entity Placement](#entity-placement)
- [Event System Integration](#event-system-integration)
- [Query System](#query-system)
- [API Reference](#api-reference)
- [Examples](#examples)
- [Testing](#testing)

## Overview

The spatial module is designed to handle 2D spatial positioning for tabletop RPGs and provides:

- **Multiple Grid Systems**: Square grids (D&D 5e style), hex grids, and gridless (theater-of-mind) systems
- **Entity Management**: Place, move, and track entities in spatial environments
- **Event Integration**: Automatic event publishing for spatial changes
- **Query System**: Efficient spatial queries for game mechanics
- **Line of Sight**: Calculate visibility and obstacles
- **Distance Calculation**: Grid-appropriate distance calculations

## Key Concepts

### Position
A `Position` represents a 2D coordinate in space:
```go
type Position struct {
    X float64 `json:"x"`
    Y float64 `json:"y"`
}
```

### Grid
The `Grid` interface defines how spatial calculations work:
- **Square Grid**: Uses D&D 5e distance rules (Chebyshev distance)
- **Hex Grid**: Uses cube coordinate system for hexagonal grids
- **Gridless**: Uses Euclidean distance for theater-of-mind play

### Room
A `Room` is a spatial container that implements `core.Entity` and manages:
- Entity placement and movement
- Grid-based spatial calculations
- Event publishing for changes
- Line of sight and range queries

### Placeable
Entities that can be placed in rooms should implement the `Placeable` interface:
```go
type Placeable interface {
    core.Entity
    GetSize() int
    BlocksMovement() bool
    BlocksLineOfSight() bool
}
```

## Quick Start

### 1. Basic Setup

```go
package main

import (
    "github.com/KirkDiggler/rpg-toolkit/core"
    "github.com/KirkDiggler/rpg-toolkit/events"
    "github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

func main() {
    // Create event bus
    eventBus := events.NewBus()
    
    // Create a 20x20 square grid
    grid := spatial.NewSquareGrid(spatial.SquareGridConfig{
        Width:  20,
        Height: 20,
    })
    
    // Create a room
    room := spatial.NewBasicRoom(spatial.BasicRoomConfig{
        ID:       "dungeon-room-1",
        Type:     "dungeon",
        Grid:     grid,
        EventBus: eventBus,
    })
    
    // Setup query system
    queryHandler := spatial.NewSpatialQueryHandler()
    queryHandler.RegisterRoom(room)
    queryHandler.RegisterWithEventBus(eventBus)
    
    // Create query utilities
    queryUtils := spatial.NewQueryUtils(eventBus)
}
```

### 2. Entity Placement

```go
// Create an entity (must implement core.Entity)
type Character struct {
    id   string
    name string
}

func (c *Character) GetID() string   { return c.id }
func (c *Character) GetType() string { return "character" }

// Create and place entity
hero := &Character{id: "hero-1", name: "Aragorn"}
position := spatial.Position{X: 10, Y: 10}

err := room.PlaceEntity(hero, position)
if err != nil {
    log.Fatal(err)
}
```

### 3. Movement and Queries

```go
// Move entity
newPosition := spatial.Position{X: 12, Y: 10}
err := room.MoveEntity("hero-1", newPosition)

// Query entities in range
entities := room.GetEntitiesInRange(position, 5.0)

// Check line of sight
losPositions := room.GetLineOfSight(position, newPosition)
blocked := room.IsLineOfSightBlocked(position, newPosition)
```

## Grid Systems

### Square Grid (D&D 5e Style)

Best for traditional tabletop RPGs with square battle mats:

```go
grid := spatial.NewSquareGrid(spatial.SquareGridConfig{
    Width:  20,  // 20 squares wide
    Height: 20,  // 20 squares tall
})
```

**Features:**
- Uses D&D 5e distance rules (Chebyshev distance)
- Diagonal movement costs the same as orthogonal
- 8 neighbors per position
- Integer coordinates recommended

### Hex Grid

Perfect for hex-based games:

```go
grid := spatial.NewHexGrid(spatial.HexGridConfig{
    Width:     15,
    Height:    15,
    PointyTop: true,  // false for flat-top hexes
})
```

**Features:**
- Uses cube coordinate system
- 6 neighbors per position
- More natural movement patterns
- Supports pointy-top and flat-top orientations

### Gridless (Theater-of-Mind)

For narrative-focused games without strict positioning:

```go
grid := spatial.NewGridlessRoom(spatial.GridlessConfig{
    Width:  100.0,  // Arbitrary units
    Height: 100.0,
})
```

**Features:**
- Uses Euclidean distance
- Allows fractional positioning
- 8 neighbors (conceptual)
- Flexible positioning

## Room Management

### Creating Rooms

```go
room := spatial.NewBasicRoom(spatial.BasicRoomConfig{
    ID:       "unique-room-id",
    Type:     "dungeon",  // or "outdoors", "tavern", etc.
    Grid:     grid,
    EventBus: eventBus,
})
```

### Room Operations

```go
// Place entity
err := room.PlaceEntity(entity, position)

// Move entity
err := room.MoveEntity(entityID, newPosition)

// Remove entity
err := room.RemoveEntity(entityID)

// Check if position is occupied
occupied := room.IsPositionOccupied(position)

// Get all entities at position
entities := room.GetEntitiesAt(position)

// Get entity position
pos, exists := room.GetEntityPosition(entityID)
```

## Entity Placement

### Implementing Placeable

For entities that need spatial properties:

```go
type Monster struct {
    id   string
    size int
    solid bool
}

func (m *Monster) GetID() string              { return m.id }
func (m *Monster) GetType() string            { return "monster" }
func (m *Monster) GetSize() int               { return m.size }
func (m *Monster) BlocksMovement() bool       { return m.solid }
func (m *Monster) BlocksLineOfSight() bool    { return m.solid }
```

### Placement Rules

- Entities cannot be placed on positions that would conflict with blocking entities
- The same entity can be moved to different positions
- Position validity depends on the grid system
- Events are automatically published for placement changes

## Event System Integration

The spatial module publishes events for all spatial changes:

### Event Types

```go
// Entity events
spatial.EventEntityPlaced   // "spatial.entity.placed"
spatial.EventEntityMoved    // "spatial.entity.moved"
spatial.EventEntityRemoved  // "spatial.entity.removed"

// Room events
spatial.EventRoomCreated    // "spatial.room.created"

// Query events
spatial.EventQueryPositionsInRange // "spatial.query.positions_in_range"
spatial.EventQueryEntitiesInRange  // "spatial.query.entities_in_range"
spatial.EventQueryLineOfSight      // "spatial.query.line_of_sight"
spatial.EventQueryMovement         // "spatial.query.movement"
spatial.EventQueryPlacement        // "spatial.query.placement"
```

### Event Listening

```go
// Listen for entity placement
eventBus.SubscribeFunc(spatial.EventEntityPlaced, 0, func(ctx context.Context, event events.Event) error {
    entity := event.Data().(core.Entity)
    position, _ := event.Context().Get("position")
    roomID, _ := event.Context().Get("room_id")
    
    fmt.Printf("Entity %s placed at %v in room %s\n", entity.GetID(), position, roomID)
    return nil
})
```

## Query System

The spatial module provides two ways to perform spatial queries:

### Direct Room Queries

```go
// Get entities within range
entities := room.GetEntitiesInRange(center, radius)

// Get positions within range
positions := room.GetPositionsInRange(center, radius)

// Line of sight
losPositions := room.GetLineOfSight(from, to)
blocked := room.IsLineOfSightBlocked(from, to)
```

### Event-Based Queries

For complex scenarios or when you need to query across multiple rooms:

```go
queryUtils := spatial.NewQueryUtils(eventBus)

// Query entities in range with filtering
filter := spatial.CreateCharacterFilter()  // Only characters
entities, err := queryUtils.QueryEntitiesInRange(ctx, center, radius, roomID, filter)

// Query movement validity
valid, path, distance, err := queryUtils.QueryMovement(ctx, entity, from, to, roomID)

// Query line of sight
positions, blocked, err := queryUtils.QueryLineOfSight(ctx, from, to, roomID)
```

### Entity Filters

Built-in filters for common queries:

```go
// Pre-built filters
characterFilter := spatial.CreateCharacterFilter()
monsterFilter := spatial.CreateMonsterFilter()
combatantFilter := spatial.CreateCombatantFilter()  // Characters + monsters

// Custom filters
filter := spatial.NewSimpleEntityFilter().
    WithEntityTypes("character", "npc").
    WithExcludeIDs("hero-1")
```

## API Reference

### Core Interfaces

#### Grid Interface
```go
type Grid interface {
    GetShape() GridShape
    IsValidPosition(pos Position) bool
    GetDimensions() Dimensions
    Distance(from, to Position) float64
    GetNeighbors(pos Position) []Position
    IsAdjacent(pos1, pos2 Position) bool
    GetLineOfSight(from, to Position) []Position
    GetPositionsInRange(center Position, radius float64) []Position
}
```

#### Room Interface
```go
type Room interface {
    core.Entity
    GetGrid() Grid
    PlaceEntity(entity core.Entity, pos Position) error
    MoveEntity(entityID string, newPos Position) error
    RemoveEntity(entityID string) error
    GetEntitiesAt(pos Position) []core.Entity
    GetEntityPosition(entityID string) (Position, bool)
    GetAllEntities() map[string]core.Entity
    GetEntitiesInRange(center Position, radius float64) []core.Entity
    IsPositionOccupied(pos Position) bool
    CanPlaceEntity(entity core.Entity, pos Position) bool
    GetPositionsInRange(center Position, radius float64) []Position
    GetLineOfSight(from, to Position) []Position
    IsLineOfSightBlocked(from, to Position) bool
}
```

### Grid Constructors

```go
// Square grid
func NewSquareGrid(config SquareGridConfig) *SquareGrid

// Hex grid
func NewHexGrid(config HexGridConfig) *HexGrid

// Gridless room
func NewGridlessRoom(config GridlessConfig) *GridlessRoom
```

### Room Constructor

```go
func NewBasicRoom(config BasicRoomConfig) *BasicRoom
```

### Query System

```go
// Query handler
func NewSpatialQueryHandler() *SpatialQueryHandler

// Query utilities
func NewQueryUtils(eventBus events.EventBus) *QueryUtils
```

## Examples

### Complete Combat Scenario

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/KirkDiggler/rpg-toolkit/core"
    "github.com/KirkDiggler/rpg-toolkit/events"
    "github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type Combatant struct {
    id        string
    name      string
    entityType string
    size      int
    blocking  bool
}

func (c *Combatant) GetID() string              { return c.id }
func (c *Combatant) GetType() string            { return c.entityType }
func (c *Combatant) GetSize() int               { return c.size }
func (c *Combatant) BlocksMovement() bool       { return c.blocking }
func (c *Combatant) BlocksLineOfSight() bool    { return c.blocking }

func main() {
    // Setup
    eventBus := events.NewBus()
    
    grid := spatial.NewSquareGrid(spatial.SquareGridConfig{
        Width:  20,
        Height: 20,
    })
    
    room := spatial.NewBasicRoom(spatial.BasicRoomConfig{
        ID:       "combat-room",
        Type:     "dungeon",
        Grid:     grid,
        EventBus: eventBus,
    })
    
    queryHandler := spatial.NewSpatialQueryHandler()
    queryHandler.RegisterRoom(room)
    queryHandler.RegisterWithEventBus(eventBus)
    
    queryUtils := spatial.NewQueryUtils(eventBus)
    
    // Create combatants
    hero := &Combatant{
        id:        "hero",
        name:      "Hero",
        entityType: "character",
        size:      1,
        blocking:  true,
    }
    
    orc := &Combatant{
        id:        "orc",
        name:      "Orc",
        entityType: "monster",
        size:      1,
        blocking:  true,
    }
    
    goblin := &Combatant{
        id:        "goblin",
        name:      "Goblin",
        entityType: "monster",
        size:      1,
        blocking:  true,
    }
    
    // Place entities
    room.PlaceEntity(hero, spatial.Position{X: 5, Y: 5})
    room.PlaceEntity(orc, spatial.Position{X: 8, Y: 8})
    room.PlaceEntity(goblin, spatial.Position{X: 12, Y: 6})
    
    // Query nearby enemies
    ctx := context.Background()
    enemyFilter := spatial.CreateMonsterFilter()
    
    nearbyEnemies, err := queryUtils.QueryEntitiesInRange(
        ctx, 
        spatial.Position{X: 5, Y: 5}, // Hero's position
        10.0,                        // 10 unit range
        "combat-room",
        enemyFilter,
    )
    
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Enemies within 10 units of hero: %d\n", len(nearbyEnemies))
    
    // Check line of sight to orc
    losPositions, blocked, err := queryUtils.QueryLineOfSight(
        ctx,
        spatial.Position{X: 5, Y: 5},  // Hero
        spatial.Position{X: 8, Y: 8},  // Orc
        "combat-room",
    )
    
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Line of sight blocked: %v\n", blocked)
    fmt.Printf("LOS path length: %d\n", len(losPositions))
    
    // Validate movement
    valid, path, distance, err := queryUtils.QueryMovement(
        ctx,
        hero,
        spatial.Position{X: 5, Y: 5},  // From
        spatial.Position{X: 7, Y: 7},  // To
        "combat-room",
    )
    
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Movement valid: %v, distance: %.2f\n", valid, distance)
    fmt.Printf("Path length: %d\n", len(path))
    
    // Move hero if valid
    if valid {
        err = room.MoveEntity("hero", spatial.Position{X: 7, Y: 7})
        if err != nil {
            log.Fatal(err)
        }
        fmt.Println("Hero moved successfully!")
    }
}
```

### Grid System Comparison

```go
func compareGridSystems() {
    // Same positions for comparison
    from := spatial.Position{X: 2, Y: 2}
    to := spatial.Position{X: 6, Y: 6}
    
    // Square grid
    squareGrid := spatial.NewSquareGrid(spatial.SquareGridConfig{
        Width: 10, Height: 10,
    })
    
    // Hex grid
    hexGrid := spatial.NewHexGrid(spatial.HexGridConfig{
        Width: 10, Height: 10, PointyTop: true,
    })
    
    // Gridless
    gridlessRoom := spatial.NewGridlessRoom(spatial.GridlessConfig{
        Width: 10, Height: 10,
    })
    
    // Compare distances
    fmt.Printf("Distance from %v to %v:\n", from, to)
    fmt.Printf("Square Grid: %.2f\n", squareGrid.Distance(from, to))
    fmt.Printf("Hex Grid: %.2f\n", hexGrid.Distance(from, to))
    fmt.Printf("Gridless: %.2f\n", gridlessRoom.Distance(from, to))
    
    // Compare neighbors
    pos := spatial.Position{X: 5, Y: 5}
    fmt.Printf("\nNeighbors of %v:\n", pos)
    fmt.Printf("Square Grid: %d\n", len(squareGrid.GetNeighbors(pos)))
    fmt.Printf("Hex Grid: %d\n", len(hexGrid.GetNeighbors(pos)))
    fmt.Printf("Gridless: %d\n", len(gridlessRoom.GetNeighbors(pos)))
}
```

## Testing

The spatial module includes comprehensive tests. To run them:

```bash
go test ./...
```

### Test Structure

- `*_test.go` - Unit tests for each grid system
- `room_test.go` - Room functionality tests
- `query_handler_test.go` - Query system tests
- `examples_test.go` - Integration examples and usage patterns

### Mock Entities

For testing, use the provided mock entity pattern:

```go
type MockEntity struct {
    id       string
    entityType string
    size     int
    blocksMovement bool
    blocksLOS  bool
}

func (m *MockEntity) GetID() string              { return m.id }
func (m *MockEntity) GetType() string            { return m.entityType }
func (m *MockEntity) GetSize() int               { return m.size }
func (m *MockEntity) BlocksMovement() bool       { return m.blocksMovement }
func (m *MockEntity) BlocksLineOfSight() bool    { return m.blocksLOS }
```

## Integration with Other Modules

The spatial module integrates seamlessly with other RPG Toolkit modules:

- **Events**: Automatic event publishing for spatial changes
- **Core**: Uses `core.Entity` for type safety
- **Conditions**: Spatial conditions (e.g., "within 30 feet")
- **Spells**: Area of effect calculations
- **Combat**: Movement and positioning

## Performance Considerations

- **Query Caching**: The query system includes built-in caching
- **Event Throttling**: Consider throttling high-frequency movement events
- **Large Grids**: For very large areas, consider partitioning rooms
- **Memory Usage**: Remove entities from rooms when no longer needed

## Contributing

When contributing to the spatial module:

1. Ensure all tests pass
2. Add tests for new functionality
3. Follow the existing code patterns
4. Update this README for new features
5. Consider event system integration for new features

## License

Part of the RPG Toolkit - see main repository for license information.