package environments

import (
	"context"
	"fmt"
	"math"
	"math/rand"

	"github.com/KirkDiggler/rpg-toolkit/events"
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

	// Event integration (optional)
	EventBus events.EventBus `json:"-"` // For emergency fallback notifications
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

	// Check required paths
	for _, path := range safety.RequiredPaths {
		if !pathExists(walls, path, safety.MinPathWidth) {
			return fmt.Errorf("required path '%s' is blocked", path.Purpose)
		}
	}

	// Check all connection points are accessible
	for _, conn := range shape.Connections {
		if !isConnectionAccessible(walls, conn, safety.EntitySize) {
			return fmt.Errorf("connection '%s' is not accessible", conn.Name)
		}
	}

	return nil
}

func validateAndFixPathfinding(
	ctx context.Context, walls []WallSegment, shape *RoomShape, size spatial.Dimensions,
	safety PathSafetyParams, params PatternParams,
) ([]WallSegment, error) {
	// First, try to validate as-is
	if err := validatePathSafety(walls, shape, size, safety); err == nil {
		return walls, nil
	}

	// Try to fix pathfinding issues
	fixedWalls := walls

	// 1. Clear required paths
	for _, path := range safety.RequiredPaths {
		if !pathExists(fixedWalls, path, safety.MinPathWidth) {
			fixedWalls = clearPathway(fixedWalls, path, safety.MinPathWidth)
		}
	}

	// 2. Ensure connection accessibility
	for _, conn := range shape.Connections {
		if !isConnectionAccessible(fixedWalls, conn, safety.EntitySize) {
			fixedWalls = clearPathToConnection(fixedWalls, conn, safety.MinPathWidth)
		}
	}

	// 3. Check if we still have minimum open space
	if !hasMinimumOpenSpace(fixedWalls, shape, size, safety.MinOpenSpace) {
		fixedWalls = removeExcessWalls(fixedWalls, shape, size, safety.MinOpenSpace)
	}

	// Final validation
	if err := validatePathSafety(fixedWalls, shape, size, safety); err != nil {
		if safety.EmergencyFallback {
			// Publish emergency fallback event to notify client
			if params.EventBus != nil {
				event := events.NewGameEvent(EventEmergencyFallbackTriggered, nil, nil)
				event.Context().Set("pattern_type", "random")
				event.Context().Set("reason", "path_safety_validation_failed")
				event.Context().Set("original_density", params.Density)
				event.Context().Set("original_wall_count", len(fixedWalls))
				event.Context().Set("validation_error", err.Error())
				event.Context().Set("fallback_type", "empty_room")
				event.Context().Set("room_size", fmt.Sprintf("%.0fx%.0f", size.Width, size.Height))
				_ = params.EventBus.Publish(ctx, event)
			}

			// Emergency fallback: return empty room
			return []WallSegment{}, nil
		}
		return nil, fmt.Errorf("could not fix pathfinding issues: %w", err)
	}

	return fixedWalls, nil
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

func pathExists(walls []WallSegment, path Path, _ float64) bool {
	// Simplified path existence check
	// In production, would use proper A* pathfinding with wall collision detection

	// For now, check if path line intersects with any walls
	for _, wall := range walls {
		if wall.Properties.BlocksMovement && lineIntersectsWall(path.From, path.To, wall) {
			return false
		}
	}
	return true
}

func lineIntersectsWall(from, to spatial.Position, wall WallSegment) bool {
	// Simplified line intersection check
	// In production, would use proper line-line intersection algorithm

	// Check if path line intersects with wall segment
	return linesIntersect(from, to, wall.Start, wall.End)
}

func linesIntersect(p1, p2, p3, p4 spatial.Position) bool {
	// Simple line intersection algorithm
	denom := (p1.X-p2.X)*(p3.Y-p4.Y) - (p1.Y-p2.Y)*(p3.X-p4.X)
	if denom == 0 {
		return false // Lines are parallel
	}

	t := ((p1.X-p3.X)*(p3.Y-p4.Y) - (p1.Y-p3.Y)*(p3.X-p4.X)) / denom
	u := -((p1.X-p2.X)*(p1.Y-p3.Y) - (p1.Y-p2.Y)*(p1.X-p3.X)) / denom

	return t >= 0 && t <= 1 && u >= 0 && u <= 1
}

func isConnectionAccessible(walls []WallSegment, conn ConnectionPoint, entitySize float64) bool {
	// Check if there's a clear path to the connection point
	// For now, simple check - in production would use proper pathfinding

	// Check if connection point is blocked by walls
	for _, wall := range walls {
		if wall.Properties.BlocksMovement {
			distToWall := pointToLineDistance(conn.Position, wall.Start, wall.End)
			if distToWall < entitySize {
				return false
			}
		}
	}
	return true
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

func clearPathway(walls []WallSegment, path Path, width float64) []WallSegment {
	var clearedWalls []WallSegment

	for _, wall := range walls {
		if !wallBlocksPath(wall, path, width) {
			clearedWalls = append(clearedWalls, wall)
		}
	}

	return clearedWalls
}

func wallBlocksPath(wall WallSegment, path Path, width float64) bool {
	// Check if wall blocks the path corridor
	// Simplified check - in production would use proper corridor-line intersection

	if !wall.Properties.BlocksMovement {
		return false
	}

	// Check if wall intersects with path corridor
	distToPath := pointToLineDistance(wall.Start, path.From, path.To)
	if distToPath < width/2 {
		return true
	}

	distToPath = pointToLineDistance(wall.End, path.From, path.To)
	return distToPath < width/2
}

func clearPathToConnection(walls []WallSegment, conn ConnectionPoint, width float64) []WallSegment {
	var clearedWalls []WallSegment

	// Remove walls that block access to the connection
	for _, wall := range walls {
		if !wallBlocksConnection(wall, conn, width) {
			clearedWalls = append(clearedWalls, wall)
		}
	}

	return clearedWalls
}

func wallBlocksConnection(wall WallSegment, conn ConnectionPoint, width float64) bool {
	// Check if wall blocks access to connection
	distToConnection := pointToLineDistance(conn.Position, wall.Start, wall.End)
	return wall.Properties.BlocksMovement && distToConnection < width
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
