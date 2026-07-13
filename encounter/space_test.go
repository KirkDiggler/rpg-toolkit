package encounter_test

// space_test.go covers rpg-toolkit#757's SpaceData snapshot: InitRoom's
// ToData()/LoadFromData round-trip, and wall-blocked movement via a real
// room built through environments.QuickRoom (not a hand-rolled stub).

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
)

type SpaceSuite struct {
	suite.Suite
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
}

func TestSpaceSuite(t *testing.T) {
	suite.Run(t, new(SpaceSuite))
}

func (s *SpaceSuite) SetupTest() {
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
}

// firstBlockingWall returns the hex of the first wall segment that blocks
// movement, from an encounter's persisted Space snapshot. Fails the test if
// none exists — environments.QuickRoom with PatternRandom is deterministic
// (unseeded => RandomSeed 0), so a 10x10 room reliably generates walls.
func (s *SpaceSuite) firstBlockingWall(enc *encounter.Encounter) core.Hex {
	space := enc.ToData().Space
	s.Require().NotNil(space)
	for _, w := range space.Walls {
		if w.BlocksMovement {
			return core.HexFromCube(w.Start)
		}
	}
	s.FailNow("no movement-blocking wall found in generated room")
	return core.Hex{}
}

func (s *SpaceSuite) TestInitRoom_ToData_LoadFromData_RoundTrip() {
	enc := encounter.New(context.Background(), "enc-space-rt", s.broker)
	s.Require().NoError(enc.InitRoom(10, 10, environments.PatternRandom))
	s.Require().NotNil(enc.Room())
	s.Require().NotNil(enc.RoomOrchestrator())

	original := enc.ToData()
	s.Require().NotNil(original.Space)
	s.NotEmpty(original.Space.Walls)
	s.Equal(10, original.Space.Width)
	s.Equal(10, original.Space.Height)

	reloaded, err := encounter.LoadFromData(context.Background(), original, s.broker)
	s.Require().NoError(err)
	s.Require().NotNil(reloaded.Room(), "LoadFromData must rebuild the room from the Space snapshot")
	s.Require().NotNil(reloaded.RoomOrchestrator())

	// The reloaded room's wall count must match the snapshot exactly — a
	// replay, not a re-roll (SpaceData's whole reason to exist).
	roomFromOrch, ok := reloaded.RoomOrchestrator().GetRoom(string(reloaded.ID()))
	s.Require().True(ok)
	wallEntities := environments.GetWallEntitiesInRoom(roomFromOrch)
	s.Len(wallEntities, len(original.Space.Walls))

	// A second ToData() after the round-trip must reproduce the same
	// wall snapshot (not drift/regenerate).
	again := reloaded.ToData()
	s.Require().NotNil(again.Space)
	s.Equal(len(original.Space.Walls), len(again.Space.Walls))
}

func (s *SpaceSuite) TestLoadFromData_NilSpace_NoRoom() {
	enc := encounter.New(context.Background(), "enc-space-nil", s.broker)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: core.Hex{Q: 0, R: 0, S: 0}, SightRange: 5,
	}))
	s.Nil(enc.Room(), "InitRoom never called — pre-wave-1 fixture shape")

	data := enc.ToData()
	s.Nil(data.Space)

	reloaded, err := encounter.LoadFromData(context.Background(), data, s.broker)
	s.Require().NoError(err)
	s.Nil(reloaded.Room(), "nil Space must round-trip to nil Room, not panic or fabricate one")
}

func (s *SpaceSuite) TestMove_BlockedByWall_TruncatesAtFirstBlockedHex() {
	enc := encounter.New(context.Background(), "enc-space-move", s.broker)
	s.Require().NoError(enc.InitRoom(10, 10, environments.PatternRandom))

	wallHex := s.firstBlockingWall(enc)
	// Approach the wall from one of its neighbors so a single-hex move lands
	// exactly on it.
	start := perception.HexNeighbors(wallHex)[0]

	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: start, SightRange: 10,
	}))

	err := enc.Move("alice", []core.Hex{wallHex})
	s.Require().NoError(err, "blocked-at-first-hex is a no-op, not an error (mirrors resolver-prevented semantics)")

	after := enc.ToData().Players["alice"].View.Position
	s.Equal(start, after, "player must not have moved onto the wall hex")
}

func (s *SpaceSuite) TestMove_NilRoom_Unblocked() {
	enc := encounter.New(context.Background(), "enc-space-nil-move", s.broker)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: core.Hex{Q: 0, R: 0, S: 0}, SightRange: 10,
	}))

	// No InitRoom call — nil room must not block anything, matching the
	// pre-wave-1 behavior every other test in this package already relies on.
	s.Require().NoError(enc.Move("alice", []core.Hex{{Q: 1, R: 0, S: -1}, {Q: 2, R: 0, S: -2}}))

	after := enc.ToData().Players["alice"].View.Position
	s.Equal(core.Hex{Q: 2, R: 0, S: -2}, after)
}
