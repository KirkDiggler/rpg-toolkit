package spawn

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// RoomWiringSuite covers rpg-toolkit#757: getRoomFromSpatial and
// placeEntityInRoom were both literal stubs (the first always errored, the
// second silently discarded the entity without ever calling
// room.PlaceEntity). This suite proves PopulateRoom's happy path now
// actually occupies a registered room, not just that it stops erroring.
type RoomWiringSuite struct {
	suite.Suite
	room     *spatial.BasicRoom
	orch     spatial.RoomOrchestrator
	engine   *BasicSpawnEngine
	registry *BasicSelectablesRegistry
}

func (s *RoomWiringSuite) SetupTest() {
	grid := spatial.NewHexGrid(spatial.HexGridConfig{
		Width:       10,
		Height:      10,
		Orientation: spatial.HexOrientationPointyTop,
	})
	s.room = spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "test-room",
		Type: "test",
		Grid: grid,
	})

	s.orch = spatial.NewBasicRoomOrchestrator(spatial.BasicRoomOrchestratorConfig{ID: "test-orch"})
	s.Require().NoError(s.orch.AddRoom(s.room))

	s.registry = NewBasicSelectablesRegistry()
	s.Require().NoError(s.registry.RegisterTable("goblins", []core.Entity{
		&MockEntity{id: "goblin-1", entityType: "monster"},
	}))

	s.engine = NewBasicSpawnEngine(BasicSpawnEngineConfig{
		ID:               "test-engine",
		SelectablesReg:   s.registry,
		RoomOrchestrator: s.orch,
		MaxAttempts:      10,
	})
}

// TestGetRoomFromSpatial_NoOrchestrator: an engine with no RoomOrchestrator
// configured still fails clearly, rather than panicking on a nil dereference.
func (s *RoomWiringSuite) TestGetRoomFromSpatial_NoOrchestrator() {
	bare := NewBasicSpawnEngine(BasicSpawnEngineConfig{SelectablesReg: s.registry})
	_, err := bare.getRoomFromSpatial("test-room")
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "no RoomOrchestrator")
}

// TestGetRoomFromSpatial_UnknownRoom: a configured orchestrator that simply
// doesn't have the requested room ID still errors clearly.
func (s *RoomWiringSuite) TestGetRoomFromSpatial_UnknownRoom() {
	_, err := s.engine.getRoomFromSpatial("no-such-room")
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "not found")
}

// TestPopulateRoom_HappyPath_PlacesEntityInRegisteredRoom is the regression
// test for #757's spawn-stub fix: PopulateRoom against a registered room
// must both report success AND leave the entity actually occupying the
// spatial room — before this fix, placeEntityInRoom silently no-op'd, so
// result.Success could be true while the room stayed empty.
func (s *RoomWiringSuite) TestPopulateRoom_HappyPath_PlacesEntityInRegisteredRoom() {
	one := 1
	config := SpawnConfig{
		EntityGroups: []EntityGroup{
			{
				ID:             "goblins",
				Type:           "monster",
				SelectionTable: "goblins",
				Quantity:       QuantitySpec{Fixed: &one},
			},
		},
		Pattern: PatternScattered,
	}

	result, err := s.engine.PopulateRoom(context.Background(), "test-room", config)
	s.Require().NoError(err)
	s.Assert().True(result.Success)
	s.Require().Len(result.SpawnedEntities, 1)
	s.Assert().Empty(result.Failures)

	spawned := result.SpawnedEntities[0]
	s.Assert().Equal("goblin-1", spawned.Entity.GetID())

	// The regression check: the entity is actually IN the room, not just
	// reported as spawned.
	roomEntities := s.room.GetAllEntities()
	s.Require().Contains(roomEntities, "goblin-1")
	placedAt, ok := s.room.GetEntityPosition("goblin-1")
	s.Require().True(ok)
	s.Assert().Equal(spawned.Position, placedAt)
}

func TestRoomWiringSuite(t *testing.T) {
	suite.Run(t, new(RoomWiringSuite))
}
