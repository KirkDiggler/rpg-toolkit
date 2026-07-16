package spawn

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// ConstraintSolver validates and enforces spatial constraints during spawning.
// Purpose: Phase 3 implementation of constraint validation per ADR-0013.
type ConstraintSolver struct {
	maxAttempts int
	random      *rand.Rand
}

// NewConstraintSolver creates a new constraint solver with default settings.
// Purpose: Standard constructor for constraint validation system.
func NewConstraintSolver() *ConstraintSolver {
	return &ConstraintSolver{
		maxAttempts: 100,
		// Default source when a caller doesn't pass an explicit rng (e.g. via
		// SpawnConfig.Seed) to FindValidPositions. Real randomness by
		// default; callers that want reproducible spawns pass their own
		// seeded *rand.Rand instead.
		random: rand.New(rand.NewSource(time.Now().UnixNano())), // #nosec G404
	}
}

// ValidatePosition checks if a position satisfies all spatial constraints.
// Purpose: Core constraint validation for entity placement. Does NOT check
// room-derived validity (bounds, walls, occupancy) — callers must also
// check Room.CanPlaceEntity; FindValidPositions does both.
func (cs *ConstraintSolver) ValidatePosition(
	room spatial.Room, position spatial.Position, entity core.Entity,
	constraints SpatialConstraints, existingEntities []SpawnedEntity,
) error {
	// Validate minimum distance constraints
	if err := cs.validateMinDistance(position, entity, constraints.MinDistance, existingEntities); err != nil {
		return fmt.Errorf("min distance constraint: %w", err)
	}

	// Validate wall proximity constraint
	if err := cs.validateWallProximity(room, position, constraints.WallProximity); err != nil {
		return fmt.Errorf("wall proximity constraint: %w", err)
	}

	// Validate line of sight constraints
	if err := cs.validateLineOfSight(room, position, entity, constraints.LineOfSight, existingEntities); err != nil {
		return fmt.Errorf("line of sight constraint: %w", err)
	}

	// Validate area of effect constraints
	if err := cs.validateAreaOfEffect(position, entity, constraints.AreaOfEffect, existingEntities); err != nil {
		return fmt.Errorf("area of effect constraint: %w", err)
	}

	return nil
}

// FindValidPositions finds positions that satisfy room-derived validity
// (bounds, walls/occupancy via Room.CanPlaceEntity), an optional
// caller-supplied PositionOracle, and the given spatial constraints.
// Purpose: Generate valid placement options for constraint-aware spawning.
//
// rng controls candidate ordering for gridless sampling; pass nil to use
// the solver's own default (real, non-reproducible) source — see
// SpawnConfig.Seed for how callers opt into reproducible spawns.
func (cs *ConstraintSolver) FindValidPositions(
	room spatial.Room, entity core.Entity, constraints SpatialConstraints,
	existingEntities []SpawnedEntity, maxPositions int, oracle PositionOracle, rng *rand.Rand,
) ([]spatial.Position, error) {
	if rng == nil {
		rng = cs.random
	}

	grid := room.GetGrid()
	if cs.isGridlessRoom(grid) {
		// For gridless rooms: use finer sampling for smooth positioning
		return cs.findValidPositionsGridless(room, entity, constraints, existingEntities, maxPositions, oracle, rng)
	}

	var validPositions []spatial.Position
	dims := grid.GetDimensions()
	minX, minY := gridOrigin(grid, dims)
	for x := minX; x < minX+dims.Width && len(validPositions) < maxPositions; x++ {
		for y := minY; y < minY+dims.Height && len(validPositions) < maxPositions; y++ {
			position := spatial.Position{X: x, Y: y}
			if !grid.IsValidPosition(position) {
				continue // defensive: today's grids are rectangular, but the interface doesn't promise it
			}
			if !room.CanPlaceEntity(entity, position) {
				continue // wall-occupied or otherwise blocked
			}
			if oracle != nil && !oracle(position) {
				continue
			}
			if cs.ValidatePosition(room, position, entity, constraints, existingEntities) == nil {
				validPositions = append(validPositions, position)
			}
		}
	}

	if len(validPositions) == 0 {
		return nil, fmt.Errorf(
			"no valid positions found for entity %s with given constraints in a %s room",
			entity.GetType(), dims,
		)
	}

	return validPositions, nil
}

// gridOrigin returns the lower bound (minX, minY) of grid's valid
// coordinate range, so callers can sweep exactly [minX, minX+dims.Width) x
// [minY, minY+dims.Height) instead of guessing. Grids in this package use
// one of two conventions: 0-based offset (SquareGrid, HexGrid — valid range
// starts at (0,0)) or axial, centered on zero (AxialHexGrid — valid range
// starts at (-dims.Width/2, -dims.Height/2)). (0,0) is a valid position
// under both conventions, so this probes with IsValidPosition instead of
// assuming one — a linear sweep anchored at the wrong origin would silently
// skip an entire grid's worth of valid cells.
func gridOrigin(grid spatial.Grid, dims spatial.Dimensions) (minX, minY float64) {
	if grid.IsValidPosition(spatial.Position{X: -dims.Width / 2, Y: 0}) {
		minX = -dims.Width / 2
	}
	if grid.IsValidPosition(spatial.Position{X: 0, Y: -dims.Height / 2}) {
		minY = -dims.Height / 2
	}
	return minX, minY
}

// validateMinDistance checks minimum distance requirements between entity types.
func (cs *ConstraintSolver) validateMinDistance(
	position spatial.Position, entity core.Entity, minDistances map[string]float64,
	existingEntities []SpawnedEntity,
) error {
	entityType := string(entity.GetType()) // Convert to string for map operations

	for _, existing := range existingEntities {
		existingType := string(existing.Entity.GetType()) // Convert to string for map operations

		// Check if there's a minimum distance requirement
		if requiredDistance, exists := minDistances[entityType+":"+existingType]; exists {
			distance := cs.calculateDistance(position, existing.Position)
			if distance < requiredDistance {
				return fmt.Errorf("entity %s too close to %s: %.2f < %.2f required",
					entityType, existingType, distance, requiredDistance)
			}
		}

		// Check reverse relationship
		if requiredDistance, exists := minDistances[existingType+":"+entityType]; exists {
			distance := cs.calculateDistance(position, existing.Position)
			if distance < requiredDistance {
				return fmt.Errorf("entity %s too close to %s: %.2f < %.2f required",
					entityType, existingType, distance, requiredDistance)
			}
		}
	}

	return nil
}

// validateWallProximity ensures entities maintain minimum distance from both
// the room boundary and any nearby wall/blocking entity.
func (cs *ConstraintSolver) validateWallProximity(
	room spatial.Room, position spatial.Position, minWallDistance float64,
) error {
	if minWallDistance <= 0 {
		return nil // No wall proximity constraint
	}

	// Probe minWallDistance out from position in each cardinal direction:
	// if any probe falls outside the grid's valid range, position is too
	// close to the boundary. This works regardless of the grid's coordinate
	// origin — 0-based offset grids (SquareGrid, HexGrid) and axial grids
	// centered on zero (AxialHexGrid) alike — since it only relies on the
	// grid's own IsValidPosition, never an assumed 0..Width/Height range
	// (a room.GetGrid() of nil, which the Room interface doesn't forbid,
	// skips the boundary probe rather than panicking).
	if grid := room.GetGrid(); grid != nil {
		probes := []spatial.Position{
			{X: position.X - minWallDistance, Y: position.Y},
			{X: position.X + minWallDistance, Y: position.Y},
			{X: position.X, Y: position.Y - minWallDistance},
			{X: position.X, Y: position.Y + minWallDistance},
		}
		for _, probe := range probes {
			if !grid.IsValidPosition(probe) {
				return fmt.Errorf("position too close to room boundary: (%.2f, %.2f), minimum distance %.2f",
					position.X, position.Y, minWallDistance)
			}
		}
	}

	for _, nearby := range room.GetEntitiesInRange(position, minWallDistance) {
		if placeable, ok := nearby.(spatial.Placeable); ok && placeable.BlocksMovement() {
			return fmt.Errorf("position too close to a wall (%s): (%.2f, %.2f), minimum distance %.2f",
				nearby.GetID(), position.X, position.Y, minWallDistance)
		}
	}

	return nil
}

// validateLineOfSight ensures line of sight requirements are met, using the
// real room's wall-aware Room.IsLineOfSightBlocked rather than a distance
// heuristic.
func (cs *ConstraintSolver) validateLineOfSight(
	room spatial.Room, position spatial.Position, entity core.Entity,
	losRules LineOfSightRules, existingEntities []SpawnedEntity,
) error {
	entityType := string(entity.GetType()) // Convert to string for comparisons

	// Check required sight relationships
	for _, pair := range losRules.RequiredSight {
		if pair.From == entityType {
			// This entity must see entities of pair.To type
			if err := cs.checkRequiredSight(room, position, pair.To, existingEntities); err != nil {
				return fmt.Errorf("required sight from %s to %s: %w", pair.From, pair.To, err)
			}
		}
		if pair.To == entityType {
			// Entities of pair.From type must see this entity
			if err := cs.checkCanBeSeen(room, position, pair.From, existingEntities); err != nil {
				return fmt.Errorf("required sight from %s to %s: %w", pair.From, pair.To, err)
			}
		}
	}

	// Check blocked sight relationships
	for _, pair := range losRules.BlockedSight {
		if pair.From == entityType {
			// This entity must NOT see entities of pair.To type
			if err := cs.checkBlockedSight(room, position, pair.To, existingEntities); err != nil {
				return fmt.Errorf("blocked sight from %s to %s: %w", pair.From, pair.To, err)
			}
		}
		if pair.To == entityType {
			// Entities of pair.From type must NOT see this entity
			if err := cs.checkCannotBeSeen(room, position, pair.From, existingEntities); err != nil {
				return fmt.Errorf("blocked sight from %s to %s: %w", pair.From, pair.To, err)
			}
		}
	}

	return nil
}

// validateAreaOfEffect ensures area of effect buffer zones are maintained.
func (cs *ConstraintSolver) validateAreaOfEffect(
	position spatial.Position, entity core.Entity, aoeRules map[string]float64,
	existingEntities []SpawnedEntity,
) error {
	entityType := string(entity.GetType()) // Convert to string for map lookup

	// Check if this entity type has area of effect requirements
	if aoeRadius, exists := aoeRules[entityType]; exists && aoeRadius > 0 {
		for _, existing := range existingEntities {
			distance := cs.calculateDistance(position, existing.Position)
			if distance < aoeRadius {
				return fmt.Errorf("entity %s within area of effect: %.2f < %.2f required",
					existing.Entity.GetType(), distance, aoeRadius)
			}
		}
	}

	// Check if existing entities have area of effect that would affect this position
	for _, existing := range existingEntities {
		existingType := string(existing.Entity.GetType()) // Convert to string for map lookup
		if aoeRadius, exists := aoeRules[existingType]; exists && aoeRadius > 0 {
			distance := cs.calculateDistance(position, existing.Position)
			if distance < aoeRadius {
				return fmt.Errorf("position within %s area of effect: %.2f < %.2f required",
					existingType, distance, aoeRadius)
			}
		}
	}

	return nil
}

// checkRequiredSight verifies that position has line of sight to required entity types.
func (cs *ConstraintSolver) checkRequiredSight(
	room spatial.Room, position spatial.Position, targetType string, existingEntities []SpawnedEntity,
) error {
	// Find entities of target type
	targetEntities := cs.getEntitiesByType(existingEntities, targetType)
	if len(targetEntities) == 0 {
		return nil // No target entities to check
	}

	// Check if we can see at least one target entity
	for _, target := range targetEntities {
		if cs.hasLineOfSight(room, position, target.Position) {
			return nil // Found at least one visible target
		}
	}

	return fmt.Errorf("no line of sight to any %s entities", targetType)
}

// checkCanBeSeen verifies that entities of fromType can see this position.
func (cs *ConstraintSolver) checkCanBeSeen(
	room spatial.Room, position spatial.Position, fromType string, existingEntities []SpawnedEntity,
) error {
	// Find entities of fromType
	fromEntities := cs.getEntitiesByType(existingEntities, fromType)
	if len(fromEntities) == 0 {
		return nil // No entities to check
	}

	// Check if at least one fromType entity can see this position
	for _, fromEntity := range fromEntities {
		if cs.hasLineOfSight(room, fromEntity.Position, position) {
			return nil // At least one entity can see this position
		}
	}

	return fmt.Errorf("no %s entities can see this position", fromType)
}

// checkBlockedSight verifies that position does NOT have line of sight to blocked entity types.
func (cs *ConstraintSolver) checkBlockedSight(
	room spatial.Room, position spatial.Position, targetType string, existingEntities []SpawnedEntity,
) error {
	// Find entities of target type
	targetEntities := cs.getEntitiesByType(existingEntities, targetType)

	// Check that we cannot see any target entity
	for _, target := range targetEntities {
		if cs.hasLineOfSight(room, position, target.Position) {
			return fmt.Errorf("has line of sight to %s at (%.2f, %.2f)",
				targetType, target.Position.X, target.Position.Y)
		}
	}

	return nil // No line of sight to any target entities
}

// checkCannotBeSeen verifies that entities of fromType cannot see this position.
func (cs *ConstraintSolver) checkCannotBeSeen(
	room spatial.Room, position spatial.Position, fromType string, existingEntities []SpawnedEntity,
) error {
	// Find entities of fromType
	fromEntities := cs.getEntitiesByType(existingEntities, fromType)

	// Check that no fromType entity can see this position
	for _, fromEntity := range fromEntities {
		if cs.hasLineOfSight(room, fromEntity.Position, position) {
			return fmt.Errorf("%s at (%.2f, %.2f) can see this position",
				fromType, fromEntity.Position.X, fromEntity.Position.Y)
		}
	}

	return nil // No entity can see this position
}

// hasLineOfSight checks if there's a clear line of sight between two
// positions, using the real room's wall-aware IsLineOfSightBlocked (backed
// by Placeable.BlocksLineOfSight on occupying entities) rather than a
// distance heuristic.
func (cs *ConstraintSolver) hasLineOfSight(room spatial.Room, from, to spatial.Position) bool {
	return !room.IsLineOfSightBlocked(from, to)
}

// calculateDistance computes Euclidean distance between two positions.
func (cs *ConstraintSolver) calculateDistance(pos1, pos2 spatial.Position) float64 {
	dx := pos1.X - pos2.X
	dy := pos1.Y - pos2.Y
	return math.Sqrt(dx*dx + dy*dy)
}

// isGridlessRoom checks if the room's grid is gridless (theater-of-mind:
// continuous positioning rather than discrete cells). GridlessRoom is
// itself a Grid implementation (GetShape() == GridShapeGridless), so this
// is a shape check, not a nil check — a nil grid is treated as gridless
// too, defensively, since there are no discrete cells to sweep.
func (cs *ConstraintSolver) isGridlessRoom(grid spatial.Grid) bool {
	return grid == nil || grid.GetShape() == spatial.GridShapeGridless
}

// findValidPositionsGridless finds valid positions for gridless rooms via
// random continuous sampling within the room's real dimensions, checking
// room-derived validity (Room.CanPlaceEntity), the optional oracle, and the
// given constraints.
// Purpose: Optimized position finding for continuous/smooth positioning systems.
func (cs *ConstraintSolver) findValidPositionsGridless(
	room spatial.Room, entity core.Entity, constraints SpatialConstraints,
	existingEntities []SpawnedEntity, maxPositions int, oracle PositionOracle, rng *rand.Rand,
) ([]spatial.Position, error) {
	if rng == nil {
		rng = cs.random
	}

	var validPositions []spatial.Position

	// A nil grid routes here via isGridlessRoom, and the Room interface
	// doesn't forbid GetGrid() returning nil — fall back to a reasonable
	// default span instead of panicking on GetDimensions().
	roomDimensions := spatial.Dimensions{Width: 10.0, Height: 10.0}
	if grid := room.GetGrid(); grid != nil {
		roomDimensions = grid.GetDimensions()
	}
	attempts := 0
	maxAttempts := cs.maxAttempts * 2 // More attempts for gridless

	for len(validPositions) < maxPositions && attempts < maxAttempts {
		// Generate random position within room bounds with some margin
		margin := 1.0
		x := margin + (roomDimensions.Width-2*margin)*rng.Float64()
		y := margin + (roomDimensions.Height-2*margin)*rng.Float64()
		position := spatial.Position{X: x, Y: y}
		attempts++

		if !room.CanPlaceEntity(entity, position) {
			continue // wall-occupied or otherwise blocked
		}
		if oracle != nil && !oracle(position) {
			continue
		}
		if cs.ValidatePosition(room, position, entity, constraints, existingEntities) == nil {
			validPositions = append(validPositions, position)
		}
	}

	if len(validPositions) == 0 {
		return nil, fmt.Errorf("no valid positions found for entity %s in gridless room after %d attempts",
			entity.GetType(), attempts)
	}

	return validPositions, nil
}

// getEntitiesByType filters existing entities by type.
func (cs *ConstraintSolver) getEntitiesByType(
	entities []SpawnedEntity, entityType string,
) []SpawnedEntity {
	var filtered []SpawnedEntity
	for _, entity := range entities {
		if string(entity.Entity.GetType()) == entityType { // Convert core.EntityType to string
			filtered = append(filtered, entity)
		}
	}
	return filtered
}
