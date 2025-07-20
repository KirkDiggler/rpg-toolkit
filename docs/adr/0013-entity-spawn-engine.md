# ADR-0013: Entity Spawn Engine Architecture

## Status
Proposed

## Context
The RPG Toolkit needs a flexible and powerful system for populating rooms with entities (players, enemies, treasure chests, items, etc.). Currently, the spatial module provides excellent room management and positioning capabilities, but there's no higher-level system that handles the complex logic of deciding what to place where based on game rules, spatial constraints, and procedural generation needs.

Game developers using the toolkit need to:
- Populate rooms with appropriate entities based on context
- Handle different spawn patterns (formations, clusters, scattered placement)
- Manage spatial constraints (minimum distances, line of sight, pathing)
- Support both deterministic and procedural placement
- Handle overflow scenarios when entities don't fit
- Integrate with loot tables and weighted selection systems

## Decision
We will create an Entity Spawn Engine as a new tool (`tools/spawn`) that provides a comprehensive solution for entity placement within rooms. The engine will be built on top of our existing spatial, selectables, and events modules.

### Key Architectural Decision: Entity Management Strategy

**Decision**: The spawn engine will NOT create entities. Instead, clients provide categorized pools of pre-existing entities, and the spawn engine uses selectables for selection and spatial module for positioning. Optional helper configs are provided for client convenience.

**Rationale**: This approach aligns with RPG Toolkit's philosophy of "infrastructure, not implementation":
- **Client Control**: Games define their own entity categories (e.g., "goblinoids", "treasure", "environmental")
- **No Factory Pattern**: Avoids entity creation complexity and maintains clean separation of concerns
- **Natural Selectables Integration**: Entity pools become selection tables with game-specific weights
- **Maximum Flexibility**: Supports any game genre with any categorization scheme
- **Compositional Spawning**: Mix and match categories for rich scenarios
- **Helper Configs**: Optional convenience configs for common patterns

**Implementation Pattern**:
```go
// Client provides categorized entity pools
entityPools := map[string][]core.Entity{
    "goblinoids": {orc1, goblin1, bugbear1},
    "treasure": {coins1, gems1, sword1},
    "environmental": {torch1, table1, chest1},
}

// Client provides selection tables for each category
selectionTables := map[string]selectables.SelectionTable[core.Entity]{
    "goblinoids": goblinoidTable,  // 60% goblins, 30% orcs, 10% bugbears
    "treasure": treasureTable,     // Game-specific treasure weights
    "environmental": decorTable,   // Environmental element weights
}

// Spawn engine handles selection + positioning
result := spawnEngine.PopulateRoom(roomID, entityPools, selectionTables, spawnConfig)

// Optional: Helper config for common patterns
helperConfig := CreateDungeonRoomConfig(entityCount, difficulty)
result := spawnEngine.PopulateRoomWithHelper(roomID, entityPools, helperConfig)
```

This decision eliminates entity factory patterns and provides a clean, flexible foundation for all subsequent design decisions.

### Key Design Decisions Summary

#### 1. Context Handling Strategy

**Decision**: Use both Go `context.Context` and selectables `SelectionContext` following established toolkit patterns.

```go
func (e *SpawnEngine) PopulateRoom(ctx context.Context, roomID string, 
    config SpawnConfig, selectionCtx selectables.SelectionContext) (SpawnResult, error)
```

**Rationale**: 
- **Go Context**: Enables cancellation, timeouts, and request tracing for long-running spawn operations
- **SelectionContext**: Provides dice roller and game state for selectables integration, consistent with existing module patterns
- **Event Context**: Carries request context through to event publishing for observability

#### 2. Selectables Integration Pattern

**Decision**: Hybrid table registry approach where clients register selection tables with spawn engine, spawn config references tables by ID.

```go
// Client registers tables once
spawnEngine.RegisterTable("goblinoids", goblinoidTable)
spawnEngine.RegisterTable("treasure", treasureTable)

// Spawn config references by ID
config := SpawnConfig{
    EntityGroups: []EntityGroup{
        {Type: "enemies", SelectionTable: "goblinoids", Quantity: QuantitySpec{Fixed: &three}},
    },
}
```

**Rationale**:
- **Reusable Tables**: Register once, use across multiple spawn operations
- **Clean Separation**: Table configuration separated from spawn configuration
- **Flexible Control**: Client has full control over selectables setup while spawn engine handles references
- **Consistent Patterns**: Follows existing selectables module conventions

#### 3. Space Calculation and Room Scaling Strategy ✅ IMPLEMENTED

**Decision**: Capacity-first approach using environment package for space calculations, with room scaling and split-awareness for overcrowding scenarios.

**Use Case**: Client provides room config with too many entities to fit comfortably. Spawn engine detects this, scales room if needed, communicates split recommendations as passthrough to client, and adapts spawning logic to work with whatever room structure results.

**Responsibility Separation**:
- **Environment Package**: Provides capacity analysis and split recommendations
- **Spawn Engine**: Detects capacity issues, relays recommendations, adapts to any room structure
- **Client/Upstream**: Decides whether to split rooms, creates split rooms when needed

**Implementation Pattern Using Environment Package Queries**:
```go
// 1. Check if current room can handle the entities (request split options)
capacityQuery := environments.CapacityQuery{
    RoomSize:            currentRoomDimensions,  // Client's original room config
    EntityCount:         len(selectedEntities),  // Number of entities to spawn  
    IncludeSplitOptions: true,                   // Always get split recommendations
    Constraints: environments.CapacityConstraints{
        TargetSpatialFeeling:      environments.SpatialFeelingNormal,
        MinEntitySpacing:          2.0,
        MinMovementSpace:          0.6,
        WallDensityModifier:       0.5,
        RequiredPathwayMultiplier: 1.2,
    },
}

response, err := environmentQueryHandler.HandleCapacityQuery(ctx, capacityQuery)

// 2. If room is too small, calculate optimal size and scale
if !response.Satisfied {
    sizingQuery := environments.SizingQuery{
        Intent:        environments.GetDefaultSpatialIntentProfile(environments.SpatialFeelingNormal),
        EntityCount:   len(selectedEntities),     // The "too many" entities
        MinDimensions: currentRoomDimensions,     // Don't go smaller than original
        AdditionalSpace: 1.2,                    // 20% buffer space
    }
    
    newDimensions, err := environmentQueryHandler.HandleSizingQuery(ctx, sizingQuery)
    
    // Scale up the room to fit all entities
    scaleFactor := newDimensions.Width / currentRoomDimensions.Width
    err = h.scaleExistingRoom(roomID, scaleFactor)
    
    // Emit scaling event with detailed reasoning
    h.publishRoomScalingEvent(ctx, roomID, currentRoomDimensions, newDimensions, 
        len(selectedEntities), response.Alternatives)
}

// 3. Communicate split recommendations to client (passthrough role)
if len(response.SplitOptions) > 0 {
    h.publishSplitRecommendationEvent(ctx, roomID, response.SplitOptions)
    // Client decides whether to split - spawn engine adapts to result
}

// 4. Adapt spawning logic to room structure (single or split)
roomStructure := h.analyzeRoomStructure(roomID)
if roomStructure.IsSplit {
    // Handle multi-room spawning with entity distribution
    err = h.populateSplitRooms(roomStructure.ConnectedRooms, selectedEntities, config)
} else {
    // Handle single room spawning
    err = h.populateSingleRoom(roomID, selectedEntities, config)
}
```

**Environment Package Functions Used**:
- `HandleCapacityQuery()` - Analyzes if entities fit in current room
- `HandleSizingQuery()` - Calculates optimal room dimensions for entity count  
- `EstimateRoomCapacity()` - Core capacity analysis with spatial feeling support
- `CalculateOptimalRoomSize()` - Mathematical sizing based on entity density and movement needs

**Capacity Analysis Response with Split Options**:
```go
response := CapacityQueryResponse{
    Satisfied: false,                    // Room too small for entity count
    Alternatives: []string{              // What spawn engine can relay to client
        "Consider scaling the room up",
        "Reduce entity count", 
        "Split room into multiple rooms",
    },
    Estimate: CapacityEstimate{
        RecommendedEntityCount: 8,       // Room comfortably fits 8 entities
        MaxEntityCount:         12,      // Could squeeze 12 entities max
        SpatialFeelingActual:   SpatialFeelingTight,  // Would feel cramped
        QualityScore:           0.3,     // Poor spatial quality with overcrowding
    },
    SplitOptions: []RoomSplit{           // Detailed split recommendations from environment
        {
            SuggestedSize: spatial.Dimensions{Width: 10, Height: 20},
            ConnectionPoints: []spatial.Position{{X: 10, Y: 10}},
            RecommendedEntityDistribution: map[string]int{
                "room_1": 10, "room_2": 10,  // Distribute 20 entities across split
            },
            RecommendedConnectionType: "door",
            SplitReason: "Horizontal split for manageable entity density",
            EstimatedCapacityImprovement: 0.8,
        },
        // Additional split patterns available...
    },
}
```

**Rationale**:
- **Eliminates Constraint Solving**: Pre-calculating space requirements avoids complex backtracking algorithms
- **Leverages Proven Infrastructure**: Environment package provides mathematically sound space calculations
- **Clean Separation of Concerns**: Spawn engine focuses on entity placement, not room structure decisions
- **Split-Aware Design**: Spawn engine works seamlessly with single rooms or multi-room configurations
- **Client Autonomy**: Upstream clients retain full control over room splitting decisions
- **Event Transparency**: Room scaling and split recommendation events provide debugging visibility
- **Quality Assurance**: Maintains proper spatial relationships regardless of room structure

#### 4. Entity Size and Spatial Integration

**Decision**: Leverage existing spatial module `Placeable` interface for entity sizing and movement constraints.

**Existing Infrastructure Utilized**:
```go
// From spatial.Placeable interface (already implemented)
entity.GetSize() int              // Multi-space entity support
entity.BlocksMovement() bool      // Pathability considerations  
entity.BlocksLineOfSight() bool   // Line of sight constraints
```

**Space Calculation Logic**:
- Use `entity.GetSize()` for space requirements (default to 1 for non-Placeable entities)
- Account for `BlocksMovement()` in pathability calculations
- Factor `BlocksLineOfSight()` into LOS constraint validation

#### 5. Environment Package Query Integration ✅ IMPLEMENTED

**Decision**: Leverage existing environment package query capabilities for space calculations and room analysis.

**Available Query Types in Environment Package**:
```go
// Space capacity analysis
type CapacityQuery struct {
    RoomID              string
    Intent              SpatialIntentProfile
    EntityCount         int
    RoomSize            spatial.Dimensions
    Constraints         CapacityConstraints
    ExistingEntityCount int
    IncludeSplitOptions bool
}

// Room sizing calculations  
type SizingQuery struct {
    Intent          SpatialIntentProfile
    EntityCount     int
    AdditionalSpace float64
    MinDimensions   spatial.Dimensions
    MaxDimensions   spatial.Dimensions
}

// Multi-room entity queries
type EntityQuery struct {
    Center      *spatial.Position
    Radius      float64
    RoomIDs     []string
    EntityTypes []string
    InTheme     string
    HasFeature  string
    Limit       int
}

// Room discovery and filtering
type RoomQuery struct {
    RoomTypes      []string
    Themes         []string
    Features       []string
    NearPosition   *spatial.Position
    ConnectedTo    string
    MinConnections int
    MaxConnections int
}
```

**Integration Pattern for Spawn Engine**:
```go
// Use environment queries for space planning
capacityQuery := environments.CapacityQuery{
    EntityCount: len(selectedEntities),
    RoomSize:    currentRoomDimensions,
    Constraints: environments.CapacityConstraints{
        TargetSpatialFeeling: SpatialFeelingNormal,
        MinEntitySpacing:     2.0,
    },
}
response, err := environmentQueryHandler.HandleCapacityQuery(ctx, capacityQuery)

// Scale room if needed
if !response.Satisfied {
    sizingQuery := environments.SizingQuery{
        Intent:      environments.GetDefaultSpatialIntentProfile(SpatialFeelingNormal),
        EntityCount: len(selectedEntities),
    }
    newDimensions, err := environmentQueryHandler.HandleSizingQuery(ctx, sizingQuery)
    // Apply scaling using existing environment capabilities
}
```

**Rationale**:
- **Existing Infrastructure**: Environment package already provides comprehensive space calculation
- **Clean Separation**: Spawn engine orchestrates, environment package calculates
- **Proven Implementation**: Space queries are implemented and tested
- **Consistent API**: Follows established toolkit query patterns

#### 6. Error Recovery and Placement Strategy

**Decision**: Best-effort placement with progressive constraint relaxation.

**Strategy**:
- **Primary**: Attempt placement with full constraints (formations, spacing, LOS)
- **Fallback**: Relax non-critical constraints (formation precision, exact spacing)
- **Final**: Ensure every entity gets placed somewhere valid, even if not optimal
- **Transparency**: Report placement failures and constraint violations via events

**Rationale**:
- **Guaranteed Results**: Every entity gets placed rather than failing entirely
- **Graceful Degradation**: Maintains game functionality even when ideal placement impossible
- **Debug Visibility**: Events show what constraints were relaxed for post-analysis

### Performance and Quality Trade-offs

**Decision**: Prioritize reliability and simplicity over optimal placement efficiency.

**Approach**:
- **Time Limits**: Hard timeout for placement operations (configurable)
- **Iteration Limits**: Maximum attempts per entity placement
- **Quality Metrics**: Track and report placement efficiency in spawn results
- **Caching**: Cache valid positions within single spawn operation for reuse

**Rationale**: For RPG scenarios, ensuring all entities are placed is more important than finding mathematically optimal positions. The capacity-first approach with room scaling eliminates most performance bottlenecks.

### Existing Infrastructure Research and Usage Patterns

#### Spatial Module Integration Analysis

**Key Capabilities Discovered**:
- **Entity Sizing**: `spatial.Placeable.GetSize() int` already supports multi-space entities
- **Movement Blocking**: `spatial.Placeable.BlocksMovement() bool` for pathability calculations
- **Line of Sight**: `spatial.Placeable.BlocksLineOfSight() bool` for LOS constraints
- **Room Management**: `spatial.Room` interface provides entity placement, collision detection, and spatial queries
- **Position Validation**: `Room.CanPlaceEntity()` and `Room.IsPositionOccupied()` for placement validation
- **Spatial Queries**: `Room.GetEntitiesInRange()`, `Room.GetLineOfSight()`, `Room.IsLineOfSightBlocked()`

**Usage Patterns for Spawn Engine**:
```go
// Entity placement using existing spatial infrastructure
room := spatialQueryHandler.GetRoom(roomID)
grid := room.GetGrid()
dimensions := grid.GetDimensions()

// Validate placement using existing methods
if room.CanPlaceEntity(entity, position) {
    err := room.PlaceEntity(entity, position)
    // Handle placement result
}

// Line of sight validation using spatial queries
losPath := room.GetLineOfSight(fromPos, toPos)
blocked := room.IsLineOfSightBlocked(fromPos, toPos)

// Multi-space entity handling
if placeable, ok := entity.(spatial.Placeable); ok {
    entitySize := placeable.GetSize()  // 1 = single space, 2+ = multi-space
    blocksMovement := placeable.BlocksMovement()
    blocksLOS := placeable.BlocksLineOfSight()
}
```

#### 7. Gridless Room Design Intent ✅ IMPLEMENTED

**Decision**: For gridless rooms, the spawn engine provides simple entity placement within room boundaries without grid constraints.

**Design Intent**:
- **Continuous Positioning**: Gridless rooms use floating-point coordinates for smooth, realistic positioning
- **Simplified Placement**: No grid alignment requirements - entities can be placed at any valid coordinate
- **Boundary Respect**: Entities respect room boundaries and wall proximity constraints
- **Natural Distribution**: Uses random sampling within room bounds for organic entity distribution
- **Constraint Compatibility**: All spatial constraints (distance, line of sight, etc.) work with gridless positioning

**Implementation Strategy**:
```go
// Gridless room detection and handling
func (cs *ConstraintSolver) isGridlessRoom(grid spatial.Grid) bool {
    return grid == nil // Gridless rooms have no grid system
}

// Optimized position finding for continuous positioning
func (cs *ConstraintSolver) findValidPositionsGridless(
    room spatial.Room, entity core.Entity, constraints SpatialConstraints,
    existingEntities []SpawnedEntity, maxPositions int,
) ([]spatial.Position, error) {
    // Use random sampling within room bounds
    roomDimensions := room.GetDimensions()
    margin := 1.0 // Wall proximity buffer
    
    for attempts := 0; attempts < maxAttempts; attempts++ {
        x := margin + (roomDimensions.Width-2*margin)*rand.Float64()
        y := margin + (roomDimensions.Height-2*margin)*rand.Float64()
        position := spatial.Position{X: x, Y: y}
        
        if cs.ValidatePosition(room, position, entity, constraints, existingEntities) == nil {
            validPositions = append(validPositions, position)
        }
    }
}
```

**Rationale**:
- **Flexibility**: Supports both tactical grid-based games and fluid, realistic positioning systems
- **Performance**: Random sampling is more efficient than exhaustive grid searches for large spaces
- **Natural Placement**: Avoids artificial grid-locked positioning in continuous environments
- **Toolkit Philosophy**: Provides infrastructure for any positioning system, not imposing specific mechanics

#### Environment Package Integration Analysis

**Key Capabilities Discovered**:
- **Room Building**: `BasicRoomBuilder` with configurable dimensions, shapes, and patterns
- **Shape Management**: Predefined shapes (rectangle, square) with connection points for doorways
- **Wall Entities**: `WallEntity` implements `spatial.Placeable` with destruction and property support
- **Room Scaling**: Percentage-based scaling (0.0-1.0) already supported in room generation
- **Connection Points**: Rooms have defined entrance/exit positions for multi-room scenarios

**Missing Capabilities Requiring Extension**:
- **Space Calculation Queries**: Need `SpaceRequirementQuery` and `SpaceAvailabilityQuery`
- **Pathability Analysis**: Need algorithms to ensure traversable paths between connection points
- **Capacity Planning**: Need methods to calculate minimum room size for entity counts

**Proposed Environment Package Extensions**:
```go
// Add to environment package query system
type SpaceRequirementQuery struct {
    Entities     []core.Entity
    Constraints  SpaceConstraints
    RoomType     string
}

type SpaceQueryResult struct {
    RequiredSpaces   int
    AvailableSpaces  int
    PathingSpaces    int
    BufferSpaces     int
    RecommendedScale float64
    IsAdequate       bool
}

// Extend existing QueryHandler interface
func (h *QueryHandler) ProcessSpaceQuery(query SpaceRequirementQuery) SpaceQueryResult
```

#### Selectables Module Integration Analysis

**Key Capabilities Utilized**:
- **Generic Selection Tables**: `SelectionTable[T comparable]` with full type safety
- **Multiple Selection Modes**: `Select()`, `SelectMany()`, `SelectUnique()`, `SelectVariable()`
- **Context-Aware Selection**: `SelectionContext` provides dice roller and game state
- **Event Integration**: Selection events for debugging and analytics
- **Weighted Selection**: Configurable weights with min/max bounds

**Integration Patterns for Spawn Engine**:
```go
// Table registration and usage
spawnEngine.RegisterTable("goblinoids", goblinoidTable)

// Selection with context
selectedEntities, err := table.SelectMany(selectionCtx, count)

// Quantity determination using dice expressions
quantity, err := e.resolveQuantity(group.Quantity, selectionCtx)
if spec.DiceRoll != nil {
    roller := selectionCtx.GetDiceRoller()
    quantity, err := e.parseDiceExpression(*spec.DiceRoll, roller)
}

// Table creation following established patterns
table := selectables.NewBasicTable[core.Entity](selectables.BasicTableConfig{
    ID:       "entity_table",
    EventBus: eventBus,
    Configuration: selectables.TableConfiguration{
        EnableEvents: true,
        MinWeight:    1,
        MaxWeight:    100,
    },
})
```

#### Events Module Integration Analysis

**Key Capabilities Utilized**:
- **Event Publishing**: `EventBus.Publish(ctx, event)` with Go context support
- **Event Subscription**: `EventBus.SubscribeFunc()` for game logic integration
- **Event Types**: String-based event types following dot notation (e.g., "spawn.entity.spawned")
- **Event Data**: Structured event data with source entities and rich context

**Spawn Engine Event Strategy**:
```go
// Event constants following toolkit patterns
const (
    EventSpawnOperationStarted   = "spawn.operation.started"
    EventEntitySpawned          = "spawn.entity.spawned"
    EventEntitySpawnFailed      = "spawn.entity.failed"
    EventRoomModified           = "spawn.room.modified"
    EventSpawnOperationCompleted = "spawn.operation.completed"
    EventAdaptiveScaling        = "spawn.room.scaled"
)

// Event publishing patterns
func (e *SpawnEngine) publishSpawnEvents(ctx context.Context, roomID string, 
    config SpawnConfig, result SpawnResult) {
    
    roomEntity := &SpawnRoomEntity{id: roomID, roomType: "spawn_target"}
    
    // Individual entity spawn events
    for _, spawnedEntity := range result.SpawnedEntities {
        entityData := EntitySpawnEventData{
            Entity:      spawnedEntity.Entity,
            Position:    spawnedEntity.Position,
            GroupID:     spawnedEntity.GroupID,
            RoomID:      roomID,
        }
        event := events.NewGameEvent(EventEntitySpawned, roomEntity, entityData)
        e.eventBus.Publish(ctx, event)
    }
}
```

#### Core Module Integration Analysis

**Key Patterns Utilized**:
- **Entity Interface**: `core.Entity` with `GetID()` and `GetType()` for consistent entity handling
- **Configuration Patterns**: Config structs for dependency injection and clean initialization
- **Error Handling**: Rich error context with suggestions and detailed information
- **Interface Design**: Clean separation of concerns with focused interfaces

**Spawn Engine Core Integration**:
```go
// Configuration following toolkit patterns
type BasicSpawnEngineConfig struct {
    ID                  string
    SpatialQueryHandler spatial.QueryHandler
    SelectablesRegistry SelectablesRegistry
    EventBus           events.EventBus
    Configuration      SpawnEngineConfiguration
}

// Entity handling using core.Entity interface
func (e *SpawnEngine) processEntity(entity core.Entity) error {
    entityID := entity.GetID()
    entityType := entity.GetType()
    
    // Handle spatial entities
    if placeable, ok := entity.(spatial.Placeable); ok {
        size := placeable.GetSize()
        blocksMovement := placeable.BlocksMovement()
        // Entity-specific placement logic
    }
    
    return nil
}
```

### Core Architecture

#### Primary Interface
```go
type SpawnEngine interface {
    // Core spawning - works with single rooms or split room configurations
    PopulateSpace(roomOrGroup interface{}, config SpawnConfig) (SpawnResult, error)
    
    // Legacy single-room interface for backwards compatibility
    PopulateRoom(roomID string, config SpawnConfig) (SpawnResult, error)
    
    // Multi-room spawning for split room scenarios
    PopulateSplitRooms(connectedRooms []string, config SpawnConfig) (SpawnResult, error)
    
    // Room transition handling
    HandleRoomTransition(entityID, fromRoom, toRoom, connectionID string) (Position, error)
    
    // Configuration validation
    ValidateSpawnConfig(config SpawnConfig) error
    
    // Room structure analysis for split-awareness
    AnalyzeRoomStructure(roomID string) RoomStructureInfo
}

// Room structure information for split-aware spawning
type RoomStructureInfo struct {
    IsSplit         bool     `json:"is_split"`
    ConnectedRooms  []string `json:"connected_rooms"`  
    ConnectionTypes []string `json:"connection_types"`
    TotalCapacity   int      `json:"total_capacity"`
}
```

#### Configuration Structure
```go
type SpawnConfig struct {
    // What to spawn
    EntityGroups     []EntityGroup       `json:"entity_groups"`
    LootTables       map[string]string   `json:"loot_tables"`       // selectables table IDs
    
    // How to spawn
    Pattern          SpawnPattern        `json:"pattern"`
    TeamConfiguration *TeamConfig        `json:"team_config,omitempty"`
    
    // Constraints
    SpatialRules     SpatialConstraints  `json:"spatial_rules"`
    Placement        PlacementRules      `json:"placement"`
    
    // Behavior
    Strategy         SpawnStrategy       `json:"strategy"`
    AdaptiveScaling  *ScalingConfig      `json:"adaptive_scaling,omitempty"`
}

type EntityGroup struct {
    Type            string              `json:"type"`               // "player", "enemy", "treasure"
    SelectionTable  string              `json:"selection_table"`    // selectables table ID
    Quantity        QuantitySpec        `json:"quantity"`
    Priority        int                 `json:"priority"`           // for conflict resolution
}

type SpawnPattern string
const (
    PatternFormation SpawnPattern = "formation"    // Structured arrangements
    PatternClustered SpawnPattern = "clustered"    // Groups with spacing
    PatternScattered SpawnPattern = "scattered"    // Random distribution
    PatternTeamBased SpawnPattern = "team_based"   // Red vs Blue zones
    PatternCustom    SpawnPattern = "custom"       // User-defined patterns
)

type SpawnStrategy string
const (
    StrategyDeterministic SpawnStrategy = "deterministic"  // Same result each time
    StrategyRandomized    SpawnStrategy = "randomized"     // Random within constraints
    StrategyBalanced      SpawnStrategy = "balanced"       // Optimize for gameplay
)
```

#### Spatial Constraints System
```go
type SpatialConstraints struct {
    MinDistance      map[string]float64  `json:"min_distance"`       // Between entity types
    LineOfSight      LineOfSightRules    `json:"line_of_sight"`
    AreaOfEffect     map[string]float64  `json:"area_of_effect"`     // Buffer zones
    PathingRules     PathingConstraints  `json:"pathing"`
    WallProximity    float64             `json:"wall_proximity"`     // Min distance from walls
}

type LineOfSightRules struct {
    RequiredSight    []EntityPair        `json:"required_sight"`     // Must see each other
    BlockedSight     []EntityPair        `json:"blocked_sight"`      // Must NOT see each other
}

type PathingConstraints struct {
    MaintainExitAccess   bool            `json:"maintain_exit_access"`
    MinPathWidth         float64         `json:"min_path_width"`
    RequiredConnections  []string        `json:"required_connections"` // Connection IDs
}
```

#### Result and Event System
```go
type SpawnResult struct {
    Success           bool                    `json:"success"`
    SpawnedEntities   []SpawnedEntity         `json:"spawned_entities"`
    Failures          []SpawnFailure          `json:"failures"`
    RoomModifications []RoomModification      `json:"room_modifications"`
    SplitRecommendations []RoomSplit          `json:"split_recommendations"`  // Passthrough from environment
    RoomStructure     RoomStructureInfo       `json:"room_structure"`         // What structure was used
    Metadata          SpawnMetadata           `json:"metadata"`
}

type SpawnedEntity struct {
    Entity           core.Entity             `json:"entity"`
    Position         spatial.Position        `json:"position"`
    RoomID           string                  `json:"room_id"`          // Which room in split configuration
    GroupID          string                  `json:"group_id"`
    SpawnReason      string                  `json:"spawn_reason"`
}

type SpawnFailure struct {
    EntityType       string                  `json:"entity_type"`
    Reason           string                  `json:"reason"`
    AttemptedPosition *spatial.Position       `json:"attempted_position,omitempty"`
    Suggestions      []string                `json:"suggestions"`
}
```

### Integration Points

#### With Spatial Module
- **Room Management**: Query room dimensions, validate positions, check collisions
- **Grid Systems**: Support square, hex, and gridless positioning
- **Multi-Room**: Handle spawning during room transitions
- **Connection Points**: Smart placement near entrances/exits

#### With Selectables Module
- **Loot Tables**: Use weighted selection for "what to spawn" decisions
- **Quantity Determination**: Dice expressions for entity counts
- **Conditional Selection**: Context-aware entity selection

#### With Events Module
- **Spawn Events**: Entity placement, failures, room modifications
- **Overflow Events**: When adaptive scaling occurs
- **Debugging Events**: Detailed placement decision logging

### Advanced Features

#### Formation System
```go
type FormationPattern struct {
    Name             string                  `json:"name"`
    Positions        []RelativePosition      `json:"positions"`
    Scaling          FormationScaling        `json:"scaling"`
    Constraints      FormationConstraints    `json:"constraints"`
}

type FormationScaling struct {
    AllowRotation    bool                    `json:"allow_rotation"`
    AllowStretching  bool                    `json:"allow_stretching"`
    PreserveRatios   bool                    `json:"preserve_ratios"`
}
```

#### Adaptive Room Scaling
```go
type ScalingConfig struct {
    Enabled          bool                    `json:"enabled"`
    MaxAttempts      int                     `json:"max_attempts"`
    ScalingFactor    float64                 `json:"scaling_factor"`
    PreserveAspect   bool                    `json:"preserve_aspect"`
    EmitEvents       bool                    `json:"emit_events"`
}
```

#### Team-Based Spawning
**Purpose**: Keep allied entities grouped together and separated from opposing teams, not scattered randomly.

```go
type TeamConfig struct {
    Teams            []Team                  `json:"teams"`
    SeparationRules  SeparationConstraints   `json:"separation_rules"`
    SpawnZones       []SpawnZone             `json:"spawn_zones"`
}

type Team struct {
    ID               string                  `json:"id"`                // "friendlies", "enemies", "neutrals"
    EntityTypes      []string                `json:"entity_types"`      // Entity types for this team
    Formation        *FormationPattern       `json:"formation,omitempty"` // How team members are arranged
    PreferredZone    string                  `json:"preferred_zone"`    // Spawn zone for this team
    Cohesion         float64                 `json:"cohesion"`          // How tightly grouped (0.0-1.0)
}

type SeparationConstraints struct {
    MinTeamDistance  float64                 `json:"min_team_distance"`  // Minimum distance between teams
    BufferZones      []BufferZone            `json:"buffer_zones"`       // No-spawn areas between teams  
    TeamPlacement    TeamPlacementStrategy   `json:"team_placement"`     // "corners", "opposite_sides", "random"
}

// Example: Friendlies spawn together in northwest, enemies together in southeast
// No random mixing - tactical positioning for clear team boundaries
```

### Performance and Quality Trade-offs ✅ DECIDED

**Decision**: Prioritize reliability and simplicity over optimal placement efficiency.

**Approach**:
- **Operation Timeout**: 30 seconds maximum for spawn operations (configurable)
- **Iteration Limits**: 1000 placement attempts per entity maximum
- **Quality Threshold**: Accept 80% constraint satisfaction as "good enough" placement
- **Caching Strategy**: Cache constraint validation results within single spawn operation for reuse
- **Graceful Degradation**: Ensure every entity gets placed rather than failing entirely

**Rationale**: For RPG scenarios, ensuring all entities are placed is more important than finding mathematically optimal positions. The capacity-first approach with room scaling eliminates most performance bottlenecks. Games need predictable behavior over perfect optimization.

### Error Recovery Strategy 📝 TODO

**Decision**: Plan progressive constraint relaxation during implementation phase.

**Areas to Define**:
- Exact priority order for constraint relaxation
- Which constraints are "critical" vs. "nice-to-have"
- Fallback mechanisms when all placement attempts fail
- Event reporting for constraint violations and recoveries

## Consequences

### Positive
- **Comprehensive Solution**: Addresses all major entity placement needs in RPGs
- **Modular Integration**: Builds naturally on existing spatial, selectables, and events modules
- **Flexible Configuration**: Supports both simple and complex spawn scenarios
- **Event-Driven**: Full observability for debugging and analytics
- **Adaptive**: Handles edge cases like room overflow gracefully
- **Performance**: Leverages existing spatial optimizations and caching

### Negative
- **Complexity**: Large feature set may be overwhelming for simple use cases
- **Performance**: Complex constraint solving could be expensive for large rooms
- **Testing**: Comprehensive test coverage will require extensive scenario testing
- **Dependencies**: Relies heavily on spatial module capabilities

### Mitigation Strategies
- **Progressive Complexity**: Start with simple patterns, add advanced features incrementally
- **Performance Optimization**: Use spatial queries efficiently, cache constraint checks
- **Comprehensive Testing**: Test suite with varied scenarios and edge cases
- **Clear Documentation**: Examples for common use cases, advanced feature guides

## Implementation Plan

### Phase 1: Core Infrastructure
- Basic SpawnEngine interface and implementation
- Simple spawn patterns (scattered, formation)
- Integration with spatial module for position validation
- Basic event publishing

### Phase 2: Advanced Patterns
- Formation system with predefined patterns
- Team-based spawning with zone management
- Clustered spawning with group cohesion
- Room transition handling

### Phase 3: Constraint System
- Spatial constraint validation
- Line of sight calculations
- Area of effect buffer zones
- Pathing requirement enforcement

### Phase 4: Adaptive Features
- Room scaling when entities don't fit
- Conflict resolution for overlapping constraints
- Performance optimization for large rooms
- Comprehensive error handling and recovery

### Phase 5: Integration and Polish
- Selectables integration for loot table spawning
- Advanced event system with detailed metadata
- Configuration validation and helpful error messages
- Documentation and usage examples

## Integration Patterns and Examples

### Spatial Module Integration

The spawn engine leverages the spatial module for all positioning, collision detection, and room management.

```go
// Example: Room interaction patterns
func (e *BasicSpawnEngine) PopulateRoom(roomID string, config SpawnConfig) (SpawnResult, error) {
    // Get room from spatial system
    room, err := e.spatialQueryHandler.GetRoom(roomID)
    if err != nil {
        return SpawnResult{}, fmt.Errorf("room not found: %w", err)
    }
    
    // Check room capacity and dimensions
    grid := room.GetGrid()
    dimensions := grid.GetDimensions()
    
    // Use spatial queries for placement validation
    for _, entityGroup := range config.EntityGroups {
        // Find valid positions using spatial constraints
        validPositions := e.findValidPositions(room, entityGroup.Constraints)
        
        // Place entities using Room.PlaceEntity
        for _, entity := range selectedEntities {
            position := e.selectOptimalPosition(validPositions, constraints)
            err := room.PlaceEntity(entity, position)
            if err != nil {
                // Handle placement failure
                continue
            }
            
            // Verify placement doesn't break line of sight rules
            if config.SpatialRules.LineOfSight.RequiredSight != nil {
                if !e.validateLineOfSight(room, position, config.SpatialRules.LineOfSight) {
                    room.RemoveEntity(entity.GetID())
                    continue
                }
            }
        }
    }
}

// Example: Line of sight validation using spatial queries
func (e *BasicSpawnEngine) validateLineOfSight(room spatial.Room, pos spatial.Position, 
    rules LineOfSightRules) bool {
    
    for _, pair := range rules.RequiredSight {
        entitiesOfType := e.getEntitiesByType(room, pair.From)
        for _, entity := range entitiesOfType {
            entityPos, _ := room.GetEntityPosition(entity.GetID())
            
            // Use spatial line of sight calculation
            losPath := room.GetLineOfSight(pos, entityPos)
            if rules.CheckWalls && room.IsLineOfSightBlocked(pos, entityPos) {
                return false
            }
        }
    }
    return true
}

// Example: Room transition spawning
func (e *BasicSpawnEngine) HandleRoomTransition(entityID, fromRoom, toRoom, 
    connectionID string) (spatial.Position, error) {
    
    // Get connection details from spatial orchestrator
    connection, err := e.roomOrchestrator.GetConnection(connectionID)
    if err != nil {
        return spatial.Position{}, err
    }
    
    // Get target room
    targetRoom, err := e.spatialQueryHandler.GetRoom(toRoom)
    if err != nil {
        return spatial.Position{}, err
    }
    
    // Find placement near entrance using spatial queries
    entrancePos := connection.GetTargetPosition(toRoom)
    nearbyPositions := targetRoom.GetPositionsInRange(entrancePos, 3.0)
    
    // Filter for valid positions
    for _, pos := range nearbyPositions {
        if targetRoom.CanPlaceEntity(entity, pos) {
            return pos, nil
        }
    }
    
    return spatial.Position{}, errors.New("no valid positions near entrance")
}
```

### Selectables Module Integration

The spawn engine uses selectables for "what to spawn" decisions and loot table integration.

```go
// Example: Entity selection using selectables tables
func (e *BasicSpawnEngine) selectEntitiesFromGroup(group EntityGroup, 
    ctx SelectionContext) ([]core.Entity, error) {
    
    // Get selectables table
    table, err := e.tablesRegistry.GetTable(group.SelectionTable)
    if err != nil {
        return nil, fmt.Errorf("selection table not found: %w", err)
    }
    
    // Determine quantity using dice/range
    quantity, err := e.resolveQuantity(group.Quantity, ctx)
    if err != nil {
        return nil, err
    }
    
    // Perform selection based on group constraints
    var selectedItems []string
    switch group.Type {
    case "treasure":
        // Use unique selection for treasure (no duplicates)
        selectedItems, err = table.SelectUnique(ctx, quantity)
    case "enemy":
        // Allow duplicates for enemies
        selectedItems, err = table.SelectMany(ctx, quantity)
    case "player":
        // Deterministic selection for players
        selectedItems, err = table.Select(ctx) // single selection
    }
    
    if err != nil {
        return nil, fmt.Errorf("selection failed: %w", err)
    }
    
    // Convert selected items to entities
    entities := make([]core.Entity, 0, len(selectedItems))
    for _, item := range selectedItems {
        entity, err := e.entityFactory.CreateEntity(item, group.Type)
        if err != nil {
            continue // log warning and continue
        }
        entities = append(entities, entity)
    }
    
    return entities, nil
}

// Example: Quantity resolution using dice integration
func (e *BasicSpawnEngine) resolveQuantity(spec QuantitySpec, 
    ctx SelectionContext) (int, error) {
    
    if spec.Fixed != nil {
        return *spec.Fixed, nil
    }
    
    if spec.DiceRoll != nil {
        // Use dice roller from context (same pattern as selectables)
        roller := ctx.GetDiceRoller()
        if roller == nil {
            return 0, errors.New("dice roller required for dice expressions")
        }
        
        // Parse and roll dice expression
        quantity, err := e.parseDiceExpression(*spec.DiceRoll, roller)
        return quantity, err
    }
    
    if spec.MinMax != nil {
        // Random between min/max
        roller := ctx.GetDiceRoller()
        range_size := spec.MinMax.Max - spec.MinMax.Min + 1
        roll, err := roller.Roll(range_size)
        if err != nil {
            return 0, err
        }
        return spec.MinMax.Min + roll - 1, nil
    }
    
    return 1, nil // default
}

// Example: Creating selectables table for spawn configuration
func CreateLootTable(eventBus events.EventBus) selectables.SelectionTable[string] {
    table := selectables.NewBasicTable[string](selectables.BasicTableConfig{
        ID:       "dungeon_loot",
        EventBus: eventBus,
        Configuration: selectables.TableConfiguration{
            EnableEvents: true,
            MinWeight:    1,
            MaxWeight:    100,
        },
    })
    
    return table.
        Add("gold_coins", 40).
        Add("health_potion", 30).
        Add("magic_sword", 15).
        Add("ancient_scroll", 10).
        Add("legendary_artifact", 5)
}
```

### Events Module Integration

The spawn engine publishes comprehensive events for observability, debugging, and game logic integration.

```go
// Event constants following toolkit patterns
const (
    EventSpawnOperationStarted   = "spawn.operation.started"
    EventEntitySpawned          = "spawn.entity.spawned"
    EventEntitySpawnFailed      = "spawn.entity.failed"
    EventRoomModified           = "spawn.room.modified"
    EventRoomScaled             = "spawn.room.scaled"
    EventSplitRecommended       = "spawn.split.recommended"      // Passthrough event to client
    EventMultiRoomSpawn         = "spawn.multiroom.completed"    // Split room spawning completed
    EventSpawnOperationCompleted = "spawn.operation.completed"
    EventFormationApplied       = "spawn.formation.applied"
    EventConstraintViolation    = "spawn.constraint.violation"
)

// Example: Event publishing patterns with split-awareness
func (e *BasicSpawnEngine) publishSpawnEvents(roomOrGroup interface{}, config SpawnConfig, 
    result SpawnResult) {
    
    if e.eventBus == nil || !e.config.EnableEvents {
        return
    }
    
    // Handle both single room and split room scenarios
    var roomEntity core.Entity
    var roomID string
    
    if result.RoomStructure.IsSplit {
        roomID = strings.Join(result.RoomStructure.ConnectedRooms, "+")
        roomEntity = &SpawnRoomGroupEntity{
            id:              roomID, 
            roomType:        "split_spawn_target",
            connectedRooms:  result.RoomStructure.ConnectedRooms,
        }
    } else {
        roomID = roomOrGroup.(string)
        roomEntity = &SpawnRoomEntity{id: roomID, roomType: "single_spawn_target"}
    }
    
    // Publish split recommendations if available (passthrough to client)
    if len(result.SplitRecommendations) > 0 {
        splitData := SplitRecommendationEventData{
            OriginalRoomID:   roomID,
            SplitOptions:     result.SplitRecommendations,
            EntityCount:      len(config.EntityGroups),
            Reason:           "Capacity analysis suggests room splitting",
        }
        splitEvent := events.NewGameEvent(EventSplitRecommended, roomEntity, splitData)
        e.eventBus.Publish(context.Background(), splitEvent)
    }
    
    // Publish operation completed event
    operationData := SpawnOperationEventData{
        RoomOrGroup:       roomOrGroup,
        Configuration:     config,
        Result:           result,
        TotalEntities:    len(result.SpawnedEntities),
        FailedEntities:   len(result.Failures),
        ExecutionTime:    result.Metadata.ExecutionTime,
        RoomModifications: result.RoomModifications,
        UsedSplitRooms:   result.RoomStructure.IsSplit,
    }
    
    eventType := EventSpawnOperationCompleted
    if result.RoomStructure.IsSplit {
        eventType = EventMultiRoomSpawn
    }
    
    event := events.NewGameEvent(eventType, roomEntity, operationData)
    e.eventBus.Publish(context.Background(), event)
    
    // Publish individual entity spawn events (now with room info for splits)
    for _, spawnedEntity := range result.SpawnedEntities {
        entityData := EntitySpawnEventData{
            Entity:      spawnedEntity.Entity,
            Position:    spawnedEntity.Position,
            RoomID:      spawnedEntity.RoomID,  // Specific room in split config
            GroupID:     spawnedEntity.GroupID,
            SpawnReason: spawnedEntity.SpawnReason,
        }
        
        entityEvent := events.NewGameEvent(EventEntitySpawned, roomEntity, entityData)
        e.eventBus.Publish(context.Background(), entityEvent)
    }
}

// Example: Event subscription for game logic integration
func SetupSpawnEventHandlers(eventBus events.EventBus, gameState *GameState) {
    // React to successful entity spawns
    eventBus.SubscribeFunc(EventEntitySpawned, 0, func(ctx context.Context, event events.Event) error {
        data := event.Data().(EntitySpawnEventData)
        
        // Update game state
        gameState.AddEntity(data.Entity, data.Position, data.RoomID)
        
        // Trigger welcome logic for players
        if data.Entity.GetType() == "player" {
            gameState.TriggerPlayerWelcome(data.Entity.GetID())
        }
        
        // Start AI behavior for enemies
        if data.Entity.GetType() == "enemy" {
            gameState.StartAIBehavior(data.Entity.GetID())
        }
        
        return nil
    })
    
    // React to room modifications
    eventBus.SubscribeFunc(EventRoomModified, 0, func(ctx context.Context, event events.Event) error {
        data := event.Data().(RoomModificationEventData)
        
        // Log room changes for debugging
        log.Printf("Room %s modified: %s - %v -> %v", 
            data.RoomID, data.Type, data.OldValue, data.NewValue)
        
        // Update minimap or UI
        gameState.UpdateRoomVisualization(data.RoomID)
        
        return nil
    })
}
```

### Core Module Integration

Following established entity patterns and error handling conventions.

```go
// Example: Entity creation and management
func (e *BasicSpawnEngine) createSpawnableEntity(entityType, subType string, 
    metadata map[string]interface{}) (core.Entity, error) {
    
    // Use entity factory pattern consistent with toolkit
    entity := &SpawnableEntity{
        id:       generateEntityID(),
        typeInfo: entityType + "." + subType,
        metadata: metadata,
        created:  time.Now(),
    }
    
    return entity, nil
}

// Example: Error handling following toolkit patterns
func (e *BasicSpawnEngine) handleSpawnError(operation string, entityType string, 
    reason error) SpawnFailure {
    
    // Create rich error context (similar to selectables errors)
    failure := SpawnFailure{
        EntityType: entityType,
        Reason:     reason.Error(),
        Suggestions: e.generateSuggestions(reason),
    }
    
    // Add constraint-specific information
    if constraintErr, ok := reason.(*ConstraintViolationError); ok {
        failure.ConstraintsFailed = constraintErr.FailedConstraints
        failure.AttemptedPosition = constraintErr.Position
    }
    
    return failure
}

// Example: Configuration validation following toolkit patterns
func (e *BasicSpawnEngine) ValidateSpawnConfig(config SpawnConfig) error {
    var errors []string
    
    // Validate entity groups
    for i, group := range config.EntityGroups {
        if group.ID == "" {
            errors = append(errors, fmt.Sprintf("entity group %d missing ID", i))
        }
        
        if group.SelectionTable == "" {
            errors = append(errors, fmt.Sprintf("entity group %s missing selection table", group.ID))
        }
        
        // Validate quantity specification
        if err := e.validateQuantitySpec(group.Quantity); err != nil {
            errors = append(errors, fmt.Sprintf("entity group %s: %v", group.ID, err))
        }
    }
    
    // Validate spatial constraints
    if err := e.validateSpatialConstraints(config.SpatialRules); err != nil {
        errors = append(errors, fmt.Sprintf("spatial constraints: %v", err))
    }
    
    if len(errors) > 0 {
        return fmt.Errorf("spawn config validation failed: %s", strings.Join(errors, "; "))
    }
    
    return nil
}
```

### Module Dependency Management

The spawn engine follows toolkit patterns for clean dependency injection and module composition.

```go
// Example: Engine configuration following toolkit patterns
type BasicSpawnEngineConfig struct {
    ID                  string                    // Engine identifier
    SpatialQueryHandler spatial.QueryHandler      // Required: spatial queries
    RoomOrchestrator    spatial.RoomOrchestrator  // Optional: multi-room support
    SelectablesRegistry SelectablesRegistry       // Required: selection tables
    EventBus           events.EventBus           // Optional: event publishing
    EntityFactory      EntityFactory             // Required: entity creation
    Configuration      SpawnEngineConfiguration  // Engine behavior settings
}

type SpawnEngineConfiguration struct {
    EnableEvents        bool    `json:"enable_events"`
    EnableDebugging     bool    `json:"enable_debugging"`
    MaxPlacementAttempts int    `json:"max_placement_attempts"`
    DefaultTimeout      int     `json:"default_timeout_seconds"`
    PerformanceMode     string  `json:"performance_mode"` // "fast", "thorough", "balanced"
}

// Example: Engine initialization
func NewBasicSpawnEngine(config BasicSpawnEngineConfig) SpawnEngine {
    if config.ID == "" {
        config.ID = generateEngineID()
    }
    
    // Set defaults
    if config.Configuration.MaxPlacementAttempts == 0 {
        config.Configuration.MaxPlacementAttempts = 100
    }
    
    engine := &BasicSpawnEngine{
        id:                  config.ID,
        spatialQueryHandler: config.SpatialQueryHandler,
        roomOrchestrator:    config.RoomOrchestrator,
        selectablesRegistry: config.SelectablesRegistry,
        eventBus:           config.EventBus,
        entityFactory:      config.EntityFactory,
        config:             config.Configuration,
        formationRegistry:  NewFormationRegistry(),
        constraintSolver:   NewConstraintSolver(),
    }
    
    return engine
}
```

## Related ADRs
- ADR-0008: Tools Directory Structure
- ADR-0009: Multi-room Orchestration
- ADR-0012: Selectables Tool Architecture

## References
- Spatial module capabilities and interfaces
- Selectables module for weighted selection
- Events module for observability patterns
- Existing toolkit patterns for configuration and error handling