package spawn

import (
	"fmt"
	"math"
	"math/rand"

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
		random:      rand.New(rand.NewSource(42)), //nolint:gosec // Fixed seed for consistent testing
	}
}

// ValidatePosition checks if a position satisfies all spatial constraints.
// Purpose: Core constraint validation for entity placement.
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

// FindValidPositions finds all positions that satisfy constraints.
// Purpose: Generate valid placement options for constraint-aware spawning.
func (cs *ConstraintSolver) FindValidPositions(
	room spatial.Room, entity core.Entity, constraints SpatialConstraints,
	existingEntities []SpawnedEntity, maxPositions int,
) ([]spatial.Position, error) {
	var validPositions []spatial.Position

	// Check if room uses gridless positioning
	roomGrid := room.GetGrid()
	if cs.isGridlessRoom(roomGrid) {
		// For gridless rooms: use finer sampling for smooth positioning
		return cs.findValidPositionsGridless(room, entity, constraints, existingEntities, maxPositions)
	}

	// Get room dimensions from grid
	dimensions := roomGrid.GetDimensions()
	maxX := dimensions.Width
	maxY := dimensions.Height

	// For grid-based rooms: align to grid positions
	for x := 1.0; x < maxX && len(validPositions) < maxPositions; x += 1.0 {
		for y := 1.0; y < maxY && len(validPositions) < maxPositions; y += 1.0 {
			position := spatial.Position{X: x, Y: y}

			// Check if position is blocked by existing entities (walls, obstacles)
			if !room.CanPlaceEntity(entity, position) {
				continue
			}

			if cs.ValidatePosition(room, position, entity, constraints, existingEntities) == nil {
				validPositions = append(validPositions, position)
			}
		}
	}

	if len(validPositions) == 0 {
		return nil, fmt.Errorf("no valid positions found for entity %s with given constraints", entity.GetType())
	}

	return validPositions, nil
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

// validateWallProximity ensures entities maintain minimum distance from walls.
func (cs *ConstraintSolver) validateWallProximity(
	_ spatial.Room, position spatial.Position, minWallDistance float64,
) error {
	if minWallDistance <= 0 {
		return nil // No wall proximity constraint
	}

	// Phase 3: Simple boundary check
	// Real implementation would query spatial room for wall positions
	if position.X < minWallDistance || position.Y < minWallDistance {
		return fmt.Errorf("position too close to wall: (%.2f, %.2f), minimum distance %.2f",
			position.X, position.Y, minWallDistance)
	}

	// Check distance from right and top walls (assuming 10x10 room)
	if position.X > (10.0-minWallDistance) || position.Y > (10.0-minWallDistance) {
		return fmt.Errorf("position too close to wall: (%.2f, %.2f), minimum distance %.2f",
			position.X, position.Y, minWallDistance)
	}

	return nil
}

// validateLineOfSight ensures line of sight requirements are met.
func (cs *ConstraintSolver) validateLineOfSight(
	_ spatial.Room, position spatial.Position, entity core.Entity,
	losRules LineOfSightRules, existingEntities []SpawnedEntity,
) error {
	entityType := string(entity.GetType()) // Convert to string for comparisons

	// Check required sight relationships
	for _, pair := range losRules.RequiredSight {
		if pair.From == entityType {
			// This entity must see entities of pair.To type
			if err := cs.checkRequiredSight(position, pair.To, existingEntities); err != nil {
				return fmt.Errorf("required sight from %s to %s: %w", pair.From, pair.To, err)
			}
		}
		if pair.To == entityType {
			// Entities of pair.From type must see this entity
			if err := cs.checkCanBeSeen(position, pair.From, existingEntities); err != nil {
				return fmt.Errorf("required sight from %s to %s: %w", pair.From, pair.To, err)
			}
		}
	}

	// Check blocked sight relationships
	for _, pair := range losRules.BlockedSight {
		if pair.From == entityType {
			// This entity must NOT see entities of pair.To type
			if err := cs.checkBlockedSight(position, pair.To, existingEntities); err != nil {
				return fmt.Errorf("blocked sight from %s to %s: %w", pair.From, pair.To, err)
			}
		}
		if pair.To == entityType {
			// Entities of pair.From type must NOT see this entity
			if err := cs.checkCannotBeSeen(position, pair.From, existingEntities); err != nil {
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
	position spatial.Position, targetType string, existingEntities []SpawnedEntity,
) error {
	// Find entities of target type
	targetEntities := cs.getEntitiesByType(existingEntities, targetType)
	if len(targetEntities) == 0 {
		return nil // No target entities to check
	}

	// Check if we can see at least one target entity
	for _, target := range targetEntities {
		if cs.hasLineOfSight(position, target.Position) {
			return nil // Found at least one visible target
		}
	}

	return fmt.Errorf("no line of sight to any %s entities", targetType)
}

// checkCanBeSeen verifies that entities of fromType can see this position.
func (cs *ConstraintSolver) checkCanBeSeen(
	position spatial.Position, fromType string, existingEntities []SpawnedEntity,
) error {
	// Find entities of fromType
	fromEntities := cs.getEntitiesByType(existingEntities, fromType)
	if len(fromEntities) == 0 {
		return nil // No entities to check
	}

	// Check if at least one fromType entity can see this position
	for _, fromEntity := range fromEntities {
		if cs.hasLineOfSight(fromEntity.Position, position) {
			return nil // At least one entity can see this position
		}
	}

	return fmt.Errorf("no %s entities can see this position", fromType)
}

// checkBlockedSight verifies that position does NOT have line of sight to blocked entity types.
func (cs *ConstraintSolver) checkBlockedSight(
	position spatial.Position, targetType string, existingEntities []SpawnedEntity,
) error {
	// Find entities of target type
	targetEntities := cs.getEntitiesByType(existingEntities, targetType)

	// Check that we cannot see any target entity
	for _, target := range targetEntities {
		if cs.hasLineOfSight(position, target.Position) {
			return fmt.Errorf("has line of sight to %s at (%.2f, %.2f)",
				targetType, target.Position.X, target.Position.Y)
		}
	}

	return nil // No line of sight to any target entities
}

// checkCannotBeSeen verifies that entities of fromType cannot see this position.
func (cs *ConstraintSolver) checkCannotBeSeen(
	position spatial.Position, fromType string, existingEntities []SpawnedEntity,
) error {
	// Find entities of fromType
	fromEntities := cs.getEntitiesByType(existingEntities, fromType)

	// Check that no fromType entity can see this position
	for _, fromEntity := range fromEntities {
		if cs.hasLineOfSight(fromEntity.Position, position) {
			return fmt.Errorf("%s at (%.2f, %.2f) can see this position",
				fromType, fromEntity.Position.X, fromEntity.Position.Y)
		}
	}

	return nil // No entity can see this position
}

// hasLineOfSight checks if there's a clear line of sight between two positions.
func (cs *ConstraintSolver) hasLineOfSight(from, to spatial.Position) bool {
	// Phase 3: Simple line of sight calculation
	// Real implementation would use spatial room queries for wall intersections
	distance := cs.calculateDistance(from, to)
	return distance <= 8.0 // Assume 8-unit line of sight range
}

// calculateDistance computes Euclidean distance between two positions.
func (cs *ConstraintSolver) calculateDistance(pos1, pos2 spatial.Position) float64 {
	dx := pos1.X - pos2.X
	dy := pos1.Y - pos2.Y
	return math.Sqrt(dx*dx + dy*dy)
}

// isGridlessRoom checks if the room uses gridless positioning.
func (cs *ConstraintSolver) isGridlessRoom(grid spatial.Grid) bool {
	// Check if grid is nil (indicates gridless) or if it's specifically a gridless type
	if grid == nil {
		return true
	}
	// Could also check grid type, but nil check covers most cases
	return false
}

// findValidPositionsGridless finds valid positions for gridless rooms.
// Purpose: Optimized position finding for continuous/smooth positioning systems.
func (cs *ConstraintSolver) findValidPositionsGridless(
	room spatial.Room, entity core.Entity, constraints SpatialConstraints,
	existingEntities []SpawnedEntity, maxPositions int,
) ([]spatial.Position, error) {
	var validPositions []spatial.Position

	// For gridless rooms: use finer resolution sampling for smooth positioning
	// Also use random sampling to avoid predictable patterns
	roomGrid := room.GetGrid()
	var roomDimensions spatial.Dimensions
	if roomGrid != nil {
		roomDimensions = roomGrid.GetDimensions()
	} else {
		// Default dimensions for gridless rooms
		roomDimensions = spatial.Dimensions{Width: 10.0, Height: 10.0}
	}
	attempts := 0
	maxAttempts := cs.maxAttempts * 2 // More attempts for gridless

	for len(validPositions) < maxPositions && attempts < maxAttempts {
		// Generate random position within room bounds with some margin
		margin := 1.0
		x := margin + (roomDimensions.Width-2*margin)*cs.random.Float64()
		y := margin + (roomDimensions.Height-2*margin)*cs.random.Float64()
		position := spatial.Position{X: x, Y: y}

		// Check if position is blocked by existing entities (walls, obstacles)
		if !room.CanPlaceEntity(entity, position) {
			attempts++
			continue
		}

		if cs.ValidatePosition(room, position, entity, constraints, existingEntities) == nil {
			validPositions = append(validPositions, position)
		}
		attempts++
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
