# RPG Toolkit Event Bus Migration Specification

## Executive Summary

This document specifies the migration of spatial, environment, and spawn modules from the legacy string-based event system to the new type-safe event bus architecture. The migration eliminates runtime event matching in favor of compile-time type safety and explicit event flow visualization through the `.On(bus)` pattern.

## Migration Overview

### Current State Analysis

**Legacy Pattern**: String-based events with context data injection
```go
// Old: Runtime string matching, no type safety
event := events.NewGameEvent("spatial.entity.placed", entity, nil)
eventBus.Publish(context.TODO(), event)

// Old: String-based subscription
eventBus.SubscribeFunc("spatial.entity.placed", 0, func(_ context.Context, event events.Event) error {
    // Type assertion required, runtime errors possible
    return nil
})
```

**Target Pattern**: Type-safe topics with explicit connections
```go
// New: Compile-time type safety, explicit connections
var EntityPlacedTopic = events.DefineTypedTopic[EntityPlacedEvent]("spatial.entity.placed")

// THE MAGIC: Explicit connection visualization
placements := EntityPlacedTopic.On(bus)

// Type-safe subscription
placements.Subscribe(ctx, func(ctx context.Context, e EntityPlacedEvent) error {
    // No type assertions, compile-time safety
    fmt.Printf("Entity %s placed at %v", e.EntityID, e.Position)
    return nil
})

// Type-safe publishing
placements.Publish(ctx, EntityPlacedEvent{
    EntityID: entity.GetID(),
    Position: position,
    RoomID:   roomID,
})
```

## Module-Specific Migration Plans

### 1. Spatial Module Migration

#### Current Event Inventory

**Entity Lifecycle Events**:
- `spatial.entity.placed` → `EntityPlacedEvent`
- `spatial.entity.moved` → `EntityMovedEvent`
- `spatial.entity.removed` → `EntityRemovedEvent`

**Room Lifecycle Events**:
- `spatial.room.created` → `RoomCreatedEvent`
- `spatial.orchestrator.room_added` → `RoomAddedEvent`

**Query Events** (Special Consideration):
- `spatial.query.positions_in_range` → `PositionQueryEvent`
- `spatial.query.entities_in_range` → `EntityQueryEvent`
- Query events require request/response pattern evaluation

#### Migration Implementation

**1. Event Type Definitions**

```go
// File: tools/spatial/events.go

// Entity lifecycle events
type EntityPlacedEvent struct {
    EntityID string
    Position spatial.Position
    RoomID   string
    GridType string // "square", "hex", "gridless"
}

type EntityMovedEvent struct {
    EntityID     string
    FromPosition spatial.Position
    ToPosition   spatial.Position
    RoomID       string
    MovementType string // "normal", "teleport", "forced"
}

type EntityRemovedEvent struct {
    EntityID   string
    Position   spatial.Position
    RoomID     string
    RemovalType string // "normal", "destroyed", "teleported"
}

// Room lifecycle events
type RoomCreatedEvent struct {
    RoomID       string
    RoomType     string
    GridConfig   spatial.GridConfig
    Dimensions   spatial.Dimensions
    CreationTime time.Time
}

type RoomAddedEvent struct {
    RoomID         string
    OrchestratorID string
    ConnectionInfo map[string]interface{}
}
```

**2. Topic Definitions**

```go
// File: tools/spatial/topics.go

var (
    // Entity lifecycle topics
    EntityPlacedTopic  = events.DefineTypedTopic[EntityPlacedEvent]("spatial.entity.placed")
    EntityMovedTopic   = events.DefineTypedTopic[EntityMovedEvent]("spatial.entity.moved")
    EntityRemovedTopic = events.DefineTypedTopic[EntityRemovedEvent]("spatial.entity.removed")
    
    // Room lifecycle topics  
    RoomCreatedTopic = events.DefineTypedTopic[RoomCreatedEvent]("spatial.room.created")
    RoomAddedTopic   = events.DefineTypedTopic[RoomAddedEvent]("spatial.orchestrator.room_added")
)
```

**3. Room Implementation Update**

```go
// File: tools/spatial/room.go

type Room struct {
    // ... existing fields ...
    
    // New: Type-safe event publishers
    entityPlacements TypedTopic[EntityPlacedEvent]
    entityMovements  TypedTopic[EntityMovedEvent]
    entityRemovals   TypedTopic[EntityRemovedEvent]
}

// New: Event bus connection method
func (r *Room) ConnectToEventBus(bus events.EventBus) {
    r.entityPlacements = EntityPlacedTopic.On(bus)
    r.entityMovements = EntityMovedTopic.On(bus)
    r.entityRemovals = EntityRemovedTopic.On(bus)
}

// Updated: Type-safe event publishing
func (r *Room) PlaceEntity(ctx context.Context, entity core.Entity, position Position) error {
    // ... existing placement logic ...
    
    // Publish type-safe event
    if r.entityPlacements != nil {
        err := r.entityPlacements.Publish(ctx, EntityPlacedEvent{
            EntityID: entity.GetID(),
            Position: position,
            RoomID:   r.id,
            GridType: r.grid.Type(),
        })
        if err != nil {
            // Log but don't fail the operation
            fmt.Printf("Failed to publish entity placed event: %v", err)
        }
    }
    
    return nil
}
```

**4. Orchestrator Implementation Update**

```go
// File: tools/spatial/orchestrator.go

type Orchestrator struct {
    // ... existing fields ...
    
    // New: Type-safe event publishers
    roomAdditions TypedTopic[RoomAddedEvent]
}

func (o *Orchestrator) ConnectToEventBus(bus events.EventBus) {
    o.roomAdditions = RoomAddedTopic.On(bus)
    
    // Connect all managed rooms
    for _, room := range o.rooms {
        room.ConnectToEventBus(bus)
    }
}

func (o *Orchestrator) AddRoom(ctx context.Context, room *Room) error {
    // ... existing logic ...
    
    // Connect new room to event bus
    if o.roomAdditions != nil {
        room.ConnectToEventBus(/* need bus reference */)
        
        err := o.roomAdditions.Publish(ctx, RoomAddedEvent{
            RoomID:         room.ID(),
            OrchestratorID: o.id,
            ConnectionInfo: map[string]interface{}{
                "room_type": room.Type(),
                "grid_type": room.Grid().Type(),
            },
        })
        if err != nil {
            fmt.Printf("Failed to publish room added event: %v", err)
        }
    }
    
    return nil
}
```

#### Query Events Consideration

**Problem**: Current query events use request/response pattern which doesn't fit pure notification model.

**Solution Options**:
1. **Direct Function Calls**: Keep queries as direct method calls, not events
2. **Dual Pattern**: Use events for notifications, direct calls for queries
3. **Event-Driven Queries**: Implement request/response pattern with typed events

**Recommendation**: Use dual pattern - events for state changes, direct calls for queries.

### 2. Environment Module Migration

#### Current Event Inventory

**Generation Lifecycle**:
- `environment.generation.started` → `GenerationStartedEvent`
- `environment.generation.completed` → `GenerationCompletedEvent`  
- `environment.generation.failed` → `GenerationFailedEvent`
- `environment.generation.progress` → `GenerationProgressEvent`
- `environment.emergency_fallback.triggered` → `EmergencyFallbackEvent`

**Environment Lifecycle**:
- `environment.generated` → `EnvironmentGeneratedEvent`
- `environment.destroyed` → `EnvironmentDestroyedEvent`
- `environment.metadata.changed` → `EnvironmentMetadataEvent`

**Feature Management**:
- `environment.feature.added` → `FeatureAddedEvent`
- `environment.feature.removed` → `FeatureRemovedEvent`
- `environment.hazard.triggered` → `HazardTriggeredEvent`

#### Migration Implementation

**1. Event Type Definitions**

```go
// File: tools/environments/events.go

// Generation lifecycle events
type GenerationStartedEvent struct {
    GenerationID string
    RequestID    string
    Parameters   map[string]interface{}
    StartTime    time.Time
}

type GenerationProgressEvent struct {
    GenerationID string
    Stage        string // "layout", "walls", "features", "validation"
    Progress     float64 // 0.0 to 1.0
    Message      string
    Timestamp    time.Time
}

type GenerationCompletedEvent struct {
    GenerationID  string
    RequestID     string
    Environment   *Environment
    Duration      time.Duration
    Statistics    GenerationStatistics
    CompletedAt   time.Time
}

type GenerationFailedEvent struct {
    GenerationID string
    RequestID    string
    Error        error
    Stage        string
    FailedAt     time.Time
}

type EmergencyFallbackEvent struct {
    GenerationID string
    Trigger      string
    FallbackType string
    Parameters   map[string]interface{}
    Timestamp    time.Time
}

// Environment lifecycle events  
type EnvironmentGeneratedEvent struct {
    EnvironmentID string
    Type          string
    Theme         string
    Dimensions    spatial.Dimensions
    Features      []string
    GeneratedAt   time.Time
}

type EnvironmentDestroyedEvent struct {
    EnvironmentID string
    Reason        string
    DestroyedAt   time.Time
}

// Feature management events
type FeatureAddedEvent struct {
    EnvironmentID string
    FeatureID     string
    FeatureType   string
    Position      spatial.Position
    Properties    map[string]interface{}
    AddedAt       time.Time
}

type HazardTriggeredEvent struct {
    EnvironmentID string
    HazardID      string
    HazardType    string
    TriggerEntity string
    Position      spatial.Position
    Effect        string
    TriggeredAt   time.Time
}
```

**2. Topic Definitions**

```go  
// File: tools/environments/topics.go

var (
    // Generation lifecycle topics
    GenerationStartedTopic   = events.DefineTypedTopic[GenerationStartedEvent]("environment.generation.started")
    GenerationProgressTopic  = events.DefineTypedTopic[GenerationProgressEvent]("environment.generation.progress")
    GenerationCompletedTopic = events.DefineTypedTopic[GenerationCompletedEvent]("environment.generation.completed")
    GenerationFailedTopic    = events.DefineTypedTopic[GenerationFailedEvent]("environment.generation.failed")
    EmergencyFallbackTopic   = events.DefineTypedTopic[EmergencyFallbackEvent]("environment.emergency_fallback.triggered")
    
    // Environment lifecycle topics
    EnvironmentGeneratedTopic = events.DefineTypedTopic[EnvironmentGeneratedEvent]("environment.generated")
    EnvironmentDestroyedTopic = events.DefineTypedTopic[EnvironmentDestroyedEvent]("environment.destroyed")
    
    // Feature management topics
    FeatureAddedTopic   = events.DefineTypedTopic[FeatureAddedEvent]("environment.feature.added")
    HazardTriggeredTopic = events.DefineTypedTopic[HazardTriggeredEvent]("environment.hazard.triggered")
)
```

**3. Generator Implementation Update**

```go
// File: tools/environments/graph_generator.go

type GraphGenerator struct {
    // ... existing fields ...
    
    // New: Type-safe event publishers
    generationStarted   TypedTopic[GenerationStartedEvent]
    generationProgress  TypedTopic[GenerationProgressEvent]
    generationCompleted TypedTopic[GenerationCompletedEvent]
    generationFailed    TypedTopic[GenerationFailedEvent]
    emergencyFallback   TypedTopic[EmergencyFallbackEvent]
}

func (g *GraphGenerator) ConnectToEventBus(bus events.EventBus) {
    g.generationStarted = GenerationStartedTopic.On(bus)
    g.generationProgress = GenerationProgressTopic.On(bus)
    g.generationCompleted = GenerationCompletedTopic.On(bus)
    g.generationFailed = GenerationFailedTopic.On(bus)
    g.emergencyFallback = EmergencyFallbackTopic.On(bus)
}

func (g *GraphGenerator) Generate(ctx context.Context, req *GenerationRequest) (*Environment, error) {
    generationID := generateID()
    
    // Publish generation started
    if g.generationStarted != nil {
        _ = g.generationStarted.Publish(ctx, GenerationStartedEvent{
            GenerationID: generationID,
            RequestID:    req.ID,
            Parameters:   req.Parameters,
            StartTime:    time.Now(),
        })
    }
    
    // Progress updates during generation
    g.publishProgress(ctx, generationID, "layout", 0.2, "Creating room layout")
    
    // ... generation logic ...
    
    // Success case
    if g.generationCompleted != nil {
        _ = g.generationCompleted.Publish(ctx, GenerationCompletedEvent{
            GenerationID: generationID,
            RequestID:    req.ID,
            Environment:  environment,
            Duration:     time.Since(startTime),
            Statistics:   stats,
            CompletedAt:  time.Now(),
        })
    }
    
    return environment, nil
}

func (g *GraphGenerator) publishProgress(ctx context.Context, generationID, stage string, progress float64, message string) {
    if g.generationProgress != nil {
        _ = g.generationProgress.Publish(ctx, GenerationProgressEvent{
            GenerationID: generationID,
            Stage:        stage,
            Progress:     progress,
            Message:      message,
            Timestamp:    time.Now(),
        })
    }
}
```

### 3. Spawn Module Migration

#### Current Event Inventory

**Entity Operations**:
- `spawn.entity.spawned` → `EntitySpawnedEvent`

**Capacity Management**:
- `spawn.split.recommended` → `SplitRecommendedEvent`
- `spawn.room.scaled` → `RoomScaledEvent`

#### Migration Implementation

**1. Event Type Definitions**

```go
// File: tools/spawn/events.go

type EntitySpawnedEvent struct {
    EntityID     string
    Position     spatial.Position
    RoomID       string
    SpawnType    string // "scattered", "formation", "player_choice"
    SpawnGroupID string
    Constraints  []string
    SpawnedAt    time.Time
}

type SplitRecommendedEvent struct {
    RoomID          string
    CurrentCapacity int
    RequiredCapacity int
    RecommendationType string
    Reason          string
    Timestamp       time.Time
}

type RoomScaledEvent struct {
    RoomID      string
    OldCapacity int
    NewCapacity int
    ScaleReason string
    ScaledAt    time.Time
}
```

**2. Topic Definitions**

```go
// File: tools/spawn/topics.go

var (
    EntitySpawnedTopic     = events.DefineTypedTopic[EntitySpawnedEvent]("spawn.entity.spawned")
    SplitRecommendedTopic  = events.DefineTypedTopic[SplitRecommendedEvent]("spawn.split.recommended")
    RoomScaledTopic        = events.DefineTypedTopic[RoomScaledEvent]("spawn.room.scaled")
)
```

**3. Engine Implementation Update**

```go
// File: tools/spawn/basic_engine.go

type BasicSpawnEngine struct {
    // ... existing fields ...
    
    // New: Type-safe event publishers
    entitySpawned     TypedTopic[EntitySpawnedEvent]
    splitRecommended  TypedTopic[SplitRecommendedEvent]
    roomScaled        TypedTopic[RoomScaledEvent]
}

func (e *BasicSpawnEngine) ConnectToEventBus(bus events.EventBus) {
    e.entitySpawned = EntitySpawnedTopic.On(bus)
    e.splitRecommended = SplitRecommendedTopic.On(bus)
    e.roomScaled = RoomScaledTopic.On(bus)
}

func (e *BasicSpawnEngine) PlaceEntity(ctx context.Context, room spatial.Room, entity core.Entity, constraints []spawn.Constraint) error {
    // ... existing placement logic ...
    
    // Publish type-safe event
    if e.entitySpawned != nil {
        _ = e.entitySpawned.Publish(ctx, EntitySpawnedEvent{
            EntityID:     entity.GetID(),
            Position:     position,
            RoomID:       room.ID(),
            SpawnType:    e.currentSpawnType,
            SpawnGroupID: e.currentGroupID,
            Constraints:  constraintNames(constraints),
            SpawnedAt:    time.Now(),
        })
    }
    
    return nil
}
```

## Cross-Module Event Integration

### Event Bus Connection Pattern

All modules implement a standardized connection pattern:

```go
// Standard interface for event bus connection
type EventBusConnectable interface {
    ConnectToEventBus(bus events.EventBus)
}

// Usage in initialization - simple dependency injection
func InitializeToolkitModules(bus events.EventBus) {
    // Connect all modules to the shared event bus
    spatialOrchestrator.ConnectToEventBus(bus)
    environmentGenerator.ConnectToEventBus(bus)  
    spawnEngine.ConnectToEventBus(bus)
}
```

### Cross-Module Event Subscriptions

**Environment listens to Spatial events**:
```go
// In environment module initialization
func (g *GraphGenerator) ConnectToEventBus(bus events.EventBus) {
    // ... connect own publishers ...
    
    // Subscribe to spatial events for environment updates
    roomCreations := spatial.RoomCreatedTopic.On(bus)
    roomCreations.Subscribe(ctx, g.handleRoomCreated)
}

func (g *GraphGenerator) handleRoomCreated(ctx context.Context, e spatial.RoomCreatedEvent) error {
    // Update environment when new rooms are created
    return g.updateEnvironmentForRoom(ctx, e.RoomID)
}
```

**Spawn listens to Spatial and Environment events**:
```go
func (e *BasicSpawnEngine) ConnectToEventBus(bus events.EventBus) {
    // ... connect own publishers ...
    
    // Subscribe to relevant events
    roomCreations := spatial.RoomCreatedTopic.On(bus)
    roomCreations.Subscribe(ctx, e.handleNewRoom)
    
    environmentGenerated := environments.EnvironmentGeneratedTopic.On(bus)
    environmentGenerated.Subscribe(ctx, e.handleEnvironmentReady)
}
```

## Migration Timeline and Phases

### Phase 1: Foundation & Module Migration (Week 1)

**Goals**:
- Define all event types and topics for all modules
- Replace legacy string-based events with type-safe events
- Remove old event system code completely

**Deliverables**:
- `tools/spatial/events.go`, `topics.go` - Entity/room lifecycle events
- `tools/environments/events.go`, `topics.go` - Generation/environment events  
- `tools/spawn/events.go`, `topics.go` - Spawn/capacity events
- Updated Room, Orchestrator, GraphGenerator, BasicSpawnEngine implementations
- Complete removal of legacy event constants and publishing code

### Phase 2: Integration & Testing (Week 2)

**Goals**:
- Cross-module event subscriptions and integration
- Comprehensive testing of new event system
- Performance validation against legacy system

**Deliverables**:
- Cross-module event flow implementation (environment ← spatial, spawn ← spatial/environment)
- Unit tests for all event types and topic connections
- Integration tests for cross-module event flows
- Performance benchmarks vs. legacy system baseline

### Phase 3: Polish & Documentation (Week 3)

**Goals**:
- Final cleanup and optimization
- Documentation updates
- Address any performance or integration issues

**Deliverables**:
- Optimized event publishing patterns
- Updated module documentation and examples
- Performance optimization (if needed)
- Migration validation and sign-off

### Phase 4: Buffer/Contingency (Week 4)

**Goals**:
- Handle unexpected issues
- Final integration validation
- Future planning

**Deliverables**:
- Resolution of any integration edge cases
- Final performance validation
- Post-migration analysis and lessons learned

## Testing Strategy

### Unit Testing

**Event Type Tests**:
```go
func TestEntityPlacedEvent(t *testing.T) {
    event := EntityPlacedEvent{
        EntityID: "test-entity",
        Position: spatial.Position{X: 5, Y: 10},
        RoomID:   "test-room",
        GridType: "square",
    }
    
    assert.Equal(t, "test-entity", event.EntityID)
    assert.Equal(t, 5, event.Position.X)
    assert.Equal(t, 10, event.Position.Y)
}
```

**Topic Connection Tests**:
```go
func TestEntityPlacedTopicConnection(t *testing.T) {
    bus := events.NewEventBus()
    
    // Test topic connection
    placements := EntityPlacedTopic.On(bus)
    assert.NotNil(t, placements)
    
    // Test subscription
    received := make(chan EntityPlacedEvent, 1)
    _, err := placements.Subscribe(context.Background(), func(ctx context.Context, e EntityPlacedEvent) error {
        received <- e
        return nil
    })
    assert.NoError(t, err)
    
    // Test publishing
    testEvent := EntityPlacedEvent{EntityID: "test"}
    err = placements.Publish(context.Background(), testEvent)
    assert.NoError(t, err)
    
    // Verify event received
    select {
    case event := <-received:
        assert.Equal(t, "test", event.EntityID)
    case <-time.After(time.Second):
        t.Fatal("Event not received")
    }
}
```

### Integration Testing

**Cross-Module Event Flow**:
```go
func TestSpatialToEnvironmentEventFlow(t *testing.T) {
    bus := events.NewEventBus()
    ctx := context.Background()
    
    // Set up environment listener
    roomCreations := spatial.RoomCreatedTopic.On(bus)
    environmentUpdates := make(chan bool, 1)
    
    _, err := roomCreations.Subscribe(ctx, func(ctx context.Context, e spatial.RoomCreatedEvent) error {
        // Simulate environment response
        environmentUpdates <- true
        return nil
    })
    assert.NoError(t, err)
    
    // Create spatial room (triggers event)
    room := spatial.NewBasicRoom(/* config */)
    room.ConnectToEventBus(bus)
    
    // Verify environment received the event
    select {
    case <-environmentUpdates:
        // Success
    case <-time.After(time.Second):
        t.Fatal("Environment did not receive room creation event")
    }
}
```

### Performance Testing

**Event Throughput Benchmarks**:
```go
func BenchmarkEntityPlacedEvents(b *testing.B) {
    bus := events.NewEventBus()
    placements := EntityPlacedTopic.On(bus)
    
    ctx := context.Background()
    event := EntityPlacedEvent{EntityID: "bench-entity"}
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _ = placements.Publish(ctx, event)
    }
}
```

## Migration Risks and Mitigations

### Risk: Integration Issues

**Problem**: Cross-module event flows may break during migration
**Mitigation**:
- Comprehensive integration testing before legacy code removal
- Test all event subscription patterns between modules
- Validate event ordering and dependencies

### Risk: Performance Impact

**Problem**: Type-safe events may have overhead compared to string-based system
**Mitigation**:
- Establish performance baseline before migration
- Event pooling for high-frequency events if needed
- Benchmark-driven optimization

### Risk: Missing Event Coverage

**Problem**: Some legacy events might be missed during conversion
**Mitigation**:
- Complete audit of all existing event constants
- Search codebase for all event publishing and subscription points  
- Systematic removal of legacy event code to catch missed conversions

### Risk: Event Type Design Issues

**Problem**: Event structures may not capture all needed information
**Mitigation**:
- Review all current event usage patterns
- Design event types with extension points for future needs
- Validate event types against actual usage scenarios

## Success Metrics

### Functional Metrics

- **Complete Migration**: Zero legacy string-based events remain in codebase
- **Type Safety**: All event publishing and subscription is type-safe at compile-time
- **Cross-Module Integration**: All modules successfully exchange typed events
- **Event Coverage**: All previous event functionality preserved with new system

### Performance Metrics

- **Event Throughput**: >= baseline performance of legacy system
- **Memory Usage**: <= 10% increase over legacy system  
- **CPU Overhead**: <= 5% increase in event processing
- **No Performance Regressions**: All existing functionality maintains performance

### Developer Experience Metrics

- **Code Clarity**: Event flow is explicit and visible through `.On(bus)` pattern
- **IDE Support**: Full autocomplete and type checking for all events
- **Simplified Debugging**: Event types are self-documenting with structured data
- **Compile-time Safety**: Impossible to subscribe to wrong event types

## Conclusion

This migration transforms the rpg-toolkit from a runtime string-based event system to a compile-time type-safe architecture. The `.On(bus)` pattern provides explicit visualization of event flow while maintaining the flexibility and power of the existing event-driven architecture.

With no external users requiring backwards compatibility, this becomes a straightforward refactoring that can be completed in 3-4 weeks. The clean cut-over approach eliminates migration complexity while comprehensive testing ensures functional correctness and performance validation.

Upon completion, the toolkit will have a more maintainable, discoverable, and reliable event system that provides compile-time safety and explicit event flow visualization for the complex interactions between spatial, environment, and spawn modules.