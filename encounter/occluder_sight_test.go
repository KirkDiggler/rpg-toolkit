package encounter_test

// occluder_sight_test.go pins what an occluding cell does to sight after
// spatial v0.9.1 (rpg-toolkit#1022/#1025, adopted by #1026): sight is a
// LANE, not a line. An obstacle standing on the direct ray is a corner to
// lean around, not a curtain — unless the geometry offers no corner.
//
// These are the module's own occluder pins, and they exist because the rest
// of the suite cannot see this behavior at all. Every other sight fixture
// here is built on a straight hex axis or on two cells either side of one
// obstacle, and both of those are exactly the geometries the lane rule does
// NOT change: the bump that carried it flipped 994 of 10302 sightlines in a
// measured two-chamber dungeon and not one test in this module failed.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type OccluderSightSuite struct {
	suite.Suite
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
	viewer    core.Hex
}

func TestOccluderSightSuite(t *testing.T) {
	suite.Run(t, new(OccluderSightSuite))
}

func (s *OccluderSightSuite) SetupTest() {
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
	// Offset (9,9) of the 20x20 room every case below builds: far enough
	// from every edge that a six-hex sightline in any direction stays in
	// bounds, so nothing here is decided by the room boundary.
	s.viewer = core.HexFromPosition(spatial.Position{X: 9, Y: 9})
}

// shifted returns the hex reached from the viewer by the given cube delta.
func (s *OccluderSightSuite) shifted(q, r, t int) core.Hex {
	return core.Hex{Q: s.viewer.Q + q, R: s.viewer.R + r, S: s.viewer.S + t}
}

// roomWithPillar builds a 20x20 room whose only occluder is a single
// sight-blocking, movement-free obstacle at pillar, and requires that the
// pillar actually lies on the direct ray to target — so a case that stops
// blocking has stopped because of the RULE, not because the fixture missed.
func (s *OccluderSightSuite) roomWithPillar(id core.EncounterID, pillar, target core.Hex, want int) spatial.Room {
	enc := encounter.New(context.Background(), id, s.broker)
	s.Require().NoError(enc.InitRoom(20, 20, environments.PatternEmpty))
	s.Require().NoError(enc.AddObstacle("pillar-1", "dnd5e:obstacles:pillar", pillar, false, true))

	room := enc.Room()
	s.Require().NotNil(room)
	s.Require().Equal(want, perception.HexDistance(s.viewer, target), "fixture geometry drifted")

	onRay := false
	for _, position := range room.GetLineOfSight(s.viewer.ToPosition(), target.ToPosition()) {
		if position == pillar.ToPosition() {
			onRay = true
		}
	}
	s.Require().True(onRay, "the pillar must stand on the direct ray or this case pins nothing")
	return room
}

// TestAPillarOnTheAxisStillBlocks: a straight hex axis offers no corner.
// Every neighbour of the viewer except the next cell on the line itself is
// the SAME distance from the target, not closer, so the lane rule's
// strict-progress requirement rules them all out — and the one neighbour
// that does make progress meets the same pillar. Sight stays blocked, and
// that is the fix behaving, not the fix missing.
func (s *OccluderSightSuite) TestAPillarOnTheAxisStillBlocks() {
	target := s.shifted(6, -6, 0)
	pillar := s.shifted(3, -3, 0)
	room := s.roomWithPillar("enc-occluder-axis", pillar, target, 6)

	s.True(room.IsLineOfSightBlocked(s.viewer.ToPosition(), target.ToPosition()),
		"an occluder on a straight-axis sightline has no corner to lean around")
	s.True(room.IsLineOfSightBlocked(target.ToPosition(), s.viewer.ToPosition()),
		"and the answer must not depend on which end is asking")
}

// TestAPillarOffTheAxisIsLeanedAround: the half of the rule the old pin got
// wrong. The pillar stands on the direct ray here too, but the target is off
// axis, so a neighbour of the viewer is strictly closer to it and has a
// clear lane — the corner a player leans around. Under the spatial version
// this module pinned before #1026 this pair was BLOCKED; that over-blocking
// is what Kirk watched swallow sightlines in play.
func (s *OccluderSightSuite) TestAPillarOffTheAxisIsLeanedAround() {
	target := s.shifted(5, -3, -2)
	pillar := s.shifted(2, -1, -1)
	room := s.roomWithPillar("enc-occluder-off-axis", pillar, target, 5)

	s.False(room.IsLineOfSightBlocked(s.viewer.ToPosition(), target.ToPosition()),
		"a single occluder off the axis must be seen past, not treated as a curtain")
	s.False(room.IsLineOfSightBlocked(target.ToPosition(), s.viewer.ToPosition()),
		"and the answer must not depend on which end is asking")
}
