# 012: Spatial Module Design

## Context

We're designing a spatial module for the RPG Toolkit to handle room generation, entity placement, and spatial queries. This is inspired by the [dnd5e-roomgen](https://github.com/fadedpez/dnd5e-roomgen) project but adapted to fit the toolkit's event-driven architecture.

## Key Requirements

1. **Grid Flexibility**: Support for square grids, hex grids, and gridless rooms
2. **Event-Driven**: Integrate with toolkit's event system for spatial changes
3. **Storage Agnostic**: Follow toolkit patterns for persistence independence
4. **Comprehensive Queries**: Rich spatial query capabilities for game mechanics
5. **Toolkit Integration**: Use core.Entity interface and events.EventBus

## Design Overview

### Core Architecture

```
tools/spatial/
├── interfaces.go         # Core interfaces (Room, Grid, Placeable)
├── position.go           # Position and dimension types
├── room.go              # Room implementation
├── square_grid.go       # Square grid implementation
├── hex_grid.go          # Hex grid implementation
├── gridless.go          # Gridless room implementation
├── events.go            # Event types and data structures
├── query_handler.go     # Query handler implementation
├── query_utils.go       # Query utility functions
└── *_test.go           # Comprehensive test suite
```

### Core Interfaces

**Grid Interface** - Pluggable grid system:
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

**Room Interface** - Main spatial container:
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
}
```

**Placeable Interface** - For entities that can be placed:
```go
type Placeable interface {
    core.Entity
    GetSize() int
    BlocksMovement() bool
    BlocksLineOfSight() bool
}
```

### Grid Implementations

**Square Grid**: Traditional RPG grid with 8-directional movement
- Uses standard X,Y coordinates
- Euclidean distance calculations
- 8 neighbors per position

**Hex Grid**: Hexagonal grid with 6-directional movement
- Uses offset coordinates for display, cube coordinates for calculations
- Supports both pointy-top and flat-top orientations
- Cube coordinate system simplifies distance and neighbor calculations
- 6 neighbors per position

**Gridless**: Approximate positioning for theater-of-mind style play
- Positions are suggestions rather than strict constraints
- Distance calculations are approximate
- More flexible placement rules

### Event System Integration

**Core Events**:
- `spatial.entity.placed` - Entity placed in room
- `spatial.entity.moved` - Entity moved to new position
- `spatial.entity.removed` - Entity removed from room
- `spatial.room.created` - Room created

**Event Data Structures**:
```go
type EntityPlacedData struct {
    Entity   core.Entity
    Position Position
    Room     Room
}

type EntityMovedData struct {
    Entity      core.Entity
    OldPosition Position
    NewPosition Position
    Room        Room
}
```

### Spatial Query System

**Comprehensive Query Types**:
1. **Position Queries**: Get all positions within range (occupied/unoccupied)
2. **Entity Queries**: Get entities within range, at position, by type
3. **Line of Sight**: Check visibility between positions
4. **Movement Queries**: Valid movement positions within range
5. **Shape Queries**: Positions within circles, cones, lines, rectangles
6. **Pathfinding**: Find routes between positions
7. **Proximity**: Find nearest entities of specific types

**Query Event Pattern**:
```go
const (
    EventQueryPositionsInRange = "spatial.query.positions_in_range"
    EventQueryEntitiesInRange  = "spatial.query.entities_in_range"
    EventQueryLineOfSight      = "spatial.query.line_of_sight"
    // ... more query types
)
```

## Key Design Decisions

### 1. Hex Grid Coordinate System

**Challenge**: Hex grids don't map naturally to X,Y coordinates.

**Solution**: Use cube coordinates internally for calculations, convert to/from offset coordinates for display.

**Benefits**:
- Hex distance becomes simple: `(|x1-x2| + |y1-y2| + |z1-z2|) / 2`
- Neighbor calculations are consistent
- Rotation and pathfinding algorithms work naturally

### 2. Triple Entity Tracking

**Pattern**: Room maintains three maps:
- `entities map[string]core.Entity` - ID to entity lookup
- `positions map[string]Position` - ID to position lookup  
- `occupancy map[Position][]string` - Position to entity IDs lookup

**Benefits**:
- O(1) lookups for all common operations
- Efficient spatial queries
- Easy cleanup when entities are removed

### 3. Event-Driven State Changes

**Pattern**: All spatial changes emit events that other modules can listen to.

**Benefits**:
- Combat systems can react to movement
- Trap systems can respond to placement
- Lighting systems can update visibility
- Maintains loose coupling between systems

### 4. Pluggable Grid System

**Pattern**: Grid interface allows different spatial rules without changing Room logic.

**Benefits**:
- Easy to add new grid types
- Room code stays focused on entity management
- Games can choose appropriate spatial model

## Integration Points

### With Existing Toolkit Modules

1. **Core**: Uses core.Entity interface for all spatial objects
2. **Events**: Emits events for all spatial state changes
3. **Conditions**: Conditions can listen for spatial events
4. **Effects**: Effects can modify spatial queries (e.g., flight, tremorsense)
5. **Resources**: Movement uses resource consumption patterns

### With Game Implementations

1. **Combat**: Line of sight, movement validation, area effects
2. **Spells**: Target selection, area of effect calculations
3. **Exploration**: Room generation, entity placement
4. **Interactions**: Proximity validation, reachability checks

## Testing Standards

The spatial module will follow the established RPG Toolkit testing patterns:

**Testing Framework:**
- **testify**: `github.com/stretchr/testify/assert` and `github.com/stretchr/testify/require`
- **gomock**: `go.uber.org/mock/gomock` for interface mocking
- **External package testing**: Tests in `package_test` not `package`

**Key Testing Patterns:**
1. **Error handling**: Always use `require.NoError(t, err)` for critical failures
2. **Assertions**: Use `require` for critical failures, `assert` for comparisons
3. **Mock generation**: Use `//go:generate mockgen` comments for interface mocks
4. **Event testing**: Mock event buses, verify event emissions
5. **Bounds testing**: Test edge cases, invalid inputs, and boundary conditions
6. **Integration testing**: Test with real event bus and entity implementations

**Test Structure Example:**
```go
package spatial_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/KirkDiggler/rpg-toolkit/events"
    "github.com/KirkDiggler/rpg-toolkit/mechanics/spatial"
)

func TestSquareGridCreation(t *testing.T) {
    grid := spatial.NewSquareGrid(10, 10)
    require.NotNil(t, grid)
    assert.Equal(t, spatial.GridShapeSquare, grid.GetShape())
}

func TestEntityPlacement(t *testing.T) {
    bus := events.NewBus()
    grid := spatial.NewSquareGrid(5, 5)
    room := spatial.NewRoom("test-room", grid, bus)
    
    entity := &MockEntity{id: "test-entity", typ: "character"}
    pos := spatial.Position{X: 2, Y: 2}
    
    err := room.PlaceEntity(entity, pos)
    require.NoError(t, err)
    
    retrievedPos, exists := room.GetEntityPosition("test-entity")
    assert.True(t, exists)
    assert.Equal(t, pos, retrievedPos)
}
```

**Test Coverage Requirements:**
- All public interface methods must be tested
- Error conditions must be tested (invalid positions, entity not found, etc.)
- Event emissions must be verified
- Grid-specific behavior must be tested for each grid type
- Spatial query accuracy must be validated
- Concurrent access patterns should be tested where applicable

## Next Steps

1. **Implement Core Interfaces**: Start with basic Room and Grid interfaces
2. **Square Grid First**: Implement and test square grid thoroughly
3. **Event Integration**: Add event emission to Room operations
4. **Hex Grid Implementation**: Add hex grid with cube coordinate system
5. **Query System**: Implement comprehensive spatial query capabilities
6. **Gridless Support**: Add approximate positioning for theater-of-mind
7. **Performance Optimization**: Profile and optimize hot paths

## Open Questions

1. **Multi-level Rooms**: How to handle rooms with multiple floors/levels?
2. **Large Entity Placement**: How to handle entities that occupy multiple grid spaces?
3. **Dynamic Grid Changes**: Should grids be mutable (walls appearing/disappearing)?
4. **Lighting Integration**: How to integrate with vision/lighting systems?
5. **Performance Scaling**: How will this perform with hundreds of entities?

## Lessons From dnd5e-roomgen

1. **Configuration Flexibility**: The original library's flexible configuration system worked well
2. **Entity Types**: Supporting different entity types (players, monsters, items) is crucial
3. **Placement Priorities**: Entity placement order matters for generation
4. **Inventory Systems**: NPCs need inventory support for trading/interactions
5. **Encounter Balancing**: Room generation benefits from encounter balancing logic

This spatial module will provide the foundation for rich, event-driven spatial gameplay in the RPG Toolkit while maintaining the flexibility and extensibility that makes the toolkit valuable.

## Implementation Completion

**Status**: ✅ Complete (July 16, 2025)

### What Was Implemented

The spatial module has been fully implemented in `tools/spatial/` with all planned features:

**Core Implementation**:
- ✅ Complete Grid interface with pluggable grid system
- ✅ Room interface with triple entity tracking (entities, positions, occupancy)
- ✅ Square Grid with D&D 5e Chebyshev distance calculation
- ✅ Hex Grid with cube coordinate system for accurate distance/neighbor calculations
- ✅ Gridless room with Euclidean distance for theater-of-mind play
- ✅ Full event integration with toolkit event bus
- ✅ Comprehensive spatial query system with event-driven queries

**Key Features Delivered**:
- **Grid-Agnostic Architecture**: Room works seamlessly with all three grid types
- **Event-Driven State Changes**: All spatial operations emit events
- **Comprehensive Query System**: Position queries, entity queries, LOS, movement validation
- **Thread-Safe Operations**: All room operations are mutex-protected
- **Flexible Entity Filtering**: Advanced filtering by type, ID, exclusions
- **Utility Functions**: Convenient query utilities and common filters

**Testing**:
- ✅ Complete test suite using testify suite pattern
- ✅ All grid types tested with grid interface
- ✅ Event integration thoroughly tested
- ✅ Query system tested both directly and through events
- ✅ Comprehensive examples and usage documentation

**Architecture Insights**:
- **Triple Entity Tracking**: Using three maps (entities, positions, occupancy) provides O(1) lookups
- **Grid-Specific Distance**: Different grids calculate distance differently (Chebyshev vs Hex vs Euclidean)
- **Event-Driven Queries**: Query system works both directly and through event bus
- **Filter Composition**: Entity filters can be combined for complex queries

### Architecture Decision: tools/ Directory

Following ADR-0008, the spatial module was implemented in `tools/spatial/` rather than `mechanics/spatial/`, establishing the three-tier architecture:

1. **Foundation** (`core/`, `events/`, `dice/`) - Used by everything
2. **Tools** (`tools/spatial/`) - Specialized infrastructure  
3. **Mechanics** (`mechanics/conditions/`, etc.) - Game mechanics

This creates clear separation between infrastructure tools and game mechanics.

### Grid-Specific Implementations

**Square Grid (D&D 5e)**:
- Distance: `max(|x1-x2|, |y1-y2|)` (Chebyshev)
- 8 neighbors per position
- Bresenham's line algorithm for line of sight

**Hex Grid (Cube Coordinates)**:
- Distance: `(|x1-x2| + |y1-y2| + |z1-z2|) / 2`
- 6 neighbors per position
- Ring and spiral patterns for area queries

**Gridless (Theater-of-Mind)**:
- Distance: `sqrt((x1-x2)² + (y1-y2)²)` (Euclidean)
- 8 neighbors with sampling for continuous positioning
- Flexible placement rules

### Query System Architecture

The query system provides both direct API access and event-driven queries:

```go
// Direct API
queryHandler := spatial.NewSpatialQueryHandler()
result, err := queryHandler.HandleQuery(ctx, query)

// Event-driven (through QueryUtils)
queryUtils := spatial.NewQueryUtils(eventBus)
entities, err := queryUtils.QueryEntitiesInRange(ctx, pos, radius, roomID, filter)
```

**Query Types**:
- Position queries (positions in range)
- Entity queries (entities in range, with filtering)
- Line of sight queries (positions + blocked status)
- Movement queries (validity, path, distance)
- Placement queries (can entity be placed)

### Files Implemented

- `interfaces.go` - Core interfaces (Grid, Room, Placeable, QueryHandler, EntityFilter)
- `position.go` - Position, Dimensions, coordinate types
- `room.go` - BasicRoom implementation with event integration
- `square_grid.go` - Square grid with D&D 5e distance calculations
- `hex_grid.go` - Hex grid with cube coordinate system
- `gridless.go` - Gridless room with Euclidean distance
- `events.go` - Event constants and data structures
- `query_handler.go` - Spatial query handler with event integration
- `query_utils.go` - Convenient query utilities and filters
- `*_test.go` - Comprehensive test suite for all components
- `examples_test.go` - Usage examples and documentation

### Integration Points Achieved

**With Toolkit Modules**:
- ✅ Uses `core.Entity` interface for all spatial objects
- ✅ Emits events through `events.EventBus` for all state changes
- ✅ Follows toolkit patterns for configuration and error handling

**Ready for Game Integration**:
- ✅ Combat systems can react to spatial events
- ✅ Spell systems can use spatial queries for targeting
- ✅ Condition systems can listen for movement events
- ✅ Vision systems can use line of sight queries