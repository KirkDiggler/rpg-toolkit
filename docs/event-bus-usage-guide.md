# RPG Toolkit Event Bus Usage Guide

## Overview

The RPG Toolkit uses a type-safe event bus architecture that allows modules to communicate through strongly-typed events without direct coupling. This guide shows you how to use the event bus effectively in real game scenarios.

## Key Concepts

### Type-Safe Events
Every event is defined with a specific data structure and compiled-time type checking:

```go
// Event types are defined in each module's topics.go file
type EntityMovedEvent struct {
    RoomID      string    `json:"room_id"`
    EntityID    string    `json:"entity_id"`
    OldPosition Position  `json:"old_position"`
    NewPosition Position  `json:"new_position"`
    MovedAt     time.Time `json:"moved_at"`
}

// Topics are strongly typed
var EntityMovedTopic = events.DefineTypedTopic[EntityMovedEvent]("spatial.entity.moved")
```

### Connection Pattern
All components must connect to the event bus to publish or subscribe to events:

```go
// Connect typed topics to the event bus
component.ConnectToEventBus(eventBus)

// This enables both publishing and subscribing through typed topics
```

## Complete Usage Example: Entity Movement Chain

Let's walk through a complete example where a player moves an entity, triggering a cascade of events across multiple modules.

### 1. Setting Up the Event Bus

```go
// Create the event bus
eventBus := events.NewSimpleEventBus()

// Create all game components
room := spatial.NewBasicRoom(spatial.BasicRoomConfig{
    ID:   "dungeon-room-1",
    Type: "dungeon",
    Grid: spatial.NewSquareGrid(20, 20),
})

environment := environments.NewBasicEnvironment(environments.BasicEnvironmentConfig{
    ID:           "dark-dungeon",
    Type:         "dungeon",
    Theme:        "dark",
    Orchestrator: orchestrator,
    QueryHandler: queryHandler,
})

spawnEngine := spawn.NewBasicSpawnEngine(spawn.BasicSpawnEngineConfig{
    ID:   "monster-spawner",
    Type: "dungeon-spawner",
})

lootTable := selectables.NewBasicTable[LootItem](selectables.BasicTableConfig{
    ID:   "trap-loot",
    Type: "treasure",
})

// CRITICAL: Connect all components to event bus
room.ConnectToEventBus(eventBus)
environment.ConnectToEventBus(eventBus)
spawnEngine.ConnectToEventBus(eventBus)
lootTable.ConnectToEventBus(eventBus)
```

### 2. Spatial Module: Publishing Movement Events

```go
// In tools/spatial/room.go
func (r *BasicRoom) MoveEntity(ctx context.Context, entityID string, newPos Position) error {
    // Get current position
    oldPos, exists := r.entities[entityID]
    if !exists {
        return fmt.Errorf("entity %s not found in room", entityID)
    }

    // Perform the move
    r.entities[entityID] = newPos

    // Publish typed event - this is where the magic happens
    _ = r.entityMoved.Publish(ctx, EntityMovedEvent{
        RoomID:      r.id,
        EntityID:    entityID,
        OldPosition: oldPos,
        NewPosition: newPos,
        MovedAt:     time.Now(),
    })

    return nil
}

// Connection method enables publishing
func (r *BasicRoom) ConnectToEventBus(bus events.EventBus) {
    r.entityMoved = EntityMovedTopic.On(bus)    // Now we can publish
    r.entityPlaced = EntityPlacedTopic.On(bus)
    r.entityRemoved = EntityRemovedTopic.On(bus)
    
    // Subscribe to events from other modules if needed
    _, _ = environments.HazardTriggeredTopic.On(bus).Subscribe(ctx,
        func(ctx context.Context, event environments.HazardTriggeredEvent) error {
            return r.handleEnvironmentalHazard(ctx, event)
        })
}
```

### 3. Environments Module: Subscribing and Reacting

```go
// In tools/environments/environment.go
func (e *BasicEnvironment) ConnectToEventBus(bus events.EventBus) {
    // Set up publishers
    e.hazardTriggered = HazardTriggeredTopic.On(bus)
    e.themeChanged = ThemeChangedTopic.On(bus)
    
    // Subscribe to spatial events with type-safe handlers
    _, _ = spatial.EntityMovedTopic.On(bus).Subscribe(ctx,
        func(ctx context.Context, event spatial.EntityMovedEvent) error {
            return e.handleEntityMovement(ctx, event)
        })
        
    _, _ = spatial.EntityPlacedTopic.On(bus).Subscribe(ctx,
        func(ctx context.Context, event spatial.EntityPlacedEvent) error {
            return e.handleEntityPlacement(ctx, event)
        })
}

func (e *BasicEnvironment) handleEntityMovement(ctx context.Context, event spatial.EntityMovedEvent) error {
    // Check for environmental hazards at the new position
    hazards := e.getHazardsAtPosition(event.NewPosition)
    
    for _, hazard := range hazards {
        // Publish hazard triggered event
        _ = e.hazardTriggered.Publish(ctx, HazardTriggeredEvent{
            EnvironmentID: e.id,
            HazardID:      hazard.ID,
            HazardType:    hazard.Type,
            TriggerEntity: event.EntityID,
            RoomID:        event.RoomID,
            Effect:        hazard.Effect,
            TriggeredAt:   time.Now(),
        })
    }

    // Check if movement triggers theme changes
    if e.shouldChangeTheme(event.NewPosition) {
        e.updateTheme("combat")
    }

    return nil
}

func (e *BasicEnvironment) updateTheme(newTheme string) {
    oldTheme := e.theme
    e.theme = newTheme

    // Publish theme change event
    _ = e.themeChanged.Publish(context.Background(), ThemeChangedEvent{
        EnvironmentID: e.id,
        OldTheme:      oldTheme,
        NewTheme:      newTheme,
        AffectedRooms: e.getAllRoomIDs(),
        ChangedAt:     time.Now(),
    })
}
```

### 4. Spawn Module: Reacting to Environmental Events

```go
// In tools/spawn/basic_engine.go
func (e *BasicSpawnEngine) ConnectToEventBus(bus events.EventBus) {
    // Set up publishers
    e.entitySpawned = EntitySpawnedTopic.On(bus)
    
    // Subscribe to environmental hazard events
    _, _ = environments.HazardTriggeredTopic.On(bus).Subscribe(ctx,
        func(ctx context.Context, event environments.HazardTriggeredEvent) error {
            return e.handleHazardTriggered(ctx, event)
        })
        
    // Subscribe to theme changes for dynamic spawning
    _, _ = environments.ThemeChangedTopic.On(bus).Subscribe(ctx,
        func(ctx context.Context, event environments.ThemeChangedEvent) error {
            return e.handleThemeChange(ctx, event)
        })
}

func (e *BasicSpawnEngine) handleHazardTriggered(ctx context.Context, event environments.HazardTriggeredEvent) error {
    // Certain hazards trigger monster spawns
    if event.HazardType == "alarm_trap" {
        // Spawn guards in response to alarm
        spawnRequest := SpawnRequest{
            EntityType:  "guard",
            Count:       2,
            RoomID:      event.RoomID,
            TriggerType: "hazard_response",
        }
        
        entities, err := e.spawnEntities(ctx, spawnRequest)
        if err != nil {
            return err
        }

        // Publish spawn events
        for _, entity := range entities {
            _ = e.entitySpawned.Publish(ctx, EntitySpawnedEvent{
                EntityID:    entity.ID,
                EntityType:  entity.Type,
                RoomID:      event.RoomID,
                SpawnReason: "hazard_triggered",
                SpawnedAt:   time.Now(),
            })
        }
    }

    return nil
}

func (e *BasicSpawnEngine) handleThemeChange(ctx context.Context, event environments.ThemeChangedEvent) error {
    // Theme change from "exploration" to "combat" triggers reinforcements
    if event.OldTheme == "exploration" && event.NewTheme == "combat" {
        // Spawn additional monsters for combat encounter
        for _, roomID := range event.AffectedRooms {
            e.spawnCombatReinforcements(ctx, roomID)
        }
    }

    return nil
}
```

### 5. Selectables Module: Dynamic Loot Generation

```go
// In tools/selectables/basic_table.go
func (t *BasicTable[T]) ConnectToEventBus(bus events.EventBus) {
    // Subscribe to hazard events for loot drops
    _, _ = environments.HazardTriggeredTopic.On(bus).Subscribe(ctx,
        func(ctx context.Context, event environments.HazardTriggeredEvent) error {
            return t.handleHazardLoot(ctx, event)
        })
        
    // Subscribe to spawn events for creature loot
    _, _ = spawn.EntitySpawnedTopic.On(bus).Subscribe(ctx,
        func(ctx context.Context, event spawn.EntitySpawnedEvent) error {
            return t.handleSpawnLoot(ctx, event)
        })
}

func (t *BasicTable[T]) handleHazardLoot(ctx context.Context, event environments.HazardTriggeredEvent) error {
    // Only certain hazards drop loot
    if event.HazardType == "treasure_trap" || event.HazardType == "chest_trap" {
        // Generate loot based on hazard type
        context := SelectionContext{
            "hazard_type": event.HazardType,
            "room_id":     event.RoomID,
            "trigger":     "hazard",
        }
        
        loot, err := t.SelectMany(context, 1) // Roll for one item
        if err != nil {
            return err
        }

        // In a real game, you'd place this loot in the room
        // This could trigger another event: LootGeneratedEvent
        log.Printf("Generated loot from %s: %+v", event.HazardType, loot)
    }

    return nil
}
```

## Event Flow Visualization

Here's what happens when a player moves an entity:

```
Player Action: Move Hero to (10, 5)
    ↓
[SPATIAL] EntityMovedEvent
    ↓
[ENVIRONMENTS] Detects trap at (10, 5)
    ↓
[ENVIRONMENTS] HazardTriggeredEvent (trap)
    ↓                    ↓
[SPAWN] Alarm trap    [SELECTABLES] Treasure trap
spawns guards         generates loot
    ↓                    ↓
[SPAWN] EntitySpawnedEvent    [GAME] Places loot in room
    ↓
[ENVIRONMENTS] Guards trigger combat theme
    ↓
[ENVIRONMENTS] ThemeChangedEvent
    ↓
[SPAWN] Combat theme spawns reinforcements
```

## Best Practices

### 1. Always Connect to Event Bus
```go
// ✅ DO: Connect all components
component.ConnectToEventBus(eventBus)

// ❌ DON'T: Try to publish without connecting
component.PublishSomething() // Will fail - topics not connected
```

### 2. Use Type-Safe Event Handlers
```go
// ✅ DO: Use strongly typed handlers
_, _ = SomeEventTopic.On(bus).Subscribe(ctx,
    func(ctx context.Context, event SomeEvent) error {
        // event.Field is strongly typed
        return nil
    })

// ❌ DON'T: Use untyped interfaces
eventBus.Subscribe(ctx, "some.event", func(data interface{}) {
    // Need to cast, no compile-time safety
})
```

### 3. Handle Subscription Errors
```go
// ✅ DO: Check subscription errors in production
subscriptionID, err := SomeEventTopic.On(bus).Subscribe(ctx, handler)
if err != nil {
    return fmt.Errorf("failed to subscribe to events: %w", err)
}

// ❌ DON'T: Ignore errors (okay for examples, bad for production)
_, _ = SomeEventTopic.On(bus).Subscribe(ctx, handler)
```

### 4. Design Events for Reusability
```go
// ✅ DO: Include relevant context in events
type EntityMovedEvent struct {
    RoomID      string    // Other modules can filter by room
    EntityID    string    // Clear identification
    EntityType  string    // Allows type-specific behavior
    OldPosition Position  // Full movement context
    NewPosition Position
    MovedAt     time.Time // Temporal context
}

// ❌ DON'T: Make events too specific to one use case
type HeroMovedToTrapEvent struct {
    // Too specific, hard to reuse
}
```

## Event Topics Reference

### Spatial Module Events
- `EntityPlacedTopic` - Entity added to room
- `EntityMovedTopic` - Entity moved within room
- `EntityRemovedTopic` - Entity removed from room
- `RoomCreatedTopic` - New room created
- `ConnectionAddedTopic` - Rooms connected

### Environments Module Events  
- `HazardTriggeredTopic` - Environmental hazard activated
- `ThemeChangedTopic` - Environment theme changed
- `EnvironmentEntityAddedTopic` - Entity added to environment
- `FeatureAddedTopic` - Environmental feature added

### Spawn Module Events
- `EntitySpawnedTopic` - New entity spawned
- `SplitRecommendedTopic` - Room capacity analysis suggests split
- `RoomScaledTopic` - Room size adjusted for spawning

### Selectables Module Events
- `SelectionCompletedTopic` - Selection made from table
- `ItemAddedTopic` - Item added to selection table
- `WeightChangedTopic` - Selection weights modified

## Advanced Patterns

### Cross-Module Chains
Events can trigger chains across multiple modules:

```
[USER] → [SPATIAL] → [ENVIRONMENTS] → [SPAWN] → [SELECTABLES]
```

### Event Filtering
Use event data to filter subscriptions:

```go
_, _ = EntityMovedTopic.On(bus).Subscribe(ctx,
    func(ctx context.Context, event EntityMovedEvent) error {
        // Only care about movements in specific rooms
        if event.RoomID == "boss-room" {
            return handleBossRoomMovement(ctx, event)
        }
        return nil // Ignore other rooms
    })
```

### Conditional Event Publishing
Only publish events when they matter:

```go
func (r *Room) MoveEntity(ctx context.Context, entityID string, newPos Position) error {
    oldPos := r.entities[entityID]
    r.entities[entityID] = newPos

    // Only publish if position actually changed
    if !oldPos.Equals(newPos) {
        _ = r.entityMoved.Publish(ctx, EntityMovedEvent{...})
    }

    return nil
}
```

## Testing with Event Bus

### Test Setup
```go
func TestEntityMovementChain(t *testing.T) {
    // Create test event bus
    eventBus := events.NewSimpleEventBus()
    
    // Create components
    room := spatial.NewBasicRoom(testConfig)
    env := environments.NewBasicEnvironment(testConfig)
    
    // Connect to event bus
    room.ConnectToEventBus(eventBus)
    env.ConnectToEventBus(eventBus)
    
    // Test the chain
    err := room.MoveEntity(ctx, "test-entity", newPosition)
    assert.NoError(t, err)
    
    // Verify events were published and handled
    // (Implementation depends on your test framework)
}
```

## Troubleshooting Common Issues

### Events Not Publishing
- **Cause**: Component not connected to event bus
- **Solution**: Call `component.ConnectToEventBus(eventBus)` before use

### Events Not Received
- **Cause**: Subscriber connected after publisher
- **Solution**: Connect all components to event bus during initialization

### Type Errors
- **Cause**: Wrong event type in handler
- **Solution**: Use compiler errors to guide correct event types

### Performance Issues
- **Cause**: Too many event subscriptions or heavy handlers
- **Solution**: Use event filtering and async processing where appropriate

---

This event bus system provides type-safe, loosely-coupled communication between RPG Toolkit modules while maintaining compile-time guarantees and clear event flow visibility.