# ADR-0015: Abstract Room Connections

## Status
Proposed

## Justification for Separate ADR

This decision warrants a separate ADR from ADR-0014 (Environment Selectables Integration) because it addresses a fundamentally different architectural concern:

- **ADR-0014** focuses on **room generation variety and intelligence** through selectables integration and capacity management
- **ADR-0015** addresses **multi-room spatial relationship modeling** and connection architecture

These are orthogonal concerns that could be implemented independently:
- Teams might want abstract connections without selectables integration (simpler connection model)
- Teams might want selectables integration without changing connection architecture (keep precise connections)
- The decisions have different risk profiles, implementation timelines, and architectural implications

Additionally, abstract connections affects multiple modules (spatial, environments, multi-room orchestration) while selectables integration is primarily contained within the environments package.

## Context

The current multi-room connection system requires precise spatial alignment between rooms, which creates significant constraints and complexity:

### Current Rigid System Problems

1. **Placement Complexity**: When placing rooms, we must consider both:
   - Where connections align spatially
   - Avoiding collisions with previously placed rooms
   - Risk of "impossible placement" scenarios that crash generation

2. **Shape Limitations**: Room shapes are constrained by their predefined connection points:
   - Rectangle always has entrance at `{X: 0.5, Y: 0}`
   - T-shapes have specific connection locations
   - Incompatible shapes cannot connect despite logical gameplay connections

3. **Layout Algorithm Complexity**: Multi-room orchestration becomes a constraint satisfaction problem:
   - "Place room B so its north door aligns with room A's south door"
   - "Ensure no overlap with existing rooms"
   - "Handle cases where no valid placement exists"

4. **Limited Flexibility**: System works well for tactical miniature games (like Betrayal at House on the Hill) but creates unnecessary complexity for most RPG scenarios where exact spatial relationships don't matter.

### Real-World Usage Patterns

Most RPG scenarios care about **logical connections** rather than **precise spatial alignment**:
- "The tavern connects to the basement" (not "the tavern's south door aligns with basement's north door")
- "Moving from forest to cave" (not precise positioning requirements)
- "Teleporter leads to wizard tower" (abstract connection by definition)

## Decision

We will implement **Abstract Room Connections** as the default connection model, with precise spatial connections available as an opt-in advanced feature.

### Abstract Connection Model

#### Core Principle
Connections represent **logical relationships** between rooms, not spatial constraints.

```go
type Connection struct {
    ID       string
    FromRoom string  
    ToRoom   string
    Type     ConnectionType  // door, passage, teleporter, etc.
    // Notably absent: precise positions, alignment requirements, spatial constraints
}
```

#### Connection Creation
```go
// Simplified - no spatial validation required
func (o *Orchestrator) ConnectRooms(fromRoom, toRoom, connectionType string) error {
    connection := Connection{
        ID:       generateConnectionID(),
        FromRoom: fromRoom,
        ToRoom:   toRoom, 
        Type:     parseConnectionType(connectionType),
    }
    o.connections[connection.ID] = connection
    return nil // No complex constraint satisfaction
}
```

#### Entity Transitions
```go
func (o *Orchestrator) MoveEntityBetweenRooms(entityID, fromRoom, toRoom, connectionID string) error {
    // 1. Remove from source room
    err := o.rooms[fromRoom].RemoveEntity(entityID)
    if err != nil {
        return fmt.Errorf("failed to remove entity from source room: %w", err)
    }
    
    // 2. Entity placement handled by spawn module or game logic
    //    No precise positioning required at connection level
    
    // 3. Emit transition event for game to handle positioning
    if o.eventBus != nil {
        event := events.NewGameEvent("entity.room_transition", entity, nil)
        event.Context().Set("entity_id", entityID)
        event.Context().Set("from_room", fromRoom)
        event.Context().Set("to_room", toRoom)
        event.Context().Set("connection_id", connectionID)
        _ = o.eventBus.Publish(context.Background(), event)
    }
    
    return nil
}
```

### Configuration Options

```go
type ConnectionMode string
const (
    ConnectionModeAbstract ConnectionMode = "abstract"  // Default: logical connections only
    ConnectionModePrecise  ConnectionMode = "precise"   // Opt-in: spatial alignment required
)

type RoomOrchestratorConfig struct {
    ConnectionMode ConnectionMode
    // ... other config
}
```

### Precise Mode (Opt-in for Specific Games)

For games requiring exact spatial relationships (tactical miniatures, dungeon crawlers with precise movement):

```go
type PreciseConnection struct {
    Connection                    // Embed base connection
    FromPosition spatial.Position // Exact position in source room
    ToPosition   spatial.Position // Exact position in destination room
    Alignment    ConnectionAlignment // Spatial alignment requirements
}
```

## Benefits

### 1. Eliminates Placement Complexity
- **Room placement** becomes simple: "place room anywhere that doesn't overlap"
- **No constraint satisfaction** problems or impossible placement scenarios
- **No collision detection** required between room layouts

### 2. Maximum Shape Variety
- **Any shape can connect to any other shape** regardless of predefined connection points
- **Selectables system** (ADR-0014) can freely choose any shape combination
- **No connection compatibility constraints** limiting room generation variety

### 3. Simplified Multi-Room Orchestration
- **Connection creation** is always successful (no spatial validation)
- **Layout algorithms** focus on room placement, not connection alignment
- **Fewer edge cases** and error conditions to handle

### 4. Better Module Separation
- **Spatial module** handles individual room mechanics
- **Environment module** handles room generation and logical connections
- **Spawn module** handles entity placement (including post-transition positioning)
- **Game layer** handles visual/narrative connection representation

### 5. Supports Diverse Game Types
- **Abstract RPGs**: "You enter the tavern" - no precise positioning needed
- **Exploration Games**: Focus on room-to-room navigation, not exact layouts
- **Theater of Mind**: Connections exist logically, visual details handled by game
- **Tactical Games**: Can opt into precise mode when spatial accuracy matters

## Consequences

### Positive
- **Dramatically simplified** room placement and connection logic
- **Maximum flexibility** for room shape combinations
- **Better performance** (no complex constraint solving)
- **Easier testing** (fewer edge cases and failure modes)
- **Cleaner integration** with spawn module for entity placement

### Considerations
- **Loss of automatic spatial precision** for games that might benefit from it
- **Game layer responsibility** for visual connection representation increases
- **Entity positioning** after room transitions requires game-level handling
- **Precise mode complexity** still exists for games that need it

### Migration Strategy
- **Default to abstract mode** for new implementations
- **Existing precise mode code** remains available as opt-in feature
- **Gradual migration** path for existing games
- **Clear documentation** on when to use each mode

## Implementation Details

### Multi-Room Orchestrator Changes

#### Simplified Connection Management
```go
type BasicRoomOrchestrator struct {
    // ... existing fields
    connectionMode ConnectionMode
    connections    map[string]Connection    // Simplified connection storage
    // Remove: spatial layout management, constraint solving
}

func (o *BasicRoomOrchestrator) AddConnection(fromRoom, toRoom string, connectionType ConnectionType) error {
    // Validate rooms exist
    if _, exists := o.rooms[fromRoom]; !exists {
        return fmt.Errorf("source room %s not found", fromRoom)
    }
    if _, exists := o.rooms[toRoom]; !exists {
        return fmt.Errorf("destination room %s not found", toRoom)
    }
    
    // Create logical connection (no spatial validation)
    connection := Connection{
        ID:       fmt.Sprintf("conn_%s_%s_%d", fromRoom, toRoom, len(o.connections)),
        FromRoom: fromRoom,
        ToRoom:   toRoom,
        Type:     connectionType,
    }
    
    o.connections[connection.ID] = connection
    
    // Emit connection creation event
    if o.eventBus != nil {
        event := events.NewGameEvent(EventConnectionCreated, nil, nil)
        event.Context().Set("connection", connection)
        _ = o.eventBus.Publish(context.Background(), event)
    }
    
    return nil
}
```

#### Room Placement Simplification
```go
func (o *BasicRoomOrchestrator) AddRoom(room spatial.Room) error {
    // Simple validation - just check for ID conflicts
    if _, exists := o.rooms[room.GetID()]; exists {
        return fmt.Errorf("room with ID %s already exists", room.GetID())
    }
    
    o.rooms[room.GetID()] = room
    // No spatial placement logic required
    
    return nil
}
```

### Event Integration

#### New Events for Abstract Connections
```go
const (
    EventConnectionCreated     = "connection.created"
    EventEntityRoomTransition  = "entity.room_transition"
    EventRoomConnectionQuery   = "room.connection_query"
)
```

#### Entity Transition Events
```go
// Game can listen for transition events and handle positioning
type RoomTransitionEvent struct {
    EntityID     string
    FromRoom     string
    ToRoom       string
    ConnectionID string
    // Game determines where to place entity in destination room
}
```

### Integration with Existing Modules

#### Spatial Module Integration
- **No changes required** to core spatial module interfaces
- **Room interface** remains unchanged
- **Connection abstraction** handled at orchestration level

#### Environment Module Integration
- **Room generation** unaffected by connection model choice
- **Shape selection** (ADR-0014) benefits from unlimited shape combinations
- **Wall pattern generation** remains independent of connection system

#### Spawn Module Integration
- **Entity placement** after room transitions becomes spawn module responsibility
- **Clean separation** between connection logic and positioning logic
- **Flexible positioning** strategies can be implemented in spawn module

### Testing Strategy

#### Abstract Connection Testing
```go
func TestAbstractConnections(t *testing.T) {
    orchestrator := NewBasicRoomOrchestrator(BasicRoomOrchestratorConfig{
        ConnectionMode: ConnectionModeAbstract,
    })
    
    // Any shape can connect to any other shape
    room1 := createRoom("hexagon", 15, 15)
    room2 := createRoom("t_shape", 20, 10)
    room3 := createRoom("rectangle", 12, 8)
    
    orchestrator.AddRoom(room1)
    orchestrator.AddRoom(room2)
    orchestrator.AddRoom(room3)
    
    // All connections should succeed regardless of shape compatibility
    assert.NoError(t, orchestrator.AddConnection("room1", "room2", ConnectionTypeDoor))
    assert.NoError(t, orchestrator.AddConnection("room2", "room3", ConnectionTypePassage))
    assert.NoError(t, orchestrator.AddConnection("room1", "room3", ConnectionTypeTeleporter))
    
    // Verify logical connections exist
    connections := orchestrator.GetConnections("room1")
    assert.Len(t, connections, 2) // Connected to room2 and room3
}
```

#### Entity Transition Testing
```go
func TestEntityTransition(t *testing.T) {
    // Test that entity is removed from source room
    // Test that transition event is emitted with correct context
    // Test that connection relationship is maintained
    // No need to test precise positioning (spawn module's responsibility)
}
```

## Future Considerations

### Visual Representation
Games using abstract connections will need to handle visual connection representation:
- **UI indicators** for available connections
- **Narrative descriptions** of connection types
- **Map visualizations** showing logical relationships

### Pathfinding Between Rooms
For games needing cross-room pathfinding:
- **Logical path existence** can be determined from connection graph
- **Precise pathfinding** would require game-specific spatial positioning
- **Hybrid approaches** possible (abstract for planning, precise for execution)

### Performance Optimization
Abstract connections enable performance optimizations:
- **Graph-based room queries** become simpler
- **Connection caching** more straightforward
- **Reduced spatial calculations** during room layout

## Implementation Dependencies

### Spawn Layer Integration (Critical)

**Abstract connections require the spawn layer to handle entity placement after room transitions.**

#### Current Limitation
- The spatial module's `MoveEntityBetweenRooms` now only removes entities from source rooms and emits `entity.room_transition` events
- Entity placement in destination rooms must be handled by the spawn layer via event subscription
- **Tests requiring complete entity movement are currently skipped** until spawn layer implementation

#### Required Spawn Layer Functionality
```go
// Spawn layer must subscribe to room transition events
func (s *SpawnEngine) HandleRoomTransition(ctx context.Context, event events.Event) error {
    entityID := event.Context().Get("entity_id").(string)
    toRoom := event.Context().Get("to_room").(string)
    connectionID := event.Context().Get("connection_id").(string)
    
    // Determine appropriate placement position in destination room
    position := s.calculateTransitionPosition(toRoom, connectionID)
    
    // Place entity in destination room
    return s.PlaceEntityInRoom(entityID, toRoom, position)
}
```

#### Integration Points
- **Event Type**: `"entity.room_transition"`
- **Event Context**: `entity_id`, `from_room`, `to_room`, `connection_id`
- **Spawn Responsibility**: Calculate placement position and execute `room.PlaceEntity()`
- **Orchestrator Responsibility**: Track logical room assignment via `entityRooms` mapping

#### Testing Impact
- **Skipped Test**: `TestEntityMovementBetweenRooms` - requires spawn layer for complete entity transitions
- **Working Tests**: Connection creation, logical room tracking, event emission
- **Future Work**: Re-enable entity movement tests once spawn layer handles placement events

This dependency ensures clean separation between connection logic (spatial module) and placement logic (spawn module), following the toolkit's modular architecture principles.

This ADR establishes abstract connections as the default approach while preserving precise spatial connections for games that specifically require exact spatial relationships. The decision significantly simplifies multi-room orchestration while maximizing flexibility for room generation and shape variety.