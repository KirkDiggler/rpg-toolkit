# Spatial Module

The spatial module provides 2D spatial positioning and movement capabilities for the RPG Toolkit. It supports multiple grid systems, entity placement, movement tracking, and event-driven spatial queries.

## Table of Contents

- [Overview](#overview)
- [Key Concepts](#key-concepts)
- [Quick Start](#quick-start)
- [Grid Systems](#grid-systems)
- [Room Management](#room-management)
- [Multi-Room Orchestration](#multi-room-orchestration)
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
- **Multi-Room Orchestration**: Connect and manage multiple rooms with typed connections
- **Event Integration**: Automatic event publishing for spatial changes
- **Query System**: Efficient spatial queries for game mechanics
- **Line of Sight**: Calculate visibility and obstacles
- **Distance Calculation**: Grid-appropriate distance calculations
- **Layout Patterns**: Common spatial arrangements (towers, dungeons, towns)

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

## Multi-Room Orchestration

The spatial module includes a powerful orchestration system for managing multiple connected rooms, enabling complex multi-room environments like dungeons, towns, towers, and more.

### Key Concepts

#### RoomOrchestrator
A `RoomOrchestrator` manages multiple rooms and their connections:
- **Room Management**: Add, remove, and track multiple rooms
- **Connection System**: Define how rooms link together
- **Entity Tracking**: Track entities across all managed rooms
- **Layout Patterns**: Organize rooms using common spatial arrangements
- **Event Integration**: Publish events for all orchestration changes

#### Connection Types
The orchestrator supports different connection types for various scenarios:

```go
// Connection types
spatial.ConnectionTypeDoor     // Standard doorway
spatial.ConnectionTypeStairs   // Vertical connections (floors)
spatial.ConnectionTypePassage  // Open corridors/hallways
spatial.ConnectionTypePortal   // Magical transport
spatial.ConnectionTypeBridge   // Spanning gaps/obstacles
spatial.ConnectionTypeTunnel   // Underground passages
```

#### Layout Patterns
Common spatial arrangements for multiple rooms:

```go
// Layout types
spatial.LayoutTypeTower      // Vertical stacking (floors)
spatial.LayoutTypeBranching  // Hub and spoke pattern
spatial.LayoutTypeGrid       // 2D grid arrangement
spatial.LayoutTypeOrganic    // Irregular connections
```

### Basic Orchestrator Usage

#### 1. Creating an Orchestrator

```go
// Create orchestrator
orchestrator := spatial.NewBasicRoomOrchestrator(spatial.BasicRoomOrchestratorConfig{
    ID:       "dungeon-orchestrator",
    Type:     "orchestrator",
    EventBus: eventBus,
    Layout:   spatial.LayoutTypeOrganic,
})
```

#### 2. Adding Rooms

```go
// Create rooms
room1 := spatial.NewBasicRoom(spatial.BasicRoomConfig{
    ID:       "entrance-hall",
    Type:     "chamber",
    Grid:     spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 20, Height: 20}),
    EventBus: eventBus,
})

room2 := spatial.NewBasicRoom(spatial.BasicRoomConfig{
    ID:       "treasure-room",
    Type:     "chamber", 
    Grid:     spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 15, Height: 15}),
    EventBus: eventBus,
})

// Add to orchestrator
err := orchestrator.AddRoom(room1)
err = orchestrator.AddRoom(room2)
```

#### 3. Creating Connections

```go
// Create a door connection
door := spatial.CreateDoorConnection(
    "main-door",
    "entrance-hall",        // From room
    "treasure-room",        // To room
    spatial.Position{X: 19, Y: 10}, // Exit position in entrance-hall
    spatial.Position{X: 0, Y: 7},   // Entry position in treasure-room
)

// Add connection to orchestrator
err := orchestrator.AddConnection(door)
```

#### 4. Moving Entities Between Rooms

```go
// Place entity in first room
hero := &Character{id: "hero", entityType: "character"}
err := room1.PlaceEntity(hero, spatial.Position{X: 10, Y: 10})

// Move entity to second room through connection
err = orchestrator.MoveEntityBetweenRooms(
    "hero",           // Entity ID
    "entrance-hall",  // From room
    "treasure-room",  // To room
    "main-door",      // Connection ID
)

// Check which room contains the entity
roomID, exists := orchestrator.GetEntityRoom("hero")
// roomID will be "treasure-room"
```

### Connection Helper Functions

The module provides helper functions for creating different connection types:

#### Door Connections
```go
// Bidirectional door (most common)
door := spatial.CreateDoorConnection(
    "door-1", "room-a", "room-b",
    spatial.Position{X: 10, Y: 0},  // Exit position
    spatial.Position{X: 5, Y: 19},  // Entry position
)
// Cost: 1.0, Reversible: true, Requirements: none
```

#### Stair Connections
```go
// Stairs between floors
stairs := spatial.CreateStairsConnection(
    "stairs-up", "floor-1", "floor-2",
    spatial.Position{X: 10, Y: 10},
    spatial.Position{X: 10, Y: 10},
    true, // goingUp - adds "can_climb" requirement
)
// Cost: 2.0, Reversible: true, Requirements: ["can_climb"] if going up
```

#### Portal Connections
```go
// Magical portal
portal := spatial.CreatePortalConnection(
    "magic-portal", "material-plane", "feywild",
    spatial.Position{X: 5, Y: 5},
    spatial.Position{X: 12, Y: 8},
    true, // bidirectional
)
// Cost: 0.5, Requirements: ["can_use_portals"]
```

#### Secret Passages
```go
// Hidden passage
secret := spatial.CreateSecretPassageConnection(
    "secret-passage", "library", "hidden-chamber",
    spatial.Position{X: 0, Y: 10},
    spatial.Position{X: 15, Y: 10},
    []string{"found_secret", "has_key"}, // Custom requirements
)
// Cost: 1.0, Reversible: true, Requirements: custom
```

### Layout Patterns

#### Tower Layout (Vertical Stacking)
Perfect for multi-floor buildings:

```go
// Create tower floors
floors := make([]spatial.Room, 5)
for i := 0; i < 5; i++ {
    floors[i] = spatial.NewBasicRoom(spatial.BasicRoomConfig{
        ID:   fmt.Sprintf("floor-%d", i+1),
        Type: "floor",
        Grid: spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 20, Height: 20}),
        EventBus: eventBus,
    })
    orchestrator.AddRoom(floors[i])
}

// Connect floors with stairs
for i := 0; i < len(floors)-1; i++ {
    stairs := spatial.CreateStairsConnection(
        fmt.Sprintf("stairs-%d-%d", i+1, i+2),
        floors[i].GetID(),
        floors[i+1].GetID(),
        spatial.Position{X: 10, Y: 10},
        spatial.Position{X: 10, Y: 10},
        true, // going up
    )
    orchestrator.AddConnection(stairs)
}

// Set tower layout
orchestrator.SetLayout(spatial.LayoutTypeTower)
```

#### Branching Layout (Hub and Spoke)
Great for dungeons with a central hub:

```go
// Create central hub
hub := spatial.NewBasicRoom(spatial.BasicRoomConfig{
    ID:   "central-hub",
    Type: "chamber",
    Grid: spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 30, Height: 30}),
    EventBus: eventBus,
})
orchestrator.AddRoom(hub)

// Create branching rooms
branches := []string{"north-wing", "south-wing", "east-wing", "west-wing"}
positions := []spatial.Position{
    {X: 15, Y: 0},   // North exit
    {X: 15, Y: 29},  // South exit
    {X: 29, Y: 15},  // East exit
    {X: 0, Y: 15},   // West exit
}

for i, branchID := range branches {
    // Create branch room
    branch := spatial.NewBasicRoom(spatial.BasicRoomConfig{
        ID:   branchID,
        Type: "chamber",
        Grid: spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 20, Height: 20}),
        EventBus: eventBus,
    })
    orchestrator.AddRoom(branch)
    
    // Connect to hub
    door := spatial.CreateDoorConnection(
        fmt.Sprintf("door-to-%s", branchID),
        "central-hub",
        branchID,
        positions[i],
        spatial.Position{X: 10, Y: 10}, // Center of branch room
    )
    orchestrator.AddConnection(door)
}

orchestrator.SetLayout(spatial.LayoutTypeBranching)
```

### Advanced Features

#### Pathfinding Between Rooms
```go
// Find path between rooms
path, err := orchestrator.FindPath("entrance-hall", "treasure-room", hero)
if err != nil {
    log.Fatal(err)
}

// path contains room IDs in order: ["entrance-hall", "treasure-room"]
fmt.Printf("Path: %v\n", path)
```

#### Entity Tracking
```go
// Track entity location
roomID, exists := orchestrator.GetEntityRoom("hero")
if exists {
    fmt.Printf("Hero is in room: %s\n", roomID)
}

// Get all entities in orchestrator
allRooms := orchestrator.GetAllRooms()
for roomID, room := range allRooms {
    entities := room.GetAllEntities()
    fmt.Printf("Room %s has %d entities\n", roomID, len(entities))
}
```

#### Connection Management
```go
// Get all connections for a room
connections := orchestrator.GetRoomConnections("entrance-hall")
for _, conn := range connections {
    fmt.Printf("Connection: %s (%s)\n", conn.GetID(), conn.GetConnectionType())
}

// Check if entity can move through connection
canMove := orchestrator.CanMoveEntityBetweenRooms("hero", "room-a", "room-b", "door-1")
```

#### Layout Metrics
```go
// Get layout information
orchestrator.SetLayout(spatial.LayoutTypeGrid)

// Layout metrics are available through events
eventBus.SubscribeFunc(spatial.EventLayoutChanged, 0, func(ctx context.Context, event events.Event) error {
    metrics, _ := event.Context().Get("metrics")
    layoutMetrics := metrics.(spatial.LayoutMetrics)
    
    fmt.Printf("Layout: %s\n", layoutMetrics.LayoutType)
    fmt.Printf("Rooms: %d\n", layoutMetrics.TotalRooms)
    fmt.Printf("Connections: %d\n", layoutMetrics.TotalConnections)
    fmt.Printf("Connectivity: %.2f\n", layoutMetrics.Connectivity)
    
    return nil
})
```

### Event System Integration

The orchestrator publishes events for all operations:

```go
// Subscribe to orchestrator events
eventBus.SubscribeFunc(spatial.EventRoomAdded, 0, func(ctx context.Context, event events.Event) error {
    orchestratorID, _ := event.Context().Get("orchestrator_id")
    room, _ := event.Context().Get("room")
    fmt.Printf("Room added to orchestrator %s\n", orchestratorID)
    return nil
})

eventBus.SubscribeFunc(spatial.EventConnectionAdded, 0, func(ctx context.Context, event events.Event) error {
    connection, _ := event.Context().Get("connection")
    conn := connection.(spatial.Connection)
    fmt.Printf("Connection added: %s -> %s\n", conn.GetFromRoom(), conn.GetToRoom())
    return nil
})

eventBus.SubscribeFunc(spatial.EventEntityTransitionBegan, 0, func(ctx context.Context, event events.Event) error {
    transition, _ := event.Context().Get("transition")
    fmt.Printf("Entity transition started\n")
    return nil
})
```

### Complete Multi-Room Example

```go
func CreateDungeonExample() {
    // Setup
    eventBus := events.NewBus()
    orchestrator := spatial.NewBasicRoomOrchestrator(spatial.BasicRoomOrchestratorConfig{
        ID:       "dungeon-orchestrator",
        Type:     "orchestrator",
        EventBus: eventBus,
        Layout:   spatial.LayoutTypeBranching,
    })
    
    // Create rooms
    entrance := spatial.NewBasicRoom(spatial.BasicRoomConfig{
        ID:       "entrance",
        Type:     "chamber",
        Grid:     spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 20, Height: 20}),
        EventBus: eventBus,
    })
    
    corridor := spatial.NewBasicRoom(spatial.BasicRoomConfig{
        ID:       "corridor",
        Type:     "hallway",
        Grid:     spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 30, Height: 10}),
        EventBus: eventBus,
    })
    
    treasureRoom := spatial.NewBasicRoom(spatial.BasicRoomConfig{
        ID:       "treasure",
        Type:     "chamber",
        Grid:     spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 15, Height: 15}),
        EventBus: eventBus,
    })
    
    // Add rooms to orchestrator
    orchestrator.AddRoom(entrance)
    orchestrator.AddRoom(corridor)
    orchestrator.AddRoom(treasureRoom)
    
    // Create connections
    door1 := spatial.CreateDoorConnection(
        "entrance-to-corridor",
        "entrance", "corridor",
        spatial.Position{X: 19, Y: 10},
        spatial.Position{X: 0, Y: 5},
    )
    
    door2 := spatial.CreateDoorConnection(
        "corridor-to-treasure",
        "corridor", "treasure",
        spatial.Position{X: 29, Y: 5},
        spatial.Position{X: 0, Y: 7},
    )
    
    orchestrator.AddConnection(door1)
    orchestrator.AddConnection(door2)
    
    // Place entities
    hero := &Character{id: "hero", entityType: "character"}
    monster := &Character{id: "orc", entityType: "monster"}
    
    entrance.PlaceEntity(hero, spatial.Position{X: 5, Y: 5})
    treasureRoom.PlaceEntity(monster, spatial.Position{X: 10, Y: 10})
    
    // Move hero through dungeon
    orchestrator.MoveEntityBetweenRooms("hero", "entrance", "corridor", "entrance-to-corridor")
    orchestrator.MoveEntityBetweenRooms("hero", "corridor", "treasure", "corridor-to-treasure")
    
    // Hero is now in treasure room with the monster
    heroRoom, _ := orchestrator.GetEntityRoom("hero")
    fmt.Printf("Hero is in: %s\n", heroRoom) // "treasure"
}
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

#### RoomOrchestrator Interface
```go
type RoomOrchestrator interface {
    core.Entity
    EventBusIntegration
    
    // Room management
    AddRoom(room Room) error
    RemoveRoom(roomID string) error
    GetRoom(roomID string) (Room, bool)
    GetAllRooms() map[string]Room
    
    // Connection management
    AddConnection(connection Connection) error
    RemoveConnection(connectionID string) error
    GetConnection(connectionID string) (Connection, bool)
    GetRoomConnections(roomID string) []Connection
    GetAllConnections() map[string]Connection
    
    // Entity movement
    MoveEntityBetweenRooms(entityID, fromRoom, toRoom, connectionID string) error
    CanMoveEntityBetweenRooms(entityID, fromRoom, toRoom, connectionID string) bool
    GetEntityRoom(entityID string) (string, bool)
    
    // Pathfinding
    FindPath(fromRoom, toRoom string, entity core.Entity) ([]string, error)
    
    // Layout management
    GetLayout() LayoutType
    SetLayout(layout LayoutType) error
}
```

#### Connection Interface
```go
type Connection interface {
    core.Entity
    
    GetConnectionType() ConnectionType
    GetFromRoom() string
    GetToRoom() string
    GetFromPosition() Position
    GetToPosition() Position
    IsPassable(entity core.Entity) bool
    GetTraversalCost(entity core.Entity) float64
    IsReversible() bool
    GetRequirements() []string
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

#### RoomOrchestrator Interface
```go
type RoomOrchestrator interface {
    core.Entity
    EventBusIntegration
    
    // Room management
    AddRoom(room Room) error
    RemoveRoom(roomID string) error
    GetRoom(roomID string) (Room, bool)
    GetAllRooms() map[string]Room
    
    // Connection management
    AddConnection(connection Connection) error
    RemoveConnection(connectionID string) error
    GetConnection(connectionID string) (Connection, bool)
    GetRoomConnections(roomID string) []Connection
    GetAllConnections() map[string]Connection
    
    // Entity movement
    MoveEntityBetweenRooms(entityID, fromRoom, toRoom, connectionID string) error
    CanMoveEntityBetweenRooms(entityID, fromRoom, toRoom, connectionID string) bool
    GetEntityRoom(entityID string) (string, bool)
    
    // Pathfinding
    FindPath(fromRoom, toRoom string, entity core.Entity) ([]string, error)
    
    // Layout management
    GetLayout() LayoutType
    SetLayout(layout LayoutType) error
}
```

#### Connection Interface
```go
type Connection interface {
    core.Entity
    
    GetConnectionType() ConnectionType
    GetFromRoom() string
    GetToRoom() string
    GetFromPosition() Position
    GetToPosition() Position
    IsPassable(entity core.Entity) bool
    GetTraversalCost(entity core.Entity) float64
    IsReversible() bool
    GetRequirements() []string
}
```

### Constructors

#### Grid Constructors
```go
// Square grid
func NewSquareGrid(config SquareGridConfig) *SquareGrid

// Hex grid
func NewHexGrid(config HexGridConfig) *HexGrid

// Gridless room
func NewGridlessRoom(config GridlessConfig) *GridlessRoom
```

#### Room Constructor
```go
func NewBasicRoom(config BasicRoomConfig) *BasicRoom
```

#### Orchestrator Constructor
```go
func NewBasicRoomOrchestrator(config BasicRoomOrchestratorConfig) *BasicRoomOrchestrator
```

#### Connection Constructor
```go
func NewBasicConnection(config BasicConnectionConfig) *BasicConnection
```

### Connection Helper Functions

The module provides helper functions for creating common connection types:

```go
// Door connection (bidirectional, cost 1.0)
func CreateDoorConnection(id, fromRoom, toRoom string, fromPos, toPos Position) *BasicConnection

// Stair connection (bidirectional, cost 2.0, may have climb requirement)
func CreateStairsConnection(id, fromRoom, toRoom string, fromPos, toPos Position, goingUp bool) *BasicConnection

// Secret passage (bidirectional, cost 1.0, requires discovery)
func CreateSecretPassageConnection(id, fromRoom, toRoom string, fromPos, toPos Position, requirements []string) *BasicConnection

// Portal connection (configurable direction, cost 0.5, requires portal use)
func CreatePortalConnection(id, fromRoom, toRoom string, fromPos, toPos Position, bidirectional bool) *BasicConnection

// Bridge connection (bidirectional, cost 1.0)
func CreateBridgeConnection(id, fromRoom, toRoom string, fromPos, toPos Position) *BasicConnection

// Tunnel connection (bidirectional, cost 1.5)
func CreateTunnelConnection(id, fromRoom, toRoom string, fromPos, toPos Position) *BasicConnection
```

### Query System

```go
// Query handler
func NewSpatialQueryHandler() *SpatialQueryHandler

// Query utilities
func NewQueryUtils(eventBus events.EventBus) *QueryUtils
```

### Event Constants

#### Single Room Events
```go
const (
    EventEntityPlaced   = "spatial.entity.placed"
    EventEntityMoved    = "spatial.entity.moved"
    EventEntityRemoved  = "spatial.entity.removed"
    EventRoomCreated    = "spatial.room.created"
)
```

#### Multi-Room Orchestration Events
```go
const (
    EventRoomAdded             = "spatial.orchestrator.room_added"
    EventRoomRemoved           = "spatial.orchestrator.room_removed"
    EventConnectionAdded       = "spatial.orchestrator.connection_added"
    EventConnectionRemoved     = "spatial.orchestrator.connection_removed"
    EventEntityTransitionBegan = "spatial.orchestrator.entity_transition_began"
    EventEntityTransitionEnded = "spatial.orchestrator.entity_transition_ended"
    EventLayoutChanged         = "spatial.orchestrator.layout_changed"
)
```

#### Query Events
```go
const (
    EventQueryPositionsInRange = "spatial.query.positions_in_range"
    EventQueryEntitiesInRange  = "spatial.query.entities_in_range"
    EventQueryLineOfSight      = "spatial.query.line_of_sight"
    EventQueryMovement         = "spatial.query.movement"
    EventQueryPlacement        = "spatial.query.placement"
)
```

### Types and Constants

#### Connection Types
```go
const (
    ConnectionTypeDoor    ConnectionType = "door"
    ConnectionTypeStairs  ConnectionType = "stairs"
    ConnectionTypePassage ConnectionType = "passage"
    ConnectionTypePortal  ConnectionType = "portal"
    ConnectionTypeBridge  ConnectionType = "bridge"
    ConnectionTypeTunnel  ConnectionType = "tunnel"
)
```

#### Layout Types
```go
const (
    LayoutTypeTower     LayoutType = "tower"
    LayoutTypeBranching LayoutType = "branching"
    LayoutTypeGrid      LayoutType = "grid"
    LayoutTypeOrganic   LayoutType = "organic"
)
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

## Advanced Usage Patterns

### Error Handling and Validation

```go
// Always check for errors when working with orchestrators
orchestrator := spatial.NewBasicRoomOrchestrator(config)

// Validate room addition
if err := orchestrator.AddRoom(room); err != nil {
    log.Printf("Failed to add room: %v", err)
    return err
}

// Validate connection requirements
if !orchestrator.CanMoveEntityBetweenRooms(entityID, fromRoom, toRoom, connectionID) {
    log.Printf("Entity %s cannot move through connection %s", entityID, connectionID)
    return errors.New("movement blocked")
}

// Safe entity movement with rollback
if err := orchestrator.MoveEntityBetweenRooms(entityID, fromRoom, toRoom, connectionID); err != nil {
    log.Printf("Movement failed: %v", err)
    // Handle failure (entity remains in original room)
}
```

### Event-Driven Game Logic

```go
// Set up event handlers for game mechanics
eventBus.SubscribeFunc(spatial.EventEntityTransitionBegan, 0, func(ctx context.Context, event events.Event) error {
    // Handle entity entering new room
    transition := event.Context().Get("transition").(spatial.Transition)
    
    // Trigger room-specific events (traps, encounters, etc.)
    return triggerRoomEvents(transition.GetToRoom(), transition.GetEntity())
})

eventBus.SubscribeFunc(spatial.EventConnectionAdded, 0, func(ctx context.Context, event events.Event) error {
    // Update minimap or UI when new connections are discovered
    connection := event.Context().Get("connection").(spatial.Connection)
    return updateGameMap(connection)
})
```

### Dynamic Connection Management

```go
// Create locked door that can be unlocked
door := spatial.CreateDoorConnection("treasure-door", "hallway", "treasure-room", pos1, pos2)
door.SetPassable(false) // Initially locked
door.AddRequirement("has_key")
orchestrator.AddConnection(door)

// Unlock door when key is found
func unlockDoor(orchestrator spatial.RoomOrchestrator, connectionID string) error {
    if conn, exists := orchestrator.GetConnection(connectionID); exists {
        if basicConn, ok := conn.(*spatial.BasicConnection); ok {
            basicConn.SetPassable(true)
            basicConn.RemoveRequirement("has_key")
            return nil
        }
    }
    return errors.New("connection not found")
}
```

### Performance Optimization

```go
// For large orchestrators, consider batching operations
func addMultipleRooms(orchestrator spatial.RoomOrchestrator, rooms []spatial.Room) error {
    // Add rooms in batch to reduce event overhead
    for _, room := range rooms {
        if err := orchestrator.AddRoom(room); err != nil {
            return fmt.Errorf("failed to add room %s: %w", room.GetID(), err)
        }
    }
    return nil
}

// Use pathfinding sparingly for large orchestrators
func findOptimalPath(orchestrator spatial.RoomOrchestrator, entity core.Entity, fromRoom, toRoom string) ([]string, error) {
    // Cache paths for frequently used routes
    cacheKey := fmt.Sprintf("%s-%s-%s", entity.GetType(), fromRoom, toRoom)
    if cachedPath, exists := pathCache[cacheKey]; exists {
        return cachedPath, nil
    }
    
    path, err := orchestrator.FindPath(fromRoom, toRoom, entity)
    if err == nil {
        pathCache[cacheKey] = path
    }
    return path, err
}
```

### Layout-Specific Patterns

```go
// Tower layout - vertical progression
func createTowerDungeon(orchestrator spatial.RoomOrchestrator, floors int) {
    orchestrator.SetLayout(spatial.LayoutTypeTower)
    
    for i := 0; i < floors; i++ {
        // Create floor room
        floor := createFloorRoom(i)
        orchestrator.AddRoom(floor)
        
        // Connect to previous floor
        if i > 0 {
            stairs := spatial.CreateStairsConnection(
                fmt.Sprintf("stairs-%d", i),
                fmt.Sprintf("floor-%d", i-1),
                fmt.Sprintf("floor-%d", i),
                spatial.Position{X: 10, Y: 10},
                spatial.Position{X: 10, Y: 10},
                true, // going up
            )
            orchestrator.AddConnection(stairs)
        }
    }
}

// Branching layout - hub and spoke
func createBranchingDungeon(orchestrator spatial.RoomOrchestrator, hubRoom spatial.Room, branches []spatial.Room) {
    orchestrator.SetLayout(spatial.LayoutTypeBranching)
    orchestrator.AddRoom(hubRoom)
    
    for i, branch := range branches {
        orchestrator.AddRoom(branch)
        
        // Connect each branch to hub
        door := spatial.CreateDoorConnection(
            fmt.Sprintf("hub-to-branch-%d", i),
            hubRoom.GetID(),
            branch.GetID(),
            getHubExitPosition(i),
            getBranchEntryPosition(),
        )
        orchestrator.AddConnection(door)
    }
}
```

### Testing Orchestrator Behavior

```go
func TestOrchestratorBehavior(t *testing.T) {
    // Setup
    eventBus := events.NewBus()
    orchestrator := spatial.NewBasicRoomOrchestrator(spatial.BasicRoomOrchestratorConfig{
        ID:       "test-orchestrator",
        EventBus: eventBus,
    })
    
    // Create test scenario
    room1 := createTestRoom("room1")
    room2 := createTestRoom("room2")
    orchestrator.AddRoom(room1)
    orchestrator.AddRoom(room2)
    
    door := spatial.CreateDoorConnection("door1", "room1", "room2", pos1, pos2)
    orchestrator.AddConnection(door)
    
    // Test entity movement
    entity := createTestEntity("hero")
    room1.PlaceEntity(entity, spatial.Position{X: 5, Y: 5})
    
    // Verify initial state
    roomID, exists := orchestrator.GetEntityRoom("hero")
    assert.True(t, exists)
    assert.Equal(t, "room1", roomID)
    
    // Test movement
    err := orchestrator.MoveEntityBetweenRooms("hero", "room1", "room2", "door1")
    assert.NoError(t, err)
    
    // Verify final state
    roomID, exists = orchestrator.GetEntityRoom("hero")
    assert.True(t, exists)
    assert.Equal(t, "room2", roomID)
    
    // Test pathfinding
    path, err := orchestrator.FindPath("room1", "room2", entity)
    assert.NoError(t, err)
    assert.Equal(t, []string{"room1", "room2"}, path)
}
```

## Integration with Other Modules

The spatial module integrates seamlessly with other RPG Toolkit modules:

- **Events**: Automatic event publishing for spatial changes
- **Core**: Uses `core.Entity` for type safety
- **Conditions**: Spatial conditions (e.g., "within 30 feet")
- **Spells**: Area of effect calculations
- **Combat**: Movement and positioning
- **Resources**: Movement costs and ability usage
- **Mechanics**: Integration with game rule systems

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