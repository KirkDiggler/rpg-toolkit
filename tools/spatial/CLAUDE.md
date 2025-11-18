# Spatial Module Development Guidelines

## Module Purpose

2D spatial positioning and movement infrastructure **WITHOUT game-specific rules**.

This module provides the mathematical foundation for position-based game systems:
- Grid systems (square, hex, gridless)
- Entity placement and movement
- Multi-room orchestration
- Spatial queries and line of sight

**We do NOT implement**: Game rules, combat mechanics, movement costs based on terrain.
**Game implementations decide**: How to use spatial data, what entities can do, rule interpretations.

## Current Status: Production Ready

✅ **All core features implemented and tested**
✅ **Events v0.6.1 compliant** (import ordering follows standard)
✅ **Comprehensive documentation** (43KB README.md)
✅ **Thread-safe operations** (proper mutex usage)
✅ **Event-driven architecture** (typed topics)

### Dependencies
- `events v0.6.0` (v0.6.1 compatible, upgrade when available)
- `core v0.9.0` (Entity interface)
- `game` (Context pattern for data persistence)

## Key Architectural Decisions

### ADR-0015: Abstract Connections (Critical Understanding)

Connections are **abstract links** between rooms, NOT physical objects:
- Connections do NOT have positions themselves
- Positions are managed by the game layer (e.g., door entities placed in rooms)
- Supports bidirectional and unidirectional movement
- Requirements and costs can be entity-specific

**Example**: A door connection links room A position (9,5) to room B position (0,5), but the door entity itself is placed in the room at that position by the game layer.

### Event-Driven Architecture

Uses typed topics from events v0.6.0+:
```go
// Entity lifecycle
EntityPlacedTopic
EntityMovedTopic
EntityRemovedTopic

// Room lifecycle
RoomCreatedTopic
RoomAddedTopic
RoomRemovedTopic

// Orchestrator lifecycle
ConnectionAddedTopic
ConnectionRemovedTopic
EntityTransitionBeganTopic
EntityTransitionEndedTopic
EntityRoomTransitionTopic
LayoutChangedTopic
```

**Important**: Always call `ConnectToEventBus()` after creating rooms/orchestrators.

### Thread Safety Pattern

Both `BasicRoom` and `BasicRoomOrchestrator` use `sync.RWMutex`:
- Read operations use `RLock()/RUnlock()`
- Write operations use `Lock()/Unlock()`
- Triple-tracking system for efficient lookups (entities map, positions map, occupancy map)

## Grid Systems

### Three Grid Types (All Complete)

1. **Square Grid**: D&D 5e Chebyshev distance, 8 neighbors
   - Use for traditional grid-based games
   - Distance = max(abs(dx), abs(dy))

2. **Hex Grid**: Cube coordinates, 6 neighbors
   - Supports pointy-top and flat-top orientation
   - Use for tactical hex-based games
   - Distance = (abs(x) + abs(y) + abs(z)) / 2

3. **Gridless**: Euclidean distance, continuous positioning
   - Use for theater-of-mind or free-form positioning
   - Distance = sqrt(dx² + dy²)

### Distance Calculation Philosophy

**Position type does NOT enforce distance calculations** - Each Grid implementation handles its own math.

This allows:
- Grid-dependent distance rules
- Flexibility in distance calculation methods
- Separation of data (Position) from behavior (Grid)

## Data Persistence Pattern

### RoomData Structure

Serializable room state for saving/loading:
```go
type RoomData struct {
    RoomID      string
    RoomType    string
    GridType    string
    Width       int
    Height      int
    Orientation string  // For hex grids: "pointy" or "flat"
    Entities    []PlaceableData
}
```

### Loading Pattern

```go
// Load from game context
room, err := LoadRoomFromContext(ctx, gameCtx)
if err != nil {
    return err
}

// ALWAYS connect to event bus after creation
room.ConnectToEventBus(eventBus)
```

**Critical**: Event bus connection is separate from room creation to allow flexibility in when events start publishing.

## Testing Patterns

### Always Use Testify Suite

```go
type MyTestSuite struct {
    suite.Suite
    room      *spatial.BasicRoom
    eventBus  events.EventBus
}

func (s *MyTestSuite) SetupTest() {
    s.eventBus = events.NewEventBus()
    // Create room...
}

func (s *MyTestSuite) TestSomething() {
    s.Run("descriptive subtest name", func() {
        // Test code
    })
}

func TestMyTestSuite(t *testing.T) {
    suite.Run(t, new(MyTestSuite))
}
```

### Import Ordering (Events v0.6.1 Compliant)

**Third-party imports BEFORE local imports**:
```go
import (
    "context"
    "testing"

    "github.com/stretchr/testify/suite"  // ✅ Testify FIRST

    "github.com/KirkDiggler/rpg-toolkit/core"
    "github.com/KirkDiggler/rpg-toolkit/events"
    "github.com/KirkDiggler/rpg-toolkit/tools/spatial"  // ✅ Local AFTER
)
```

This is the v0.6.1 standard enforced by `goimports`.

## Unimplemented Interfaces (Future Work)

These interfaces are defined but **NOT implemented** - they are forward-looking designs:

### LayoutOrchestrator (connection.go:108-121)

For automatic room positioning and layout metrics:
- `ArrangeRooms()` - Auto-position rooms based on connections
- `CalculateRoomPositions()` - Compute spatial layout
- `ValidateLayout()` - Check layout constraints
- `GetLayoutMetrics()` - Layout quality metrics

**When to implement**: When you need visual generation of room layouts or automatic positioning.

### TransitionSystem (connection.go:135-150)

For progress tracking during room transitions:
- `BeginTransition()` - Start tracking entity movement
- `CompleteTransition()` - Finish transition
- `CancelTransition()` - Abort transition
- `GetActiveTransitions()` - Query in-progress transitions
- `GetTransition()` - Get specific transition details

**When to implement**: When you need real-time movement animation or granular transition states.

**Current behavior**: Transitions work via direct entity movement (`MoveEntityBetweenRooms`) without progress tracking.

## Common Implementation Patterns

### Creating Multi-Room Scenarios

```go
// 1. Create orchestrator with layout type
orchestrator := spatial.NewBasicRoomOrchestrator(spatial.BasicRoomOrchestratorConfig{
    ID:       "dungeon-orch",
    Type:     "orchestrator",
    EventBus: eventBus,
    Layout:   spatial.LayoutTypeOrganic,
})

// 2. Create and add rooms
room1 := spatial.NewBasicRoom(spatial.BasicRoomConfig{...})
room2 := spatial.NewBasicRoom(spatial.BasicRoomConfig{...})
orchestrator.AddRoom(room1)
orchestrator.AddRoom(room2)

// 3. Connect rooms with typed connections
door := spatial.CreateDoorConnection("door-1", "room-1", "room-2",
    spatial.Position{X: 9, Y: 5}, spatial.Position{X: 0, Y: 5})
orchestrator.AddConnection(door)

// 4. Track entities via event subscriptions
spatial.EntityMovedTopic.On(eventBus).Subscribe(func(ctx context.Context, event spatial.EntityMovedEvent) error {
    // Handle entity movement
    return nil
})

// 5. Use pathfinding for AI movement
path, err := orchestrator.FindPath("room-1", "room-2")
```

### Entity Filtering

Use pre-built filters from `query_utils.go`:
```go
// Filter for specific entity types
filter := spatial.CreateCharacterFilter()
filter := spatial.CreateMonsterFilter()
filter := spatial.CreateCombatantFilter()

// Include/exclude specific entities
filter := spatial.CreateIncludeFilter(entityIDs...)
filter := spatial.CreateExcludeFilter(entityIDs...)

// Use in queries
entities := room.GetEntitiesInRange(center, radius, filter)
```

### Connection Types

Six pre-built connection helpers:
```go
spatial.CreateDoorConnection()      // Standard doors
spatial.CreateStairsConnection()    // Vertical movement (one-way by default)
spatial.CreatePassageConnection()   // Open hallways
spatial.CreatePortalConnection()    // Magical/instant transport
spatial.CreateBridgeConnection()    // Crossable gaps
spatial.CreateTunnelConnection()    // Underground passages
```

## Performance Considerations

### Query System

- **Single-room queries**: Use room methods directly (`GetEntitiesInRange()`)
- **Multi-room queries**: Use SpatialQueryHandler with event-based queries
- **Built-in caching**: QueryHandler caches results until entity positions change

### Large Orchestrators

- Consider batching room additions (reduces event overhead)
- Cache frequently used paths (pathfinding can be expensive)
- Use entity filters to reduce query result sizes
- Profile before optimizing (current implementation handles 100+ rooms efficiently)

## Documentation Standards

### README.md is Authoritative

The `README.md` file (43KB) is the comprehensive guide:
- Keep README in sync with code changes
- Update examples when API changes
- Document new grid types or connection types
- Include integration examples

### All Public APIs Must Have Comments

Follow existing patterns:
```go
// NewBasicRoom creates a new room with the specified configuration.
// The room will not publish events until ConnectToEventBus is called.
func NewBasicRoom(config BasicRoomConfig) *BasicRoom {
```

- Explain purpose and behavior
- Document when events are published
- Note error conditions
- Include usage hints

## Working with Other Modules

### Core Module

All entities implement `core.Entity`:
```go
type Entity interface {
    GetID() string
    GetType() EntityType
}
```

The `Placeable` interface extends this for spatial entities.

### Events Module

- Uses typed topics (v0.6.0+)
- Import ordering must follow v0.6.1 standard
- Always subscribe before publishing events

### Game Module

- RoomData uses `game.Context` for persistence
- LoadRoomFromContext integrates with game infrastructure
- Event bus passed through game context

## Upgrading to Events v0.6.1

When v0.6.1 is available:

```bash
cd /home/kirk/personal/rpg-toolkit/tools/spatial
go get github.com/KirkDiggler/rpg-toolkit/events@v0.6.1
go mod tidy
```

**No code changes required** - import ordering is already compliant.

The v0.6.1 change was only import ordering standardization (testify before local imports).

## Common Pitfalls

### 1. Forgetting to Connect Event Bus

```go
// ❌ BAD - events won't publish
room := spatial.NewBasicRoom(config)

// ✅ GOOD - events will publish
room := spatial.NewBasicRoom(config)
room.ConnectToEventBus(eventBus)
```

### 2. Confusing Connections with Entities

```go
// ❌ WRONG - connections don't have positions
connection.GetPosition()  // This method doesn't exist

// ✅ RIGHT - connections link positions in two rooms
connection := spatial.CreateDoorConnection(
    "door-1",
    "room-1", "room-2",
    spatial.Position{X: 9, Y: 5},  // Position in room-1
    spatial.Position{X: 0, Y: 5},  // Position in room-2
)

// The door entity itself would be placed in the room by the game layer
room1.PlaceEntity(doorEntity, spatial.Position{X: 9, Y: 5})
```

### 3. Mixing Grid Distance Calculations

```go
// ❌ BAD - using wrong distance calculation
distance := math.Sqrt(dx*dx + dy*dy)  // Euclidean for square grid

// ✅ GOOD - let the grid handle it
distance := grid.Distance(pos1, pos2)
```

### 4. Race Conditions in Tests

```go
// ❌ BAD - event might not have processed yet
room.PlaceEntity(entity, pos)
// Immediately check event handler state

// ✅ GOOD - use synchronous event handlers or wait
var eventReceived bool
spatial.EntityPlacedTopic.On(eventBus).Subscribe(func(ctx context.Context, event spatial.EntityPlacedEvent) error {
    eventReceived = true
    return nil
})
room.PlaceEntity(entity, pos)
s.Eventually(func() bool { return eventReceived }, time.Second, 10*time.Millisecond)
```

## Questions to Ask Before Adding Features

1. **Is this spatial infrastructure or game rules?**
   - Spatial: Distance calculations, position tracking, movement validation
   - Game rules: Movement costs, terrain types, special movement abilities

2. **Does this belong in spatial or game layer?**
   - Spatial: Where entities are, how far apart
   - Game: What entities can do, why they can do it

3. **Is this a Grid concern or a Room concern?**
   - Grid: Mathematical calculations (distance, neighbors, line of sight)
   - Room: Entity management (placement, tracking, queries)

4. **Should this be an event or a direct call?**
   - Event: When other systems need to react (entity moved)
   - Direct call: When you need immediate results (can entity move here?)

## Remember

- **Spatial is infrastructure, not game rules**
- **Event bus connection is always separate from creation**
- **Import ordering matters for v0.6.1 compatibility**
- **Thread safety is built-in - don't add extra locks**
- **README.md is the source of truth for usage patterns**
- **Tests use testify suite pattern exclusively**
- **Grid type determines distance calculation method**
- **Connections are abstract links, not physical objects**
