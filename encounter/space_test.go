package encounter_test

// space_test.go covers rpg-toolkit#757's SpaceData snapshot: InitRoom's
// ToData()/LoadFromData round-trip, and wall-blocked movement via a real
// room built through environments.QuickRoom (not a hand-rolled stub).

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
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

// spaceTestSeed is the explicit seed this file's tests pass to InitRoom.
// InitRoom is entropy-seeded by default (rpg-toolkit#787), so an unseeded
// 10x10 PatternRandom room only generates walls part of the time (its
// safety validation falls back to an empty room for most seeds at this
// size); tests that need a real wall to interact with must pin one that's
// verified to produce blocking walls, rather than relying on the old
// accidental every-call-gets-seed-0 determinism.
const spaceTestSeed = int64(4)

// firstBlockingWall returns the hex of the first wall segment that blocks
// movement, from an encounter's persisted Space snapshot. Fails the test if
// none exists.
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
	s.Require().NoError(enc.InitRoom(10, 10, environments.PatternRandom, spaceTestSeed))
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
	s.Require().NoError(enc.InitRoom(10, 10, environments.PatternRandom, spaceTestSeed))

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

// TestInitRoom_DimensionsThatPreviouslyCollided covers the actual reported
// blocker directly (not just a hand-built proxy): before snapshotWalls
// deduped by rounded cube coordinate, environments.QuickRoom's own wall
// generator produced discretized wall hexes that rounded to the same cell at
// several width/height combinations — gate review on PR #759 deterministically
// reproduced failures at 5x20, 7x15, and 20x15 (22 of 512 dimension pairs
// probed failed); 10x10, used by every other test in this file, happens not
// to collide. InitRoom must succeed at dimension pairs that DID collide
// pre-fix. Pins spaceTestSeed for reproducibility (rpg-toolkit#787 made
// InitRoom entropy-seeded by default); the dedup fix under test doesn't
// depend on which seed produces the collision.
func (s *SpaceSuite) TestInitRoom_DimensionsThatPreviouslyCollided() {
	for _, dims := range [][2]int{{5, 20}, {7, 15}, {20, 15}} {
		id := core.EncounterID(fmt.Sprintf("enc-space-collide-%dx%d", dims[0], dims[1]))
		enc := encounter.New(context.Background(), id, s.broker)
		err := enc.InitRoom(dims[0], dims[1], environments.PatternRandom, spaceTestSeed)
		s.Require().NoError(err, "InitRoom(%d,%d) must not fail on duplicate rounded wall cells", dims[0], dims[1])
		s.Require().NotNil(enc.Room())
	}
}

// TestLoadFromData_DuplicateWallEntries_Tolerated covers the Copilot catch on
// PR #759: room.PlaceEntity rejects stacking a blocking entity on an occupied
// hex, so a snapshot carrying two wall entries on the same cube coordinate
// (hand-built, or written before snapshotWalls deduped) must not fail the
// whole LoadFromData — the rebuild skips the duplicate and the wall still
// blocks.
func (s *SpaceSuite) TestLoadFromData_DuplicateWallEntries_Tolerated() {
	// Hex{3,-5,2} = offset position (3,3) under pointy-top — safely inside
	// the 10x10 grid with all its neighbors in-bounds too.
	wallCube := core.Hex{Q: 3, R: -5, S: 2}.ToCube()
	dupWall := environments.WallSegmentData{
		Start: wallCube, End: wallCube, BlocksMovement: true, BlocksLoS: true,
	}
	data := encounter.NewData("enc-space-dup")
	data.Space = &encounter.SpaceData{
		Walls:  []environments.WallSegmentData{dupWall, dupWall},
		Width:  10,
		Height: 10,
	}

	reloaded, err := encounter.LoadFromData(context.Background(), data, s.broker)
	s.Require().NoError(err, "duplicate wall entries must be skipped, not fail the load")
	s.Require().NotNil(reloaded.Room())

	wallHex := core.HexFromCube(wallCube)
	start := perception.HexNeighbors(wallHex)[0]
	s.Require().NoError(reloaded.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: start, SightRange: 10,
	}))
	s.Require().NoError(reloaded.Move("alice", []core.Hex{wallHex}))
	s.Equal(start, reloaded.ToData().Players["alice"].View.Position,
		"the deduplicated wall must still block movement")
}

// TestMove_SparsePathChecksInteriorCellsAndFailsClosedOutOfGrid ensures a
// caller cannot skip a legacy generated cell blocker by supplying only a far
// endpoint, and that a malformed cube/out-of-grid segment stops at the prior
// safe requested waypoint.
func (s *SpaceSuite) TestMove_SparsePathChecksInteriorCellsAndFailsClosedOutOfGrid() {
	enc := encounter.New(context.Background(), "enc-space-sparse", s.broker)
	s.Require().NoError(enc.InitRoom(10, 10, environments.PatternEmpty))
	blocker := core.HexFromPosition(spatial.Position{X: 2, Y: 1})
	var start, far core.Hex
	grid := enc.Room().GetGrid()
	for _, fromPos := range grid.GetNeighbors(blocker.ToPosition()) {
		for _, toPos := range grid.GetNeighbors(blocker.ToPosition()) {
			ray := grid.GetLineOfSight(fromPos, toPos)
			if len(ray) == 3 && ray[1].Equals(blocker.ToPosition()) {
				start, far = core.HexFromPosition(fromPos), core.HexFromPosition(toPos)
				break
			}
		}
		if far != (core.Hex{}) {
			break
		}
	}
	s.Require().NotEqual(core.Hex{}, far, "fixture requires a direct ray through the blocker")
	s.Require().NoError(enc.AddObstacle("sparse-cell-blocker", "test", blocker, true, true))
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice", Position: start, SightRange: 1,
	}))
	s.Require().NoError(enc.Move("alice", []core.Hex{far}))
	s.Equal(start, enc.ToData().Players["alice"].View.Position,
		"an endpoints-only request must not jump over an interior cell blocker")

	clean := encounter.New(context.Background(), "enc-space-invalid", s.broker)
	s.Require().NoError(clean.InitRoom(10, 10, environments.PatternEmpty))
	s.Require().NoError(clean.AddPlayer(encounter.PlayerInput{
		PlayerID: bobPlayerID, EntityID: bobEntityID, Position: start, SightRange: 1,
	}))
	safe := core.HexFromPosition(clean.Room().GetGrid().GetNeighbors(start.ToPosition())[0])
	invalidCube := core.Hex{Q: 1, R: 1, S: 1}
	s.Require().NoError(clean.Move(bobPlayerID, []core.Hex{safe, invalidCube}))
	s.Equal(safe, clean.ToData().Players[bobPlayerID].View.Position,
		"an invalid later segment must fail closed without discarding the safe prefix")
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
