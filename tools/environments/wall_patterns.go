package environments

import (
	"context"
	"fmt"
	"math"
	"math/rand"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// WallSegment represents a wall within a room
// Purpose: Defines walls with destruction properties for tactical gameplay
type WallSegment struct {
	Start      spatial.Position `json:"start"`      // Wall start position
	End        spatial.Position `json:"end"`        // Wall end position
	Type       WallType         `json:"type"`       // Destructible, indestructible, etc.
	Properties WallProperties   `json:"properties"` // Wall behavior properties
}

// WallType categorizes wall behavior
type WallType int

const (
	// WallTypeIndestructible represents permanent structural walls.
	WallTypeIndestructible WallType = iota // Permanent structural walls
	// WallTypeDestructible represents walls that can be destroyed by players.
	WallTypeDestructible // Can be destroyed by players
	// WallTypeTemporary represents walls that can be bypassed with effort.
	WallTypeTemporary // Can be bypassed with effort
	// WallTypeConditional represents walls that require specific tools/abilities.
	WallTypeConditional // Requires specific tools/abilities
)

// WallProperties defines wall behavior and appearance
type WallProperties struct {
	// Destruction properties
	HP           int      `json:"hp,omitempty"`            // Health points
	Resistance   []string `json:"resistance,omitempty"`    // Damage types resisted
	Weakness     []string `json:"weakness,omitempty"`      // Extra damage types
	RequiredTool string   `json:"required_tool,omitempty"` // Specific tool needed

	// Physical properties
	Material  string  `json:"material"`  // Stone, wood, metal, etc.
	Thickness float64 `json:"thickness"` // Wall thickness
	Height    float64 `json:"height"`    // Wall height

	// Gameplay properties
	BlocksLoS      bool `json:"blocks_los"`      // Blocks line of sight
	BlocksMovement bool `json:"blocks_movement"` // Blocks movement
	ProvidesCover  bool `json:"provides_cover"`  // Combat cover bonus

	// Audio/Visual
	DestroySound string `json:"destroy_sound,omitempty"` // Sound when destroyed
	Texture      string `json:"texture,omitempty"`       // Visual texture
}

// PatternParams configures wall pattern generation
type PatternParams struct {
	// Generation parameters
	Density           float64 `json:"density"`            // Wall density (0.0-1.0)
	DestructibleRatio float64 `json:"destructible_ratio"` // % of walls that are destructible
	RandomSeed        int64   `json:"random_seed"`        // Seed for reproducible generation

	// Safety parameters
	Safety PathSafetyParams `json:"safety"` // Path validation parameters
	Grid   spatial.Grid     `json:"-"`      // Grid for validation

	// Customization
	Material   string                 `json:"material"`    // Default wall material
	WallHeight float64                `json:"wall_height"` // Default wall height
	Properties map[string]interface{} `json:"properties"`  // Custom properties

	// Event integration (optional) - use typed topics instead
	// EmergencyFallbackTopic will be used via ConnectToEventBus pattern
}

// PathSafetyParams ensures generated rooms are navigable
type PathSafetyParams struct {
	MinPathWidth      float64 `json:"min_path_width"`     // Minimum corridor width
	MinOpenSpace      float64 `json:"min_open_space"`     // % of room that must remain open
	EntitySize        float64 `json:"entity_size"`        // Size of entities that need to move
	RequiredPaths     []Path  `json:"required_paths"`     // Specific paths that must exist
	EmergencyFallback bool    `json:"emergency_fallback"` // Fall back to empty room if validation fails
}

// Path represents a required path through the room
type Path struct {
	From    spatial.Position `json:"from"`    // Path start
	To      spatial.Position `json:"to"`      // Path end
	Width   float64          `json:"width"`   // Required path width
	Purpose string           `json:"purpose"` // "entrance", "exit", "feature_access"
}

// WallPatternFunc generates wall patterns algorithmically
type WallPatternFunc func(
	ctx context.Context, shape *RoomShape, size spatial.Dimensions, params PatternParams,
) ([]WallSegment, error)

// WallPatterns is the registry for available wall patterns.
var WallPatterns = map[string]WallPatternFunc{
	"empty":  EmptyPattern,
	"random": RandomPattern,
}

// Pattern implementations

// EmptyPattern generates no internal walls
func EmptyPattern(
	_ context.Context, shape *RoomShape, size spatial.Dimensions, params PatternParams,
) ([]WallSegment, error) {
	// No walls to generate, but still validate the empty room
	var walls []WallSegment

	// Validate that the empty room meets safety requirements
	if err := validatePathSafety(walls, shape, size, params.Safety); err != nil {
		return nil, fmt.Errorf("empty room failed safety validation: %w", err)
	}

	return walls, nil
}

// RandomPattern generates random wall segments based on density parameter
func RandomPattern(
	ctx context.Context, shape *RoomShape, size spatial.Dimensions, params PatternParams,
) ([]WallSegment, error) {
	// #nosec G404 - Using math/rand for seeded, reproducible wall pattern generation
	// Same RandomSeed must produce identical wall layouts for consistent gameplay
	random := rand.New(rand.NewSource(params.RandomSeed))
	var walls []WallSegment

	// Calculate number of walls based on density
	roomArea := float64(size.Width * size.Height)
	maxWalls := int(roomArea * params.Density / 10.0) // Roughly 1 wall per 10 square units at density 1.0
	if maxWalls < 1 {
		maxWalls = 1
	}
	if maxWalls > 12 {
		maxWalls = 12 // Reasonable upper limit
	}

	// Generate random walls
	for i := 0; i < maxWalls; i++ {
		wall := generateRandomWall(shape, size, params, random)
		if wall != nil {
			walls = append(walls, *wall)
		}
	}

	// Apply destructible ratio
	walls = applyDestructibleRatio(walls, params.DestructibleRatio, random)

	// Validate and fix pathfinding
	validatedWalls, err := validateAndFixPathfinding(ctx, walls, shape, size, params.Safety, params)
	if err != nil {
		return nil, fmt.Errorf("random pattern failed validation: %w", err)
	}

	return validatedWalls, nil
}

// Helper functions for wall generation

func generateRandomWall(
	_ *RoomShape, size spatial.Dimensions, params PatternParams, random *rand.Rand,
) *WallSegment {
	// Generate a random wall segment
	// Place randomly within the room bounds, away from edges

	margin := math.Max(2.0, params.Safety.MinPathWidth)

	// Random center position with margin
	centerX := margin + random.Float64()*(float64(size.Width)-2*margin)
	centerY := margin + random.Float64()*(float64(size.Height)-2*margin)

	// Random wall orientation (horizontal or vertical)
	horizontal := random.Float64() < 0.5

	// Wall length between 2-8 units (more variety)
	length := 2.0 + random.Float64()*6.0

	var start, end spatial.Position
	if horizontal {
		start = spatial.Position{X: centerX - length/2, Y: centerY}
		end = spatial.Position{X: centerX + length/2, Y: centerY}
	} else {
		start = spatial.Position{X: centerX, Y: centerY - length/2}
		end = spatial.Position{X: centerX, Y: centerY + length/2}
	}

	// Ensure wall is within bounds
	if start.X < margin || start.Y < margin || end.X > float64(size.Width)-margin || end.Y > float64(size.Height)-margin {
		return nil // Skip this wall
	}

	return &WallSegment{
		Start: start,
		End:   end,
		Type:  WallTypeDestructible, // Will be adjusted by applyDestructibleRatio
		Properties: WallProperties{
			Material:       params.Material,
			Height:         params.WallHeight,
			Thickness:      0.5,
			BlocksLoS:      true,
			BlocksMovement: true,
			ProvidesCover:  true,
		},
	}
}

func applyDestructibleRatio(walls []WallSegment, ratio float64, random *rand.Rand) []WallSegment {
	for i := range walls {
		if random.Float64() < ratio {
			walls[i].Type = WallTypeDestructible
			walls[i].Properties.HP = 10 + random.Intn(20) // 10-30 HP
		} else {
			walls[i].Type = WallTypeIndestructible
		}
	}
	return walls
}

// Path validation functions

func validatePathSafety(walls []WallSegment, shape *RoomShape, size spatial.Dimensions, safety PathSafetyParams) error {
	// Check minimum open space
	if !hasMinimumOpenSpace(walls, shape, size, safety.MinOpenSpace) {
		return fmt.Errorf("room does not have minimum open space (%.1f%%)", safety.MinOpenSpace*100)
	}

	// Convert wall segments to blocked hex positions for A* pathfinding
	blockedHexes := wallSegmentsToBlockedHexes(walls)

	// Check all connection pairs are reachable via A* pathfinding
	if len(shape.Connections) >= 2 {
		pathfinder := spatial.NewSimplePathFinder()

		for i := 0; i < len(shape.Connections); i++ {
			for j := i + 1; j < len(shape.Connections); j++ {
				fromConn := shape.Connections[i]
				toConn := shape.Connections[j]

				// Convert connection positions to cube coordinates
				fromCube := positionToCubeCoordinate(fromConn.Position, size)
				toCube := positionToCubeCoordinate(toConn.Position, size)

				// Run A* pathfinding
				path := pathfinder.FindPath(fromCube, toCube, blockedHexes)
				if len(path) == 0 && fromCube != toCube {
					return fmt.Errorf("no path between connections '%s' and '%s'", fromConn.Name, toConn.Name)
				}
			}
		}
	}

	return nil
}

func validateAndFixPathfinding(
	_ context.Context, walls []WallSegment, shape *RoomShape, size spatial.Dimensions,
	safety PathSafetyParams, _ PatternParams,
) ([]WallSegment, error) {
	// First, try to validate as-is
	if err := validatePathSafety(walls, shape, size, safety); err == nil {
		return walls, nil
	}

	// Try to fix pathfinding issues by removing blocking walls
	fixedWalls := walls

	// Convert to blocked hexes for pathfinding
	pathfinder := spatial.NewSimplePathFinder()

	// Iteratively remove walls until all connections are reachable
	maxIterations := len(walls) + 1
	for iteration := 0; iteration < maxIterations; iteration++ {
		blockedHexes := wallSegmentsToBlockedHexes(fixedWalls)

		// Find first blocked path
		var blockedFrom, blockedTo *ConnectionPoint
		for i := 0; i < len(shape.Connections) && blockedFrom == nil; i++ {
			for j := i + 1; j < len(shape.Connections); j++ {
				fromConn := shape.Connections[i]
				toConn := shape.Connections[j]

				fromCube := positionToCubeCoordinate(fromConn.Position, size)
				toCube := positionToCubeCoordinate(toConn.Position, size)

				path := pathfinder.FindPath(fromCube, toCube, blockedHexes)
				if len(path) == 0 && fromCube != toCube {
					blockedFrom = &fromConn
					blockedTo = &toConn
					break
				}
			}
		}

		// If no blocked paths, we're done
		if blockedFrom == nil {
			break
		}

		// Remove walls along the straight line between blocked connections
		fixedWalls = removeWallsBetweenConnections(fixedWalls, *blockedFrom, *blockedTo, size, safety.MinPathWidth)

		// If we've removed all walls and still no path, give up
		if len(fixedWalls) == 0 {
			break
		}
	}

	// 3. Check if we still have minimum open space
	if !hasMinimumOpenSpace(fixedWalls, shape, size, safety.MinOpenSpace) {
		fixedWalls = removeExcessWalls(fixedWalls, shape, size, safety.MinOpenSpace)
	}

	// Final validation
	if err := validatePathSafety(fixedWalls, shape, size, safety); err != nil {
		if safety.EmergencyFallback {
			// Emergency fallback: return empty room
			return []WallSegment{}, nil
		}
		return nil, fmt.Errorf("could not fix pathfinding issues: %w", err)
	}

	return fixedWalls, nil
}

// wallSegmentsToBlockedHexes converts wall segments to a map of blocked cube coordinates.
// This discretizes the continuous wall segments into actual hex positions for A* pathfinding.
func wallSegmentsToBlockedHexes(walls []WallSegment) map[spatial.CubeCoordinate]bool {
	blocked := make(map[spatial.CubeCoordinate]bool)

	for _, wall := range walls {
		if !wall.Properties.BlocksMovement {
			continue
		}

		// Calculate positions along the wall segment (same logic as discretizeWallSegment)
		positions := calculateWallPositions(wall.Start, wall.End, wall.Properties.Thickness)

		for _, pos := range positions {
			// Convert position to cube coordinate
			cube := spatial.OffsetCoordinateToCube(pos)
			blocked[cube] = true
		}
	}

	return blocked
}

// positionToCubeCoordinate converts a normalized position to a cube coordinate.
// Positions from connections are in room-relative coordinates.
func positionToCubeCoordinate(pos spatial.Position, size spatial.Dimensions) spatial.CubeCoordinate {
	// Scale normalized position to actual room size and convert to cube
	scaledPos := spatial.Position{
		X: pos.X * size.Width,
		Y: pos.Y * size.Height,
	}
	return spatial.OffsetCoordinateToCube(scaledPos)
}

// removeWallsBetweenConnections removes walls that block the path between two connections.
func removeWallsBetweenConnections(
	walls []WallSegment, from, to ConnectionPoint, size spatial.Dimensions, pathWidth float64,
) []WallSegment {
	// Scale connection positions to actual size
	fromPos := spatial.Position{X: from.Position.X * size.Width, Y: from.Position.Y * size.Height}
	toPos := spatial.Position{X: to.Position.X * size.Width, Y: to.Position.Y * size.Height}

	var keptWalls []WallSegment
	for _, wall := range walls {
		// Check if wall blocks the corridor between connections
		if wallBlocksPathBetweenPoints(wall, fromPos, toPos, pathWidth) {
			continue // Remove this wall
		}
		keptWalls = append(keptWalls, wall)
	}

	return keptWalls
}

// wallBlocksPathBetweenPoints checks if a wall blocks the corridor between two points.
func wallBlocksPathBetweenPoints(wall WallSegment, from, to spatial.Position, pathWidth float64) bool {
	if !wall.Properties.BlocksMovement {
		return false
	}

	// Check if wall start or end is within the path corridor
	distStart := pointToLineDistance(wall.Start, from, to)
	distEnd := pointToLineDistance(wall.End, from, to)

	return distStart < pathWidth || distEnd < pathWidth
}

func hasMinimumOpenSpace(walls []WallSegment, _ *RoomShape, size spatial.Dimensions, minPercent float64) bool {
	totalArea := float64(size.Width * size.Height)
	wallArea := calculateWallArea(walls)
	openPercent := (totalArea - wallArea) / totalArea
	return openPercent >= minPercent
}

func calculateWallArea(walls []WallSegment) float64 {
	var totalArea float64
	for _, wall := range walls {
		dx := wall.End.X - wall.Start.X
		dy := wall.End.Y - wall.Start.Y
		length := math.Sqrt(dx*dx + dy*dy)
		thickness := wall.Properties.Thickness
		if thickness == 0 {
			thickness = 0.5 // Default thickness
		}
		totalArea += length * thickness
	}
	return totalArea
}

func pointToLineDistance(point, lineStart, lineEnd spatial.Position) float64 {
	// Calculate distance from point to line segment
	dx := lineEnd.X - lineStart.X
	dy := lineEnd.Y - lineStart.Y

	if dx == 0 && dy == 0 {
		// Line is a point
		deltaX := point.X - lineStart.X
		deltaY := point.Y - lineStart.Y
		return math.Sqrt(deltaX*deltaX + deltaY*deltaY)
	}

	t := ((point.X-lineStart.X)*dx + (point.Y-lineStart.Y)*dy) / (dx*dx + dy*dy)
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}

	closestX := lineStart.X + t*dx
	closestY := lineStart.Y + t*dy

	deltaX := point.X - closestX
	deltaY := point.Y - closestY
	return math.Sqrt(deltaX*deltaX + deltaY*deltaY)
}

func removeExcessWalls(
	walls []WallSegment, shape *RoomShape, size spatial.Dimensions, minOpenSpace float64,
) []WallSegment {
	// Remove walls until we have minimum open space
	// Priority: remove destructible walls first

	var indestructibleWalls []WallSegment
	var destructibleWalls []WallSegment

	for _, wall := range walls {
		if wall.Type == WallTypeIndestructible {
			indestructibleWalls = append(indestructibleWalls, wall)
		} else {
			destructibleWalls = append(destructibleWalls, wall)
		}
	}

	// Start with all indestructible walls
	result := indestructibleWalls

	// Add destructible walls until we hit the limit
	for _, wall := range destructibleWalls {
		if hasMinimumOpenSpace(append(result, wall), shape, size, minOpenSpace) {
			result = append(result, wall)
		} else {
			break // Adding this wall would violate minimum open space
		}
	}

	return result
}
