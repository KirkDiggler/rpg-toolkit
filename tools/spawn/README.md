# Spawn Engine

The Spawn Engine provides intelligent entity placement capabilities for game spaces, implementing the complete ADR-0013 specification with split-aware architecture, constraint validation, and adaptive room scaling.

## Purpose

The spawn engine serves as the intelligent orchestrator for populating game spaces with entities (players, enemies, treasure, environmental objects). It bridges the gap between high-level game design decisions ("spawn some goblins in this dungeon room") and low-level spatial placement mechanics.

**Core Responsibilities:**
- **Entity Selection**: Uses selectables tables for weighted, context-aware entity selection
- **Spatial Placement**: Positions entities using spatial constraints and placement patterns
- **Constraint Validation**: Ensures line of sight, distance, and area of effect requirements
- **Capacity Management**: Integrates with environment package for room sizing and splitting
- **Multi-Room Awareness**: Handles spawning across connected room configurations

## Scope

### ✅ Implemented (Phases 1-4)

**Phase 1: Basic Infrastructure**
- Core SpawnEngine interface with all methods
- BasicSpawnEngine implementation with dependency injection
- Scattered spawning pattern (random placement)
- Entity selection using selectables tables
- Event publishing for spawn operations

**Phase 2: Advanced Patterns**
- Formation-based spawning (structured arrangements)
- Team-based spawning (ally/enemy separation)
- Player choice spawning (designated spawn zones)
- Clustered spawning (grouped entity placement)
- Configuration validation and error handling

**Phase 3: Constraint System**
- Spatial constraint validation (minimum distances, wall proximity)
- Line of sight requirements (required and blocked sight)
- Area of effect constraints (exclusion zones)
- Gridless room support with continuous positioning
- Constraint solver with position finding algorithms

**Phase 4: Environment Integration**
- Capacity analysis using environment package queries
- Adaptive room scaling when entities don't fit
- Split recommendations as passthrough from environment package
- Room modification tracking and event publishing

**Cross-Cutting Features:**
- Comprehensive event system for observability
- Split-aware architecture (works with single or connected rooms)
- Configuration-driven behavior with validation
- Error handling with detailed failure reporting

### 🚧 Planned Future Enhancements

- **Advanced Formation Patterns**: Complex geometric arrangements, formation rotation
- **Dynamic Constraints**: Runtime constraint modification based on game state
- **Performance Optimization**: Spatial indexing for large rooms with many entities
- **AI Integration**: Smart placement based on tactical considerations

## Architecture

The spawn engine follows RPG Toolkit's philosophy of **infrastructure, not implementation**:

- **Entity Agnostic**: Works with any entity type implementing `core.Entity`
- **Selection Flexible**: Uses selectables module for configurable entity selection
- **Spatially Integrated**: Leverages spatial module for all positioning operations
- **Environmentally Aware**: Integrates with environment package for capacity analysis
- **Event Driven**: Publishes events for all operations for external observability

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Game Logic    │    │  Spawn Engine    │    │ Spatial Module  │
│                 │────┤                  │────┤                 │
│ - Define pools  │    │ - Entity selection│    │ - Room mgmt     │
│ - Set patterns  │    │ - Constraint validation │ - Position validation │
│ - Handle events │    │ - Pattern application │ │ - Collision detection │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                                │
                       ┌──────────────────┐
                       │ Environment Pkg  │
                       │                  │
                       │ - Capacity analysis │
                       │ - Room scaling   │
                       │ - Split recommendations │
                       └──────────────────┘
```

## Usage Examples

### Basic Setup

```go
package main

import (
    "context"
    "log"
    
    "github.com/KirkDiggler/rpg-toolkit/core"
    "github.com/KirkDiggler/rpg-toolkit/events"
    "github.com/KirkDiggler/rpg-toolkit/tools/spawn"
    "github.com/KirkDiggler/rpg-toolkit/tools/spatial"
    "github.com/KirkDiggler/rpg-toolkit/tools/environments"
)

func main() {
    // Create dependencies
    eventBus := events.NewBasicEventBus()
    
    // Create spawn engine with dependencies
    engine := spawn.NewBasicSpawnEngine(spawn.BasicSpawnEngineConfig{
        ID:                 "dungeon-spawner",
        SpatialHandler:     spatialHandler,      // Your spatial query handler
        EnvironmentHandler: environmentHandler,  // Your environment query handler  
        SelectablesReg:     selectablesRegistry, // Your entity selection tables
        EventBus:           eventBus,
        EnableEvents:       true,
    })
    
    // Register entity selection tables
    enemies := []core.Entity{
        &GameEntity{ID: "orc1", Type: "enemy"},
        &GameEntity{ID: "goblin1", Type: "enemy"},
        &GameEntity{ID: "skeleton1", Type: "enemy"},
    }
    
    selectablesRegistry.RegisterTable("basic-enemies", enemies)
}
```

### Simple Enemy Spawning

```go
// Basic scattered spawning
config := spawn.SpawnConfig{
    EntityGroups: []spawn.EntityGroup{
        {
            ID:             "room-enemies",
            Type:           "enemy", 
            SelectionTable: "basic-enemies",
            Quantity:       spawn.QuantitySpec{Fixed: &[]int{3}[0]},
        },
    },
    Pattern: spawn.PatternScattered,
}

result, err := engine.PopulateRoom(ctx, "dungeon-room-1", config)
if err != nil {
    log.Fatal(err)
}

log.Printf("Spawned %d entities, %d failures", 
    len(result.SpawnedEntities), len(result.Failures))
```

### Constraint-Based Spawning

```go
// Spawning with spatial constraints
config := spawn.SpawnConfig{
    EntityGroups: []spawn.EntityGroup{
        {
            ID:             "guards",
            Type:           "guard",
            SelectionTable: "elite-enemies", 
            Quantity:       spawn.QuantitySpec{Fixed: &[]int{2}[0]},
        },
        {
            ID:             "treasure",
            Type:           "treasure",
            SelectionTable: "valuable-loot",
            Quantity:       spawn.QuantitySpec{Fixed: &[]int{1}[0]},
        },
    },
    Pattern: spawn.PatternScattered,
    SpatialRules: spawn.SpatialConstraints{
        MinDistance: map[string]float64{
            "guard:treasure": 3.0, // Guards stay close to treasure
        },
        WallProximity: 1.0, // All entities 1 unit from walls
        LineOfSight: spawn.LineOfSightRules{
            RequiredSight: []spawn.EntityPair{
                {From: "guard", To: "treasure"}, // Guards must see treasure
            },
        },
    },
}

result, err := engine.PopulateRoom(ctx, "treasure-room", config)
```

### Team-Based Spawning

```go
// Tactical team placement
config := spawn.SpawnConfig{
    EntityGroups: []spawn.EntityGroup{
        {
            ID:             "defenders",
            Type:           "ally",
            SelectionTable: "town-guards",
            Quantity:       spawn.QuantitySpec{Fixed: &[]int{4}[0]},
        },
        {
            ID:             "attackers", 
            Type:           "enemy",
            SelectionTable: "orc-raiders",
            Quantity:       spawn.QuantitySpec{Fixed: &[]int{6}[0]},
        },
    },
    Pattern: spawn.PatternTeamBased,
    TeamConfiguration: &spawn.TeamConfig{
        Teams: []spawn.Team{
            {
                ID:          "defenders",
                EntityTypes: []string{"ally"},
                Allegiance:  "town",
            },
            {
                ID:          "attackers", 
                EntityTypes: []string{"enemy"},
                Allegiance:  "orc-clan",
            },
        },
        CohesionRules: spawn.TeamCohesionRules{
            KeepFriendliesTogether: true,
            KeepEnemiesTogether:    true,
            MinTeamSeparation:      5.0,
        },
    },
}

result, err := engine.PopulateRoom(ctx, "battlefield", config)
```

### Player Choice Spawning

```go
// Allow players to choose spawn positions
config := spawn.SpawnConfig{
    EntityGroups: []spawn.EntityGroup{
        {
            ID:             "player-characters",
            Type:           "player",
            SelectionTable: "active-players",
            Quantity:       spawn.QuantitySpec{Fixed: &[]int{4}[0]},
        },
    },
    Pattern: spawn.PatternPlayerChoice,
    PlayerSpawnZones: []spawn.SpawnZone{
        {
            ID:          "safe-zone",
            Area:        spatial.Rectangle{Position: spatial.Position{X: 1, Y: 1}, Width: 3, Height: 3},
            EntityTypes: []string{"player"},
            MaxEntities: 4,
        },
    },
    PlayerChoices: []spawn.PlayerSpawnChoice{
        {PlayerID: "hero1", ZoneID: "safe-zone", Position: spatial.Position{X: 2, Y: 2}},
        // Other players auto-assigned to zone
    },
}

result, err := engine.PopulateRoom(ctx, "starting-room", config)
```

### Multi-Room Spawning

```go
// Spawning across connected rooms
connectedRooms := []string{"room-1", "room-2", "room-3"}

config := spawn.SpawnConfig{
    EntityGroups: []spawn.EntityGroup{
        {
            ID:             "distributed-enemies",
            Type:           "enemy",
            SelectionTable: "dungeon-monsters",
            Quantity:       spawn.QuantitySpec{Fixed: &[]int{12}[0]},
        },
    },
    Pattern: spawn.PatternScattered,
}

// Spawn engine automatically distributes entities across connected rooms
result, err := engine.PopulateSplitRooms(ctx, connectedRooms, config)
```

### Adaptive Scaling

```go
// Automatic room scaling when entities don't fit
config := spawn.SpawnConfig{
    EntityGroups: []spawn.EntityGroup{
        {
            ID:             "large-army",
            Type:           "soldier", 
            SelectionTable: "imperial-legion",
            Quantity:       spawn.QuantitySpec{Fixed: &[]int{50}[0]},
        },
    },
    Pattern: spawn.PatternFormation,
    AdaptiveScaling: &spawn.ScalingConfig{
        Enabled:        true,
        ScalingFactor:  1.5,
        PreserveAspect: true,
        EmitEvents:     true,
    },
}

result, err := engine.PopulateRoom(ctx, "throne-room", config)

// Check if room was scaled
for _, modification := range result.RoomModifications {
    if modification.Type == "scaled" {
        log.Printf("Room scaled: %s", modification.Reason)
    }
}

// Check for split recommendations
if len(result.SplitRecommendations) > 0 {
    log.Printf("Consider splitting room into %d parts", len(result.SplitRecommendations))
}
```

### Gridless Room Support

The spawn engine automatically detects gridless rooms and adapts its positioning strategy:

```go
// Create gridless room
gridlessRoom := spatial.NewGridlessRoom(spatial.GridlessConfig{
    Width:  20.0,
    Height: 15.0,
})

// Spawn engine automatically uses continuous positioning
config := spawn.SpawnConfig{
    EntityGroups: []spawn.EntityGroup{
        {
            ID:             "forest-creatures",
            Type:           "animal",
            SelectionTable: "woodland-animals", 
            Quantity:       spawn.QuantitySpec{Fixed: &[]int{8}[0]},
        },
    },
    Pattern: spawn.PatternScattered,
    SpatialRules: spawn.SpatialConstraints{
        WallProximity: 0.5, // Smaller margins for natural environments
    },
}

// Entities placed with smooth, continuous coordinates
result, err := engine.PopulateRoom(ctx, "forest-clearing", config)
```

### Event Handling

```go
// Subscribe to spawn events for game logic integration
eventBus.SubscribeFunc("spawn.entity.spawned", 10, func(ctx context.Context, event events.Event) error {
    entityID := event.Context().Get("entity_id").(string)
    entityType := event.Context().Get("entity_type").(string)
    posX := event.Context().Get("position_x").(float64)
    posY := event.Context().Get("position_y").(float64)
    
    log.Printf("Entity spawned: %s (%s) at (%.2f, %.2f)", entityID, entityType, posX, posY)
    
    // Trigger game-specific logic (AI activation, animations, etc.)
    return gameLogic.HandleEntitySpawn(entityID, entityType, posX, posY)
})

eventBus.SubscribeFunc("spawn.room.scaled", 10, func(ctx context.Context, event events.Event) error {
    roomID := event.Context().Get("room_id").(string)
    scaleFactor := event.Context().Get("scale_factor").(float64)
    
    log.Printf("Room %s scaled by factor %.2f", roomID, scaleFactor)
    
    // Update UI, notify players, etc.
    return gameUI.NotifyRoomScaled(roomID, scaleFactor)
})
```

## Configuration Reference

### SpawnConfig Structure

The main configuration object controlling all spawn behavior:

```go
type SpawnConfig struct {
    // What to spawn
    EntityGroups []EntityGroup `json:"entity_groups"`
    
    // How to spawn
    Pattern           SpawnPattern `json:"pattern"`
    TeamConfiguration *TeamConfig  `json:"team_config,omitempty"`
    
    // Constraints  
    SpatialRules SpatialConstraints `json:"spatial_rules"`
    Placement    PlacementRules     `json:"placement"`
    
    // Behavior
    Strategy        SpawnStrategy  `json:"strategy"`
    AdaptiveScaling *ScalingConfig `json:"adaptive_scaling,omitempty"`
    
    // Player spawn zones and choices
    PlayerSpawnZones []SpawnZone         `json:"player_spawn_zones,omitempty"`
    PlayerChoices    []PlayerSpawnChoice `json:"player_choices,omitempty"`
}
```

### EntityGroup Definition

Defines a group of entities to spawn with selection rules:

```go
type EntityGroup struct {
    ID             string       `json:"id"`              // Unique identifier
    Type           string       `json:"type"`            // Entity category
    SelectionTable string       `json:"selection_table"` // Selectables table ID
    Quantity       QuantitySpec `json:"quantity"`        // How many to spawn
}

type QuantitySpec struct {
    Fixed    *int    `json:"fixed,omitempty"`     // Exact count
    DiceRoll *string `json:"dice_roll,omitempty"` // Future: "2d6+1" 
    Min      *int    `json:"min,omitempty"`       // Future: range
    Max      *int    `json:"max,omitempty"`       // Future: range
}
```

### Spawn Patterns

Available placement patterns:

- **`PatternScattered`**: Random placement throughout room
- **`PatternFormation`**: Structured geometric arrangements  
- **`PatternTeamBased`**: Team separation with cohesion rules
- **`PatternPlayerChoice`**: Player-selected positions within zones
- **`PatternClustered`**: Grouped placement in clusters

### Spatial Constraints

Fine-grained control over entity positioning:

```go
type SpatialConstraints struct {
    MinDistance   map[string]float64  `json:"min_distance"`   // "type1:type2" -> distance
    LineOfSight   LineOfSightRules    `json:"line_of_sight"`  // Visibility requirements
    WallProximity float64             `json:"wall_proximity"` // Distance from walls
    AreaOfEffect  map[string]float64  `json:"area_of_effect"` // Exclusion zones
    PathingRules  PathingConstraints  `json:"pathing_rules"`  // Movement constraints
}

type LineOfSightRules struct {
    RequiredSight []EntityPair `json:"required_sight"` // Must see each other
    BlockedSight  []EntityPair `json:"blocked_sight"`  // Must NOT see each other
}

type EntityPair struct {
    From string `json:"from"` // Source entity type
    To   string `json:"to"`   // Target entity type  
}
```

## Error Handling

The spawn engine provides comprehensive error reporting:

```go
type SpawnResult struct {
    Success              bool               `json:"success"`
    SpawnedEntities      []SpawnedEntity    `json:"spawned_entities"`
    Failures             []SpawnFailure     `json:"failures"`
    RoomModifications    []RoomModification `json:"room_modifications"`
    SplitRecommendations []RoomSplit        `json:"split_recommendations"`
    RoomStructure        RoomStructureInfo  `json:"room_structure"`
}

type SpawnFailure struct {
    EntityType string `json:"entity_type"`
    Reason     string `json:"reason"`
}
```

**Common Error Scenarios:**
- **Selection Failures**: Entity table not found, insufficient entities in table
- **Constraint Violations**: No positions satisfy spatial constraints
- **Capacity Issues**: Room too small for requested entities (triggers scaling/splitting)
- **Configuration Errors**: Invalid spawn patterns, malformed constraints

## Events

Published events for external integration:

- **`spawn.entity.spawned`**: Individual entity placement
- **`spawn.operation.completed`**: Full spawn operation finished
- **`spawn.room.scaled`**: Room dimensions modified
- **`spawn.split.recommended`**: Room splitting suggested
- **`spawn.constraint.violation`**: Placement constraint failed

## Implementation Status

This implementation represents **Phases 1-4** of ADR-0013:

**✅ Complete:**
- Basic spawn engine infrastructure
- All spawn patterns (scattered, formation, team-based, player choice, clustered)
- Spatial constraint system with validation
- Environment integration for capacity analysis and room scaling
- Split-aware architecture for multi-room scenarios
- Comprehensive event system
- Gridless room support
- **Full test coverage including environment integration tests**

**🔄 Future Phases:**
- Advanced formation patterns with complex geometries
- Dynamic constraint modification during gameplay
- Performance optimization for large-scale scenarios
- AI-driven tactical placement algorithms

## Dependencies

- **`core`**: Base entity interfaces
- **`events`**: Event bus for observability  
- **`tools/spatial`**: Room management and positioning
- **`tools/environments`**: Capacity analysis and room scaling
- **`tools/selectables`**: Weighted entity selection tables

For complete implementation details, see [ADR-0013: Entity Spawn Engine](../../docs/adr/0013-entity-spawn-engine.md).