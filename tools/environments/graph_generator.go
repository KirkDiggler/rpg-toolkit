package environments

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// Room type constants
const (
	RoomTypeEntrance = "entrance"
	RoomTypeBoss     = "boss"
	RoomTypeCorridor = "corridor"
	RoomTypeTreasure = "treasure"
	RoomTypeTrap     = "trap"
	RoomTypeExit     = "exit"
	RoomTypeChamber  = "chamber"
	RoomTypeJunction = "junction"
)

// GraphBasedGenerator implements environment generation using graph algorithms
// Purpose: Creates environments by first building abstract graphs of rooms and
// connections, then placing them spatially. This provides the flexibility
// prioritized in ADR-0011 while leveraging existing spatial infrastructure.
type GraphBasedGenerator struct {
	// Core identity
	id  string
	typ string

	// Dependencies - we are clients of these systems
	eventBus     events.EventBus
	spatialQuery *spatial.QueryUtils

	// Graph generation state
	random       *rand.Rand
	capabilities GeneratorCapabilities

	// Component factories for custom room types
	roomFactories map[string]ComponentFactory

	// Thread safety
	mutex sync.RWMutex
}

// GraphBasedGeneratorConfig follows toolkit config pattern
type GraphBasedGeneratorConfig struct {
	ID            string                      `json:"id"`
	Type          string                      `json:"type"`
	EventBus      events.EventBus             `json:"-"`
	SpatialQuery  *spatial.QueryUtils         `json:"-"`
	Seed          int64                       `json:"seed"`
	RoomFactories map[string]ComponentFactory `json:"-"`
}

// NewGraphBasedGenerator creates a new graph-based environment generator
func NewGraphBasedGenerator(config GraphBasedGeneratorConfig) *GraphBasedGenerator {
	seed := config.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}

	generator := &GraphBasedGenerator{
		id:           config.ID,
		typ:          config.Type,
		eventBus:     config.EventBus,
		spatialQuery: config.SpatialQuery,
		// #nosec G404 - Using math/rand for seeded, reproducible procedural generation
		// Same seed must produce identical environments for gameplay consistency
		random:        rand.New(rand.NewSource(seed)),
		roomFactories: config.RoomFactories,
		capabilities: GeneratorCapabilities{
			SupportedTypes:   []GenerationType{GenerationTypeGraph, GenerationTypeHybrid},
			SupportedLayouts: []LayoutType{LayoutTypeLinear, LayoutTypeBranching, LayoutTypeGrid, LayoutTypeOrganic},
			SupportedSizes: []EnvironmentSize{
				EnvironmentSizeSmall, EnvironmentSizeMedium, EnvironmentSizeLarge, EnvironmentSizeCustom,
			},
			MaxRoomCount:        200, // Technical limit for graph-based generation
			SupportsConstraints: true,
			SupportsCustomRooms: true,
		},
	}

	if generator.roomFactories == nil {
		generator.roomFactories = make(map[string]ComponentFactory)
	}

	return generator
}

// EnvironmentGenerator interface implementation

// GetID returns the unique identifier of the generator.
func (g *GraphBasedGenerator) GetID() string {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.id
}

// GetType returns the type of the generator.
func (g *GraphBasedGenerator) GetType() string {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.typ
}

// Generate creates a new environment based on the provided configuration.
func (g *GraphBasedGenerator) Generate(ctx context.Context, config GenerationConfig) (Environment, error) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	// Validate configuration first
	if err := g.validateUnsafe(config); err != nil {
		return nil, fmt.Errorf("invalid generation config: %w", err)
	}

	// Publish generation started event
	if g.eventBus != nil {
		event := events.NewGameEvent(EventGenerationStarted, g, nil)
		event.Context().Set("config", config)
		_ = g.eventBus.Publish(ctx, event)
	}

	// Set random seed for reproducible generation
	if config.Seed != 0 {
		g.random.Seed(config.Seed)
	}

	// Step 1: Generate abstract room graph
	roomGraph, err := g.generateRoomGraphUnsafe(ctx, config)
	if err != nil {
		g.publishGenerationFailedUnsafe(ctx, err, "room graph generation failed")
		return nil, fmt.Errorf("failed to generate room graph: %w", err)
	}

	// Step 2: Create spatial orchestrator for this environment
	orchestrator := g.createOrchestratorUnsafe(config)

	// Step 3: Place rooms spatially using the graph
	if err := g.placeRoomsSpatiallyUnsafe(ctx, roomGraph, orchestrator, config); err != nil {
		g.publishGenerationFailedUnsafe(ctx, err, "spatial placement failed")
		return nil, fmt.Errorf("failed to place rooms spatially: %w", err)
	}

	// Step 4: Create connections based on graph relationships
	if err := g.createConnectionsUnsafe(roomGraph, orchestrator, config); err != nil {
		g.publishGenerationFailedUnsafe(ctx, err, "connection creation failed")
		return nil, fmt.Errorf("failed to create connections: %w", err)
	}

	// Step 5: Create environment wrapper
	environment := g.createEnvironmentUnsafe(orchestrator, config)

	// Publish generation completed event
	if g.eventBus != nil {
		event := events.NewGameEvent(EventGenerationCompleted, g, environment)
		event.Context().Set("config", config)
		event.Context().Set("room_count", len(roomGraph.nodes))
		event.Context().Set("connection_count", len(roomGraph.edges))
		_ = g.eventBus.Publish(ctx, event)
	}

	return environment, nil
}

// GetGenerationType returns the type of generation this generator performs.
func (g *GraphBasedGenerator) GetGenerationType() GenerationType {
	return GenerationTypeGraph
}

// Validate checks if the provided configuration is valid for this generator.
func (g *GraphBasedGenerator) Validate(config GenerationConfig) error {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.validateUnsafe(config)
}

// GetCapabilities returns the capabilities of this generator.
func (g *GraphBasedGenerator) GetCapabilities() GeneratorCapabilities {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.capabilities
}

// Graph generation core logic

// RoomNode represents a room in the abstract graph
type RoomNode struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Theme      string                 `json:"theme"`
	Size       spatial.Dimensions     `json:"size"`
	Features   []Feature              `json:"features"`
	Properties map[string]interface{} `json:"properties"`
	Position   *spatial.Position      `json:"position,omitempty"` // Set during spatial placement
}

// ConnectionEdge represents a connection in the abstract graph
type ConnectionEdge struct {
	ID            string  `json:"id"`
	FromRoomID    string  `json:"from_room_id"`
	ToRoomID      string  `json:"to_room_id"`
	Type          string  `json:"type"`
	Bidirectional bool    `json:"bidirectional"`
	Cost          float64 `json:"cost"`
	Required      bool    `json:"required"`
}

// RoomGraph represents the abstract graph structure
type RoomGraph struct {
	nodes map[string]*RoomNode
	edges map[string]*ConnectionEdge
	// Adjacency list for graph algorithms
	adjacency map[string][]string
}

func (g *GraphBasedGenerator) generateRoomGraphUnsafe(
	ctx context.Context, config GenerationConfig,
) (*RoomGraph, error) {
	// Create the graph structure
	graph := &RoomGraph{
		nodes:     make(map[string]*RoomNode),
		edges:     make(map[string]*ConnectionEdge),
		adjacency: make(map[string][]string),
	}

	// Determine room count based on size
	roomCount := g.determineRoomCountUnsafe(config)

	// Generate rooms based on layout pattern
	switch config.Layout {
	case LayoutTypeLinear:
		return g.generateLinearLayoutUnsafe(ctx, graph, roomCount, config)
	case LayoutTypeBranching:
		return g.generateBranchingLayoutUnsafe(ctx, graph, roomCount, config)
	case LayoutTypeGrid:
		return g.generateGridLayoutUnsafe(ctx, graph, roomCount, config)
	case LayoutTypeOrganic:
		return g.generateOrganicLayoutUnsafe(ctx, graph, roomCount, config)
	default:
		return nil, fmt.Errorf("unsupported layout type: %v", config.Layout)
	}
}

func (g *GraphBasedGenerator) determineRoomCountUnsafe(config GenerationConfig) int {
	// Use explicit room count if provided
	if config.RoomCount > 0 {
		return config.RoomCount
	}

	// Otherwise use size-based defaults
	switch config.Size {
	case EnvironmentSizeSmall:
		return 5 + g.random.Intn(11) // 5-15 rooms
	case EnvironmentSizeMedium:
		return 15 + g.random.Intn(36) // 15-50 rooms
	case EnvironmentSizeLarge:
		return 50 + g.random.Intn(101) // 50-150 rooms
	default:
		return 10 // Safe default
	}
}

func (g *GraphBasedGenerator) generateLinearLayoutUnsafe(
	ctx context.Context, graph *RoomGraph, roomCount int, config GenerationConfig,
) (*RoomGraph, error) {
	// Linear layout: rooms connected in sequence (classic dungeon crawl)
	var previousRoomID string

	for i := 0; i < roomCount; i++ {
		// Create room node
		roomID := fmt.Sprintf("room_%d", i)
		roomType := g.selectRoomTypeUnsafe(i, roomCount, config)

		room := &RoomNode{
			ID:         roomID,
			Type:       roomType,
			Theme:      config.Theme,
			Size:       g.calculateRoomSizeUnsafe(roomType, config),
			Features:   g.generateRoomFeaturesUnsafe(roomType, config),
			Properties: make(map[string]interface{}),
		}

		graph.nodes[roomID] = room
		graph.adjacency[roomID] = make([]string, 0)

		// Connect to previous room
		if previousRoomID != "" {
			connectionID := fmt.Sprintf("conn_%s_%s", previousRoomID, roomID)
			edge := &ConnectionEdge{
				ID:            connectionID,
				FromRoomID:    previousRoomID,
				ToRoomID:      roomID,
				Type:          "passage",
				Bidirectional: true,
				Cost:          1.0,
				Required:      true,
			}

			graph.edges[connectionID] = edge
			graph.adjacency[previousRoomID] = append(graph.adjacency[previousRoomID], roomID)
			graph.adjacency[roomID] = append(graph.adjacency[roomID], previousRoomID)
		}

		previousRoomID = roomID

		// Publish progress
		g.publishGenerationProgressUnsafe(ctx, float64(i+1)/float64(roomCount), "generating linear layout")
	}

	return graph, nil
}

func (g *GraphBasedGenerator) generateBranchingLayoutUnsafe(
	ctx context.Context, graph *RoomGraph, roomCount int, config GenerationConfig,
) (*RoomGraph, error) {
	// Branching layout: central hub with branches extending outward
	if roomCount < 2 {
		return nil, fmt.Errorf("branching layout requires at least 2 rooms")
	}

	// Create central hub room
	hubID := "hub_room"
	hubRoom := &RoomNode{
		ID:         hubID,
		Type:       "hub",
		Theme:      config.Theme,
		Size:       spatial.Dimensions{Width: 20, Height: 20}, // Larger hub room
		Features:   g.generateRoomFeaturesUnsafe("hub", config),
		Properties: map[string]interface{}{"is_hub": true},
	}

	graph.nodes[hubID] = hubRoom
	graph.adjacency[hubID] = make([]string, 0)

	// Create branches extending from hub
	remainingRooms := roomCount - 1
	branchCount := 3 + g.random.Intn(3) // 3-5 branches
	if branchCount > remainingRooms {
		branchCount = remainingRooms
	}

	roomsPerBranch := remainingRooms / branchCount
	extraRooms := remainingRooms % branchCount

	for branchIdx := 0; branchIdx < branchCount; branchIdx++ {
		branchSize := roomsPerBranch
		if branchIdx < extraRooms {
			branchSize++
		}

		g.createBranchUnsafe(graph, hubID, branchIdx, branchSize, config)

		progress := float64(branchIdx+1) / float64(branchCount)
		g.publishGenerationProgressUnsafe(ctx, progress, "generating branching layout")
	}

	return graph, nil
}

func (g *GraphBasedGenerator) createBranchUnsafe(
	graph *RoomGraph, hubID string, branchIdx, branchSize int, config GenerationConfig,
) {
	var previousRoomID = hubID

	for i := 0; i < branchSize; i++ {
		// Create room in branch
		roomID := fmt.Sprintf("branch_%d_room_%d", branchIdx, i)
		roomType := g.selectRoomTypeUnsafe(i, branchSize, config)

		room := &RoomNode{
			ID:         roomID,
			Type:       roomType,
			Theme:      config.Theme,
			Size:       g.calculateRoomSizeUnsafe(roomType, config),
			Features:   g.generateRoomFeaturesUnsafe(roomType, config),
			Properties: map[string]interface{}{"branch": branchIdx, "branch_position": i},
		}

		graph.nodes[roomID] = room
		graph.adjacency[roomID] = make([]string, 0)

		// Connect to previous room in branch
		connectionID := fmt.Sprintf("conn_%s_%s", previousRoomID, roomID)
		edge := &ConnectionEdge{
			ID:            connectionID,
			FromRoomID:    previousRoomID,
			ToRoomID:      roomID,
			Type:          "passage",
			Bidirectional: true,
			Cost:          1.0,
			Required:      true,
		}

		graph.edges[connectionID] = edge
		graph.adjacency[previousRoomID] = append(graph.adjacency[previousRoomID], roomID)
		graph.adjacency[roomID] = append(graph.adjacency[roomID], previousRoomID)

		previousRoomID = roomID
	}
}

// Simplified implementations for other layouts
func (g *GraphBasedGenerator) generateGridLayoutUnsafe(
	ctx context.Context, graph *RoomGraph, roomCount int, config GenerationConfig,
) (*RoomGraph, error) {
	// Grid layout: rooms arranged in a rectangular grid
	gridSize := int(float64(roomCount)*0.7) + 1 // Approximate square grid

	for i := 0; i < roomCount; i++ {
		x := i % gridSize
		y := i / gridSize

		roomID := fmt.Sprintf("grid_%d_%d", x, y)
		roomType := g.selectRoomTypeUnsafe(i, roomCount, config)

		room := &RoomNode{
			ID:         roomID,
			Type:       roomType,
			Theme:      config.Theme,
			Size:       g.calculateRoomSizeUnsafe(roomType, config),
			Features:   g.generateRoomFeaturesUnsafe(roomType, config),
			Properties: map[string]interface{}{"grid_x": x, "grid_y": y},
		}

		graph.nodes[roomID] = room
		graph.adjacency[roomID] = make([]string, 0)

		// Connect to adjacent grid positions
		if x > 0 {
			// Connect to left neighbor
			leftID := fmt.Sprintf("grid_%d_%d", x-1, y)
			if _, exists := graph.nodes[leftID]; exists {
				g.createGridConnectionUnsafe(graph, roomID, leftID)
			}
		}

		if y > 0 {
			// Connect to upper neighbor
			upID := fmt.Sprintf("grid_%d_%d", x, y-1)
			if _, exists := graph.nodes[upID]; exists {
				g.createGridConnectionUnsafe(graph, roomID, upID)
			}
		}

		if i%10 == 0 { // Progress every 10 rooms
			progress := float64(i) / float64(roomCount)
			g.publishGenerationProgressUnsafe(ctx, progress, "generating grid layout")
		}
	}

	return graph, nil
}

func (g *GraphBasedGenerator) generateOrganicLayoutUnsafe(
	ctx context.Context, graph *RoomGraph, roomCount int, config GenerationConfig,
) (*RoomGraph, error) {
	// Organic layout: irregular, natural connections
	// Start with one room and gradually add connected rooms

	if roomCount < 1 {
		return graph, nil
	}

	// Create initial room
	firstRoomID := "organic_0"
	firstRoom := &RoomNode{
		ID:         firstRoomID,
		Type:       "entrance",
		Theme:      config.Theme,
		Size:       g.calculateRoomSizeUnsafe("entrance", config),
		Features:   g.generateRoomFeaturesUnsafe("entrance", config),
		Properties: make(map[string]interface{}),
	}

	graph.nodes[firstRoomID] = firstRoom
	graph.adjacency[firstRoomID] = make([]string, 0)

	// Keep track of rooms that can have new connections
	connectableRooms := []string{firstRoomID}

	// Add remaining rooms organically
	for i := 1; i < roomCount; i++ {
		roomID := fmt.Sprintf("organic_%d", i)
		roomType := g.selectRoomTypeUnsafe(i, roomCount, config)

		room := &RoomNode{
			ID:         roomID,
			Type:       roomType,
			Theme:      config.Theme,
			Size:       g.calculateRoomSizeUnsafe(roomType, config),
			Features:   g.generateRoomFeaturesUnsafe(roomType, config),
			Properties: make(map[string]interface{}),
		}

		graph.nodes[roomID] = room
		graph.adjacency[roomID] = make([]string, 0)

		// Connect to 1-3 existing rooms
		connectionCount := 1 + g.random.Intn(3)
		if connectionCount > len(connectableRooms) {
			connectionCount = len(connectableRooms)
		}

		// Shuffle and take first N rooms for connection
		shuffled := make([]string, len(connectableRooms))
		copy(shuffled, connectableRooms)
		g.random.Shuffle(len(shuffled), func(i, j int) {
			shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
		})

		for j := 0; j < connectionCount; j++ {
			targetRoomID := shuffled[j]
			connectionID := fmt.Sprintf("conn_%s_%s", targetRoomID, roomID)

			edge := &ConnectionEdge{
				ID:            connectionID,
				FromRoomID:    targetRoomID,
				ToRoomID:      roomID,
				Type:          "passage",
				Bidirectional: true,
				Cost:          1.0,
				Required:      true,
			}

			graph.edges[connectionID] = edge
			graph.adjacency[targetRoomID] = append(graph.adjacency[targetRoomID], roomID)
			graph.adjacency[roomID] = append(graph.adjacency[roomID], targetRoomID)
		}

		// Add new room to connectable list
		connectableRooms = append(connectableRooms, roomID)

		// Limit connectable rooms to prevent over-connection
		if len(connectableRooms) > 8 {
			// Remove some older rooms from connectable list
			start := len(connectableRooms) - 6
			connectableRooms = connectableRooms[start:]
		}

		if i%5 == 0 { // Progress every 5 rooms
			progress := float64(i) / float64(roomCount)
			g.publishGenerationProgressUnsafe(ctx, progress, "generating organic layout")
		}
	}

	return graph, nil
}

// Helper methods for graph generation

func (g *GraphBasedGenerator) selectRoomTypeUnsafe(roomIndex, totalRooms int, config GenerationConfig) string {
	// Select room type based on position and available types
	if len(config.RoomTypes) == 0 {
		// Default room types
		if roomIndex == 0 {
			return RoomTypeEntrance
		}
		if roomIndex == totalRooms-1 {
			return RoomTypeBoss
		}

		types := []string{RoomTypeChamber, RoomTypeCorridor, RoomTypeTreasure, RoomTypeTrap}
		return types[g.random.Intn(len(types))]
	}

	// Use provided room types
	return config.RoomTypes[g.random.Intn(len(config.RoomTypes))]
}

func (g *GraphBasedGenerator) calculateRoomSizeUnsafe(roomType string, config GenerationConfig) spatial.Dimensions {
	// Calculate room size based on type and config constraints
	minSize := config.MinRoomSize
	maxSize := config.MaxRoomSize

	// Use defaults if not specified
	if minSize.Width == 0 || minSize.Height == 0 {
		minSize = spatial.Dimensions{Width: 8, Height: 8}
	}
	if maxSize.Width == 0 || maxSize.Height == 0 {
		maxSize = spatial.Dimensions{Width: 20, Height: 20}
	}

	// Adjust for room type
	switch roomType {
	case RoomTypeBoss:
		// Boss rooms are typically larger
		minSize.Width *= 1.5
		minSize.Height *= 1.5
	case RoomTypeCorridor:
		// Corridors are typically smaller
		maxSize.Width *= 0.7
		maxSize.Height *= 0.7
	}

	// Generate random size within bounds
	width := minSize.Width + float64(g.random.Intn(int(maxSize.Width-minSize.Width+1)))
	height := minSize.Height + float64(g.random.Intn(int(maxSize.Height-minSize.Height+1)))

	return spatial.Dimensions{Width: width, Height: height}
}

func (g *GraphBasedGenerator) generateRoomFeaturesUnsafe(roomType string, _ GenerationConfig) []Feature {
	// Generate features based on room type
	var features []Feature

	switch roomType {
	case RoomTypeTreasure:
		features = append(features, Feature{
			Type:       "chest",
			Name:       "Treasure Chest",
			Properties: map[string]interface{}{"locked": true},
		})
	case RoomTypeTrap:
		features = append(features, Feature{
			Type:       "trap",
			Name:       "Pressure Plate",
			Properties: map[string]interface{}{"damage": "1d6", "type": "pit"},
		})
	case RoomTypeBoss:
		features = append(features, Feature{
			Type:       "throne",
			Name:       "Boss Throne",
			Properties: map[string]interface{}{"imposing": true},
		})
	}

	return features
}

func (g *GraphBasedGenerator) createGridConnectionUnsafe(graph *RoomGraph, roomID1, roomID2 string) {
	connectionID := fmt.Sprintf("conn_%s_%s", roomID1, roomID2)

	edge := &ConnectionEdge{
		ID:            connectionID,
		FromRoomID:    roomID1,
		ToRoomID:      roomID2,
		Type:          "passage",
		Bidirectional: true,
		Cost:          1.0,
		Required:      true,
	}

	graph.edges[connectionID] = edge
	graph.adjacency[roomID1] = append(graph.adjacency[roomID1], roomID2)
	graph.adjacency[roomID2] = append(graph.adjacency[roomID2], roomID1)
}

// Graph-to-spatial translation implementation

func (g *GraphBasedGenerator) createOrchestratorUnsafe(
	config GenerationConfig,
) spatial.RoomOrchestrator {
	// Create spatial orchestrator for this environment
	orchestratorID := fmt.Sprintf("%s_orchestrator", g.id)

	// Determine layout type for orchestrator
	var layoutType spatial.LayoutType
	switch config.Layout {
	case LayoutTypeLinear:
		layoutType = spatial.LayoutTypeOrganic // Linear maps to organic
	case LayoutTypeBranching:
		layoutType = spatial.LayoutTypeBranching
	case LayoutTypeGrid:
		layoutType = spatial.LayoutTypeGrid
	case LayoutTypeOrganic:
		layoutType = spatial.LayoutTypeOrganic
	default:
		layoutType = spatial.LayoutTypeOrganic
	}

	// Create orchestrator using spatial module
	orchestrator := spatial.NewBasicRoomOrchestrator(spatial.BasicRoomOrchestratorConfig{
		ID:       spatial.OrchestratorID(orchestratorID),
		Type:     "environment_orchestrator",
		EventBus: g.eventBus,
		Layout:   layoutType,
	})

	return orchestrator
}

func (g *GraphBasedGenerator) placeRoomsSpatiallyUnsafe(
	ctx context.Context, graph *RoomGraph, orchestrator spatial.RoomOrchestrator, config GenerationConfig,
) error {
	// Create shape loader for room shapes
	shapeLoader := NewShapeLoader("tools/environments/shapes")

	// Place each room from the graph into spatial coordinates
	for _, roomNode := range graph.nodes {
		// Create spatial room from graph node
		spatialRoom, err := g.createSpatialRoomUnsafe(ctx, roomNode, config, shapeLoader)
		if err != nil {
			return fmt.Errorf("failed to create spatial room %s: %w", roomNode.ID, err)
		}

		// Add room to orchestrator
		if err := orchestrator.AddRoom(spatialRoom); err != nil {
			return fmt.Errorf("failed to add room %s to orchestrator: %w", roomNode.ID, err)
		}

		// Store spatial position back to graph node for connection creation
		// Note: Rooms don't have positions directly, using default for now
		roomNode.Position = &spatial.Position{
			X: 0,
			Y: 0,
		}
	}

	return nil
}

func (g *GraphBasedGenerator) createSpatialRoomUnsafe(
	ctx context.Context, roomNode *RoomNode, config GenerationConfig, shapeLoader *ShapeLoader,
) (spatial.Room, error) {
	// Select appropriate room shape based on room type
	shapeName := g.selectRoomShapeUnsafe(roomNode.Type, config)

	// Load the shape
	shape, err := shapeLoader.LoadShape(shapeName)
	if err != nil {
		// Fallback to default shape if specific shape not found
		shape = GetDefaultShapes()[ShapeRectangle]
	}

	// Scale shape to room size
	scaledShape := ScaleShape(shape, roomNode.Size)

	// Generate wall pattern for this room
	walls, err := g.generateRoomWallsUnsafe(ctx, roomNode, scaledShape, config)
	if err != nil {
		return nil, fmt.Errorf("failed to generate walls for room %s: %w", roomNode.ID, err)
	}

	// Create spatial room with generated walls
	spatialRoom, err := g.createSpatialRoomWithWallsUnsafe(roomNode, walls)
	if err != nil {
		return nil, fmt.Errorf("failed to create spatial room %s: %w", roomNode.ID, err)
	}

	return spatialRoom, nil
}

func (g *GraphBasedGenerator) selectRoomShapeUnsafe(roomType string, _ GenerationConfig) string {
	// Select shape based on room type
	switch roomType {
	case RoomTypeEntrance, RoomTypeExit:
		return ShapeRectangle // Simple entrance/exit
	case RoomTypeCorridor:
		return ShapeRectangle // Long narrow rectangle
	case RoomTypeChamber:
		return ShapeRectangle // Basic chamber
	case RoomTypeBoss:
		return "square" // Boss rooms are often square
	case RoomTypeTreasure:
		return "square" // Treasure rooms are compact
	case "hub":
		return "cross" // Hub rooms have multiple connections
	case RoomTypeJunction:
		return "t_shape" // Junction rooms connect paths
	case "corner":
		return "l_shape" // Corner rooms change direction
	default:
		return ShapeRectangle // Default to rectangle
	}
}

func (g *GraphBasedGenerator) generateRoomWallsUnsafe(
	ctx context.Context, roomNode *RoomNode, shape *RoomShape, config GenerationConfig,
) ([]WallSegment, error) {
	// Select wall pattern based on room type and theme
	patternName := g.selectWallPatternUnsafe(roomNode.Type, config)

	// Get pattern function
	patternFunc, exists := WallPatterns[patternName]
	if !exists {
		patternFunc = WallPatterns[PatternEmpty] // Fallback to empty pattern
	}

	// Configure pattern parameters
	params := PatternParams{
		Density:           g.calculateDensityUnsafe(roomNode.Type, config),
		DestructibleRatio: g.calculateDestructibleRatioUnsafe(roomNode.Type, config),
		RandomSeed:        g.random.Int63(),
		Safety: PathSafetyParams{
			MinPathWidth:      2.0,
			MinOpenSpace:      0.6,
			EntitySize:        1.0,
			EmergencyFallback: true,
			RequiredPaths:     g.createRequiredPathsUnsafe(roomNode, shape),
		},
		Material:   g.selectMaterialUnsafe(roomNode.Type, config),
		WallHeight: 3.0, // Default wall height
	}

	// Pass event bus for emergency fallback notifications
	params.EventBus = g.eventBus

	// Generate walls using pattern function
	walls, err := patternFunc(ctx, shape, roomNode.Size, params)
	if err != nil {
		return nil, fmt.Errorf("pattern generation failed: %w", err)
	}

	return walls, nil
}

func (g *GraphBasedGenerator) selectWallPatternUnsafe(roomType string, _ GenerationConfig) string {
	// Select wall pattern based on room type
	switch roomType {
	case RoomTypeEntrance, RoomTypeExit:
		return PatternEmpty // Keep entrances/exits clear
	case RoomTypeCorridor:
		return PatternEmpty // Corridors should be clear
	case RoomTypeChamber:
		return "half_cover" // Chambers have tactical cover
	case RoomTypeBoss:
		return "defensive" // Boss rooms have defensive positioning
	case RoomTypeTreasure:
		return "alcoves" // Treasure rooms have hiding spots
	case "hub":
		return "pillars" // Hub rooms have visibility blockers
	case RoomTypeJunction:
		return PatternEmpty // Junctions should be clear for movement
	case RoomTypeTrap:
		return "maze" // Trap rooms are maze-like
	default:
		return "half_cover" // Default to half cover
	}
}

func (g *GraphBasedGenerator) calculateDensityUnsafe(roomType string, _ GenerationConfig) float64 {
	// Calculate wall density based on room type
	switch roomType {
	case "entrance", "exit", "corridor", "junction":
		return 0.1 // Very low density for movement rooms
	case RoomTypeChamber:
		return 0.4 // Medium density for tactical rooms
	case RoomTypeBoss:
		return 0.6 // High density for boss encounters
	case RoomTypeTreasure:
		return 0.3 // Low-medium density for treasure rooms
	case RoomTypeTrap:
		return 0.8 // High density for trap rooms
	default:
		return 0.4 // Default medium density
	}
}

func (g *GraphBasedGenerator) calculateDestructibleRatioUnsafe(roomType string, _ GenerationConfig) float64 {
	// Calculate destructible ratio based on room type
	switch roomType {
	case RoomTypeBoss:
		return 0.3 // Boss rooms have more permanent walls
	case RoomTypeTreasure:
		return 0.5 // Treasure rooms are moderately destructible
	case RoomTypeTrap:
		return 0.8 // Trap rooms are highly destructible
	default:
		return 0.7 // Default 70% destructible
	}
}

func (g *GraphBasedGenerator) selectMaterialUnsafe(_ string, config GenerationConfig) string {
	// Select material based on theme and room type
	switch config.Theme {
	case "dungeon", "castle":
		return "stone"
	case "cave", "natural":
		return "rock"
	case "wooden", "tavern":
		return "wood"
	case "metal", "facility":
		return "metal"
	default:
		return "stone" // Default to stone
	}
}

func (g *GraphBasedGenerator) createRequiredPathsUnsafe(_ *RoomNode, shape *RoomShape) []Path {
	// Create required paths based on room connections
	var paths []Path

	// For now, create simple paths between connections
	connections := shape.Connections
	if len(connections) >= 2 {
		// Create path between first two connections
		path := Path{
			From:    connections[0].Position,
			To:      connections[1].Position,
			Width:   2.0,
			Purpose: "main_path",
		}
		paths = append(paths, path)
	}

	return paths
}

func (g *GraphBasedGenerator) createSpatialRoomWithWallsUnsafe(
	roomNode *RoomNode, walls []WallSegment,
) (spatial.Room, error) {
	// Create a grid for the room
	grid := spatial.NewSquareGrid(spatial.SquareGridConfig{
		Width:  roomNode.Size.Width,
		Height: roomNode.Size.Height,
	})

	// Create spatial room configuration
	roomConfig := spatial.BasicRoomConfig{
		ID:       roomNode.ID,
		Type:     roomNode.Type,
		Grid:     grid,
		EventBus: g.eventBus,
	}

	// Create basic spatial room
	room := spatial.NewBasicRoom(roomConfig)

	// Convert wall segments to wall entities
	wallEntities := CreateWallEntities(walls)

	// Place wall entities in the spatial room
	for _, wallEntity := range wallEntities {
		entity := wallEntity.(*WallEntity)
		err := room.PlaceEntity(entity, entity.GetPosition())
		if err != nil {
			return nil, fmt.Errorf("failed to place wall entity %s: %w", entity.GetID(), err)
		}
	}

	return room, nil
}

func (g *GraphBasedGenerator) createConnectionsUnsafe(
	graph *RoomGraph, orchestrator spatial.RoomOrchestrator, config GenerationConfig,
) error {
	// Create spatial connections based on graph edges
	for _, edge := range graph.edges {
		// Get the spatial rooms
		fromRoom, exists := orchestrator.GetRoom(edge.FromRoomID)
		if !exists {
			return fmt.Errorf("from room %s not found", edge.FromRoomID)
		}

		toRoom, exists := orchestrator.GetRoom(edge.ToRoomID)
		if !exists {
			return fmt.Errorf("to room %s not found", edge.ToRoomID)
		}

		// Create spatial connection
		connection := g.createSpatialConnectionUnsafe(edge, fromRoom, toRoom, config)

		// Add connection to orchestrator
		if err := orchestrator.AddConnection(connection); err != nil {
			return fmt.Errorf("failed to add connection %s to orchestrator: %w", edge.ID, err)
		}
	}

	return nil
}

func (g *GraphBasedGenerator) createSpatialConnectionUnsafe(
	edge *ConnectionEdge, fromRoom, toRoom spatial.Room, _ GenerationConfig,
) spatial.Connection {
	// Determine connection positions
	fromPos := g.findConnectionPositionUnsafe(fromRoom, toRoom, "exit")
	toPos := g.findConnectionPositionUnsafe(toRoom, fromRoom, "entrance")

	// Create spatial connection based on edge type
	switch edge.Type {
	case "door":
		return spatial.CreateDoorConnection(edge.ID, edge.FromRoomID, edge.ToRoomID, fromPos, toPos)
	case "stairs":
		return spatial.CreateStairsConnection(edge.ID, edge.FromRoomID, edge.ToRoomID, fromPos, toPos, true)
	case "passage":
		return spatial.CreateSecretPassageConnection(edge.ID, edge.FromRoomID, edge.ToRoomID, fromPos, toPos, []string{})
	case "portal":
		return spatial.CreatePortalConnection(edge.ID, edge.FromRoomID, edge.ToRoomID, fromPos, toPos, true)
	default:
		// Default to door connection
		return spatial.CreateDoorConnection(edge.ID, edge.FromRoomID, edge.ToRoomID, fromPos, toPos)
	}
}

func (g *GraphBasedGenerator) findConnectionPositionUnsafe(
	room spatial.Room, otherRoom spatial.Room, purpose string,
) spatial.Position {
	// Find appropriate connection position on room boundary
	// Get room dimensions from the grid
	grid := room.GetGrid()
	dimensions := grid.GetDimensions()

	// Get grid dimensions
	width := dimensions.Width
	height := dimensions.Height

	// Calculate center of room
	centerX := width / 2
	centerY := height / 2

	// TODO: Use otherRoom and purpose parameters to determine optimal connection
	// In a complete implementation, would:
	// 1. Get the other room's position to determine direction
	// 2. Find the appropriate edge based on that direction
	// 3. Account for connection purpose (entrance/exit positioning)
	// 4. Account for room shape and existing connections
	// For now, return room center as connection point
	_ = otherRoom // Acknowledge parameter until TODO is implemented
	_ = purpose   // Acknowledge parameter until TODO is implemented
	return spatial.Position{X: centerX, Y: centerY}
}

func (g *GraphBasedGenerator) createEnvironmentUnsafe(
	orchestrator spatial.RoomOrchestrator, config GenerationConfig,
) Environment {
	// Create query handler for this environment
	queryHandler := NewBasicQueryHandler(BasicQueryHandlerConfig{
		Orchestrator: orchestrator,
		SpatialQuery: g.spatialQuery,
		EventBus:     g.eventBus,
	})

	// Create environment wrapper
	environment := NewBasicEnvironment(BasicEnvironmentConfig{
		ID:           fmt.Sprintf("%s_environment", g.id),
		Type:         "generated_environment",
		Theme:        config.Theme,
		Metadata:     config.Metadata,
		EventBus:     g.eventBus,
		Orchestrator: orchestrator,
		QueryHandler: queryHandler,
	})

	return environment
}

// Event helpers

func (g *GraphBasedGenerator) publishGenerationProgressUnsafe(ctx context.Context, progress float64, stage string) {
	if g.eventBus != nil {
		event := events.NewGameEvent(EventGenerationProgress, g, nil)
		event.Context().Set("progress", progress)
		event.Context().Set("stage", stage)
		_ = g.eventBus.Publish(ctx, event)
	}
}

func (g *GraphBasedGenerator) publishGenerationFailedUnsafe(ctx context.Context, err error, stage string) {
	if g.eventBus != nil {
		event := events.NewGameEvent(EventGenerationFailed, g, nil)
		event.Context().Set("error", err.Error())
		event.Context().Set("stage", stage)
		_ = g.eventBus.Publish(ctx, event)
	}
}

// Validation

func (g *GraphBasedGenerator) validateUnsafe(config GenerationConfig) error {
	if config.Type != GenerationTypeGraph && config.Type != GenerationTypeHybrid {
		return fmt.Errorf("graph generator only supports Graph and Hybrid generation types")
	}

	if config.RoomCount < 0 {
		return fmt.Errorf("room count cannot be negative")
	}

	if config.RoomCount > g.capabilities.MaxRoomCount {
		return fmt.Errorf("room count %d exceeds maximum %d", config.RoomCount, g.capabilities.MaxRoomCount)
	}

	return nil
}

// Component factory management

// RegisterRoomFactory registers a custom room factory for a specific room type.
func (g *GraphBasedGenerator) RegisterRoomFactory(roomType string, factory ComponentFactory) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	g.roomFactories[roomType] = factory
}

// UnregisterRoomFactory removes a custom room factory for a specific room type.
func (g *GraphBasedGenerator) UnregisterRoomFactory(roomType string) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	delete(g.roomFactories, roomType)
}
