# RPG Toolkit Spawn Engine

The spawn engine provides comprehensive entity placement capabilities for RPG games, supporting everything from simple enemy spawning to complex multi-room scenarios with team coordination and player choice mechanics.

## Features

- **Split-Aware Architecture**: Works seamlessly with single rooms or multi-room configurations
- **Team-Based Spawning**: Keep allies together, enemies together, with configurable separation
- **Player Spawn Zones**: Designated areas where players can choose their starting positions
- **Environment Integration**: Automatic room scaling and capacity analysis
- **Event-Driven**: Full observability through comprehensive event publishing
- **Selectables Integration**: Dynamic entity selection from weighted tables

## Quick Start

```go
package main

import (
    "context"
    "github.com/KirkDiggler/rpg-toolkit/tools/spawn"
    "github.com/KirkDiggler/rpg-toolkit/core"
)

func main() {
    // Create spawn engine
    engine := spawn.NewBasicSpawnEngine(spawn.BasicSpawnEngineConfig{
        ID: "my-spawn-engine",
        Configuration: spawn.SpawnEngineConfiguration{
            EnableEvents: true,
            PerformanceMode: "balanced",
        },
    })

    // Basic entity spawning
    entities := []core.Entity{
        &MyEntity{id: "orc1", entityType: "enemy"},
        &MyEntity{id: "orc2", entityType: "enemy"},
    }

    config := spawn.SpawnConfig{
        EntityGroups: []spawn.EntityGroup{
            {
                ID: "enemies",
                Type: "enemy", 
                Entities: entities,
                Quantity: spawn.QuantitySpec{Fixed: &[]int{2}[0]},
            },
        },
        Pattern: spawn.PatternScattered,
        Strategy: spawn.StrategyRandomized,
    }

    result, err := engine.PopulateRoom(context.Background(), "dungeon_room_1", config)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Spawned %d entities successfully\n", len(result.SpawnedEntities))
}
```

## Core Interface: SpawnEngine

### Primary Methods

#### `PopulateSpace(ctx context.Context, roomOrGroup interface{}, config SpawnConfig) (SpawnResult, error)`

Universal spawning method that works with any room configuration:

```go
// Single room
result, err := engine.PopulateSpace(ctx, "room_id", config)

// Multiple connected rooms (split configuration)
connectedRooms := []string{"room_1", "room_2", "room_3"}
result, err := engine.PopulateSpace(ctx, connectedRooms, config)
```

#### `PopulateRoom(ctx context.Context, roomID string, config SpawnConfig) (SpawnResult, error)`

Convenience method for single-room spawning:

```go
result, err := engine.PopulateRoom(ctx, "tavern", config)
```

#### `PopulateSplitRooms(ctx context.Context, connectedRooms []string, config SpawnConfig) (SpawnResult, error)`

Explicit multi-room spawning for connected areas:

```go
dungeonRooms := []string{"entrance", "main_hall", "treasure_room"}
result, err := engine.PopulateSplitRooms(ctx, dungeonRooms, config)
```

## Configuration

### SpawnConfig Structure

```go
type SpawnConfig struct {
    // What to spawn
    EntityGroups []EntityGroup `json:"entity_groups"`
    
    // How to spawn
    Pattern          SpawnPattern  `json:"pattern"`
    TeamConfiguration *TeamConfig  `json:"team_config,omitempty"`
    
    // Spatial constraints
    SpatialRules SpatialConstraints `json:"spatial_rules"`
    Placement    PlacementRules     `json:"placement"`
    
    // Behavior
    Strategy        SpawnStrategy `json:"strategy"`
    AdaptiveScaling *ScalingConfig `json:"adaptive_scaling,omitempty"`
    
    // Player interaction
    PlayerSpawnZones []SpawnZone         `json:"player_spawn_zones,omitempty"`
    PlayerChoices    []PlayerSpawnChoice `json:"player_choices,omitempty"`
}
```

### Entity Groups

Define what entities to spawn and how many:

```go
EntityGroup{
    ID: "boss_encounter",
    Type: "enemy",
    SelectionTable: "boss_table", // Use selectables table
    Quantity: spawn.QuantitySpec{
        DiceRoll: &[]string{"1d4+1"}[0], // Spawn 2-5 entities
    },
    Priority: 1,
    TeamID: "enemies",
}

// Or use pre-provided entities
EntityGroup{
    ID: "party_members", 
    Type: "player",
    Entities: playerEntities,
    Quantity: spawn.QuantitySpec{Fixed: &[]int{4}[0]},
    TeamID: "heroes",
}
```

### Spawn Patterns

- **`PatternScattered`**: Random distribution across available space
- **`PatternClustered`**: Groups entities with spacing
- **`PatternFormation`**: Structured arrangements (line, circle, etc.)
- **`PatternTeamBased`**: Teams in separate areas with cohesion rules
- **`PatternPlayerChoice`**: Players choose positions within designated zones

## Advanced Features

### Team-Based Spawning

Keep allies together and enemies separated:

```go
config.Pattern = spawn.PatternTeamBased
config.TeamConfiguration = &spawn.TeamConfig{
    Teams: []spawn.Team{
        {
            ID: "heroes",
            EntityTypes: []string{"player", "ally"},
            Cohesion: 0.8, // Stay close together
        },
        {
            ID: "monsters", 
            EntityTypes: []string{"enemy", "boss"},
            Cohesion: 0.6,
        },
    },
    CohesionRules: spawn.TeamCohesionRules{
        KeepFriendliesTogether: true,
        KeepEnemiesTogether: true,
        MinTeamSeparation: 10.0, // Minimum distance between teams
    },
    SeparationRules: spawn.SeparationConstraints{
        MinTeamDistance: 15.0,
        TeamPlacement: spawn.TeamPlacementOppositeSides,
    },
}
```

### Player Spawn Zones

Allow players to choose their starting positions:

```go
config.Pattern = spawn.PatternPlayerChoice
config.PlayerSpawnZones = []spawn.SpawnZone{
    {
        ID: "north_entrance",
        Area: spatial.Rectangle{
            Position: spatial.Position{X: 0, Y: 0},
            Dimensions: spatial.Dimensions{Width: 10, Height: 5},
        },
        EntityTypes: []string{"player"},
        MaxEntities: 2,
    },
    {
        ID: "south_entrance", 
        Area: spatial.Rectangle{
            Position: spatial.Position{X: 0, Y: 15},
            Dimensions: spatial.Dimensions{Width: 10, Height: 5},
        },
        EntityTypes: []string{"player"},
        MaxEntities: 2,
    },
}

config.PlayerChoices = []spawn.PlayerSpawnChoice{
    {
        PlayerID: "player1",
        ZoneID: "north_entrance",
        Position: spatial.Position{X: 2, Y: 2},
    },
}
```

### Adaptive Room Scaling

Automatically scale rooms when entities don't fit:

```go
config.AdaptiveScaling = &spawn.ScalingConfig{
    Enabled: true,
    ScalingFactor: 1.5, // Increase room size by 50%
    PreserveAspect: true,
    EmitEvents: true, // Publish scaling events
}
```

### Helper Configurations

For common scenarios, use helper configs:

```go
// Quick setup for combat encounters
result, err := engine.PopulateSpaceWithHelper(
    ctx, 
    "battlefield", 
    allEntities,
    spawn.HelperConfig{
        Purpose: "combat",
        Difficulty: 3,
        TeamSeparation: true,
        AutoScale: true,
    },
)
```

## Working with Results

### SpawnResult Structure

```go
type SpawnResult struct {
    Success              bool                `json:"success"`
    SpawnedEntities      []SpawnedEntity     `json:"spawned_entities"`
    Failures             []SpawnFailure      `json:"failures"`
    RoomModifications    []RoomModification  `json:"room_modifications"`
    SplitRecommendations []RoomSplit         `json:"split_recommendations"`
    RoomStructure        RoomStructureInfo   `json:"room_structure"`
    Metadata             SpawnMetadata       `json:"metadata"`
}
```

### Processing Results

```go
result, err := engine.PopulateRoom(ctx, roomID, config)
if err != nil {
    return err
}

// Check what was spawned
for _, spawned := range result.SpawnedEntities {
    fmt.Printf("Entity %s spawned at (%.1f, %.1f) in room %s\n",
        spawned.Entity.GetID(),
        spawned.Position.X, 
        spawned.Position.Y,
        spawned.RoomID,
    )
}

// Handle any failures
for _, failure := range result.Failures {
    fmt.Printf("Failed to spawn %s: %s\n", 
        failure.EntityType, 
        failure.Reason,
    )
}

// Check if room was modified
if len(result.RoomModifications) > 0 {
    fmt.Printf("Room was scaled: %s\n", 
        result.RoomModifications[0].Reason,
    )
}

// Handle split recommendations
if len(result.SplitRecommendations) > 0 {
    fmt.Printf("Consider splitting room: %s\n",
        result.SplitRecommendations[0].Reason,
    )
}
```

## Integration Points

### Environment Package Integration

The spawn engine integrates with the environment package for:
- **Capacity Analysis**: Determine if entities fit in available space
- **Room Scaling**: Calculate optimal room dimensions
- **Split Recommendations**: Suggest when rooms should be divided

### Selectables Integration

Use weighted tables for dynamic entity selection:

```go
// Register selection tables
registry := spawn.NewBasicSelectablesRegistry()
registry.RegisterTable("monsters", monsterTable)

// Reference in spawn config
EntityGroup{
    ID: "random_monsters",
    SelectionTable: "monsters", 
    Quantity: spawn.QuantitySpec{
        DiceRoll: &[]string{"2d6"}[0],
    },
}
```

### Event System

Subscribe to spawn events for game integration:

```go
eventBus.SubscribeFunc("spawn.entity.spawned", 1, func(ctx context.Context, event events.Event) error {
    entityData := event.Context().Get("entity_data")
    // Handle entity spawned
    return nil
})

eventBus.SubscribeFunc("spawn.split.recommended", 1, func(ctx context.Context, event events.Event) error {
    splitData := event.Context().Get("split_data")
    // Handle split recommendation
    return nil
})
```

## Performance Configuration

### Engine Configuration

```go
config := spawn.SpawnEngineConfiguration{
    EnableEvents: true,
    EnableDebugging: false,
    MaxPlacementAttempts: 1000,
    DefaultTimeoutSeconds: 30,
    PerformanceMode: "balanced", // "fast", "thorough", "balanced"
    QualityThreshold: 0.8, // Accept 80% constraint satisfaction
}
```

### Performance Modes

- **`fast`**: Prioritize speed over optimal placement
- **`balanced`**: Good balance of speed and quality (default)
- **`thorough`**: Prioritize optimal placement over speed

## Error Handling

The spawn engine provides detailed error information:

```go
result, err := engine.PopulateRoom(ctx, roomID, config)
if err != nil {
    // Handle configuration errors, capacity issues, etc.
    return fmt.Errorf("spawning failed: %w", err)
}

// Check for partial failures
if !result.Success {
    fmt.Printf("Spawning completed with %d failures\n", len(result.Failures))
}
```

## Best Practices

1. **Use appropriate patterns**: `PatternTeamBased` for tactical scenarios, `PatternPlayerChoice` for player agency
2. **Configure timeouts**: Set reasonable `DefaultTimeoutSeconds` for your use case
3. **Handle split recommendations**: When rooms are too crowded, consider the provided split options
4. **Monitor events**: Subscribe to spawn events for debugging and game integration
5. **Validate configurations**: Use `ValidateSpawnConfig()` to catch issues early
6. **Consider performance**: Choose appropriate `PerformanceMode` for your needs

## Examples

See the test files (`*_test.go`) for comprehensive examples of all features and integration patterns.