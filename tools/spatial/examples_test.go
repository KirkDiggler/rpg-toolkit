package spatial_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TestSpatialSystem demonstrates how to use the spatial system
func TestSpatialSystem(t *testing.T) {
	// Setup event bus
	eventBus := events.NewBus()

	// Create a square grid room
	grid := spatial.NewSquareGrid(spatial.SquareGridConfig{
		Width:  20,
		Height: 20,
	})

	room := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:       "dungeon-room-1",
		Type:     "dungeon",
		Grid:     grid,
		EventBus: eventBus,
	})

	// Setup query system
	queryHandler := spatial.NewSpatialQueryHandler()
	queryHandler.RegisterRoom(room)
	queryHandler.RegisterWithEventBus(eventBus)

	// Create query utilities for convenience
	queryUtils := spatial.NewQueryUtils(eventBus)

	// Create some entities
	hero := NewMockEntity("hero", "character")
	orc := NewMockEntity("orc", "monster")
	goblin := NewMockEntity("goblin", "monster")
	treasure := NewMockEntity("treasure", "item")
	wall := NewMockEntity("wall", "wall").WithBlocking(true, true)

	// Place entities in the room
	err := room.PlaceEntity(hero, spatial.Position{X: 5, Y: 5})
	require.NoError(t, err)

	err = room.PlaceEntity(orc, spatial.Position{X: 10, Y: 10})
	require.NoError(t, err)

	err = room.PlaceEntity(goblin, spatial.Position{X: 12, Y: 8})
	require.NoError(t, err)

	err = room.PlaceEntity(treasure, spatial.Position{X: 15, Y: 15})
	require.NoError(t, err)

	err = room.PlaceEntity(wall, spatial.Position{X: 7, Y: 7})
	require.NoError(t, err)

	// Example 1: Find all entities within 5 units of the hero
	ctx := context.Background()
	entitiesNearHero, err := queryUtils.QueryEntitiesInRange(
		ctx,
		spatial.Position{X: 5, Y: 5},
		5.0,
		"dungeon-room-1",
		nil, // no filter
	)
	require.NoError(t, err)

	t.Logf("Entities near hero: %d", len(entitiesNearHero))
	for _, entity := range entitiesNearHero {
		t.Logf("  - %s (%s)", entity.GetID(), entity.GetType())
	}

	// Example 2: Find only monsters within 10 units of the hero
	monstersNearHero, err := queryUtils.QueryEntitiesInRange(
		ctx,
		spatial.Position{X: 5, Y: 5},
		10.0,
		"dungeon-room-1",
		spatial.CreateMonsterFilter(),
	)
	require.NoError(t, err)

	t.Logf("Monsters near hero: %d", len(monstersNearHero))
	for _, monster := range monstersNearHero {
		t.Logf("  - %s (%s)", monster.GetID(), monster.GetType())
	}

	// Example 3: Check line of sight from hero to orc
	losPositions, blocked, err := queryUtils.QueryLineOfSight(
		ctx,
		spatial.Position{X: 5, Y: 5},
		spatial.Position{X: 10, Y: 10},
		"dungeon-room-1",
	)
	require.NoError(t, err)

	t.Logf("Line of sight from hero to orc:")
	t.Logf("  - Positions: %v", losPositions)
	t.Logf("  - Blocked: %v", blocked)

	// Example 4: Check if hero can move to a specific position
	canMove, path, distance, err := queryUtils.QueryMovement(
		ctx,
		hero,
		spatial.Position{X: 5, Y: 5},
		spatial.Position{X: 8, Y: 8},
		"dungeon-room-1",
	)
	require.NoError(t, err)

	t.Logf("Hero movement query:")
	t.Logf("  - Can move: %v", canMove)
	t.Logf("  - Distance: %.2f", distance)
	t.Logf("  - Path: %v", path)

	// Example 5: Check if we can place a new entity
	newEntity := NewMockEntity("new-character", "character")
	canPlace, err := queryUtils.QueryPlacement(
		ctx,
		newEntity,
		spatial.Position{X: 6, Y: 6},
		"dungeon-room-1",
	)
	require.NoError(t, err)

	t.Logf("Can place new entity at (6,6): %v", canPlace)

	// Example 6: Find all valid positions within range
	validPositions, err := queryUtils.QueryPositionsInRange(
		ctx,
		spatial.Position{X: 5, Y: 5},
		3.0,
		"dungeon-room-1",
	)
	require.NoError(t, err)

	t.Logf("Valid positions within 3 units of hero: %d", len(validPositions))

	// Example 7: Using filters to find specific entities
	// Find all combatants (characters + monsters) excluding the hero
	combatants, err := queryUtils.QueryEntitiesInRange(
		ctx,
		spatial.Position{X: 10, Y: 10},
		15.0,
		"dungeon-room-1",
		spatial.CreateCombatantFilter(),
	)
	require.NoError(t, err)

	t.Logf("All combatants: %d", len(combatants))
	for _, combatant := range combatants {
		t.Logf("  - %s (%s)", combatant.GetID(), combatant.GetType())
	}

	// Example 8: Complex filter - find monsters but exclude goblins
	monstersExcludingGoblins, err := queryUtils.QueryEntitiesInRange(
		ctx,
		spatial.Position{X: 10, Y: 10},
		15.0,
		"dungeon-room-1",
		spatial.NewSimpleEntityFilter().
			WithEntityTypes("monster").
			WithExcludeIDs("goblin"),
	)
	require.NoError(t, err)

	t.Logf("Monsters excluding goblins: %d", len(monstersExcludingGoblins))
	for _, monster := range monstersExcludingGoblins {
		t.Logf("  - %s (%s)", monster.GetID(), monster.GetType())
	}

	// Assertions to verify the examples work correctly
	assert.True(t, len(entitiesNearHero) > 0, "Should find entities near hero")
	assert.True(t, len(monstersNearHero) > 0, "Should find monsters near hero")
	assert.True(t, len(losPositions) > 0, "Should return line of sight positions")
	assert.True(t, distance > 0, "Movement distance should be positive")
	assert.True(t, len(validPositions) > 0, "Should find valid positions")
	assert.True(t, len(combatants) > 0, "Should find combatants")
}

// TestGridComparison demonstrates different distance calculations
func TestGridComparison(t *testing.T) {
	// Create three different grid types
	squareGrid := spatial.NewSquareGrid(spatial.SquareGridConfig{
		Width:  10,
		Height: 10,
	})

	hexGrid := spatial.NewHexGrid(spatial.HexGridConfig{
		Width:     10,
		Height:    10,
		PointyTop: true,
	})

	gridlessGrid := spatial.NewGridlessRoom(spatial.GridlessConfig{
		Width:  10,
		Height: 10,
	})

	// Test positions
	from := spatial.Position{X: 2, Y: 2}
	to := spatial.Position{X: 6, Y: 6}

	// Compare distance calculations
	squareDistance := squareGrid.Distance(from, to)
	hexDistance := hexGrid.Distance(from, to)
	gridlessDistance := gridlessGrid.Distance(from, to)

	t.Logf("Distance from %v to %v:", from, to)
	t.Logf("  Square Grid (Chebyshev): %.2f", squareDistance)
	t.Logf("  Hex Grid (Cube): %.2f", hexDistance)
	t.Logf("  Gridless (Euclidean): %.2f", gridlessDistance)

	// Test neighbors
	squareNeighbors := squareGrid.GetNeighbors(from)
	hexNeighbors := hexGrid.GetNeighbors(from)
	gridlessNeighbors := gridlessGrid.GetNeighbors(from)

	t.Logf("Neighbors of %v:", from)
	t.Logf("  Square Grid: %d neighbors", len(squareNeighbors))
	t.Logf("  Hex Grid: %d neighbors", len(hexNeighbors))
	t.Logf("  Gridless: %d neighbors", len(gridlessNeighbors))

	// Verify different distance calculations
	assert.NotEqual(t, squareDistance, hexDistance, "Square and hex should have different distances")
	assert.NotEqual(t, squareDistance, gridlessDistance, "Square and gridless should have different distances")
	assert.NotEqual(t, hexDistance, gridlessDistance, "Hex and gridless should have different distances")

	// Verify neighbor counts
	assert.Equal(t, 8, len(squareNeighbors), "Square grid should have 8 neighbors")
	assert.Equal(t, 6, len(hexNeighbors), "Hex grid should have 6 neighbors")
	assert.Equal(t, 8, len(gridlessNeighbors), "Gridless should have 8 neighbors")
}

// TestEventIntegration demonstrates event-driven spatial interactions
func TestEventIntegration(t *testing.T) {
	// Setup
	eventBus := events.NewBus()

	// Track events
	var capturedEvents []events.Event

	// Subscribe to all spatial events
	eventBus.SubscribeFunc(spatial.EventEntityPlaced, 0, func(_ context.Context, event events.Event) error {
		capturedEvents = append(capturedEvents, event)
		t.Logf("Entity placed: %s", event.Source().GetID())
		return nil
	})

	eventBus.SubscribeFunc(spatial.EventEntityMoved, 0, func(_ context.Context, event events.Event) error {
		capturedEvents = append(capturedEvents, event)
		t.Logf("Entity moved: %s", event.Source().GetID())
		return nil
	})

	eventBus.SubscribeFunc(spatial.EventEntityRemoved, 0, func(_ context.Context, event events.Event) error {
		capturedEvents = append(capturedEvents, event)
		t.Logf("Entity removed: %s", event.Source().GetID())
		return nil
	})

	// Create room
	grid := spatial.NewSquareGrid(spatial.SquareGridConfig{
		Width:  10,
		Height: 10,
	})

	room := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:       "event-room",
		Type:     "test",
		Grid:     grid,
		EventBus: eventBus,
	})

	// Create entity
	entity := NewMockEntity("test-entity", "character")

	// Perform spatial operations that trigger events
	err := room.PlaceEntity(entity, spatial.Position{X: 3, Y: 3})
	require.NoError(t, err)

	err = room.MoveEntity(entity.GetID(), spatial.Position{X: 5, Y: 5})
	require.NoError(t, err)

	err = room.RemoveEntity(entity.GetID())
	require.NoError(t, err)

	// Verify events were captured
	assert.True(t, len(capturedEvents) >= 3, "Should capture placement, movement, and removal events")

	// Verify event types
	eventTypes := make(map[string]bool)
	for _, event := range capturedEvents {
		eventTypes[event.Type()] = true
	}

	assert.True(t, eventTypes[spatial.EventEntityPlaced], "Should have placement event")
	assert.True(t, eventTypes[spatial.EventEntityMoved], "Should have movement event")
	assert.True(t, eventTypes[spatial.EventEntityRemoved], "Should have removal event")

	t.Logf("Captured %d events", len(capturedEvents))
}

// Run all examples as tests
func TestExamples(t *testing.T) {
	t.Run("SpatialSystem", TestSpatialSystem)
	t.Run("GridComparison", TestGridComparison)
	t.Run("EventIntegration", TestEventIntegration)
}
