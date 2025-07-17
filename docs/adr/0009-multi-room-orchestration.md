# ADR-0009: Multi-Room Orchestration Architecture

**Date**: 2025-01-17  
**Status**: Accepted  
**Context**: Room Orchestrator design (Journey 013)  
**Implementation Completed**: 2025-01-17  

## Context

We need to extend the spatial module to handle multi-room coordination and connections. Rather than building complex template systems, we should focus on infrastructure for connecting rooms and orchestrating spatial relationships across multiple connected spaces.

## Key Requirements

1. **Connection Management**: Link rooms with typed connections (doors, stairs, passages)
2. **Layout Orchestration**: Arrange multiple rooms in common patterns (towers, dungeons, towns)
3. **Transition Handling**: Manage entity movement between connected rooms
4. **Generic Entities**: Classify entities by behavior, not specific type
5. **Event Integration**: Emit events for room connections and transitions
6. **Performance**: Efficient for large multi-room structures

## Decision

We will **extend the existing spatial module** rather than create a separate module, implementing:

### 1. Connection System
```go
type Connection struct {
    Type        ConnectionType
    FromRoom    string
    ToRoom      string  
    Position    Position
    Properties  map[string]interface{}
}

type ConnectionType int
const (
    Door ConnectionType = iota
    Stairs              // Vertical connection (floors)
    Passage             // Open connection
    Portal              // Magical transport
    Bridge              // Over obstacles
    Tunnel              // Underground passage
)
```

### 2. Generic Entity Behaviors
```go
type EntityBehavior int
const (
    Blocking EntityBehavior = iota    // Walls, trees, pillars - all the same
    Interactive                       // Doors, chests, NPCs  
    Decorative                       // Furniture, decorations
    Passable                         // Floor, water, grass
)
```

### 3. Layout Orchestration
```go
type Orchestrator struct {
    rooms       map[string]Room
    connections []Connection
    layout      LayoutType
}

// Common patterns using same infrastructure
func CreateTower(floors int, floorSize Dimensions) *Orchestrator
func CreateBranching(rooms int, branchFactor int) *Orchestrator  
func CreateGrid(width, height int, blockSize Dimensions) *Orchestrator
func CreateOrganic(density float64, area Dimensions) *Orchestrator
```

### 4. Unified Experience Types
All environments (towers, dungeons, towns, woods) use the same connection patterns:
- **Tower**: Vertical connections via stairs
- **Dungeon**: Branching connections via doors
- **Town**: Grid connections via streets/passages
- **Woods**: Organic connections via paths

## Module Structure

### Extend Spatial Module with Internal Organization
```
tools/spatial/
├── position.go        # Core types
├── interfaces.go      # Main interfaces  
├── room.go           # Basic room implementation
├── orchestrator.go   # Multi-room coordination (NEW)
├── events.go         # Event definitions
└── internal/
    ├── grids/        # Grid implementations
    ├── queries/      # Query system
    └── orchestration/ # Multi-room internals (NEW)
        ├── connections.go
        ├── layouts.go
        └── transitions.go
```

### Public API
```go
// Simple orchestration API
orchestrator := spatial.NewOrchestrator()

// Create connected rooms using layout patterns
tower := orchestrator.
    WithLayout(spatial.VerticalTower(5)).
    WithConnections(spatial.StairConnections()).
    WithRoomSize(20, 20)

woods := orchestrator.
    WithLayout(spatial.OrganicClusters(0.3)).
    WithConnections(spatial.PathConnections()).
    WithBounds(100, 100)
```

## Alternatives Considered

### A. Separate tools/rooms Module
**Pros**: Clear separation of concerns  
**Cons**: Artificial boundary, users need multiple imports, similar concepts split  
**Rejected**: Violates cohesion - all spatial relationships should be together

### B. Complex Template System
**Pros**: Rich content definition capabilities  
**Cons**: Over-engineered, content vs infrastructure confusion  
**Rejected**: Violates "infrastructure not implementation" principle

### C. Top-level world/ Module  
**Pros**: Room for expansion  
**Cons**: Premature abstraction, unclear boundaries  
**Rejected**: Too speculative for current needs

## Implementation Strategy

### Phase 1: Connection Infrastructure
- Extend spatial module with connection types
- Basic room linking capabilities
- Connection validation and management

### Phase 2: Layout Patterns
- Common arrangement algorithms (tower, grid, branching, organic)
- Orchestrator API for multi-room creation
- Integration with existing room system

### Phase 3: Transition System
- Entity movement between connected rooms
- Event handling for transitions
- State management across rooms

### Phase 4: Package Reorganization
- Move implementation details to internal/
- Clean up public API surface
- Maintain backward compatibility

## Consequences

### Positive
- Simple, focused infrastructure approach
- Extends existing well-understood spatial concepts
- Supports diverse environments with same primitives
- Clean separation of infrastructure vs content
- Single import for all spatial functionality

### Negative
- Spatial module grows larger (mitigated by internal/ organization)
- May need refactoring if multi-room concepts become very complex
- Generic entity classification may be too simple for some games

### Neutral
- Requires careful API design to avoid complexity creep
- Need clear documentation for orchestration patterns

## Acceptance Criteria

- [x] Connect two rooms with a door
- [x] Create a 3-floor tower with stairs
- [x] Generate branching dungeon layout
- [x] Move entities between connected rooms
- [x] Events fired for connections and transitions
- [x] Package reorganized with internal/ structure
- [x] Performance: Handle 100 connected rooms efficiently

## Implementation Summary

The multi-room orchestration system has been successfully implemented with the following components:

### Core Implementation
- **BasicRoomOrchestrator**: Complete implementation managing multiple rooms and connections
- **BasicConnection**: Connection system with 6 connection types (door, stairs, passage, portal, bridge, tunnel)
- **Connection Helper Functions**: Convenience functions for creating common connection types
- **Layout Management**: Support for tower, branching, grid, and organic layout patterns
- **Event System**: Full event-driven architecture with 7 orchestration-specific events

### Key Features Delivered
- **Room Management**: Add, remove, and track multiple rooms
- **Connection System**: Typed connections with requirements, costs, and passability
- **Entity Tracking**: Track entities across all managed rooms with event-driven updates
- **Pathfinding**: Breadth-first search pathfinding between connected rooms
- **Layout Patterns**: Common spatial arrangements for different environment types
- **Performance**: Efficient handling of large multi-room structures

### Files Created/Modified
- `orchestrator.go` - Core interfaces and types
- `basic_orchestrator.go` - Main implementation
- `connection.go` - Connection implementation
- `connection_helpers.go` - Helper functions for creating connections
- `orchestrator_test.go` - Comprehensive test suite
- `README.md` - Complete documentation and usage examples

### Testing
- Full test coverage for all orchestrator functionality
- Integration tests for entity movement between rooms
- Performance validation for large orchestrators
- Event system integration testing

The implementation provides a solid foundation for multi-room environments while maintaining the project's "infrastructure, not implementation" philosophy.

---

**Author**: Development Team  
**Reviewers**: Completed  
**Implementation**: Multi-room orchestration system fully implemented and documented