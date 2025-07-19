package environments

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/tools/selectables"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// Pattern constants
const (
	PatternEmpty   = "empty"
	PatternRandom  = "random"
	ShapeRectangle = "rectangle"
	ShapeSquare    = "square"
	ThemeDefault   = "default"
)

// BasicRoomBuilder implements the RoomBuilder interface
// Purpose: Provides a fluent API for building rooms with shapes, patterns, and walls
type BasicRoomBuilder struct {
	// Configuration
	shape          *RoomShape
	size           spatial.Dimensions
	pattern        string
	patternParams  PatternParams
	theme          string
	features       []Feature
	rotation       int  // Rotation angle in degrees (0, 90, 180, 270)
	randomRotation bool // Whether to apply random rotation

	// Dependencies
	eventBus    events.EventBus
	shapeLoader *ShapeLoader

	// State
	built bool
}

// BasicRoomBuilderConfig configures the room builder
type BasicRoomBuilderConfig struct {
	EventBus    events.EventBus
	ShapeLoader *ShapeLoader
}

// NewBasicRoomBuilder creates a new room builder
func NewBasicRoomBuilder(config BasicRoomBuilderConfig) *BasicRoomBuilder {
	return &BasicRoomBuilder{
		eventBus:    config.EventBus,
		shapeLoader: config.ShapeLoader,
		pattern:     PatternEmpty, // Default pattern
		patternParams: PatternParams{
			Density:           0.4,
			DestructibleRatio: 0.7,
			Safety: PathSafetyParams{
				MinPathWidth:      2.0,
				MinOpenSpace:      0.6,
				EntitySize:        1.0,
				EmergencyFallback: true,
			},
			Material:   "stone",
			WallHeight: 3.0,
		},
		theme: ThemeDefault,
	}
}

// RoomBuilder interface implementation

// WithSize sets the room size
// It accepts width and height in pixels
func (b *BasicRoomBuilder) WithSize(width, height int) RoomBuilder {
	b.size = spatial.Dimensions{Width: float64(width), Height: float64(height)}
	return b
}

// WithTheme sets the room theme for styling and generation context.
// It accepts a RoomShape object or a shape name to load from the shape loader
func (b *BasicRoomBuilder) WithTheme(theme string) RoomBuilder {
	b.theme = theme
	return b
}

// WithFeatures adds features to the room
// Features can be any spatial.Placeable entity like furniture, decorations, etc.
// It accepts a variadic list of Feature objects
func (b *BasicRoomBuilder) WithFeatures(features ...Feature) RoomBuilder {
	b.features = append(b.features, features...)
	return b
}

// WithLayout sets the room layout
// It accepts a Layout object which defines the layout type and parameters
// The layout type can be linear, branching, grid, or organic
func (b *BasicRoomBuilder) WithLayout(layout Layout) RoomBuilder {
	// Convert layout to appropriate density and pattern
	// This is a simplified implementation
	switch layout.Type {
	case LayoutTypeLinear:
		b.pattern = PatternEmpty
	case LayoutTypeBranching:
		b.pattern = PatternRandom
		b.patternParams.Density = 0.3
	case LayoutTypeGrid:
		b.pattern = PatternRandom
		b.patternParams.Density = 0.5
	case LayoutTypeOrganic:
		b.pattern = PatternRandom
		b.patternParams.Density = 0.4
	default:
		b.pattern = PatternEmpty
	}
	return b
}

// WithPrefab applies a room prefab configuration
// It accepts a RoomPrefab object which contains predefined settings for size, theme, and features
func (b *BasicRoomBuilder) WithPrefab(prefab RoomPrefab) RoomBuilder {
	// Convert prefab to builder configuration
	b.size = prefab.Size
	b.theme = prefab.Theme
	b.features = prefab.Features

	// Set shape based on prefab
	if b.shapeLoader != nil {
		shape, err := b.shapeLoader.LoadShape(prefab.Name)
		if err == nil {
			b.shape = shape
		}
	}

	return b
}

// Build finalizes the room configuration and generates the room
func (b *BasicRoomBuilder) Build() (spatial.Room, error) {
	if b.built {
		return nil, fmt.Errorf("room builder can only be used once")
	}
	b.built = true

	// Validate configuration
	if err := b.validate(); err != nil {
		return nil, fmt.Errorf("invalid room configuration: %w", err)
	}

	// Load shape if not already set
	if b.shape == nil {
		if err := b.loadDefaultShape(); err != nil {
			return nil, fmt.Errorf("failed to load room shape: %w", err)
		}
	}

	// Apply rotation if needed
	rotatedShape := b.shape
	if b.randomRotation {
		// Generate random rotation using seeded random if available
		// Note: Using math/rand (not crypto/rand) for deterministic game generation
		// This allows reproducible room layouts when using the same seed
		var rng *rand.Rand
		if b.patternParams.RandomSeed != 0 {
			//nolint:gosec // G404: Deterministic game generation, not cryptographic
			rng = rand.New(rand.NewSource(b.patternParams.RandomSeed))
		} else {
			//nolint:gosec // G404: Deterministic game generation, not cryptographic
			rng = rand.New(rand.NewSource(time.Now().UnixNano()))
		}
		// Random rotation: 0°, 90°, 180°, or 270°
		rotations := []int{0, 90, 180, 270}
		randomRotation := rotations[rng.Intn(len(rotations))]
		rotatedShape = RotateShape(b.shape, randomRotation)
	} else if b.rotation != 0 {
		// Apply specific rotation
		rotatedShape = RotateShape(b.shape, b.rotation)
	}

	// Scale shape to size
	scaledShape := ScaleShape(rotatedShape, b.size)

	// Generate wall pattern
	walls, err := b.generateWalls(context.Background(), scaledShape)
	if err != nil {
		return nil, fmt.Errorf("failed to generate walls: %w", err)
	}

	// Create spatial room
	room, err := b.createSpatialRoom(scaledShape, walls)
	if err != nil {
		return nil, fmt.Errorf("failed to create spatial room: %w", err)
	}

	// Place features
	if err := b.placeFeatures(room, scaledShape); err != nil {
		return nil, fmt.Errorf("failed to place features: %w", err)
	}

	return room, nil
}

// Extended builder API for wall patterns

// WithWallPattern sets the wall pattern
func (b *BasicRoomBuilder) WithWallPattern(pattern string) RoomBuilder {
	b.pattern = pattern
	return b
}

// WithWallDensity sets the wall density
func (b *BasicRoomBuilder) WithWallDensity(density float64) RoomBuilder {
	b.patternParams.Density = density
	return b
}

// WithDestructibleRatio sets the destructible wall ratio
func (b *BasicRoomBuilder) WithDestructibleRatio(ratio float64) RoomBuilder {
	b.patternParams.DestructibleRatio = ratio
	return b
}

// WithSafety sets the path safety parameters
func (b *BasicRoomBuilder) WithSafety(safety PathSafetyParams) RoomBuilder {
	b.patternParams.Safety = safety
	return b
}

// WithMaterial sets the wall material
func (b *BasicRoomBuilder) WithMaterial(material string) RoomBuilder {
	b.patternParams.Material = material
	return b
}

// WithShape sets the room shape by name
func (b *BasicRoomBuilder) WithShape(shapeName string) RoomBuilder {
	if b.shapeLoader != nil {
		shape, err := b.shapeLoader.LoadShape(shapeName)
		if err == nil {
			b.shape = shape
		}
	}
	return b
}

// WithRotation sets a specific rotation angle for the room shape
// Accepts angles in degrees: 0, 90, 180, 270
func (b *BasicRoomBuilder) WithRotation(degrees int) RoomBuilder {
	// Normalize angle to 0-360 range and snap to 90-degree increments
	normalizedDegrees := degrees % 360
	if normalizedDegrees < 0 {
		normalizedDegrees += 360
	}

	// Snap to nearest 90-degree increment
	b.rotation = (normalizedDegrees / 90) * 90
	b.randomRotation = false // Explicit rotation overrides random
	return b
}

// WithRandomRotation enables random rotation for the room shape
// The rotation will be randomly selected from [0, 90, 180, 270] degrees
// Uses math/rand for reproducible generation when combined with WithRandomSeed
func (b *BasicRoomBuilder) WithRandomRotation() RoomBuilder {
	b.randomRotation = true
	return b
}

// WithRandomSeed sets the random seed for reproducible generation
func (b *BasicRoomBuilder) WithRandomSeed(seed int64) RoomBuilder {
	b.patternParams.RandomSeed = seed
	return b
}

// Private methods

func (b *BasicRoomBuilder) validate() error {
	if b.size.Width <= 0 || b.size.Height <= 0 {
		return fmt.Errorf("room size must be positive (got %.0fx%.0f)", b.size.Width, b.size.Height)
	}

	if b.patternParams.Density < 0 || b.patternParams.Density > 1 {
		return fmt.Errorf("wall density must be between 0 and 1 (got %f)", b.patternParams.Density)
	}

	if b.patternParams.DestructibleRatio < 0 || b.patternParams.DestructibleRatio > 1 {
		return fmt.Errorf("destructible ratio must be between 0 and 1 (got %f)", b.patternParams.DestructibleRatio)
	}

	return nil
}

func (b *BasicRoomBuilder) loadDefaultShape() error {
	// Default to rectangle shape
	shapeName := ShapeRectangle

	// Try to load from shape loader
	if b.shapeLoader != nil {
		shape, err := b.shapeLoader.LoadShape(shapeName)
		if err == nil {
			b.shape = shape
			return nil
		}
	}

	// Fallback to default shapes
	defaultShapes := GetDefaultShapes()
	if shape, exists := defaultShapes[shapeName]; exists {
		b.shape = shape
		return nil
	}

	return fmt.Errorf("no default shape available")
}

func (b *BasicRoomBuilder) generateWalls(ctx context.Context, shape *RoomShape) ([]WallSegment, error) {
	// Get pattern function
	patternFunc, exists := WallPatterns[b.pattern]
	if !exists {
		return nil, fmt.Errorf("unknown wall pattern: %s", b.pattern)
	}

	// Configure required paths based on shape connections
	b.patternParams.Safety.RequiredPaths = b.createRequiredPaths(shape)

	// Pass event bus for emergency fallback notifications
	b.patternParams.EventBus = b.eventBus

	// Generate walls
	walls, err := patternFunc(ctx, shape, b.size, b.patternParams)
	if err != nil {
		return nil, fmt.Errorf("pattern generation failed: %w", err)
	}

	return walls, nil
}

func (b *BasicRoomBuilder) createRequiredPaths(shape *RoomShape) []Path {
	var paths []Path

	connections := shape.Connections
	if len(connections) >= 2 {
		// Create path between first two connections
		path := Path{
			From:    connections[0].Position,
			To:      connections[1].Position,
			Width:   b.patternParams.Safety.MinPathWidth,
			Purpose: "connection_path",
		}
		paths = append(paths, path)
	}

	return paths
}

func (b *BasicRoomBuilder) createSpatialRoom(_ *RoomShape, walls []WallSegment) (spatial.Room, error) {
	// Create a grid for the room
	grid := spatial.NewSquareGrid(spatial.SquareGridConfig{
		Width:  b.size.Width,
		Height: b.size.Height,
	})

	// Create spatial room configuration
	roomConfig := spatial.BasicRoomConfig{
		ID:       fmt.Sprintf("room_%s_%.0f_%.0f", b.theme, b.size.Width, b.size.Height),
		Type:     "generated_room",
		Grid:     grid,
		EventBus: b.eventBus,
	}

	// Create basic spatial room
	room := spatial.NewBasicRoom(roomConfig)

	// Convert wall segments to wall entities and place them
	wallEntities := CreateWallEntities(walls)
	for _, wallEntity := range wallEntities {
		entity := wallEntity.(*WallEntity)
		err := room.PlaceEntity(entity, entity.GetPosition())
		if err != nil {
			return nil, fmt.Errorf("failed to place wall entity %s: %w", entity.GetID(), err)
		}
	}

	return room, nil
}

func (b *BasicRoomBuilder) placeFeatures(room spatial.Room, _ *RoomShape) error {
	// Place features in the room
	for i, feature := range b.features {
		featureEntity := b.createFeatureEntity(feature, i)

		// Determine feature position
		var position spatial.Position
		if feature.Position != nil {
			position = *feature.Position
		} else {
			// Place at center if no position specified
			position = spatial.Position{
				X: float64(b.size.Width) / 2.0,
				Y: float64(b.size.Height) / 2.0,
			}
		}

		// Place feature entity
		err := room.PlaceEntity(featureEntity, position)
		if err != nil {
			return fmt.Errorf("failed to place feature %s: %w", feature.Name, err)
		}
	}

	return nil
}

func (b *BasicRoomBuilder) createFeatureEntity(feature Feature, index int) spatial.Placeable {
	return &FeatureEntity{
		id:          fmt.Sprintf("feature_%d_%s", index, feature.Type),
		featureType: feature.Type,
		name:        feature.Name,
		properties:  feature.Properties,
	}
}

// FeatureEntity represents a room feature as a spatial entity
type FeatureEntity struct {
	id          string
	featureType string
	name        string
	properties  map[string]interface{}
}

// GetID returns the unique ID of this feature entity
func (f *FeatureEntity) GetID() string { return f.id }

// GetType returns the type of this feature entity
func (f *FeatureEntity) GetType() string { return f.featureType }

// GetSize returns the size of this feature entity
func (f *FeatureEntity) GetSize() int { return 1 }

// BlocksMovement checks if this feature blocks movement
func (f *FeatureEntity) BlocksMovement() bool { return false } // Features don't block movement by default
// BlocksLineOfSight checks if this feature blocks line of sight
func (f *FeatureEntity) BlocksLineOfSight() bool { return false } // Features don't block LOS by default

// Convenience functions

// QuickRoom creates a room with sensible defaults
func QuickRoom(width, height int, pattern string) (spatial.Room, error) {
	builder := NewBasicRoomBuilder(BasicRoomBuilderConfig{
		ShapeLoader: NewShapeLoader("tools/environments/shapes"),
	})

	return builder.
		WithSize(width, height).
		WithWallPattern(pattern).
		Build()
}

// buildRoomWithTables is a helper function that reduces duplication across room generation functions
func buildRoomWithTables(width, height int, tables RoomTables) (spatial.Room, error) {
	builder := NewBasicRoomBuilder(BasicRoomBuilderConfig{
		ShapeLoader: NewShapeLoader("tools/environments/shapes"),
	})

	// Use selectables for parameter selection
	ctx := selectables.NewBasicSelectionContext()

	// Select parameters using provided tables
	densityRange, err := tables.DensityTable.Select(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to select density range: %w", err)
	}

	destructibleRange, err := tables.DestructibleTable.Select(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to select destructible range: %w", err)
	}

	pattern, err := tables.PatternTable.Select(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to select wall pattern: %w", err)
	}

	shape, err := tables.ShapeTable.Select(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to select room shape: %w", err)
	}

	safetyProfile, err := tables.SafetyTable.Select(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to select safety profile: %w", err)
	}

	// Build room with selectables-driven parameters
	return builder.
		WithSize(width, height).
		WithWallPattern(pattern).
		WithDestructibleRatio(destructibleRange.Random()).
		WithWallDensity(densityRange.Random()).
		WithShape(shape).
		WithSafety(safetyProfile.ToPathSafetyParams()).
		Build()
}

// DenseCoverRoom creates a room with high wall density using selectables
// for infinite variety within dense cover constraints (0.6-0.9 density range)
func DenseCoverRoom(width, height int) (spatial.Room, error) {
	return buildRoomWithTables(width, height, GetDenseCoverTables())
}

// SparseCoverRoom creates a room with low wall density using selectables
// for infinite variety within sparse cover constraints (0.1-0.4 density range)
func SparseCoverRoom(width, height int) (spatial.Room, error) {
	return buildRoomWithTables(width, height, GetSparseCoverTables())
}

// BalancedCoverRoom creates a room with medium wall density using selectables
// for infinite variety within balanced cover constraints (0.4-0.7 density range)
func BalancedCoverRoom(width, height int) (spatial.Room, error) {
	return buildRoomWithTables(width, height, GetBalancedCoverTables())
}
