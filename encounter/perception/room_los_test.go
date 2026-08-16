package perception_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// RoomLOSSuite covers rpg-toolkit#757's wall-aware VisibleHexesAt/CanSeeAt:
// a wall between viewer and target blocks LoS through a real spatial.Room
// (not a stub), the same hex is open when the room has no wall, SightRange
// still caps distance regardless of walls, and a nil room preserves the
// pre-wave-1 pure-radius behavior for encounters with no SpaceData.
type RoomLOSSuite struct {
	suite.Suite
	viewer core.Hex
	target core.Hex
}

func TestRoomLOSSuite(t *testing.T) {
	suite.Run(t, new(RoomLOSSuite))
}

func (s *RoomLOSSuite) SetupTest() {
	s.viewer = core.Hex{Q: 0, R: 0, S: 0}
	s.target = core.Hex{Q: 4, R: -4, S: 0} // distance 4
}

func newTestRoom(id string) *spatial.BasicRoom {
	grid := spatial.NewHexGrid(spatial.HexGridConfig{
		Width:       20,
		Height:      20,
		Orientation: spatial.HexOrientationPointyTop,
	})
	return spatial.NewBasicRoom(spatial.BasicRoomConfig{ID: id, Type: "test", Grid: grid})
}

// midpointWallPosition finds a hex strictly between from and to on the
// room's line of sight, so a wall placed there is guaranteed to sit on the
// path IsLineOfSightBlocked actually walks (rather than a hand-picked
// coordinate that may not land on the lerped hex line).
//
// Sitting on that path is necessary but no longer sufficient to block:
// since spatial v0.9.1 (rpg-toolkit#1022) sight leans, and an occluder on
// the direct ray is seen past whenever a neighbour strictly closer to the
// other end has a clear lane. What makes the suite's cases block is that
// the viewer and target sit on a straight hex axis (0,0,0 -> 4,-4,0), where
// the only cell strictly closer is the next one on the same line and meets
// the same wall. A helper that derives the wall from the ray therefore
// cannot, by itself, tell a blocking rule from a broken one — see the
// module's own occluder pins for the off-axis half.
func midpointWallPosition(s *suite.Suite, room spatial.Room, from, to core.Hex) spatial.Position {
	los := room.GetLineOfSight(from.ToPosition(), to.ToPosition())
	s.Require().GreaterOrEqual(len(los), 3, "need an intermediate hex to place a wall on")
	return los[len(los)/2]
}

func (s *RoomLOSSuite) TestCanSeeAt_BlockedByWall() {
	room := newTestRoom("blocked")
	wallPos := midpointWallPosition(&s.Suite, room, s.viewer, s.target)
	wall := environments.NewWallEntity(environments.WallEntityConfig{
		SegmentID: "wall-1",
		WallType:  environments.WallTypeIndestructible,
		Properties: environments.WallProperties{
			BlocksMovement: true,
			BlocksLoS:      true,
		},
		Position: wallPos,
	})
	s.Require().NoError(room.PlaceEntity(wall, wallPos))

	view := perception.NewView("p1", s.viewer, 10)
	s.False(perception.CanSeeAt(view, s.target, room), "wall between viewer and target must block LoS")
}

func (s *RoomLOSSuite) TestCanSeeAt_OpenRoom_NoWall() {
	room := newTestRoom("open")
	view := perception.NewView("p1", s.viewer, 10)
	s.True(perception.CanSeeAt(view, s.target, room), "no wall on the path — same room type, must be visible")
}

func (s *RoomLOSSuite) TestCanSeeAt_NilRoom_PureRadius() {
	view := perception.NewView("p1", s.viewer, 10)
	s.True(perception.CanSeeAt(view, s.target, nil), "nil room must fall back to pure-radius behavior")
}

func (s *RoomLOSSuite) TestCanSeeAt_SightRangeStillCapsDistance() {
	// Room has no wall at all — only distance should reject this.
	room := newTestRoom("far")
	view := perception.NewView("p1", s.viewer, 2) // target is distance 4, out of range
	s.False(perception.CanSeeAt(view, s.target, room))
	s.False(perception.CanSeeAt(view, s.target, nil), "distance cap applies with or without a room")
}

func (s *RoomLOSSuite) TestVisibleHexesAt_ExcludesWallBlockedHex() {
	room := newTestRoom("blocked-set")
	wallPos := midpointWallPosition(&s.Suite, room, s.viewer, s.target)
	wall := environments.NewWallEntity(environments.WallEntityConfig{
		SegmentID:  "wall-1",
		WallType:   environments.WallTypeIndestructible,
		Properties: environments.WallProperties{BlocksMovement: true, BlocksLoS: true},
		Position:   wallPos,
	})
	s.Require().NoError(room.PlaceEntity(wall, wallPos))

	visible := perception.VisibleHexesAt(s.viewer, 10, room)
	s.False(visible.Has(s.target), "target beyond the wall must be excluded from the visible set")

	nilRoomVisible := perception.VisibleHexesAt(s.viewer, 10, nil)
	s.True(nilRoomVisible.Has(s.target), "nil room must not filter by walls")
}
