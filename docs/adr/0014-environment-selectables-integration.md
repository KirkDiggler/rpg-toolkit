# ADR-0014: Environment Selectables Integration

## Status
Accepted - Phase 1 (Selectables Integration) completed

## Context

The environment package currently uses basic procedural generation with hard-coded parameters and limited room type variety. We have identified two major enhancement opportunities:

1. **Selectables Integration**: Replace hard-coded room type functions and parameter selection with the selectables module for infinite procedural variety within constraint profiles
2. **Environment Query Expansion**: Add intelligent room capacity management, feeling-based sizing, and room splitting capabilities

The current system has:
- 4 hard-coded convenience functions (`TacticalRoom`, `BossRoom`, `TreasureRoom`, `QuickRoom`)
- Fixed parameter ranges (e.g., density always 0.5 for tactical rooms)
- No intelligent space management or capacity-aware room design
- Limited room type variety despite having good underlying procedural algorithms

## Decision

We will implement both enhancements as a coordinated effort:

### Enhancement 1: Selectables Integration for Procedural Parameter Selection

**Core Principle**: Use selectables to choose **generation parameters and constraint profiles**, not replace procedural generation.

#### Parameter Range Selection
Instead of fixed values, selectables will choose parameter ranges that feed into existing algorithms:

```go
// Current: Fixed parameters
func TacticalRoom(width, height int) (spatial.Room, error) {
    return builder.WithWallDensity(0.5).WithDestructibleRatio(0.8).Build()
}

// Enhanced: Selectables-driven parameter ranges
tacticalProfile := selectables.SelectMany(ctx, []string{
    "density_range",      // Selects: DensityRange{Min: 0.4, Max: 0.7}
    "destructible_range", // Selects: DestructibleRange{Min: 0.6, Max: 0.9}
    "pattern_algorithm",  // Selects: "clustered_random" 
    "safety_profile",     // Selects: SafetyProfile{MinPathWidth: 2.5, MinOpenSpace: 0.7}
})

// Then procedurally generate within selected ranges
room := builder.
    WithWallDensity(tacticalProfile.DensityRange.Random()).     // Infinite variety
    WithDestructibleRatio(tacticalProfile.DestructibleRange.Random()). 
    WithWallPattern(tacticalProfile.PatternAlgorithm).
    WithSafety(tacticalProfile.SafetyProfile).
    Build() // Still validates pathfinding
```

#### Room Purpose-Driven Selection
```go
roomPurposeTable := selectables.NewBasicTable("room_purpose_profiles")
roomPurposeTable.AddConditionalEntry(combatProfile, 40, func(ctx) bool {
    return ctx.Get("purpose") == "combat" && ctx.Get("difficulty").(int) >= 3
})
roomPurposeTable.AddConditionalEntry(explorationProfile, 35, func(ctx) bool {
    return ctx.Get("purpose") == "exploration"
})

// Profiles define parameter ranges, not fixed values
type RoomProfile struct {
    DensityRange      Range     // {0.3-0.8} not fixed 0.5
    PatternAlgorithms []string  // ["random", "clustered"] not just "random"
    RotationMode      string    // "random", "fixed", "cardinal_only"
    SafetyProfile     SafetyProfile
}
```

#### Shape Selection by Criteria
```go
shapeSelectionTable := selectables.NewBasicTable("shapes_by_requirements")
shapeSelectionTable.AddConditionalEntry("rectangle", 60, func(ctx) bool { 
    return ctx.Get("required_connections").(int) <= 2 
})
shapeSelectionTable.AddConditionalEntry("t_shape", 40, func(ctx) bool { 
    return ctx.Get("required_connections").(int) == 3 
})
shapeSelectionTable.AddConditionalEntry("hexagon", 20, func(ctx) bool { 
    return ctx.Get("spatial_feeling") == "organic" 
})
```

### Enhancement 2: Environment Query Expansion for Intelligent Room Design

#### Feeling-Based Room Sizing
Add spatial intent profiles that translate design concepts into technical parameters:

```go
type SpatialFeeling string
const (
    SpatialFeelingTight  SpatialFeeling = "tight"   // Intimate, claustrophobic
    SpatialFeelingNormal SpatialFeeling = "normal"  // Balanced, tactical
    SpatialFeelingVast   SpatialFeeling = "vast"    // Expansive, epic
)

type SpatialIntentProfile struct {
    Feeling              SpatialFeeling
    EntityDensityTarget  float64        // Entities per unit area
    MovementFreedomIndex float64        // 0.0-1.0, how much free movement space
    VisualScopeIndex     float64        // 0.0-1.0, how far entities can see
    TacticalComplexity   float64        // 0.0-1.0, cover and positioning options
}

// Environment package provides query functions
func CalculateOptimalRoomSize(intentProfile SpatialIntentProfile, entityCount int) spatial.Dimensions
func EstimateRoomCapacity(size spatial.Dimensions, constraints CapacityConstraints) CapacityEstimate
func ShouldSplitRoom(size spatial.Dimensions, entityCount int, feeling SpatialFeeling) (bool, []RoomSplit)
```

#### Room Capacity Management with Splitting
```go
type CapacityConstraints struct {
    MaxEntitiesPerRoom    int
    MinMovementSpace      float64        // Percentage of room that must be navigable
    TargetSpatialFeeling  SpatialFeeling
    EntitySizeDistribution map[int]int   // Size -> Count
}

type CapacityEstimate struct {
    RecommendedEntityCount int
    MaxEntityCount         int
    SpatialFeelingActual   SpatialFeeling
    MovementFreedomActual  float64
    RequiresSplitting      bool
    SplitRecommendations   []RoomSplit
}

type RoomSplit struct {
    SuggestedSize        spatial.Dimensions
    ConnectionPoints     []spatial.Position
    RecommendedEntityDistribution map[string]int // RoomID -> EntityCount
}
```

#### Multi-Entity Placement Optimization
```go
type PlacementQuery struct {
    EntityCounts map[string]int  // EntityType -> Count
    PlacementRules []PlacementRule
    SpatialConstraints SpatialConstraints
}

type PlacementResult struct {
    OptimalPositions map[string][]spatial.Position  // EntityType -> Positions
    QualityScore     float64
    AlternativeLayouts []PlacementAlternative
}
```

## Integration Strategy

### Phase 1: Selectables Parameter Selection
1. Create selectables tables for parameter ranges (density, destructible ratio, safety profiles)
2. Create room purpose profiles that define parameter range sets
3. Replace hard-coded convenience functions with selectables-driven profile selection
4. Maintain all existing procedural generation and pathfinding validation

### Phase 2: Environment Query Expansion  
1. Implement feeling-based room sizing system
2. Add room capacity estimation and management
3. Implement room splitting recommendations using existing multi-room orchestration
4. Add multi-entity placement optimization queries

### Existing Infrastructure Integration

#### Selectables Module Integration
- **Context Handling**: Use `selectables.SelectionContext` for room requirements (purpose, difficulty, entity counts)
- **Table Management**: Room profile tables can be loaded from external configuration
- **Seeded Selection**: Maintains existing `PatternParams.RandomSeed` reproducibility
- **Conditional Logic**: Room constraints influence selection through context-based conditions

#### Spatial Module Integration
- **Grid Systems**: All queries respect existing grid type constraints (square, hex, gridless)
- **Room Interface**: New capacity queries extend existing `spatial.Room` interface
- **Entity Management**: Placement optimization works with existing `spatial.Placeable` entities
- **Pathfinding**: All recommendations validate against existing pathfinding requirements

#### Multi-Room Orchestration Integration
- **Room Splitting**: Leverage existing `BasicRoomOrchestrator` for entity overflow scenarios
- **Connection Management**: Use existing connection types (doors, stairs, passages, portals)
- **Entity Transitions**: Integrate with existing `MoveEntityBetweenRooms` functionality

### Benefits

#### Infinite Variety Within Constraints
- **Current**: 4 room types with fixed parameters
- **Enhanced**: Infinite room types within selectable constraint profiles
- **Maintains**: All existing procedural generation, rotation, and pathfinding validation

#### Intelligent Space Management
- **Feeling-Based Design**: Translate concepts like "goblin warren" or "royal throne room" into appropriate spatial parameters
- **Capacity Awareness**: Automatically determine optimal room sizes for entity counts
- **Overflow Handling**: Intelligent room splitting when entity counts exceed reasonable limits

#### Enhanced Developer Experience
- **Intuitive Design Control**: Use spatial feelings instead of mathematical optimization
- **Automatic Scaling**: Environment package handles technical space calculations
- **Flexible Configuration**: External table configuration for different game requirements

## Consequences

### Positive
- **Exponential Variety Increase**: From 4 room types to infinite variations within constraint profiles
- **Maintained Simplicity**: Existing builder pattern and convenience functions remain
- **Enhanced Intelligence**: Room design becomes capacity and feeling-aware
- **Better Integration**: Leverages existing selectables, spatial, and orchestration infrastructure

### Considerations
- **Complexity**: Two-phase implementation requires coordination between enhancements
- **Table Management**: Room profile tables need thoughtful design for optimal variety
- **Performance**: Capacity calculations and placement optimization need efficient algorithms
- **Testing**: More complex generation requires comprehensive test coverage

### Migration Strategy
- **Backward Compatibility**: Existing `TacticalRoom()` functions maintain same API, enhanced internally
- **Incremental Adoption**: Teams can use basic functions initially, add selectables profiles when needed
- **Configuration**: Default tables provide immediate variety, custom tables enable specialization

## Implementation Notes

### Selectables Integration Patterns
```go
// Profile-based room generation
type RoomGenerationRequest struct {
    Purpose          string                    // "combat", "exploration", "boss"
    Difficulty       int                      // 1-5 scale
    EntityCount      int                      // Expected entity count
    SpatialFeeling   SpatialFeeling          // "tight", "normal", "vast"
    RequiredConnections int                   // Number of connection points needed
    Context          map[string]interface{}   // Additional context for selection
}

func GenerateRoom(request RoomGenerationRequest) (spatial.Room, error) {
    // Select profile using selectables
    selectionCtx := selectables.NewContext()
    selectionCtx.Set("purpose", request.Purpose)
    selectionCtx.Set("difficulty", request.Difficulty)
    selectionCtx.Set("spatial_feeling", request.SpatialFeeling)
    
    profile := profileTable.Select(selectionCtx)
    
    // Use profile for procedural parameter generation
    density := profile.DensityRange.Random()
    pattern := profile.SelectPatternAlgorithm()
    
    // Determine optimal size using environment queries
    optimalSize := CalculateOptimalRoomSize(profile.SpatialIntent, request.EntityCount)
    
    // Build with procedural variety within selected constraints
    return builder.
        WithSize(int(optimalSize.Width), int(optimalSize.Height)).
        WithWallDensity(density).
        WithWallPattern(pattern).
        WithRandomRotation().
        Build()
}
```

### Environment Query Implementation
```go
// Space calculation functions
func CalculateOptimalRoomSize(profile SpatialIntentProfile, entityCount int) spatial.Dimensions {
    baseArea := float64(entityCount) / profile.EntityDensityTarget
    
    switch profile.Feeling {
    case SpatialFeelingTight:
        return spatial.Dimensions{
            Width:  math.Sqrt(baseArea * 0.8),
            Height: math.Sqrt(baseArea * 0.8),
        }
    case SpatialFeelingVast:
        return spatial.Dimensions{
            Width:  math.Sqrt(baseArea * 2.0),
            Height: math.Sqrt(baseArea * 2.0),
        }
    default: // Normal
        return spatial.Dimensions{
            Width:  math.Sqrt(baseArea),
            Height: math.Sqrt(baseArea),
        }
    }
}
```

## Implementation Decisions

### Multiple Table Selection Strategy

**Decision**: Use separate, composable tables with multiple `Select()` calls rather than complex master tables.

```go
// Separate tables for maximum modularity
densityTable := selectables.NewBasicTable[Range]("wall_density_ranges")
patternTable := selectables.NewBasicTable[string]("wall_patterns") 
safetyTable := selectables.NewBasicTable[SafetyProfile]("safety_profiles")
shapeTable := selectables.NewBasicTable[string]("room_shapes")

// Multiple selections for profile composition
func GenerateRoomProfile(ctx selectables.SelectionContext) RoomProfile {
    return RoomProfile{
        DensityRange:    densityTable.Select(ctx),
        PatternAlgorithm: patternTable.Select(ctx),
        SafetyProfile:   safetyTable.Select(ctx),
        Shape:          shapeTable.Select(ctx),
    }
}
```

**Rationale**:
- **Maximum flexibility**: Users can mix any tables they want
- **Single responsibility**: Each table does one thing well  
- **Modular composition**: Users can override just density without recreating entire profiles
- **Easier testing**: Test each aspect independently
- **Toolkit philosophy**: Provides infrastructure, doesn't dictate implementation
- **User customization**: Games can replace individual tables while keeping others

### Range-Based Parameter Selection

**Decision**: Use range types for maximum variance multiplication.

```go
type Range struct {
    Min, Max float64
}

func (r Range) Random() float64 {
    return r.Min + rand.Float64()*(r.Max-r.Min)
}

// Selectables chooses constraint profile
densityTable.AddWeightedEntry(Range{0.3, 0.6}, 40) // Light walls profile
densityTable.AddWeightedEntry(Range{0.5, 0.8}, 30) // Medium walls profile  
densityTable.AddWeightedEntry(Range{0.7, 0.9}, 30) // Heavy walls profile

// Each generation gets procedural value within selected constraints
selectedRange := densityTable.Select(ctx)     // Might select {0.5, 0.8}
actualDensity := selectedRange.Random()       // Might get 0.67, then 0.52, then 0.78...
```

**Variance Multiplication Benefits**:
- **Selectables level**: Choose from different constraint profiles (light, medium, heavy)
- **Procedural level**: Random value within chosen constraint range
- **Result**: Same "medium walls" profile generates infinite density variations
- **Maintains intent**: Room feels "medium density" but each generation is unique

### Table Organization and Management

**Decision**: Code-based default tables with complete user customization capability.

```go
// Default tables provided by environment package
func GetDefaultDensityTable() selectables.Table[Range] {
    table := selectables.NewBasicTable[Range]("default_wall_density")
    table.AddWeightedEntry(Range{0.2, 0.4}, 25) // Sparse
    table.AddWeightedEntry(Range{0.4, 0.6}, 35) // Light
    table.AddWeightedEntry(Range{0.5, 0.7}, 25) // Medium
    table.AddWeightedEntry(Range{0.7, 0.9}, 15) // Heavy
    return table
}

// User customization - complete replacement
func CustomTacticalDensityTable() selectables.Table[Range] {
    table := selectables.NewBasicTable[Range]("tactical_density")
    table.AddConditionalEntry(Range{0.6, 0.8}, 50, func(ctx) bool {
        return ctx.Get("difficulty").(int) >= 3
    })
    table.AddConditionalEntry(Range{0.3, 0.5}, 50, func(ctx) bool {
        return ctx.Get("difficulty").(int) < 3
    })
    return table
}

// Room builder accepts custom tables
builder := NewBasicRoomBuilder(BasicRoomBuilderConfig{
    DensityTable:  CustomTacticalDensityTable(), // Override density
    PatternTable:  GetDefaultPatternTable(),     // Use default pattern
    // ...
})
```

**Benefits**:
- **Immediate usability**: Default tables provide variety out of the box
- **Complete control**: Users can hand-craft any table for specific game needs
- **Incremental customization**: Override only the tables you care about
- **External configuration**: Tables can be built from config files when needed

### Context Strategy

**Decision**: Use `selectables.SelectionContext` for room generation criteria, separate from Go `context.Context`.

```go
// Selection context carries room generation criteria
func GenerateRoom(request RoomGenerationRequest) (spatial.Room, error) {
    // Use selectables.SelectionContext for game data
    selectionCtx := selectables.NewContext()
    selectionCtx.Set("purpose", request.Purpose)           // "combat", "exploration"
    selectionCtx.Set("difficulty", request.Difficulty)     // 1-5 scale
    selectionCtx.Set("player_count", request.EntityCount)  // Expected entities
    selectionCtx.Set("spatial_feeling", request.SpatialFeeling) // "tight", "normal", "vast"
    
    // Go context for cancellation/timeouts if needed
    ctx := context.Background()
    
    // Generate profile using selection context
    profile := GenerateRoomProfile(selectionCtx)
    
    // Use profile for room creation
    return builder.WithProfile(profile).Build()
}
```

**Rationale**:
- **Clear separation**: Game data (selectables context) vs system concerns (Go context)
- **Rich criteria**: Selection context can carry complex room requirements
- **Conditional logic**: Tables can use context for sophisticated selection rules
- **Performance**: Selection context optimized for frequent key-value access

### Type Definitions for Implementation

```go
// Core range type for parameter variance
type Range struct {
    Min, Max float64
}

func (r Range) Random() float64 {
    return r.Min + rand.Float64()*(r.Max-r.Min)
}

func (r Range) Contains(value float64) bool {
    return value >= r.Min && value <= r.Max
}

// Room generation profile composed from multiple table selections
type RoomProfile struct {
    DensityRange      Range         // Wall density constraints
    DestructibleRange Range         // Destructible wall ratio constraints
    PatternAlgorithm  string        // "random", "clustered", "sparse"
    Shape            string         // "rectangle", "square", "hexagon", "t_shape"
    RotationMode     string         // "random", "fixed", "cardinal_only"
    SafetyProfile    SafetyProfile  // Path safety requirements
}

// Room generation request with all criteria
type RoomGenerationRequest struct {
    Purpose             string                    // "combat", "exploration", "boss"
    Difficulty          int                      // 1-5 scale influences constraints
    EntityCount         int                      // Expected entity count for sizing
    SpatialFeeling      SpatialFeeling          // "tight", "normal", "vast"
    RequiredConnections int                      // Number of connection points needed
    CustomTables        map[string]selectables.Table[any] // Optional table overrides
    Context             map[string]interface{}   // Additional context for selection
}
```

## Design Evolution During Implementation

### Constraint Profile Refinement

**Issue Identified**: Initial room type approach (`TacticalRoom`, `BossRoom`, `TreasureRoom`) was prescriptive and violated toolkit philosophy of "infrastructure, not implementation."

**Resolution**: Evolved to **generic constraint profiles** based on wall density patterns rather than assumed use cases:

#### Removed Room Types
- **TreasureRoom**: Removed entirely. Treasure aspect is about contents/features, not wall layouts. Games determine if a room contains treasure and what to spawn in it.
- **BossRoom**: Removed as distinct type. Games decide if boss encounters need dense cover, sparse cover, or empty arenas.

#### New Generic Constraint Profiles
```go
// Generic profiles based on wall density characteristics
func GetDenseCoverTables() RoomTables        // High wall density (0.6-0.9 range)
func GetSparseCoverTables() RoomTables       // Low wall density (0.1-0.4 range)  
func GetBalancedCoverTables() RoomTables     // Medium wall density (0.4-0.7 range)
func GetDefaultRoomTables() RoomTables       // Full range variety

// Corresponding room generation functions
func DenseCoverRoom(width, height int) (spatial.Room, error)
func SparseCoverRoom(width, height int) (spatial.Room, error)
func BalancedCoverRoom(width, height int) (spatial.Room, error)
func QuickRoom(width, height int, pattern string) (spatial.Room, error) // Direct control
```

#### Game-Driven Use Case Mapping
Games determine appropriate constraint profiles for their use cases:
- **Boss fights**: `DenseCoverRoom()` for tactical positioning OR `QuickRoom(w, h, "empty")` for arena fights
- **Combat encounters**: `BalancedCoverRoom()` or `DenseCoverRoom()` based on desired complexity
- **Exploration**: `SparseCoverRoom()` for easy movement or `BalancedCoverRoom()` for discovery opportunities
- **Treasure areas**: Any profile - treasure is about contents, not layout

**Benefits**:
- **Toolkit Philosophy Alignment**: Provides infrastructure (constraint profiles), games provide implementation (use case decisions)
- **Maximum Flexibility**: Games can map any use case to any constraint profile
- **Clearer Intent**: Profiles describe what they generate (dense/sparse/balanced cover) not assumed usage
- **Reduced Assumptions**: No hardcoded "tactical" or "boss" characteristics

### Implementation Notes Updates

```go
// Generic constraint-based room generation
func GenerateRoom(coverType string, width, height int) (spatial.Room, error) {
    var tables RoomTables
    switch coverType {
    case "dense":
        tables = GetDenseCoverTables()
    case "sparse": 
        tables = GetSparseCoverTables()
    case "balanced":
        tables = GetBalancedCoverTables()
    default:
        tables = GetDefaultRoomTables()
    }
    
    // Use selectables for parameter selection within chosen constraints
    ctx := selectables.NewBasicSelectionContext()
    densityRange := tables.DensityTable.Select(ctx)
    // ... rest of generation logic
}
```

This design evolution occurred during spawn module planning when we recognized that room generation should provide generic infrastructure rather than assuming specific game use cases.

This ADR documents both enhancements as a coordinated effort to transform the environment package from limited hard-coded room types to an intelligent, infinitely variable room generation system while maintaining all existing procedural generation capabilities and pathfinding validation.