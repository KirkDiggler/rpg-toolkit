# Environment Generation Tool - Usage Guide

## Overview

The environment generation tool provides a complete system for creating tactical game environments (dungeons, buildings, outdoor areas) using procedural generation. It sits in the Tools layer of the RPG Toolkit architecture and acts as a client-friendly middleware over the spatial module.

## Core Philosophy

The environment tool follows the toolkit's philosophy of providing **infrastructure, not implementation**:

- **Simple wall generation**: Just "empty" or "random" patterns
- **Client controls parameters**: Density, destructible ratio, materials
- **Fully procedural**: No opinionated "maze" or "tactical cover" patterns
- **Maximum flexibility**: Games define their own meaning of tactical variety

## Quick Start

### 1. Basic Environment Generation

```go
// Create environment generator
generator := environments.NewGraphBasedGenerator(environments.GraphBasedGeneratorConfig{
    ID:          "dungeon-generator",
    Type:        "procedural",
    EventBus:    eventBus,
    ShapeLoader: environments.NewShapeLoader("assets/shapes"),
})

// Generate environment
config := environments.GenerationConfig{
    RoomCount:  8,
    LayoutType: environments.LayoutTypeOrganic,
    Seed:       12345,
}

environment, err := generator.Generate(config)
```

### 2. Simple Room Building

```go
// Create room with random walls
room := environments.NewRoomBuilder(config).
    WithSize(15, 12).
    WithWallPattern("random").
    WithWallDensity(0.4).               // 40% wall density
    WithDestructibleRatio(0.7).         // 70% destructible
    Build()
```

### 3. Convenience Functions

```go
// Dense cover room (high wall density 0.6-0.9)
denseCoverRoom := environments.DenseCoverRoom(20, 15)

// Sparse cover room (low wall density 0.1-0.4) 
sparseCoverRoom := environments.SparseCoverRoom(20, 15)

// Balanced cover room (medium wall density 0.4-0.7)
balancedRoom := environments.BalancedCoverRoom(20, 15)


```

## Wall System

### Simple Pattern Selection

- **"empty"**: No internal walls - open areas, social encounters
- **"random"**: Procedural wall placement with configurable parameters

### Client Control Parameters

```go
room := builder.
    WithWallPattern("random").
    WithWallDensity(0.5).           // 0.0 = no walls, 1.0 = maximum walls
    WithDestructibleRatio(0.8).     // 0.0 = all indestructible, 1.0 = all destructible
    WithMaterial("stone").          // Wall material (game-specific)
    WithRandomSeed(42).             // Reproducible generation
    Build()
```

### Wall Properties

```go
// Games can customize wall behavior
type WallProperties struct {
    // Destruction
    HP           int      // Health points
    Resistance   []string // Damage types resisted
    Weakness     []string // Damage types with extra effect
    
    // Physical
    Material     string   // Visual/audio material
    Thickness    float64  // Wall thickness
    Height       float64  // Wall height
    
    // Gameplay
    BlocksLoS      bool   // Line of sight blocking
    BlocksMovement bool   // Movement blocking
    ProvidesCover  bool   // Combat cover bonus
}
```

## Architecture Flow

### 1. Generation Process

```
Client Request → Environment Generator → Room Builder → Spatial Integration
      ↓                    ↓                  ↓              ↓
   Parameters        Abstract Graph      Room + Walls    Spatial Entities
```

### 2. Wall Integration

```go
// Walls become spatial entities automatically
type WallEntity struct {
    // Implements spatial.Placeable interface
    position   spatial.Position
    properties WallProperties
}

// Automatic spatial integration
func (w *WallEntity) BlocksMovement() bool {
    return !w.destroyed && w.properties.BlocksMovement
}
```

### 3. Event Flow

```go
// Environment publishes events
"environment.generated"     // Generation complete
"wall.pattern.applied"      // Walls added to room

// Environment consumes events
"spatial.entity.placed"     // Apply environmental effects
"turn.started"              // Process environmental changes
```

## Client Usage Patterns

### 1. Density-Based Variety

```go
// Sparse walls for open combat
sparseRoom := builder.WithWallDensity(0.2).Build()

// Dense walls for tactical maneuvering
denseRoom := builder.WithWallDensity(0.8).Build()

// No walls for social encounters
openRoom := builder.WithWallPattern("empty").Build()
```

### 2. Destructible Wall Tactics

```go
// Mostly destructible - players can reshape battlefield
flexibleRoom := builder.WithDestructibleRatio(0.9).Build()

// Mostly indestructible - fixed tactical positions
rigidRoom := builder.WithDestructibleRatio(0.1).Build()

// Mixed - some permanent structure, some flexibility
balancedRoom := builder.WithDestructibleRatio(0.6).Build()
```

### 3. Material Customization

```go
// Stone walls - high HP, fire resistant
stoneRoom := builder.WithMaterial("stone").Build()

// Wood walls - lower HP, fire weakness
woodRoom := builder.WithMaterial("wood").Build()

// Metal walls - electrical weakness
metalRoom := builder.WithMaterial("metal").Build()
```

## Wall Destruction Mechanics

### 1. Applying Damage

```go
// Get wall entity at position
entities := room.GetEntitiesAt(position)
for _, entity := range entities {
    if wall, ok := entity.(*environments.WallEntity); ok {
        // Apply damage
        destroyed := wall.TakeDamage(25, "fire")
        
        if destroyed {
            // New tactical options available
            fmt.Println("Wall destroyed! New path available.")
        }
    }
}
```

### 2. Automatic Spatial Updates

```go
// Destroyed walls automatically stop blocking
canMove := room.CanPlaceEntity(player, wallPosition)     // Now true
canSee := !room.IsLineOfSightBlocked(from, to)          // Now true
```

### 3. Repair and Reconstruction

```go
// Repair damaged walls
wall.Repair(10)

// Destroy walls immediately (scripted events)
wall.Destroy()
```

## Query System

### 1. Environment-Level Queries

```go
// Find all entities in environment
entities := environment.QueryEntitiesInRange(
    context.Background(),
    centerPos, 
    10.0,     // radius
    "",       // all rooms
    nil,      // no filter
)

// Find entities in specific room
roomEntities := environment.QueryEntitiesInRange(
    context.Background(),
    centerPos,
    10.0,
    "room-1", // specific room
    nil,
)
```

### 2. Wall-Specific Queries

```go
// Get all walls in room
walls := environments.GetWallEntitiesInRoom(room)

// Check wall segment health
current, max, destroyed := environments.GetWallSegmentHealth(room, "wall-segment-1")

// Find walls by segment
segmentWalls := environments.FindWallEntitiesBySegment(allEntities, "wall-segment-1")
```

## Extension Points

### 1. Custom Wall Patterns

```go
// Register custom pattern algorithm
environments.RegisterWallPattern("custom", func(shape RoomShape, size Dimensions, params PatternParams) []WallSegment {
    // Custom wall generation logic
    return generateCustomWalls(shape, size, params)
})

// Use custom pattern
room := builder.WithWallPattern("custom").Build()
```

### 2. Custom Materials

```go
// Define game-specific materials
environments.RegisterMaterial("magical_ice", WallProperties{
    HP:         15,
    Resistance: []string{"cold", "water"},
    Weakness:   []string{"fire"},
    Material:   "ice",
    BlocksLoS:  true,
    BlocksMovement: true,
})
```

### 3. Environmental Effects

```go
// Subscribe to wall destruction events
eventBus.Subscribe("wall.destroyed", func(event events.GameEvent) {
    // Custom game logic when walls are destroyed
    handleWallDestruction(event.Source.(*environments.WallEntity))
})
```

## Best Practices

### 1. Choosing Wall Parameters

```go
// Open areas - social encounters, large battles
builder.WithWallPattern("empty")

// Light tactical - some cover, open movement
builder.WithWallPattern("random").WithWallDensity(0.3)

// Heavy tactical - lots of cover, complex movement
builder.WithWallPattern("random").WithWallDensity(0.7)
```

### 2. Destructible Ratios by Room Purpose

```go
// Structural rooms - permanent architecture
builder.WithDestructibleRatio(0.2)

// General rooms - some tactical flexibility
builder.WithDestructibleRatio(0.6)

// Interactive rooms - heavy environmental interaction
builder.WithDestructibleRatio(0.9)
```

### 3. Performance Optimization

```go
// Use specific room IDs for better performance
entities := environment.QueryEntitiesInRange(ctx, pos, radius, "room-1", filter)

// Batch wall operations
for _, wallID := range wallsToDestroy {
    environments.DestroyWallSegment(room, wallID)
}
```

## Example Game Integration

```go
// Game-specific room factory
func CreateRoomLayout(layoutType string) spatial.Room {
    var density, destructibleRatio float64
    
    switch layoutType {
    case "open":
        density = 0.0
        destructibleRatio = 0.0
    case "sparse":
        density = 0.3
        destructibleRatio = 0.8
    case "dense":
        density = 0.7
        destructibleRatio = 0.4
    default:
        density = 0.5
        destructibleRatio = 0.6
    }
    
    return environments.NewRoomBuilder(config).
        WithSize(20, 15).
        WithWallPattern("random").
        WithWallDensity(density).
        WithDestructibleRatio(destructibleRatio).
        WithMaterial("stone").
        Build()
}
```

This simplified approach provides maximum flexibility while maintaining the toolkit's philosophy of infrastructure over implementation. Games can create endless variety by adjusting simple parameters rather than choosing from predefined patterns.
</parameter>
</invoke>