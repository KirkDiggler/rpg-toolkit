package spawn

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// blockingEntity is a minimal core.Entity + spatial.Placeable used to plant
// walls in a real spatial.Room. Room-awareness only means something against
// actual occupying entities that block movement/sight — spawn has no
// separate "wall" concept of its own.
type blockingEntity struct {
	id                string
	blocksMovement    bool
	blocksLineOfSight bool
}

func (b *blockingEntity) GetID() string            { return b.id }
func (b *blockingEntity) GetType() core.EntityType { return "wall" }
func (b *blockingEntity) GetSize() int             { return 1 }
func (b *blockingEntity) BlocksMovement() bool     { return b.blocksMovement }
func (b *blockingEntity) BlocksLineOfSight() bool  { return b.blocksLineOfSight }

// RoomAwareSuite covers rpg-toolkit#760: position search now queries the
// real spatial.Room instead of a hardcoded [0,10) sweep / distance stub.
type RoomAwareSuite struct {
	suite.Suite
}

func (s *RoomAwareSuite) newRoom(id string, width, height float64) *spatial.BasicRoom {
	grid := spatial.NewSquareGrid(spatial.SquareGridConfig{Width: width, Height: height})
	return spatial.NewBasicRoom(spatial.BasicRoomConfig{ID: id, Type: "test", Grid: grid})
}

// TestFindValidPositions_BoundsAndWalls pins the core #760 ask: every
// candidate FindValidPositions returns is (a) within the room's real
// dimensions and (b) never on a cell a wall occupies. Requesting every free
// cell in a small room with a few walls planted and checking the result is
// exactly "all cells minus the walls" also proves the search doesn't
// overshoot or undershoot the real room.
func (s *RoomAwareSuite) TestFindValidPositions_BoundsAndWalls() {
	room := s.newRoom("bounded-room", 5, 5) // 25 cells total
	wallPositions := []spatial.Position{{X: 0, Y: 0}, {X: 4, Y: 4}, {X: 2, Y: 2}}
	for i, pos := range wallPositions {
		wall := &blockingEntity{id: fmt.Sprintf("wall-%d", i), blocksMovement: true}
		s.Require().NoError(room.PlaceEntity(wall, pos))
	}

	solver := NewConstraintSolver()
	entity := &MockEntity{id: "goblin", entityType: "monster"}

	positions, err := solver.FindValidPositions(room, entity, SpatialConstraints{}, nil, 100, nil, nil)
	s.Require().NoError(err)

	// 25 cells - 3 walls = 22 free cells; maxPositions=100 asks for more
	// than exist, so this also proves the search doesn't overshoot.
	s.Assert().Len(positions, 22)

	seen := make(map[spatial.Position]bool)
	for _, pos := range positions {
		s.Assert().GreaterOrEqual(pos.X, 0.0)
		s.Assert().Less(pos.X, 5.0)
		s.Assert().GreaterOrEqual(pos.Y, 0.0)
		s.Assert().Less(pos.Y, 5.0)
		for _, wallPos := range wallPositions {
			s.Assert().NotEqual(wallPos, pos, "position must not land on a wall cell")
		}
		s.Assert().False(seen[pos], "position returned twice")
		seen[pos] = true
	}
}

// TestLineOfSight_WallAware pins the #760 ask that LineOfSight constraints
// use the room's actual wall-aware Room.IsLineOfSightBlocked instead of the
// old Euclidean-distance stub (distance <= 8.0 == "visible", walls not
// considered at all). Two candidates sit at the SAME distance (8 units)
// from a viewer: one straight line crosses a sight-blocking wall, the other
// doesn't come near it. The old stub could not tell them apart — both were
// "visible" at distance 8. The wall-aware check must.
func (s *RoomAwareSuite) TestLineOfSight_WallAware() {
	room := s.newRoom("los-room", 10, 10)
	wall := &blockingEntity{id: "wall", blocksLineOfSight: true}
	s.Require().NoError(room.PlaceEntity(wall, spatial.Position{X: 5, Y: 1}))

	solver := NewConstraintSolver()
	goblin := &MockEntity{id: "goblin", entityType: "monster"}
	viewer := SpawnedEntity{
		Entity:   &MockEntity{id: "player-1", entityType: "player"},
		Position: spatial.Position{X: 1, Y: 1},
	}
	constraints := SpatialConstraints{
		LineOfSight: LineOfSightRules{
			BlockedSight: []EntityPair{{From: "monster", To: "player"}},
		},
	}

	// Straight line from the viewer along y=1 crosses the wall at (5,1):
	// hidden, distance from viewer is exactly 8.
	behindWall := spatial.Position{X: 9, Y: 1}
	s.Assert().NoError(
		solver.ValidatePosition(room, behindWall, goblin, constraints, []SpawnedEntity{viewer}),
		"position behind the wall must be accepted as hidden",
	)

	// Straight line from the viewer along x=1 never approaches the wall:
	// visible, also at distance exactly 8 from the viewer.
	clearView := spatial.Position{X: 1, Y: 9}
	err := solver.ValidatePosition(room, clearView, goblin, constraints, []SpawnedEntity{viewer})
	s.Require().Error(err, "position with a clear line to the viewer must be rejected")
	s.Assert().Contains(err.Error(), "line of sight")
}

// TestPositionOracle_ComposesWithSearch pins the rpg-api#644 use case
// #760 exists for: a caller expresses a placement requirement the
// constraint vocabulary doesn't cover (e.g. "not visible to any viewer") as
// a PositionOracle instead of discarding the search's chosen position and
// recomputing placement itself. The oracle must AND with room-derived
// validity, not replace it — proven here by planting a wall on a cell the
// oracle accepts: if the oracle replaced room validity instead of composing
// with it, that walled cell would still come back.
func (s *RoomAwareSuite) TestPositionOracle_ComposesWithSearch() {
	room := s.newRoom("oracle-room", 4, 1) // a thin corridor: x in [0,4), y=0
	wall := &blockingEntity{id: "wall", blocksMovement: true}
	s.Require().NoError(room.PlaceEntity(wall, spatial.Position{X: 1, Y: 0}))

	solver := NewConstraintSolver()
	entity := &MockEntity{id: "goblin", entityType: "monster"}

	// Oracle accepts both x==1 (walled) and x==3 (free).
	oracle := PositionOracle(func(pos spatial.Position) bool {
		return pos.X == 1 || pos.X == 3
	})

	positions, err := solver.FindValidPositions(room, entity, SpatialConstraints{}, nil, 10, oracle, nil)
	s.Require().NoError(err)
	s.Require().Len(positions, 1, "the walled cell must be excluded even though the oracle accepts it")
	s.Assert().Equal(spatial.Position{X: 3, Y: 0}, positions[0])
}

// TestFindValidPositions_AxialGrid pins the #770 review fix: enumeration
// must work correctly on grids centered on zero (AxialHexGrid, valid range
// [-span/2, span/2) on both axes), not just 0-based offset grids — the
// search must find every valid negative-coordinate cell, not silently skip
// half the grid because it assumed a 0-based origin.
func (s *RoomAwareSuite) TestFindValidPositions_AxialGrid() {
	grid := spatial.NewAxialHexGrid(spatial.AxialHexGridConfig{SpanWidth: 4, SpanHeight: 4}) // Q,R in [-2,2)
	room := spatial.NewBasicRoom(spatial.BasicRoomConfig{ID: "axial-room", Type: "test", Grid: grid})

	solver := NewConstraintSolver()
	entity := &MockEntity{id: "goblin", entityType: "monster"}

	positions, err := solver.FindValidPositions(room, entity, SpatialConstraints{}, nil, 100, nil, nil)
	s.Require().NoError(err)

	seen := make(map[spatial.Position]bool)
	for _, pos := range positions {
		s.Assert().GreaterOrEqual(pos.X, -2.0)
		s.Assert().Less(pos.X, 2.0)
		s.Assert().GreaterOrEqual(pos.Y, -2.0)
		s.Assert().Less(pos.Y, 2.0)
		seen[pos] = true
	}
	s.Assert().Len(seen, 16, "every one of the 4x4 axial cells must be found, including negative coordinates")
}

// TestWallProximity_AxialGridNegativeCoordinates pins the real #770 review
// bug: validateWallProximity used to assume the room boundary sits at
// 0..Width/Height, so any negative coordinate on an AxialHexGrid (valid
// range centered on zero) always failed the boundary check regardless of
// its actual distance from a wall.
func (s *RoomAwareSuite) TestWallProximity_AxialGridNegativeCoordinates() {
	grid := spatial.NewAxialHexGrid(spatial.AxialHexGridConfig{SpanWidth: 10, SpanHeight: 10}) // Q,R in [-5,5)
	room := spatial.NewBasicRoom(spatial.BasicRoomConfig{ID: "axial-room", Type: "test", Grid: grid})

	solver := NewConstraintSolver()
	entity := &MockEntity{id: "goblin", entityType: "monster"}
	constraints := SpatialConstraints{WallProximity: 1.0}

	// Well inside the valid [-5,5) range on both axes — must NOT be
	// rejected just because its coordinates are negative.
	interior := spatial.Position{X: -3, Y: -3}
	s.Assert().NoError(
		solver.ValidatePosition(room, interior, entity, constraints, nil),
		"a negative-coordinate position well inside an axial grid must pass wall-proximity",
	)

	// Within 1.0 of the real boundary at X=-5 — must be rejected, proving
	// the check uses the grid's actual bound, not just "always pass
	// negative coordinates."
	tooClose := spatial.Position{X: -4.5, Y: -3}
	s.Assert().Error(
		solver.ValidatePosition(room, tooClose, entity, constraints, nil),
		"a position within minWallDistance of the axial grid's real boundary must be rejected",
	)
}

func TestRoomAwareSuite(t *testing.T) {
	suite.Run(t, new(RoomAwareSuite))
}

// FixedPositionSuite covers rpg-toolkit#760's fixed-position injection: a
// caller that already knows where an entity belongs (e.g. rpg-api's
// safeGoblinHexes workaround) can hand SpawnConfig the exact positions
// instead of taking whatever the search would have chosen and discarding
// it (rpg-api PR #645's KNOWN TOOLKIT GAP).
type FixedPositionSuite struct {
	suite.Suite
	room     *spatial.BasicRoom
	engine   *BasicSpawnEngine
	registry *BasicSelectablesRegistry
}

func (s *FixedPositionSuite) SetupTest() {
	grid := spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 10, Height: 10})
	s.room = spatial.NewBasicRoom(spatial.BasicRoomConfig{ID: "fixed-room", Type: "test", Grid: grid})

	orch := spatial.NewBasicRoomOrchestrator(spatial.BasicRoomOrchestratorConfig{ID: "orch"})
	s.Require().NoError(orch.AddRoom(s.room))

	s.registry = NewBasicSelectablesRegistry()
	s.Require().NoError(s.registry.RegisterTable("goblins", []core.Entity{
		&MockEntity{id: "goblin-1", entityType: "monster"},
		&MockEntity{id: "goblin-2", entityType: "monster"},
		&MockEntity{id: "goblin-3", entityType: "monster"},
	}))
	// Separate single-entity tables so a 1-per-group selection can never
	// sample the same underlying entity twice (BasicSelectablesRegistry
	// samples with replacement) — this test cares about each entity landing
	// at its own fixed position, and Room keys occupancy by entity ID, so a
	// duplicate-entity selection would collapse to one occupant.
	for _, id := range []string{"solo-1", "solo-2", "solo-3"} {
		s.Require().NoError(s.registry.RegisterTable(id, []core.Entity{
			&MockEntity{id: id, entityType: "monster"},
		}))
	}

	s.engine = NewBasicSpawnEngine(BasicSpawnEngineConfig{
		ID:               "fixed-position-engine",
		SelectablesReg:   s.registry,
		RoomOrchestrator: orch,
		MaxAttempts:      10,
	})
}

// TestFixedPositions_UsedVerbatim proves fixed positions bypass search
// entirely and land the entity exactly where the caller specified, in the
// real spatial room.
func (s *FixedPositionSuite) TestFixedPositions_UsedVerbatim() {
	one := 1
	fixed := []spatial.Position{{X: 2, Y: 2}, {X: 7, Y: 3}, {X: 4, Y: 8}}
	tables := []string{"solo-1", "solo-2", "solo-3"}
	groups := make([]EntityGroup, len(fixed))
	for i, table := range tables {
		groups[i] = EntityGroup{
			ID:             table,
			Type:           "monster",
			SelectionTable: table,
			Quantity:       QuantitySpec{Fixed: &one},
			FixedPositions: []spatial.Position{fixed[i]},
		}
	}
	config := SpawnConfig{
		EntityGroups: groups,
		Pattern:      PatternScattered,
	}

	result, err := s.engine.PopulateRoom(context.Background(), "fixed-room", config)
	s.Require().NoError(err)
	s.Assert().Empty(result.Failures)
	s.Require().Len(result.SpawnedEntities, 3)

	for i, spawned := range result.SpawnedEntities {
		s.Assert().Equal(fixed[i], spawned.Position)
		placedAt, ok := s.room.GetEntityPosition(spawned.Entity.GetID())
		s.Require().True(ok)
		s.Assert().Equal(fixed[i], placedAt)
	}
}

// TestFixedPositions_InvalidOneReportsFailureNotCrash proves an
// out-of-bounds fixed position is caught against the real room and
// reported as a SpawnFailure, not trusted blindly.
func (s *FixedPositionSuite) TestFixedPositions_InvalidOneReportsFailureNotCrash() {
	quantity := 1
	fixed := []spatial.Position{{X: 99, Y: 99}} // out of bounds for a 10x10 room
	config := SpawnConfig{
		EntityGroups: []EntityGroup{{
			ID:             "goblins",
			Type:           "monster",
			SelectionTable: "goblins",
			Quantity:       QuantitySpec{Fixed: &quantity},
			FixedPositions: fixed,
		}},
		Pattern: PatternScattered,
	}

	result, err := s.engine.PopulateRoom(context.Background(), "fixed-room", config)
	s.Require().NoError(err)
	s.Assert().Empty(result.SpawnedEntities)
	s.Require().Len(result.Failures, 1)
	s.Assert().Contains(result.Failures[0].Reason, "not placeable")
}

func TestFixedPositionSuite(t *testing.T) {
	suite.Run(t, new(FixedPositionSuite))
}

// TestSeed_ReproducesPositions pins SpawnConfig.Seed: two PopulateRoom
// calls against identically-shaped fresh rooms with the same Seed produce
// identical positions. Uses two separate rooms because PopulateRoom
// mutates room occupancy, so a second call against the SAME room would see
// different (already-occupied) state, not prove reproducibility.
func TestSeed_ReproducesPositions(t *testing.T) {
	newEngine := func(roomID string) *BasicSpawnEngine {
		grid := spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 10, Height: 10})
		room := spatial.NewBasicRoom(spatial.BasicRoomConfig{ID: roomID, Type: "test", Grid: grid})
		orchID := spatial.OrchestratorID(roomID + "-orch")
		orch := spatial.NewBasicRoomOrchestrator(spatial.BasicRoomOrchestratorConfig{ID: orchID})
		require.NoError(t, orch.AddRoom(room))
		registry := NewBasicSelectablesRegistry()
		require.NoError(t, registry.RegisterTable("goblins", []core.Entity{
			&MockEntity{id: "goblin-1", entityType: "monster"},
			&MockEntity{id: "goblin-2", entityType: "monster"},
		}))
		return NewBasicSpawnEngine(BasicSpawnEngineConfig{
			ID: roomID + "-engine", SelectablesReg: registry, RoomOrchestrator: orch, MaxAttempts: 10,
		})
	}

	seed := int64(12345)
	quantity := 2
	config := SpawnConfig{
		EntityGroups: []EntityGroup{{
			ID: "goblins", Type: "monster", SelectionTable: "goblins",
			Quantity: QuantitySpec{Fixed: &quantity},
		}},
		Pattern: PatternScattered,
		Seed:    &seed,
	}

	resultA, err := newEngine("room-a").PopulateRoom(context.Background(), "room-a", config)
	require.NoError(t, err)

	resultB, err := newEngine("room-b").PopulateRoom(context.Background(), "room-b", config)
	require.NoError(t, err)

	require.Len(t, resultA.SpawnedEntities, 2)
	require.Len(t, resultB.SpawnedEntities, 2)
	for i := range resultA.SpawnedEntities {
		assert.Equal(t, resultA.SpawnedEntities[i].Position, resultB.SpawnedEntities[i].Position,
			"same Seed must reproduce the same searched position")
	}
}
