package perception_test

import (
	"encoding/json"
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
	moveSlice, _, _ := perception.ProjectMove("bob", path, viewer, nil)

	s.Require().NotNil(moveSlice)
	s.Equal(path, moveSlice.SeenSegments)
}

func (s *ProjectSuite) TestProjectMove_ViewerOutOfRange() {
	viewer := perception.NewView("alice", core.Hex{}, 2)

	path := []core.Hex{
		{Q: 5, R: -2, S: -3},
		{Q: 6, R: -2, S: -4},
	}
	moveSlice, revealSlice, _ := perception.ProjectMove("bob", path, viewer, nil)

	s.Nil(moveSlice)
	s.Nil(revealSlice)
}

func (s *ProjectSuite) TestProjectDoorOpen_ViewerNearDoor() {
	viewer := perception.NewView("alice", core.Hex{}, 3)

	doorPos := core.Hex{Q: 2, R: 0, S: -2}
	doorSlice, revealSlice := perception.ProjectDoorOpen("door-1", doorPos, "bob", viewer, nil)

	s.Require().NotNil(doorSlice)
	s.True(doorSlice.Visible)
	s.Require().NotNil(revealSlice)
	s.NotEmpty(revealSlice.Hexes)
}

func (s *ProjectSuite) TestProjectDoorOpen_ViewerOutOfRange() {
	viewer := perception.NewView("alice", core.Hex{}, 1)

	doorPos := core.Hex{Q: 5, R: -2, S: -3}
	doorSlice, revealSlice := perception.ProjectDoorOpen("door-1", doorPos, "bob", viewer, nil)

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

	// Use ProjectMove to get both seenSegments and the precomputed visible set.
	moveSlice, _, visible := perception.ProjectMove("alice", path, viewer, nil)
	s.Require().NotNil(moveSlice)
	seenSegments := moveSlice.SeenSegments

	appearedAt, disappearedAt := perception.ProjectVisibilityTransition(moverStart, path, seenSegments, viewer, visible)

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

	moveSlice, _, visible := perception.ProjectMove("alice", path, viewer, nil)
	s.Require().NotNil(moveSlice)
	seenSegments := moveSlice.SeenSegments

	appearedAt, disappearedAt := perception.ProjectVisibilityTransition(moverStart, path, seenSegments, viewer, visible)

	s.Nil(appearedAt)
	s.Require().NotNil(disappearedAt, "mover left LoS — disappearedAt must not be nil")
	s.Equal(core.Hex{Q: 4, R: 0, S: -4}, *disappearedAt,
		"last seen hex should be the boundary of viewer's sight range")
}

// Mover starts outside, passes through viewer's LoS, ends outside → both events.
// Path contains only destination hexes; moverStart supplies the initial position.
func (s *ProjectSuite) TestProjectVisibilityTransition_PassThrough() {
	viewer := perception.NewView("bob", core.Hex{}, 4)

	moverStart := core.Hex{Q: -10, R: 0, S: 10} // outside — NOT in path
	path := []core.Hex{
		{Q: -3, R: 0, S: 3},   // first destination — inside (dist 3)
		{Q: 4, R: 0, S: -4},   // inside (dist 4)
		{Q: 10, R: 0, S: -10}, // outside
	}

	moveSlice, _, visible := perception.ProjectMove("alice", path, viewer, nil)
	s.Require().NotNil(moveSlice)
	seenSegments := moveSlice.SeenSegments

	appearedAt, disappearedAt := perception.ProjectVisibilityTransition(moverStart, path, seenSegments, viewer, visible)

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

	moveSlice, _, visible := perception.ProjectMove("alice", path, viewer, nil)
	s.Require().NotNil(moveSlice)
	seenSegments := moveSlice.SeenSegments

	appearedAt, disappearedAt := perception.ProjectVisibilityTransition(moverStart, path, seenSegments, viewer, visible)

	s.Nil(appearedAt, "no enter-LoS transition when mover stays visible")
	s.Nil(disappearedAt, "no leave-LoS transition when mover stays visible")
}

// Mover has no intersection with viewer's LoS → no events, empty seenSegments.
func (s *ProjectSuite) TestProjectVisibilityTransition_NeverVisible() {
	viewer := perception.NewView("bob", core.Hex{}, 2)

	moverStart := core.Hex{Q: 10, R: 0, S: -10}
	path := []core.Hex{{Q: 15, R: 0, S: -15}}

	_, _, visible := perception.ProjectMove("alice", path, viewer, nil)

	appearedAt, disappearedAt := perception.ProjectVisibilityTransition(moverStart, path, nil, viewer, visible)

	s.Nil(appearedAt)
	s.Nil(disappearedAt)
}

// Mover starts visible but path has no visible destination hexes (immediate leave) →
// last-known hex falls back to moverStart rather than silently dropping the event.
func (s *ProjectSuite) TestProjectVisibilityTransition_LeaveLoS_EmptySeenSegments() {
	viewer := perception.NewView("bob", core.Hex{}, 4)

	// moverStart is inside viewer's range; path has only one hex outside range.
	moverStart := core.Hex{Q: 2, R: 0, S: -2}
	path := []core.Hex{{Q: 10, R: 0, S: -10}} // single destination, outside

	_, _, visible := perception.ProjectMove("alice", path, viewer, nil)

	// seenSegments will be empty because the single path hex is out of range.
	appearedAt, disappearedAt := perception.ProjectVisibilityTransition(moverStart, path, nil, viewer, visible)

	s.Nil(appearedAt)
	s.Require().NotNil(disappearedAt,
		"disappearedAt must not be nil when mover was visible at start but seenSegments is empty")
	s.Equal(moverStart, *disappearedAt,
		"last-known hex falls back to moverStart when no path hex was visible")
}

// TestView_ObserveIdempotent covers rpg-toolkit#851's acceptance bar
// "duplicate reconciliation is idempotent": applying the identical
// observation twice must leave Memory unchanged, both in size and content.
// TestPlacementFacingJSONDistinguishesCurrentPresenceFromLegacyZero protects
// persisted viewer memory from the old mandatory Facing:0 encoding while
// retaining a newly authored explicit E = 0 override.
func (s *ProjectSuite) TestPlacementFacingJSONDistinguishesCurrentPresenceFromLegacyZero() {
	hex := core.Hex{Q: 1, R: -1, S: 0}
	facingEast := uint32(0)
	current := perception.NewMemory()
	current.Observe(perception.HexObservation{
		Position: hex,
		State:    perception.KnowledgeStateVisible,
		Contents: []perception.Placement{{EntityID: "authored-prop", Facing: &facingEast}},
	})

	payload, err := json.Marshal(current)
	s.Require().NoError(err)
	s.Contains(string(payload), `"facing":0`)
	s.NotContains(string(payload), `"Facing":0`)

	var reloaded perception.Memory
	s.Require().NoError(json.Unmarshal(payload, &reloaded))
	s.Require().NotNil(reloaded[hex].Contents[0].Facing)
	s.Equal(uint32(0), *reloaded[hex].Contents[0].Facing)

	const legacyMemory = `[
		{"position":{"Q":1,"R":-1,"S":0},"state":1,
		 "contents":[{"EntityID":"legacy-player","Facing":0}]}
	]`
	var legacy perception.Memory
	s.Require().NoError(json.Unmarshal([]byte(legacyMemory), &legacy))
	s.Require().Len(legacy[hex].Contents, 1)
	s.Nil(legacy[hex].Contents[0].Facing)
}

func (s *ProjectSuite) TestView_ObserveIdempotent() {
	viewer := perception.NewView("alice", core.Hex{}, 3)
	h := core.Hex{Q: 1, R: 0, S: -1}
	obs := perception.HexObservation{
		Position: h,
		State:    perception.KnowledgeStateVisible,
		ZoneID:   "chamber-1",
		Contents: []perception.Placement{{EntityID: "goblin-1"}},
	}

	viewer.Observe(obs)
	viewer.Observe(obs)

	s.Len(viewer.Memory, 1)
	s.True(viewer.Knows(h))
	s.Equal(obs, viewer.Memory[h])
}
