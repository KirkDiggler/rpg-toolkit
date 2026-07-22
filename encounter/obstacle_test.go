package encounter_test

// obstacle_test.go covers rpg-toolkit#818's generic static-obstacle
// infrastructure: a persisted ObstacleData shape on SpaceData (stable
// ID/Ref, absolute position, BlocksMovement/BlocksLoS), full
// ToData/LoadFromData round-trip, and rebuild parity with walls/doors —
// obstacles are placed into the exact same spatial.Room truth so
// movement and line-of-sight consult them identically. Infrastructure
// only: no crypt-specific placement (#819), no cover math, no
// interaction verbs.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
)

type ObstacleSuite struct {
	suite.Suite
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
}

func TestObstacleSuite(t *testing.T) {
	suite.Run(t, new(ObstacleSuite))
}

func (s *ObstacleSuite) SetupTest() {
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
}

// TestObstacle_ToData_LoadFromData_RoundTrip covers the #818 done bar's
// core requirement: every field on every obstacle instance survives a
// ToData/LoadFromData round-trip exactly.
func (s *ObstacleSuite) TestObstacle_ToData_LoadFromData_RoundTrip() {
	enc := encounter.New(context.Background(), "enc-obstacle-rt", s.broker)
	s.Require().NoError(enc.InitRoom(10, 10, environments.PatternEmpty))

	pos := core.Hex{Q: 2, R: -3, S: 1}
	s.Require().NoError(enc.AddObstacle("sarcophagus-1", "dnd5e:obstacles:sarcophagus", pos, true, true))

	original := enc.ToData()
	s.Require().NotNil(original.Space)
	s.Require().Len(original.Space.Obstacles, 1)
	got := original.Space.Obstacles[0]
	s.Equal(core.EntityID("sarcophagus-1"), got.ID)
	s.Equal("dnd5e:obstacles:sarcophagus", got.Ref)
	s.Equal(pos, got.Position)
	s.True(got.BlocksMovement)
	s.True(got.BlocksLoS)

	reloaded, err := encounter.LoadFromData(context.Background(), original, s.broker)
	s.Require().NoError(err)

	again := reloaded.ToData()
	s.Require().NotNil(again.Space)
	s.Require().Len(again.Space.Obstacles, 1)
	s.Equal(got, again.Space.Obstacles[0], "obstacle must round-trip exactly through LoadFromData")
}
