# Spawn Engine

The Spawn Engine provides comprehensive entity placement and room population capabilities for game spaces. It implements the architecture defined in [ADR-0013](../../docs/adr/0013-entity-spawn-engine.md) with support for multiple spawn patterns, capacity analysis, and adaptive room scaling.

## Purpose and Scope

The spawn engine handles the complex logic of deciding what entities to place where based on game rules, spatial constraints, and procedural generation needs. It provides:

- **Multiple spawn patterns** (scattered, formation, team-based, player choice, clustered)
- **Capacity-driven spawning** with automatic room scaling
- **Split-aware architecture** for multi-room configurations
- **Environment integration** for space calculations and room analysis
- **Event-driven observability** for debugging and game logic integration

**Important**: The spawn engine does NOT create entities. Instead, clients provide categorized pools of pre-existing entities, and the spawn engine handles selection and positioning using the selectables and spatial modules.

## Core Concepts

### Entity Groups and Selection

The spawn engine uses "entity groups" to categorize entities for spawning:

```go
// Client provides entity pools
entityPools := map[string][]core.Entity{
    "goblinoids": {orc1, goblin1, bugbear1},
    "treasure": {coins1, gems1, sword1},
}

// Register selection tables
spawnEngine.RegisterTable("goblinoids", entityPools["goblinoids"])
spawnEngine.RegisterTable("treasure", entityPools["treasure"])

// Configure spawning
config := SpawnConfig{
    EntityGroups: []EntityGroup{
        {
            ID:             "enemies",
            Type:           "enemy", 
            SelectionTable: "goblinoids",
            Quantity:       QuantitySpec{Fixed: &three},
        },
        {
            ID:             "loot",
            Type:           "treasure",
            SelectionTable: "treasure", 
            Quantity:       QuantitySpec{Fixed: &one},
        },
    },
    Pattern: PatternScattered,
}
```

### Spawn Patterns

The engine supports multiple spatial arrangement patterns:

- **Scattered**: Random distribution across available space
- **Formation**: Structured arrangements (line, wedge, circle)
- **Team-based**: Separates teams into distinct areas
- **Player Choice**: Allows players to choose spawn positions within zones
- **Clustered**: Groups entities with spacing between clusters

### Split-Aware Architecture

The spawn engine can work with both single rooms and multi-room configurations:

```go
// Single room spawning
result, err := spawnEngine.PopulateRoom(ctx, "dungeon-room-1", config)

// Multi-room spawning 
connectedRooms := []string{"room-1", "room-2", "room-3"}
result, err := spawnEngine.PopulateSplitRooms(ctx, connectedRooms, config)

// Automatic detection
result, err := spawnEngine.PopulateSpace(ctx, roomOrGroup, config)
```

## Quick Start

### Basic Setup

```go
package main

import (
    "context"
    "log"
    
    "github.com/KirkDiggler/rpg-toolkit/tools/spawn"
    "github.com/KirkDiggler/rpg-toolkit/events"
    "github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

func main() {
    // Create dependencies
    eventBus := events.NewBus()
    spatialHandler := spatial.NewQueryHandler() // Your spatial implementation
    selectablesRegistry := spawn.NewBasicSelectablesRegistry()
    
    // Create spawn engine
    engine := spawn.NewBasicSpawnEngine(spawn.BasicSpawnEngineConfig{
        ID:             "dungeon-spawner",
        SpatialHandler: spatialHandler,
        SelectablesReg: selectablesRegistry,
        EventBus:       eventBus,
        EnableEvents:   true,
    })
    
    // Register entity tables
    enemies := []core.Entity{
        &GameEntity{ID: "orc1", Type: "enemy"},
        &GameEntity{ID: "goblin1", Type: "enemy"},
    }
    
    err := selectablesRegistry.RegisterTable("basic-enemies", enemies)
    if err != nil {
        log.Fatal(err)
    }
    
    // Configure spawning
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
    
    // Spawn entities
    result, err := engine.PopulateRoom(context.Background(), "my-room", config)
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Spawned %d entities successfully", len(result.SpawnedEntities))
}
```

### Advanced Configuration

```go
// Team-based spawning with formations
config := spawn.SpawnConfig{
    EntityGroups: []spawn.EntityGroup{
        {ID: "players", Type: "player", SelectionTable: "heroes", Quantity: spawn.QuantitySpec{Fixed: &four}},
        {ID: "enemies", Type: "enemy", SelectionTable: "monsters", Quantity: spawn.QuantitySpec{Fixed: &six}},
    },
    Pattern: spawn.PatternTeamBased,
    TeamConfiguration: &spawn.TeamConfig{
        Teams: []spawn.Team{
            {
                ID:          "heroes",
                EntityTypes: []string{"player"},
                Formation:   &spawn.FormationPattern{Name: "line", Positions: linePositions},
                Cohesion:    0.8,
            },
            {
                ID:          "monsters", 
                EntityTypes: []string{"enemy"},
                Formation:   &spawn.FormationPattern{Name: "wedge", Positions: wedgePositions},
                Cohesion:    0.6,
            },
        },
        SeparationRules: spawn.SeparationConstraints{
            MinTeamDistance: 15.0,
            TeamPlacement:   spawn.TeamPlacementOppositeSides,
        },
    },
    AdaptiveScaling: &spawn.ScalingConfig{
        Enabled:        true,
        ScalingFactor:  1.5,
        PreserveAspect: true,
        EmitEvents:     true,
    },
}
```

### Player Spawn Zones

```go
// Allow players to choose spawn positions
config := spawn.SpawnConfig{
    EntityGroups: []spawn.EntityGroup{
        {ID: "players", Type: "player", SelectionTable: "party", Quantity: spawn.QuantitySpec{Fixed: &four}},
    },
    Pattern: spawn.PatternPlayerChoice,
    PlayerSpawnZones: []spawn.SpawnZone{
        {
            ID:          "safe-zone",
            Area:        spatial.Rectangle{Position: spatial.Position{X: 0, Y: 0}, Dimensions: spatial.Dimensions{Width: 10, Height: 10}},
            EntityTypes: []string{"player"},
            MaxEntities: 4,
        },
    },
    PlayerChoices: []spawn.PlayerSpawnChoice{
        {PlayerID: "hero1", ZoneID: "safe-zone", Position: spatial.Position{X: 2, Y: 3}},
        // Other players auto-assigned to zone center
    },
}
```

## Configuration Reference

### SpawnConfig Structure

```go
type SpawnConfig struct {
    // What to spawn
    EntityGroups []EntityGroup `json:"entity_groups"`
    
    // How to spawn  
    Pattern          SpawnPattern        `json:"pattern"`
    TeamConfiguration *TeamConfig        `json:"team_config,omitempty"`
    
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

### EntityGroup Configuration

```go
type EntityGroup struct {
    ID             string       `json:"id"`               // Unique identifier
    Type           string       `json:"type"`             // "player", "enemy", "treasure", etc.
    SelectionTable string       `json:"selection_table"`  // Registered table ID
    Quantity       QuantitySpec `json:"quantity"`         // How many to spawn
}

type QuantitySpec struct {
    Fixed *int `json:"fixed,omitempty"`  // Exact count
    // Future: DiceRoll, MinMax for variable quantities
}
```

### Spawn Patterns

- `PatternScattered`: Random distribution
- `PatternFormation`: Structured arrangements  
- `PatternTeamBased`: Team separation with formations
- `PatternPlayerChoice`: Player-controlled positioning
- `PatternClustered`: Grouped placement with spacing

### Spatial Constraints

```go
type SpatialConstraints struct {
    MinDistance   map[string]float64 `json:"min_distance"`    // Between entity types
    LineOfSight   LineOfSightRules   `json:"line_of_sight"`   // Visibility requirements
    WallProximity float64            `json:"wall_proximity"`  // Distance from walls
}
```

## Integration Patterns

### With Spatial Module

The spawn engine integrates with the spatial module for:
- Room dimensions and grid systems
- Entity placement and collision detection
- Line of sight calculations
- Position validation

### With Events Module

The spawn engine publishes events for:
- Individual entity spawning: `spawn.entity.spawned`
- Room modifications: `spawn.room.scaled` 
- Split recommendations: `spawn.split.recommended`
- Operation completion: `spawn.operation.completed`

```go
// Subscribe to spawn events
eventBus.SubscribeFunc("spawn.entity.spawned", 0, func(ctx context.Context, event events.Event) error {
    data := event.Data().(spawn.EntitySpawnEventData)
    log.Printf("Entity %s spawned at %v in room %s", 
        data.Entity.GetID(), data.Position, data.RoomID)
    return nil
})
```

### With Environment Module

The spawn engine uses the environment module for:
- Capacity analysis and space calculations
- Room scaling recommendations
- Split room suggestions

## Implementation Status

### ✅ Completed (Phase 1 & 4)
- Basic SpawnEngine interface and implementation
- Scattered spawning pattern
- Selectables integration for entity selection
- Environment integration for capacity analysis and room scaling
- Event publishing for observability
- Split-aware architecture

### 🚧 In Progress (Phase 2)
- Formation system with predefined patterns
- Team-based spawning with proper separation
- Player spawn zones with position validation  
- Clustered spawning with group cohesion

### 📋 Planned (Phase 3)
- Spatial constraint validation (line of sight, min distances)
- Area of effect buffer zones
- Pathing requirement enforcement
- Advanced error recovery and constraint relaxation

## Error Handling

The spawn engine uses progressive error recovery:

1. **Primary**: Attempt placement with full constraints
2. **Fallback**: Relax non-critical constraints  
3. **Final**: Ensure every entity gets placed somewhere valid
4. **Transparency**: Report failures and constraint violations via events

```go
result, err := engine.PopulateRoom(ctx, roomID, config)
if err != nil {
    log.Fatal("Spawn operation failed:", err)
}

// Check for partial failures
if len(result.Failures) > 0 {
    for _, failure := range result.Failures {
        log.Printf("Failed to spawn %s: %s", failure.EntityType, failure.Reason)
    }
}

// Check for room modifications
for _, mod := range result.RoomModifications {
    log.Printf("Room %s %s: %v -> %v (%s)", 
        mod.RoomID, mod.Type, mod.OldValue, mod.NewValue, mod.Reason)
}
```

## Testing

Run the test suite:

```bash
go test ./...
```

The test suite covers:
- Configuration validation
- Entity selection from tables
- All spawn patterns  
- Split room scenarios
- Error conditions and edge cases

## Performance Considerations

- **Operation Timeout**: 30 seconds maximum for spawn operations
- **Iteration Limits**: 1000 placement attempts per entity maximum  
- **Quality Threshold**: Accepts 80% constraint satisfaction
- **Caching**: Constraint validation results cached within operations
- **Graceful Degradation**: Every entity gets placed rather than failing entirely

## Examples and Use Cases

See the `examples/` directory for:
- Basic dungeon room population
- Multi-room encounter setup
- Player spawn configuration
- Team-based PvP scenarios
- Procedural content generation

## Contributing

Follow the patterns established in [ADR-0013](../../docs/adr/0013-entity-spawn-engine.md) and the general [toolkit contribution guidelines](../../README.md).

## Related Modules

- [Spatial](../spatial/) - Room management and positioning
- [Selectables](../selectables/) - Weighted entity selection  
- [Environment](../environments/) - Space calculation and room analysis
- [Events](../../events/) - Event publishing and subscription