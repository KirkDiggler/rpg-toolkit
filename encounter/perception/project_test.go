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

func (s *ProjectSuite) TestView_ApplyRevealIdempotent() {
	viewer := perception.NewView("alice", core.Hex{}, 3)
	h := core.Hex{Q: 1, R: 0, S: -1}

	viewer.ApplyReveal(core.NewHexSet(h))
	viewer.ApplyReveal(core.NewHexSet(h))

	s.Len(viewer.RevealedHexes, 1)
	s.True(viewer.RevealedHexes.Has(h))
}
