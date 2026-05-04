# ADR-0019: Environment Orchestrator Layer

Date: 2025-07-23

## Status

Proposed

## Context

The rpg-toolkit provides powerful low-level modules (spatial, environments, selectables, spawn) but lacks a cohesive orchestration layer that combines these modules into game-ready interfaces. Applications currently need to understand and coordinate multiple modules directly, creating complexity and duplication across different implementations.

This ADR addresses the architectural insight that emerged from rpg-api development: **the comprehensive orchestrator we built in rpg-api should have been part of the toolkit itself**. The "API over-engineering" criticism was actually about the orchestrator being in the wrong repository, not about the architectural complexity being inappropriate.

### Current Pain Points

1. **Module Coordination Complexity**: Applications must understand how spatial, environments, selectables, and spawn modules interact
2. **Duplicate Orchestration Logic**: Each application (UE games, Discord bots, web clients) reimplements the same coordination patterns
3. **Missing Game-Ready Interface**: Low-level modules are too granular for typical game development needs
4. **Inconsistent Integration Patterns**: No standard way to combine modules for common use cases

### Success Criteria

- **Single Import**: Applications import `toolkit/orchestrator` instead of juggling multiple modules
- **Game-Ready Methods**: High-level functions that solve complete game scenarios
- **Incremental Adoption**: Each orchestrator method stands alone and adds clear value
- **Module Abstraction**: Applications don't need to understand internal module coordination

## Decision

Create a new `toolkit/orchestrator` package that provides high-level, game-ready interfaces combining the existing low-level modules. The orchestrator will be built incrementally using the "baby steps" approach, with each phase delivering immediate value.

### Package Structure

```go
// toolkit/orchestrator/
├── environment.go    // Environment creation and management
├── placement.go      // Entity placement and spatial operations  
├── connections.go    // Room connections and multi-room coordination
├── types.go         // Shared types and interfaces
└── doc.go           // Package documentation
```

### Incremental Implementation Phases

#### Phase 1: "Basic Room Setup"
**Scope**: Single method for creating basic rooms
```go
type EnvironmentOrchestrator struct {
    envGen    environments.Generator
    spatial   spatial.Manager
    eventBus  events.EventBus
}

// SetupRoom creates a basic room with the specified dimensions and theme
// Returns a room ready for entity placement and gameplay
func (eo *EnvironmentOrchestrator) SetupRoom(input *SetupRoomInput) (*SetupRoomOutput, error)

type SetupRoomInput struct {
    Width  int32
    Height int32  
    Theme  string
}

type SetupRoomOutput struct {
    Room     *Room
    Metadata *GenerationMetadata
}
```

**Ship When**: Can create and return a basic room with walls

#### Phase 2: "Room Content Queries"
**Scope**: Query what's in a room
```go
// GetRoomContents returns all entities currently in the specified room
// Provides simple content listing without requiring spatial module knowledge
func (eo *EnvironmentOrchestrator) GetRoomContents(roomID string) ([]Entity, error)
```

**Ship When**: Can query room contents and get entity list

#### Phase 3: "Entity Placement"
**Scope**: Add entities to rooms
```go
// AddToRoom places the specified entities in the room
// Handles placement validation and spatial coordination internally
func (eo *EnvironmentOrchestrator) AddToRoom(roomID string, placements []EntityPlacement) error

type EntityPlacement struct {
    Entity   Entity
    Position Position // x, y coordinates within the room
}
```

**Ship When**: Can place entities in rooms with position validation

#### Phase 4: "Room Connections"
**Scope**: Connect rooms together
```go
// ConnectRooms creates a logical connection between two rooms
// Establishes the foundation for multi-room environments
func (eo *EnvironmentOrchestrator) ConnectRooms(room1ID, room2ID string, connectionType string) error

// GetConnectedRooms returns all rooms connected to the specified room
func (eo *EnvironmentOrchestrator) GetConnectedRooms(roomID string) ([]Room, error)
```

**Ship When**: Rooms can be logically connected and navigation queries work

### Design Principles

1. **Trust Internal Modules**: Orchestrator coordinates but doesn't duplicate validation logic
2. **Single Responsibility Per Method**: Each method solves one complete game scenario
3. **Incremental Value**: Each phase delivers immediate, usable functionality
4. **Module Abstraction**: Applications use orchestrator methods, not direct module calls
5. **Event Integration**: All operations emit appropriate events for game system coordination

### Input/Output Pattern

Following toolkit conventions, all orchestrator methods use structured Input/Output types:

```go
// ❌ Never this
func SetupRoom(width, height int, theme string) (*Room, error)

// ✅ Always this
func SetupRoom(input *SetupRoomInput) (*SetupRoomOutput, error)
```

### Error Handling Strategy

The orchestrator will:
- **Propagate module errors**: Don't hide underlying module failures
- **Add orchestration context**: Wrap errors with orchestrator-level context
- **Maintain error chains**: Use `fmt.Errorf` with `%w` for error wrapping
- **Provide clear failure points**: Each phase can fail independently

### Integration with Existing Modules

The orchestrator will:
- **Import existing modules**: Use published versions of spatial, environments, selectables, spawn
- **Follow dependency patterns**: No replace directives or local paths
- **Respect module boundaries**: Don't modify or extend imported modules
- **Use public interfaces only**: No internal package access

## Consequences

### Positive

- **Simplified Application Development**: Games import one package instead of coordinating multiple modules
- **Consistent Integration Patterns**: Standard way to use toolkit functionality
- **Reduced Duplication**: Common orchestration logic centralized in toolkit
- **Incremental Adoption**: Applications can adopt orchestrator methods one at a time
- **Better Testing**: Orchestration logic can be tested independently
- **Clear Scope Boundaries**: Each orchestrator method has well-defined responsibilities

### Negative

- **Additional Abstraction Layer**: One more layer between applications and core modules
- **Potential Over-Abstraction**: Risk of hiding useful module functionality behind simplified interfaces
- **Dependency Management**: Orchestrator must stay current with module changes
- **API Surface Growth**: New package adds to toolkit's public API surface

### Neutral

- **Module Independence Maintained**: Core modules remain focused and independent
- **Backward Compatibility**: Existing direct module usage continues to work
- **Testing Strategy**: Orchestrator tests can mock underlying modules
- **Documentation Burden**: New package requires comprehensive documentation

## Example

### Basic Usage Pattern

```go
// Initialize orchestrator
orchestrator := orchestrator.NewEnvironmentOrchestrator(orchestrator.Config{
    EventBus: eventBus,
    // Module dependencies injected
})

// Phase 1: Create a room
roomOutput, err := orchestrator.SetupRoom(&orchestrator.SetupRoomInput{
    Width:  20,
    Height: 15,
    Theme:  "dungeon",
})
if err != nil {
    return fmt.Errorf("failed to setup room: %w", err)
}

// Phase 2: Query contents (initially empty)
contents, err := orchestrator.GetRoomContents(roomOutput.Room.ID)
if err != nil {
    return fmt.Errorf("failed to get room contents: %w", err)
}

// Phase 3: Add entities
placements := []orchestrator.EntityPlacement{
    {Entity: treasureChest, Position: orchestrator.Position{X: 5, Y: 5}},
    {Entity: goblin, Position: orchestrator.Position{X: 10, Y: 8}},
}
err = orchestrator.AddToRoom(roomOutput.Room.ID, placements)
if err != nil {
    return fmt.Errorf("failed to add entities: %w", err)
}

// Phase 4: Connect to another room (future phase)
err = orchestrator.ConnectRooms(room1.ID, room2.ID, "door")
if err != nil {
    return fmt.Errorf("failed to connect rooms: %w", err)
}
```

### Integration in Game Applications

```go
// UE Game Integration
type GameWorld struct {
    orchestrator *orchestrator.EnvironmentOrchestrator
}

func (gw *GameWorld) CreateDungeonRoom(width, height int, theme string) (*GameRoom, error) {
    // Single orchestrator call replaces complex module coordination
    output, err := gw.orchestrator.SetupRoom(&orchestrator.SetupRoomInput{
        Width:  int32(width),
        Height: int32(height),
        Theme:  theme,
    })
    if err != nil {
        return nil, err
    }
    
    // Convert orchestrator room to game-specific representation
    return gw.convertToGameRoom(output.Room), nil
}
```

This ADR establishes the foundation for a game-ready orchestration layer that simplifies toolkit usage while maintaining the flexibility and power of the underlying modules.