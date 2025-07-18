# ADR-0011: Environment Generation Algorithms

**Date**: 2025-01-17  
**Status**: Accepted  
**Context**: Environment generator design for tools/environments module  

## Context

We are designing the next layer above the spatial module - an environment generator that creates complete game environments (dungeons, towns, wilderness areas) using room prefabs and connection systems. We need to choose the core generation algorithm(s) that will drive how environments are created.

## Problem Statement

Environment generation requires balancing multiple concerns:
- **Performance**: Generation speed and runtime queries
- **Flexibility**: Ability to modify and customize generated environments
- **Quality**: Producing well-designed, playable environments
- **Complexity**: Implementation and maintenance burden
- **Integration**: Working well with existing spatial orchestration system

## Options Considered

### Option A: Graph-Based Generation Only

**Approach**: Generate environments as abstract graphs of rooms and connections, then place spatially.

**Process**:
1. Create abstract graph of rooms and connections
2. Assign room prefabs to nodes based on requirements
3. Place rooms in spatial coordinates using graph relationships
4. Generate connections using existing spatial orchestrator

**Advantages**:
- **High flexibility**: Easy to modify room relationships
- **Better performance**: O(n) generation, optimal for pathfinding
- **Natural integration**: Works directly with spatial orchestrator
- **Simpler implementation**: Leverages existing connection system
- **Design-friendly**: Supports both procedural and custom/narrative-driven generation

**Disadvantages**:
- **Spatial efficiency**: May create awkward layouts or wasted space
- **Spatial queries**: O(n) performance for "what's near position X" queries
- **Overlap handling**: Requires additional logic to prevent room overlap

### Option B: Spatial Algorithms (BSP Trees) Only

**Approach**: Recursively divide space into regions, then assign room types to regions.

**Process**:
1. Start with large rectangle representing entire environment
2. Recursively split into smaller rectangles using BSP tree
3. Assign room prefabs to leaf rectangles
4. Create connections between adjacent rooms

**Advantages**:
- **Space efficiency**: Guaranteed efficient use of available space
- **Fast spatial queries**: O(log n) for position-based lookups
- **No overlap issues**: Spatial division prevents room overlap
- **Scalable**: Performs well for very large environments

**Disadvantages**:
- **Lower flexibility**: Hard to modify after generation
- **Complex implementation**: Requires spatial tree structures
- **Poor pathfinding**: Need to rebuild connectivity information
- **Less design control**: Harder to create specific narrative flows

### Option C: Hybrid Approach

**Approach**: Implement both algorithms and provide guidelines for when to use each.

**Implementation Strategy**:
```go
type GenerationAlgorithm int

const (
    GraphBased GenerationAlgorithm = iota
    SpatialBSP
    Hybrid  // Graph structure with spatial constraints
)

type EnvironmentGenerator interface {
    SetAlgorithm(algorithm GenerationAlgorithm)
    Generate(config GenerationConfig) Environment
}
```

**Usage Guidelines**:
- **Graph-based**: Small-medium environments (< 100 rooms), narrative-driven, frequent modifications
- **BSP**: Large environments (> 100 rooms), procedural-only, static after generation
- **Hybrid**: Medium environments needing both design control and space efficiency

## Decision

We will implement **Option A: Graph-Based Generation as Primary Solution** with the following strategy:

### Phase 1: Graph-Based Implementation (Immediate)
- Implement graph-based generation as the primary and initial algorithm
- Provide simple spatial placement with overlap detection
- Focus on integration with spatial orchestrator
- Support both procedural and custom generation
- Design API to be extensible for future algorithms

### Future Consideration: BSP Implementation (If Needed)
- **Add BSP tree-based generation only if performance requirements demand it**
- **Trigger conditions**: Large environments (> 100 rooms), frequent spatial queries, memory constraints
- **Implementation approach**: Maintain same public API for seamless switching
- **Tech debt consideration**: Avoid premature optimization - implement when there's clear need

### Rationale for Graph-First Approach
- **Meets current needs**: Graph-based handles typical RPG environment sizes efficiently
- **Lower complexity**: Reduces implementation and maintenance burden
- **Faster delivery**: Can deliver value sooner with focused implementation
- **Flexibility priority**: Graph-based provides the design flexibility we prioritize
- **Integration advantage**: Works naturally with existing spatial orchestrator

### Public API Design
```go
type GenerationConfig struct {
    Size         EnvironmentSize     // Small, Medium, Large
    Type         EnvironmentType     // Procedural, Custom, Endless
    Constraints  SpatialConstraints  // Bounds, density, etc.
    Prefabs      []PrefabType       // Available room prefabs
}

type EnvironmentGenerator interface {
    Generate(config GenerationConfig) Environment
}

// Initial implementation focuses on graph-based generation
// API designed to be extensible for future algorithms if needed
type GraphBasedGenerator struct {
    orchestrator spatial.RoomOrchestrator
    prefabs      map[PrefabType]RoomPrefab
}
```

## Implementation Complexity Assessment

### Graph-Based Implementation: **Medium Complexity**
- **Estimated effort**: 2-3 weeks for initial implementation
- **Key components**: Graph builder, spatial placement, overlap detection
- **Integration**: Direct use of spatial orchestrator
- **Risk**: Medium - spatial placement logic needs optimization

### Future BSP Implementation: **High Complexity (If Needed)**
- **Estimated effort**: 3-4 weeks when/if implemented
- **Key components**: BSP tree, space division, connection generation
- **Integration**: Requires adaptation layer for spatial orchestrator
- **Risk**: High - complex spatial algorithms and edge cases
- **Decision point**: Only implement if performance profiling shows clear need

### Current Implementation Plan: **2-3 weeks**
- Focus on graph-based generation only
- Design API to be extensible for future algorithms
- Avoid premature optimization and complexity

## Consequences

### Positive
- **Focused implementation**: Lower complexity and faster delivery
- **Flexibility**: Graph-based provides the design flexibility we prioritize
- **Performance**: Meets current RPG environment size requirements efficiently
- **Integration**: Works naturally with existing spatial orchestrator
- **Extensible API**: Designed to accommodate future algorithms if needed
- **Reduced tech debt**: Avoids premature optimization

### Negative
- **Potential future work**: May need BSP implementation for very large environments
- **Spatial efficiency**: May not be optimal for space-constrained scenarios
- **Performance ceiling**: Could hit limits with extremely large environments

### Neutral
- **Decision deferral**: BSP implementation can be added based on actual need
- **Learning opportunity**: Can gather real-world performance data before optimization
- **Backwards compatibility**: Future algorithms can be added without breaking changes

## Alternatives Considered

### Single Algorithm Approach
**Rejected** because:
- Limits flexibility for different use cases
- May not be optimal for all environment sizes
- Harder to evolve and improve over time

### Plugin Architecture
**Deferred** because:
- Adds significant complexity without clear benefit
- Can be added later if third-party algorithms are needed
- Current two algorithms cover most use cases

## Success Criteria

### Initial Implementation (Graph-Based)
- [ ] Graph-based algorithm implemented with spatial orchestrator integration
- [ ] Support for procedural, custom, and endless generation types
- [ ] Room prefab system with T, L, I, and other basic shapes
- [ ] Spatial placement with overlap detection and resolution
- [ ] Comprehensive test suite for graph-based generation
- [ ] Clear documentation and usage examples
- [ ] Performance benchmarks for typical RPG environment sizes

### Future Considerations (BSP - If Needed)
- [ ] Performance profiling data showing need for BSP implementation
- [ ] BSP algorithm implemented with efficient spatial queries
- [ ] Unified API allowing seamless algorithm switching
- [ ] Performance benchmarks demonstrating trade-offs between algorithms

---

## Architecture Design

### Component-Based Assembly System

The environment module implements a **hybrid component-based architecture** that supports both prefab loading and programmatic generation:

```go
// Core: Component-based assembly (infrastructure)
room := environments.NewRoomBuilder().
    WithLayout(layouts.Rectangular(15, 12)).
    WithWalls(walls.Stone()).
    WithFeatures(features.TreasureChest(), features.Pillars(2)).
    WithTheme(themes.AncientRuins()).
    Build()

// Convenience: Load from prefab (implementation helper)
room := environments.LoadRoomPrefab("treasure_room.yaml")

// Advanced: Combine both approaches
room := environments.LoadRoomPrefab("base_room.yaml").
    WithFeatures(features.CustomTrap()).
    WithTheme(themes.Flooded()).
    Build()
```

**Benefits:**
- **Follows toolkit patterns**: Config-based construction, composition over inheritance
- **Maximum flexibility**: Can build any room programmatically
- **Designer-friendly**: Prefabs for common cases
- **Extensible**: Easy to add new components
- **Grid-agnostic**: Components adapt to different spatial systems

### Event Bus Integration

The environment module integrates with the toolkit's event-driven architecture:

**Events Environment Publishes:**
```go
// Environment generation events
EventEnvironmentGenerated        = "environment.generated"
EventEnvironmentDestroyed        = "environment.destroyed"
EventRoomTemplateApplied        = "environment.room_template.applied"
EventConnectionTemplateApplied  = "environment.connection_template.applied"

// Environment state changes
EventThemeChanged               = "environment.theme.changed"
EventLayoutChanged              = "environment.layout.changed"
EventFeatureAdded               = "environment.feature.added"
EventFeatureRemoved             = "environment.feature.removed"

// Environmental conditions
EventEnvironmentEffectApplied   = "environment.effect.applied"
EventEnvironmentEffectRemoved   = "environment.effect.removed"
EventHazardActivated           = "environment.hazard.activated"
EventHazardDeactivated         = "environment.hazard.deactivated"
```

**Events Environment Subscribes To:**
```go
// From spatial module
"spatial.entity.placed"           // Apply environment effects to new entities
"spatial.entity.moved"            // Check for environment transitions
"spatial.room.created"            // Set up default environment conditions
"spatial.orchestrator.room_added" // Configure cross-room environment effects

// From time/turn system (if exists)
"turn.started"                    // Process time-based environment changes
"rest.long"                       // Handle environment effects on rest
"rest.short"                      // Handle short rest environment effects
```

### Query Delegation Architecture

The environment module acts as a **query aggregator** that delegates to spatial infrastructure:

```go
// Environment-level queries that delegate to spatial
func (h *EnvironmentQueryHandler) QueryEntitiesInEnvironment(
    ctx context.Context,
    center spatial.Position,
    radius float64,
    roomID string,
    filter spatial.EntityFilter,
) ([]core.Entity, error) {
    // Single room - delegate directly
    if roomID != "" {
        return h.spatialQuery.QueryEntitiesInRange(ctx, center, radius, roomID, filter)
    }
    
    // Multi-room - aggregate results
    var allEntities []core.Entity
    for _, room := range h.orchestrator.GetRooms() {
        entities, err := h.spatialQuery.QueryEntitiesInRange(ctx, center, radius, room.GetID(), filter)
        if err != nil {
            return nil, err
        }
        allEntities = append(allEntities, entities...)
    }
    return allEntities, nil
}
```

**Benefits:**
- **No code duplication**: Leverages existing spatial query implementation
- **Consistent behavior**: Same query logic across both APIs
- **Extensibility**: Environment builder can add cross-room capabilities
- **Future-proof**: Spatial improvements automatically benefit environment builder

### Responsibility Boundaries

**Environment Package Responsibilities:**
- **Generation algorithms**: Dungeon layouts, room placement patterns
- **Room templates/prefabs**: Treasure rooms, boss rooms, corridors
- **Thematic coherence**: Ancient ruins vs modern facility
- **Cross-room queries**: Line of sight between rooms, pathfinding
- **Logical grouping**: Find nearest exit, spawn in appropriate room
- **Environmental rules**: Trap placement, lighting, atmosphere

**Spatial Package Responsibilities:**
- **Individual room positioning**: Entity placement, movement within rooms
- **Grid mathematics**: Distance calculations, line of sight within rooms
- **Room orchestration**: Connections, entity transitions
- **Low-level queries**: Entities in range, valid positions

**Combat/Spawning Package Responsibilities (Separate):**
- **Enemy placement**: Encounter design, spawn point selection
- **Loot distribution**: Reward placement, treasure positioning
- **Hazard placement**: Trap positioning, environmental dangers

### Client Extension Patterns

The environment module provides hooks for games to customize generation:

```go
// Client app provides the "knobs"
dungeonSpec := DungeonSpec{
    Theme:        app.GetCurrentTheme(),        // Client's theme
    Size:         app.GetDesiredSize(),         // Client's size preference
    Density:      app.GetWallDensity(),         // Client's wall density
    Materials:    app.GetAvailableMaterials(),  // Client's materials
    Rules:        app.GetGameRules(),           // Client's rule system
}

// Environment system provides the generation framework
dungeon := environments.NewDungeonGenerator().
    WithSpec(dungeonSpec).
    WithRoomCount(10).
    WithRoomTypes([]string{"corridor", "chamber", "trap_room"}).
    WithBossRoom(true).
    WithSeed(app.GetRandomSeed()).
    Generate()

// Client app registers custom components
environments.RegisterRoomComponent("trap_room", func(spec DungeonSpec) RoomBuilder {
    return environments.NewRoomBuilder().
        WithSize(spec.Rules.GetRoomSize("trap")).
        WithTheme(spec.Theme).
        WithFeatures(spec.Rules.GetTrapsForLevel(spec.PlayerLevel))
})
```

**Benefits:**
- **Maximum flexibility**: Games can customize every aspect
- **Maintains agnosticism**: Environment doesn't know about combat, difficulty, or game mechanics
- **Extensible**: Easy to add new room types and generation rules
- **Composable**: Mix static and procedural generation

## Wall Pattern System Design

### Problem: Room Variety vs. Prefab Efficiency

Initial analysis suggested storing complete room prefabs for variety, but this would require:
- 6 base shapes × 10 wall patterns = 60 prefab files
- Multiple size variants for each combination
- Maintenance complexity for similar room layouts

### Solution: Procedural Wall Pattern Generation

**Core Insight**: Instead of storing complex room geometries, use simple base shapes with algorithmic wall pattern generation.

#### Base Shape Strategy
Store only **6 essential shape boundaries**:
- Rectangle (most common, versatile)
- Square (symmetric, balanced)
- L-Shape (natural corners/turns)
- T-Shape (three-way junctions)
- Cross/Plus (four-way intersections)
- Oval/Circle (organic flow)

#### Wall Pattern Generation
```go
// Simplified patterns - infrastructure not implementation
type WallPatternFunc func(shape RoomShape, size Dimensions, params PatternParams) []WallSegment

var WallPatterns = map[string]WallPatternFunc{
    "empty":  EmptyPattern,  // No internal walls
    "random": RandomPattern, // Procedural wall placement
}
```

#### Destructible Wall System
**Innovation**: Walls have destruction properties for tactical gameplay.

```go
type WallType int
const (
    WallTypeIndestructible WallType = iota  // Permanent structural walls
    WallTypeDestructible                    // Can be destroyed by players
    WallTypeTemporary                       // Can be bypassed with effort
    WallTypeConditional                     // Requires specific tools/abilities
)

type WallProperties struct {
    HP           int      `json:"hp,omitempty"`           // Health points
    Resistance   []string `json:"resistance,omitempty"`   // Damage types resisted
    Weakness     []string `json:"weakness,omitempty"`     // Extra damage types
    RequiredTool string   `json:"required_tool,omitempty"` // Specific tool needed
    Material     string   `json:"material"`               // Stone, wood, metal
    BlocksLoS    bool     `json:"blocks_los"`             // Line of sight blocking
    BlocksMovement bool   `json:"blocks_movement"`        // Movement blocking
    ProvidesCover bool    `json:"provides_cover"`         // Combat cover bonus
}
```

#### Tactical Gameplay Benefits
**Decision-Making Geometry**: Each room becomes a tactical puzzle.
- **Destructible shortcuts**: Players can blast through walls for new routes
- **Indestructible structure**: Forces strategic positioning and movement
- **Cover systems**: Walls provide tactical positioning options
- **Interactive environments**: Supports games with destruction mechanics

**Example Room Types**:
- **Open Combat**: Empty pattern, no walls (density = 0.0)
- **Light Tactical**: Random pattern, low density (density = 0.3), high destructible ratio (0.8)
- **Heavy Tactical**: Random pattern, high density (density = 0.7), mixed destructible ratio (0.6)
- **Siege Room**: Random pattern, medium density (0.5), low destructible ratio (0.2)

#### Storage Efficiency
**Actual Storage Required**:
- 6 shape boundary files (~1KB each)
- 0 pattern files (algorithms in code)
- **Total: ~6KB** vs. 60+ prefab files

**Runtime Generation**:
```go
room := roomBuilder.
    WithShape(LoadRoomShape("rectangle.yaml")).
    WithWallPattern("random").
    WithWallDensity(0.5).
    WithDestructibleRatio(0.7).
    WithSize(20, 15).
    Build()
```

#### Benefits
1. **Minimal Storage**: 6 files vs. 60+ combinations
2. **Infinite Variety**: Any shape + any pattern + any size
3. **Tactical Depth**: Every room becomes interactive
4. **Game Agnostic**: Games without destruction ignore destructible flags
5. **Procedural**: Different results with random seeds
6. **Extensible**: New patterns are just new functions

This design transforms simple room shapes into complex tactical environments while maintaining storage efficiency and maximum flexibility for game-specific customization.

#### Wall-Spatial Integration: Leverage Existing Obstacle System

**Design Decision**: Represent walls as `Placeable` entities within the spatial module rather than extending spatial interfaces.

**Problem**: Generated walls need to integrate with spatial queries (line of sight, movement, pathfinding) without duplicating obstacle logic.

**Solution**: Convert `WallSegment` objects into `WallEntity` objects that implement the spatial `Placeable` interface.

```go
type WallEntity struct {
    id         string
    segmentID  string
    wallType   WallType
    properties WallProperties
    position   spatial.Position
}

func (w *WallEntity) BlocksMovement() bool { return w.properties.BlocksMovement }
func (w *WallEntity) BlocksLineOfSight() bool { return w.properties.BlocksLoS }

// Convert wall segments to entities and place in spatial room
func CreateWallEntities(walls []WallSegment) []spatial.Placeable {
    // Discretize wall segments into positioned entities
    // Each wall segment becomes multiple single-position entities
}
```

**Benefits**:
1. **Zero Spatial Module Changes**: Uses existing, well-tested obstacle system
2. **Immediate Integration**: Line of sight and movement automatically consider walls
3. **Destructible Walls**: Remove wall entity when destroyed (existing spatial events)
4. **Consistent API**: Walls work exactly like other blocking entities
5. **DRY Principle**: Avoids duplicating collision detection and line-of-sight logic
6. **Event Integration**: Wall placement/removal triggers existing spatial events

**Implementation**: Environment generator converts generated wall segments into wall entities during spatial room creation, placing each wall entity at appropriate positions within the room.

This approach leverages the spatial module's existing obstacle infrastructure while maintaining clean separation between environment generation logic and spatial positioning logic.

## Implementation Status

### Completed Infrastructure
- ✅ **Spatial orchestrator** (multi-room management, connections)
- ✅ **Event bus system** (event-driven architecture)
- ✅ **Core interfaces** (Entity, EventBus patterns)

### Completed Implementation
- ✅ **Core interfaces** (Environment, Generator, QueryHandler, RoomBuilder)
- ✅ **Basic environment implementation** (event-driven wrapper around spatial orchestrator)
- ✅ **Graph-based generation algorithm** (linear, branching, grid, organic layouts)
- ✅ **Environment query delegation** (multi-room aggregation delegating to spatial)
- ✅ **Event integration** (publishes/subscribes following toolkit patterns)
- ✅ **Wall pattern system design** (procedural generation with destructible walls)

### Required Implementation
- [ ] **Wall pattern generation functions** (implement the algorithmic patterns)
- [ ] **Room shape prefab loading** (6 base shape boundary files)
- [ ] **Graph-to-spatial translation** (complete generator placeholder methods)
- [ ] **Component-based room builder** (shape + pattern + size API)
- [ ] **Client extension hooks** (custom room factories, pattern registration)
- [ ] **Comprehensive test suite** (generation, queries, event integration)

---

**Author**: Development Team  
**Reviewers**: TBD  
**Implementation**: tools/environments module