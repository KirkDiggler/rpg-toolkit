package perception_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/perception"
	"github.com/stretchr/testify/suite"
)

type ProjectSuite struct {
	suite.Suite
}

func TestProjectSuite(t *testing.T) {
	suite.Run(t, new(ProjectSuite))
}

func (s *ProjectSuite) TestProjectMove_ViewerInRange() {
	viewer := perception.NewView("alice", core.Hex{}, 5)

	path := []core.Hex{
		{Q: 1, R: 0, S: -1},
		{Q: 2, R: 0, S: -2},
		{Q: 3, R: 0, S: -3},
	}
	moveSlice, _ := perception.ProjectMove("bob", path, viewer)

	s.Require().NotNil(moveSlice)
	s.Equal(path, moveSlice.SeenSegments)
}

func (s *ProjectSuite) TestProjectMove_ViewerOutOfRange() {
	viewer := perception.NewView("alice", core.Hex{}, 2)

	path := []core.Hex{
		{Q: 5, R: -2, S: -3},
		{Q: 6, R: -2, S: -4},
	}
	moveSlice, revealSlice := perception.ProjectMove("bob", path, viewer)

	s.Nil(moveSlice)
	s.Nil(revealSlice)
}

func (s *ProjectSuite) TestProjectDoorOpen_ViewerNearDoor() {
	viewer := perception.NewView("alice", core.Hex{}, 3)

	doorPos := core.Hex{Q: 2, R: 0, S: -2}
	doorSlice, revealSlice := perception.ProjectDoorOpen("door-1", doorPos, "bob", viewer)

	s.Require().NotNil(doorSlice)
	s.True(doorSlice.Visible)
	s.Require().NotNil(revealSlice)
	s.NotEmpty(revealSlice.Hexes)
}

func (s *ProjectSuite) TestProjectDoorOpen_ViewerOutOfRange() {
	viewer := perception.NewView("alice", core.Hex{}, 1)

	doorPos := core.Hex{Q: 5, R: -2, S: -3}
	doorSlice, revealSlice := perception.ProjectDoorOpen("door-1", doorPos, "bob", viewer)

	s.Nil(doorSlice)
	s.Nil(revealSlice)
}

// ─── ProjectVisibilityTransition unit tests ────────────────────────────────

// Mover starts outside viewer's LoS and ends inside → appearedAt = path end.
func (s *ProjectSuite) TestProjectVisibilityTransition_EnterLoS() {
	viewer := perception.NewView("bob", core.Hex{}, 4)

	// moverStart is outside viewer's sight range.
	moverStart := core.Hex{Q: -10, R: 0, S: 10}
	pathEnd := core.Hex{Q: 3, R: 0, S: -3}
	path := []core.Hex{pathEnd}

	// seenSegments from ProjectMove: path end is visible.
	moveSlice, _ := perception.ProjectMove("alice", path, viewer)
	s.Require().NotNil(moveSlice)
	seenSegments := moveSlice.SeenSegments

	appearedAt, disappearedAt := perception.ProjectVisibilityTransition(moverStart, path, seenSegments, viewer)

	s.Require().NotNil(appearedAt, "mover entered LoS — appearedAt must not be nil")
	s.Equal(pathEnd, *appearedAt)
	s.Nil(disappearedAt)
}

// Mover starts inside viewer's LoS and ends outside → disappearedAt = last seen hex.
func (s *ProjectSuite) TestProjectVisibilityTransition_LeaveLoS() {
	viewer := perception.NewView("bob", core.Hex{}, 4)

	moverStart := core.Hex{Q: 2, R: 0, S: -2} // inside viewer's range of 4
	path := []core.Hex{
		{Q: 3, R: 0, S: -3},   // visible (dist 3)
		{Q: 4, R: 0, S: -4},   // visible (dist 4, edge)
		{Q: 10, R: 0, S: -10}, // outside
	}

	moveSlice, _ := perception.ProjectMove("alice", path, viewer)
	s.Require().NotNil(moveSlice)
	seenSegments := moveSlice.SeenSegments

	appearedAt, disappearedAt := perception.ProjectVisibilityTransition(moverStart, path, seenSegments, viewer)

	s.Nil(appearedAt)
	s.Require().NotNil(disappearedAt, "mover left LoS — disappearedAt must not be nil")
	s.Equal(core.Hex{Q: 4, R: 0, S: -4}, *disappearedAt,
		"last seen hex should be the boundary of viewer's sight range")
}

// Mover starts outside, passes through viewer's LoS, ends outside → both events.
func (s *ProjectSuite) TestProjectVisibilityTransition_PassThrough() {
	viewer := perception.NewView("bob", core.Hex{}, 4)

	moverStart := core.Hex{Q: -10, R: 0, S: 10} // outside
	path := []core.Hex{
		{Q: -10, R: 0, S: 10}, // outside (same as start, included explicitly)
		{Q: -3, R: 0, S: 3},   // inside (dist 3)
		{Q: 4, R: 0, S: -4},   // inside (dist 4)
		{Q: 10, R: 0, S: -10}, // outside
	}

	moveSlice, _ := perception.ProjectMove("alice", path, viewer)
	s.Require().NotNil(moveSlice)
	seenSegments := moveSlice.SeenSegments

	appearedAt, disappearedAt := perception.ProjectVisibilityTransition(moverStart, path, seenSegments, viewer)

	s.Require().NotNil(appearedAt)
	s.Require().NotNil(disappearedAt)
	s.Equal(core.Hex{Q: -3, R: 0, S: 3}, *appearedAt, "appeared at first visible hex")
	s.Equal(core.Hex{Q: 4, R: 0, S: -4}, *disappearedAt, "disappeared at last visible hex")
}

// Mover starts and ends inside viewer's LoS → no transition events.
func (s *ProjectSuite) TestProjectVisibilityTransition_StaysVisible() {
	viewer := perception.NewView("bob", core.Hex{}, 4)

	moverStart := core.Hex{Q: 1, R: 0, S: -1} // inside
	path := []core.Hex{{Q: 2, R: 0, S: -2}}   // inside

	moveSlice, _ := perception.ProjectMove("alice", path, viewer)
	s.Require().NotNil(moveSlice)
	seenSegments := moveSlice.SeenSegments

	appearedAt, disappearedAt := perception.ProjectVisibilityTransition(moverStart, path, seenSegments, viewer)

	s.Nil(appearedAt, "no enter-LoS transition when mover stays visible")
	s.Nil(disappearedAt, "no leave-LoS transition when mover stays visible")
}

// Mover has no intersection with viewer's LoS → no events, empty seenSegments.
func (s *ProjectSuite) TestProjectVisibilityTransition_NeverVisible() {
	viewer := perception.NewView("bob", core.Hex{}, 2)

	moverStart := core.Hex{Q: 10, R: 0, S: -10}
	path := []core.Hex{{Q: 15, R: 0, S: -15}}

	moveSlice, _ := perception.ProjectMove("alice", path, viewer)
	s.Nil(moveSlice, "viewer out of range — no move slice")

	appearedAt, disappearedAt := perception.ProjectVisibilityTransition(moverStart, path, nil, viewer)

	s.Nil(appearedAt)
	s.Nil(disappearedAt)
}

func (s *ProjectSuite) TestView_ApplyRevealIdempotent() {
	viewer := perception.NewView("alice", core.Hex{}, 3)
	h := core.Hex{Q: 1, R: 0, S: -1}

	viewer.ApplyReveal(core.NewHexSet(h))
	viewer.ApplyReveal(core.NewHexSet(h))

	s.Len(viewer.RevealedHexes, 1)
	s.True(viewer.RevealedHexes.Has(h))
}
