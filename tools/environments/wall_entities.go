package environments

import (
	"fmt"
	"math"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// WallEntity represents a wall as a spatial entity
// Purpose: Integrates generated walls with spatial module's existing obstacle system
// by implementing the Placeable interface. This allows walls to automatically
// affect line of sight, movement, and pathfinding without duplicating logic.
type WallEntity struct {
	id         string
	segmentID  string           // ID of the original wall segment
	wallType   WallType         // Destructible, indestructible, etc.
	properties WallProperties   // Wall behavior properties
	position   spatial.Position // Single position this entity occupies

	// Destruction tracking
	currentHP int  // Current health points
	destroyed bool // Whether this wall segment has been destroyed
}

// WallEntityConfig configures wall entity creation
type WallEntityConfig struct {
	SegmentID  string
	WallType   WallType
	Properties WallProperties
	Position   spatial.Position
}

// NewWallEntity creates a new wall entity
func NewWallEntity(config WallEntityConfig) *WallEntity {
	entity := &WallEntity{
		id:         fmt.Sprintf("wall_%s_%d_%d", config.SegmentID, int(config.Position.X), int(config.Position.Y)),
		segmentID:  config.SegmentID,
		wallType:   config.WallType,
		properties: config.Properties,
		position:   config.Position,
		destroyed:  false,
	}

	// Set initial HP
	entity.currentHP = config.Properties.HP
	if entity.currentHP <= 0 {
		entity.currentHP = 10 // Default HP for walls without specific HP
	}

	return entity
}

// GetID returns the unique ID of this entity
func (w *WallEntity) GetID() string {
	return w.id
}

// GetType returns the type of this entity
func (w *WallEntity) GetType() core.EntityType {
	return core.EntityType("wall")
}

// GetSize returns the size of this entity
func (w *WallEntity) GetSize() int {
	return 1 // Each wall entity occupies a single position
}

// BlocksMovement checks if this wall blocks movement
func (w *WallEntity) BlocksMovement() bool {
	// Destroyed walls don't block movement
	if w.destroyed {
		return false
	}

	return w.properties.BlocksMovement
}

// BlocksLineOfSight checks if this wall blocks line of sight
func (w *WallEntity) BlocksLineOfSight() bool {
	// Destroyed walls don't block line of sight
	if w.destroyed {
		return false
	}

	return w.properties.BlocksLoS
}

// Wall-specific methods

// GetSegmentID returns the ID of the original wall segment
func (w *WallEntity) GetSegmentID() string {
	return w.segmentID
}

// GetWallType returns the wall type (destructible, indestructible, etc.)
func (w *WallEntity) GetWallType() WallType {
	return w.wallType
}

// GetProperties returns the wall properties
func (w *WallEntity) GetProperties() WallProperties {
	return w.properties
}

// GetPosition returns the position this wall entity occupies
func (w *WallEntity) GetPosition() spatial.Position {
	return w.position
}

// IsDestroyed returns whether this wall has been destroyed
func (w *WallEntity) IsDestroyed() bool {
	return w.destroyed
}

// GetCurrentHP returns the current health points
func (w *WallEntity) GetCurrentHP() int {
	return w.currentHP
}

// TakeDamage applies damage to the wall
func (w *WallEntity) TakeDamage(damage int, damageType string) bool {
	// Indestructible walls can't be damaged
	if w.wallType == WallTypeIndestructible {
		return false
	}

	// Check if wall is already destroyed
	if w.destroyed {
		return false
	}

	// Apply resistances and weaknesses
	actualDamage := w.calculateActualDamage(damage, damageType)

	// Apply damage
	w.currentHP -= actualDamage

	// Check if destroyed
	if w.currentHP <= 0 {
		w.destroyed = true
		return true // Wall was destroyed
	}

	return false // Wall was damaged but not destroyed
}

// Repair restores health points to the wall
func (w *WallEntity) Repair(healAmount int) {
	if w.destroyed {
		return // Can't repair destroyed walls
	}

	w.currentHP += healAmount

	// Cap at maximum HP
	if w.currentHP > w.properties.HP {
		w.currentHP = w.properties.HP
	}
}

// Destroy immediately destroys the wall (bypasses HP)
func (w *WallEntity) Destroy() {
	w.destroyed = true
	w.currentHP = 0
}

// calculateActualDamage applies resistances and weaknesses
func (w *WallEntity) calculateActualDamage(damage int, damageType string) int {
	actualDamage := float64(damage)

	// Check resistances
	for _, resistance := range w.properties.Resistance {
		if resistance == damageType {
			actualDamage *= 0.5 // 50% resistance
			break
		}
	}

	// Check weaknesses
	for _, weakness := range w.properties.Weakness {
		if weakness == damageType {
			actualDamage *= 2.0 // 200% damage (double damage)
			break
		}
	}

	return int(math.Max(1, actualDamage)) // Minimum 1 damage
}

// Wall segment conversion functions

// CreateWallEntities converts wall segments into wall entities
// Purpose: Discretizes wall segments into individual positioned entities
// that can be placed in spatial rooms for obstacle detection
func CreateWallEntities(walls []WallSegment) []spatial.Placeable {
	entities := make([]spatial.Placeable, 0, len(walls)*4) // Estimate 4 entities per wall

	for _, wall := range walls {
		wallEntities := discretizeWallSegment(wall)
		for _, entity := range wallEntities {
			entities = append(entities, entity)
		}
	}

	return entities
}

// discretizeWallSegment converts a wall segment into positioned entities
func discretizeWallSegment(wall WallSegment) []*WallEntity {
	// Calculate positions along the wall segment
	positions := calculateWallPositions(wall.Start, wall.End, wall.Properties.Thickness)

	entities := make([]*WallEntity, 0, len(positions))
	// Create a wall entity for each position
	for i, pos := range positions {
		config := WallEntityConfig{
			SegmentID:  fmt.Sprintf("%s_seg_%d", wall.Start.String(), i),
			WallType:   wall.Type,
			Properties: wall.Properties,
			Position:   pos,
		}

		entity := NewWallEntity(config)
		entities = append(entities, entity)
	}

	return entities
}

// calculateWallPositions calculates discrete positions for a wall segment
func calculateWallPositions(start, end spatial.Position, thickness float64) []spatial.Position {
	var positions []spatial.Position

	// Calculate wall length
	dx := end.X - start.X
	dy := end.Y - start.Y
	length := math.Sqrt(dx*dx + dy*dy)

	// If wall is very short, just use start position
	if length < 0.5 {
		return []spatial.Position{start}
	}

	// Calculate number of positions needed (one per unit length)
	numPositions := int(math.Ceil(length))
	if numPositions < 1 {
		numPositions = 1
	}

	// Calculate positions along the wall
	for i := 0; i < numPositions; i++ {
		t := float64(i) / float64(numPositions-1) // 0.0 to 1.0
		if numPositions == 1 {
			t = 0.0
		}

		x := start.X + t*dx
		y := start.Y + t*dy

		positions = append(positions, spatial.Position{X: x, Y: y})
	}

	// If wall has thickness > 1, add perpendicular positions
	if thickness > 1.0 {
		thickPositions := addWallThickness(positions, start, end, thickness)
		positions = thickPositions
	}

	return positions
}

// addWallThickness adds positions for thick walls
func addWallThickness(
	basePositions []spatial.Position, start, end spatial.Position, thickness float64,
) []spatial.Position {
	allPositions := make([]spatial.Position, 0, len(basePositions)*2) // Estimate 2 positions per base position

	// Calculate perpendicular direction
	dx := end.X - start.X
	dy := end.Y - start.Y
	length := math.Sqrt(dx*dx + dy*dy)

	if length == 0 {
		return basePositions
	}

	// Normalize and rotate 90 degrees for perpendicular
	perpX := -dy / length
	perpY := dx / length

	// Add positions on both sides of the wall
	thicknessSides := int(math.Floor(thickness))
	for _, basePos := range basePositions {
		// Add center position
		allPositions = append(allPositions, basePos)

		// Add positions on both sides
		for side := 1; side <= thicknessSides/2; side++ {
			offset := float64(side)

			// One side
			pos1 := spatial.Position{
				X: basePos.X + perpX*offset,
				Y: basePos.Y + perpY*offset,
			}
			allPositions = append(allPositions, pos1)

			// Other side
			pos2 := spatial.Position{
				X: basePos.X - perpX*offset,
				Y: basePos.Y - perpY*offset,
			}
			allPositions = append(allPositions, pos2)
		}
	}

	return allPositions
}

// Wall entity management functions

// FindWallEntitiesBySegment finds all wall entities that belong to a specific segment
func FindWallEntitiesBySegment(entities []spatial.Placeable, segmentID string) []*WallEntity {
	var wallEntities []*WallEntity

	for _, entity := range entities {
		if wallEntity, ok := entity.(*WallEntity); ok {
			if wallEntity.GetSegmentID() == segmentID {
				wallEntities = append(wallEntities, wallEntity)
			}
		}
	}

	return wallEntities
}

// DestroyWallSegment destroys all wall entities belonging to a segment
func DestroyWallSegment(room spatial.Room, segmentID string) error {
	// Get all entities in the room
	allEntities := room.GetAllEntities()

	// Find and remove wall entities with matching segment ID
	for _, entity := range allEntities {
		if wallEntity, ok := entity.(*WallEntity); ok {
			if wallEntity.GetSegmentID() == segmentID {
				// Remove from room
				err := room.RemoveEntity(wallEntity.GetID())
				if err != nil {
					return fmt.Errorf("failed to remove wall entity %s: %w", wallEntity.GetID(), err)
				}
			}
		}
	}

	return nil
}

// GetWallEntitiesInRoom returns all wall entities in a room
func GetWallEntitiesInRoom(room spatial.Room) []*WallEntity {
	var wallEntities []*WallEntity

	allEntities := room.GetAllEntities()
	for _, entity := range allEntities {
		if wallEntity, ok := entity.(*WallEntity); ok {
			wallEntities = append(wallEntities, wallEntity)
		}
	}

	return wallEntities
}

// RepairWallSegment repairs all wall entities belonging to a segment
func RepairWallSegment(room spatial.Room, segmentID string, healAmount int) error {
	wallEntities := GetWallEntitiesInRoom(room)

	for _, wallEntity := range wallEntities {
		if wallEntity.GetSegmentID() == segmentID {
			wallEntity.Repair(healAmount)
		}
	}

	return nil
}

// GetWallSegmentHealth returns the health status of a wall segment
func GetWallSegmentHealth(room spatial.Room, segmentID string) (current, maximum int, destroyed bool) {
	wallEntities := GetWallEntitiesInRoom(room)

	totalCurrent := 0
	totalMax := 0
	allDestroyed := true
	found := false

	for _, wallEntity := range wallEntities {
		if wallEntity.GetSegmentID() == segmentID {
			found = true
			totalCurrent += wallEntity.GetCurrentHP()
			totalMax += wallEntity.GetProperties().HP

			if !wallEntity.IsDestroyed() {
				allDestroyed = false
			}
		}
	}

	if !found {
		return 0, 0, true // Segment not found, consider destroyed
	}

	return totalCurrent, totalMax, allDestroyed
}
