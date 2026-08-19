// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// step_test.go covers the composition's public step verb (rpg-toolkit#1059):
// ONE cell, named in dungeon-absolute space, with the composition deciding
// whether that cell is next door or through a door.
//
// The verb exists because the decision was being made twice. The session SDK
// resolved a walk itself — locating each cell, comparing rooms, scanning the
// projected doorway list, then choosing between Move and Traverse — while the
// pump made the same decision here, off the raw connection list. Two
// implementations of "what is crossable" is a defect waiting for the first
// rule that distinguishes doorways from each other: a locked door, a one-way
// door, a doorway wider than one cell pair. Whichever side learned it first
// would let a player walk somewhere a monster could not follow, or the reverse.
//
// So the tests below are not only "does the verb work". The load-bearing one
// is TestAStepAndAMonsterStepAgreeOnWhatIsCrossable, which drives the same
// cell pair through both callers and asserts they answer the same way — a
// property that now holds BY CONSTRUCTION, since both run stepMember.
//
// NOTHING IN THIS FILE IS ANCHORED AT THE ORIGIN, for pursuit_test.go's
// reason: where a room sits at (0,0) its local coordinates and its absolute
// ones are the same numbers, and a verb that answered in the wrong frame would
// pass every assertion anyway.
type StepSuite struct {
	suite.Suite

	enc *encounter.Encounter
}

func TestStepSuite(t *testing.T) {
	suite.Run(t, new(StepSuite))
}

const (
	stepWest = "west-chamber"
	stepEast = "east-chamber"
	stepDoor = "connecting-door"
)

// The two chambers touch along their whole shared edge, and the west chamber
// WALLS that edge except at one cell pair. That is the fixture's whole point:
// absolute adjacency is not permission, and what withholds permission is a
// wall — an edge on the map, drawn by a room across its own boundary onto the
// cell beyond (rpg-toolkit#1106). It used to be the absence of a doorway in a
// connection list, because a wall there was inexpressible.
var (
	stepWestOrigin = spatial.Position{X: 20, Y: 10}
	stepEastOrigin = spatial.Position{X: 26, Y: 10}

	// The doorway, room-local on each side. Absolute: (25,13) and (26,13).
	stepDoorWestLocal = spatial.Position{X: 5, Y: 3}
	stepDoorEastLocal = spatial.Position{X: 0, Y: 3}
)

// abs projects a room-local cell the way the fixture's own anchors say it
// should be — derived from the origins above rather than written out, so a
// changed anchor cannot leave a stale literal passing.
func stepAbs(origin, local spatial.Position) spatial.Position {
	return local.Add(origin)
}

// stepSeamWall is the west chamber's east wall: an edge from each of its own
// last-column cells to the cell beyond it, every row but the doorway's. The far
// endpoint belongs to the EAST chamber, which is exactly the thing a room could
// not say before the field became one canvas.
func stepSeamWall() []spatial.Boundary { return squareSeamWall(5, 6, int(stepDoorWestLocal.Y)) }

// stepField is the shared set, optionally with a decider driving the goblin.
func stepField() encounter.FieldInput {
	return encounter.FieldInput{
		Rooms: []encounter.RoomInput{
			{ID: stepWest, Width: 6, Height: 6, Origin: stepWestOrigin,
				Boundaries: stepSeamWall()},
			{ID: stepEast, Width: 6, Height: 6, Origin: stepEastOrigin},
		},
		Connections: []encounter.ConnectionInput{{
			ID: stepDoor, From: stepWest, To: stepEast,
			FromPosition: stepDoorWestLocal,
			ToPosition:   stepDoorEastLocal,
		}},
	}
}

// SetupTest opens the set with alice alone in the west chamber, one cell north
// of the threshold. Alone, so nothing forms a fight and each test's verb is
// the only thing that happens.
func (s *StepSuite) SetupTest() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		Field: stepField(),
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Room: stepWest,
				Position: spatial.Position{X: 5, Y: 2}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	s.enc = enc
}

// standsAt reads a member's cell off the roster, which reports absolute.
func (s *StepSuite) standsAt(id encounter.MemberID) spatial.Position {
	members, err := s.enc.Members()
	s.Require().NoError(err)
	for _, m := range members {
		if m.ID == id {
			return m.Position
		}
	}
	s.Require().Fail("no such member", string(id))
	return spatial.Position{}
}

// lastBeat decodes the most recent story beat one member was told about.
func (s *StepSuite) lastBeat(audience encounter.MemberID) map[string]any {
	story, err := s.enc.Story(&encounter.StoryInput{Audience: audience})
	s.Require().NoError(err)
	s.Require().NotEmpty(story)

	var beat map[string]any
	s.Require().NoError(json.Unmarshal(story[len(story)-1].Payload, &beat))
	return beat
}

// beatCell reads a beat's position back as a Position, since JSON gives it
// back as a map of floats.
func (s *StepSuite) beatCell(beat map[string]any) spatial.Position {
	raw, ok := beat["position"].(map[string]any)
	s.Require().True(ok, "the beat carries a position")
	return spatial.Position{X: raw["x"].(float64), Y: raw["y"].(float64)}
}

// TestAStepWithinARoomIsAMove is the ordinary case: a cell the walker's own
// room owns, carried by the same mechanism the Move verb uses, narrated with
// the same beat.
//
// "Narrated the same" is asserted rather than assumed because the step verb is
// meant to REPLACE the seam's use of Move, and a host reading the story must
// not be able to tell which verb produced a movement.
func (s *StepSuite) TestAStepWithinARoomIsAMove() {
	from := stepAbs(stepWestOrigin, spatial.Position{X: 5, Y: 2})
	to := stepAbs(stepWestOrigin, spatial.Position{X: 4, Y: 2})

	out, err := s.enc.Step(&encounter.StepInput{Member: alice, To: to})
	s.Require().NoError(err)

	s.Equal(alice, out.Stepped.Member)
	s.Equal(from, out.Stepped.From, "where she was, on the map")
	s.Equal(to, out.Stepped.To, "where she is, on the map")
	s.Empty(out.Crossing, "no doorway was involved")
	s.Equal(to, s.standsAt(alice), "and the roster agrees")

	beat := s.lastBeat(alice)
	s.Equal("moved", beat["beat"])
	s.Equal(to, s.beatCell(beat))
}

// TestAStepThroughADoorwayIsJustAStep is the same sentence one cell along, and
// the story cannot tell the two apart.
//
// The step names a cell, exactly as the one above does; that this cell happens
// to be on the far side of a doorway is the composition's business to notice.
// This is what W3 bought — a doorway's two endpoints are adjacent absolute
// cells, so "the cell next to me" and "the cell through that door" are written
// identically. The BEAT says "moved" for both: a crossing stopped being a
// second kind of movement when the field stopped being a set of rooms
// (rpg-toolkit#1106), and the doorway's name rides along as narration.
func (s *StepSuite) TestAStepThroughADoorwayIsJustAStep() {
	threshold := stepAbs(stepWestOrigin, stepDoorWestLocal)
	farSide := stepAbs(stepEastOrigin, stepDoorEastLocal)

	_, err := s.enc.Step(&encounter.StepInput{Member: alice, To: threshold})
	s.Require().NoError(err)

	out, err := s.enc.Step(&encounter.StepInput{Member: alice, To: farSide})
	s.Require().NoError(err)

	s.Equal(threshold, out.Stepped.From)
	s.Equal(farSide, out.Stepped.To)
	s.Equal(stepDoor, out.Crossing, "and the verb says which doorway carried it")
	s.Equal(farSide, s.standsAt(alice))

	beat := s.lastBeat(alice)
	s.Equal("moved", beat["beat"], "the same beat an ordinary step writes")
	s.Equal(stepDoor, beat["connection"], "with the doorway named beside it")
	s.Equal(farSide, s.beatCell(beat))
}

// TestADoorwayIsCrossableBothWays pins the half a fixture will not reach by
// accident: the way BACK.
//
// A connection is declared with a From room and a To room, and every scene in
// this package walks it the declared way. Match only that direction and every
// crossing test still passes while every door in the game becomes one-way —
// caught here by a surviving mutant, which is exactly the "locked, one-way, or
// wide doorway" rule #1059 says must land in one place rather than two.
func (s *StepSuite) TestADoorwayIsCrossableBothWays() {
	threshold := stepAbs(stepWestOrigin, stepDoorWestLocal)
	farSide := stepAbs(stepEastOrigin, stepDoorEastLocal)

	_, err := s.enc.Step(&encounter.StepInput{Member: alice, To: threshold})
	s.Require().NoError(err)
	_, err = s.enc.Step(&encounter.StepInput{Member: alice, To: farSide})
	s.Require().NoError(err)

	// And back through the same door, against the direction it was declared in.
	out, err := s.enc.Step(&encounter.StepInput{Member: alice, To: threshold})
	s.Require().NoError(err)
	s.Equal(stepDoor, out.Crossing, "the same doorway carried her home")
	s.Equal(farSide, out.Stepped.From)
	s.Equal(threshold, out.Stepped.To)
	s.Equal(threshold, s.standsAt(alice))
}

// TestAStepSpeaksTheMapAtBothEnds is the frame pin.
//
// Both chambers are anchored well away from the origin, so every cell has a
// local twin that is a DIFFERENT number. A verb reporting the room-local cell
// — the frame the mechanisms underneath actually work in — would produce (4,2)
// here instead of (24,12), and a host has nothing to check that against.
func (s *StepSuite) TestAStepSpeaksTheMapAtBothEnds() {
	local := spatial.Position{X: 4, Y: 2}
	to := stepAbs(stepWestOrigin, local)

	out, err := s.enc.Step(&encounter.StepInput{Member: alice, To: to})
	s.Require().NoError(err)

	s.NotEqual(local, out.Stepped.To, "the local cell is not the answer")
	s.Equal(to, out.Stepped.To)
	s.Equal(to, s.standsAt(alice), "and the roster reports the same cell")
	s.Equal(stepWest, s.roomOf(alice), "which is still in the chamber that authored it")
}

// roomOf reads which authored chamber a member is standing in, off the roster.
func (s *StepSuite) roomOf(id encounter.MemberID) string {
	members, err := s.enc.Members()
	s.Require().NoError(err)
	for _, m := range members {
		if m.ID == id {
			return m.Region
		}
	}
	s.Require().Fail("no such member", string(id))
	return ""
}

// TestAStepIntoTheVoidIsRefused: void is not floor. The private twin skips
// silently; this one has to SAY so, which is the whole reason it exists.
func (s *StepSuite) TestAStepIntoTheVoidIsRefused() {
	before := s.standsAt(alice)

	_, err := s.enc.Step(&encounter.StepInput{Member: alice, To: spatial.Position{X: 500, Y: 500}})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrBadPlacement)
	s.Equal(before, s.standsAt(alice), "and she did not move")
}

// TestAStepIntoTheWallBetweenTwoChambersIsRefused is the seam, walled.
//
// (25,12) and (26,12) are adjacent absolute cells in different chambers with a
// wall between them. The refusal used to have a sentinel of its own, because
// what withheld permission was the absence of a doorway rather than anything on
// the map. Now a wall does it, and a wall refuses a step the way every other
// wall does — ErrBadPlacement, from the canvas itself.
func (s *StepSuite) TestAStepIntoTheWallBetweenTwoChambersIsRefused() {
	before := s.standsAt(alice)
	acrossTheSeam := stepAbs(stepEastOrigin, spatial.Position{X: 0, Y: 2})

	// The premise: that cell is real floor, in the chamber next door — the
	// refusal is about the wall, not about the destination.
	s.Require().Equal(encounter.RegionID(stepEast), s.regionAtCell(acrossTheSeam))

	_, err := s.enc.Step(&encounter.StepInput{Member: alice, To: acrossTheSeam})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrBadPlacement)
	s.Equal(before, s.standsAt(alice))
}

// regionAtCell names the region whose cell set holds a cell. It used to walk
// the Atlas's enumerated cells to get there; the field answers it directly now
// (rpg-toolkit#1108), which is the same question asked of the thing that owns
// it rather than of a list.
func (s *StepSuite) regionAtCell(cell spatial.Position) encounter.RegionID {
	region, _ := s.enc.RegionAt(cell)
	return region
}

// TestAStepAndAMonsterStepAgreeOnWhatIsCrossable is why this issue was filed.
//
// The same two cell pairs, asked twice: once as a player's step through the
// public verb, once as a monster's intended step through the pump. One pair is
// joined by the doorway and one has a wall across it, and the two callers must
// answer identically on both.
//
// Before the step verb they could not have: the SDK scanned the projected
// Atlas doorway list, the pump scanned the raw connection inputs, and the two
// scans agreed only because neither had learned anything the other had not. A
// locked door, a one-way door, or a doorway spanning more than one cell pair
// would have had to land in both, in lockstep, forever.
func (s *StepSuite) TestAStepAndAMonsterStepAgreeOnWhatIsCrossable() {
	seamWest := stepAbs(stepWestOrigin, spatial.Position{X: 5, Y: 2})
	seamEast := stepAbs(stepEastOrigin, spatial.Position{X: 0, Y: 2})
	threshold := stepAbs(stepWestOrigin, stepDoorWestLocal)
	farSide := stepAbs(stepEastOrigin, stepDoorEastLocal)

	// A goblin standing on the bare seam, intending the cell across it.
	refused := &onceStepDecider{to: seamEast}
	enc := s.sceneWithMonster(spatial.Position{X: 5, Y: 2}, refused)
	_, err := enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)
	s.Require().True(refused.called, "the decider was consulted")
	s.Equal(seamWest, s.whereIn(enc, goblin), "the monster did not walk through a wall")

	// Alice, on the same cell, intending the same one: same answer.
	_, err = s.enc.Step(&encounter.StepInput{Member: alice, To: seamEast})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrBadPlacement)

	// And through the real doorway, both go.
	crossed := &onceStepDecider{to: farSide}
	enc = s.sceneWithMonster(stepDoorWestLocal, crossed)
	_, err = enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)
	s.Equal(farSide, s.whereIn(enc, goblin), "the monster went through the door")

	_, err = s.enc.Step(&encounter.StepInput{Member: alice, To: threshold})
	s.Require().NoError(err)
	out, err := s.enc.Step(&encounter.StepInput{Member: alice, To: farSide})
	s.Require().NoError(err)
	s.Equal(farSide, out.Stepped.To, "and so did she")
}

// sceneWithMonster opens a second encounter on the same set with a goblin ALONE
// in the west chamber, so nothing can form a bubble before the pump under test
// runs — a monster in a fight is not the world's to move, and these tests are
// about what the world does with one.
//
// It used to park a player in the east chamber for that job, on the reasoning
// that a room boundary hid her. It does not (rpg-toolkit#1106): the goblin
// stands ON the doorway cell in half these cases, looking straight through the
// opening at the whole chamber beyond. An empty room is the honest way to say
// "nobody has seen anybody".
func (s *StepSuite) sceneWithMonster(at spatial.Position, decider encounter.Decider) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		Field: stepField(),
		Members: []encounter.MemberInput{
			{ID: goblin, Kind: encounter.KindMonster, Room: stepWest,
				Position: at, Decider: decider},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	return enc
}

// whereIn reads a member's absolute cell out of an encounter this suite does
// not hold in its own field.
func (s *StepSuite) whereIn(enc *encounter.Encounter, id encounter.MemberID) spatial.Position {
	members, err := enc.Members()
	s.Require().NoError(err)
	for _, m := range members {
		if m.ID == id {
			return m.Position
		}
	}
	s.Require().Fail("no such member", string(id))
	return spatial.Position{}
}

// TestAStepFiresAReachedPositionEnding — an ending underfoot closes the
// encounter, and the step is what put a foot there.
func (s *StepSuite) TestAStepFiresAReachedPositionEnding() {
	target := spatial.Position{X: 4, Y: 2}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		Field: stepField(),
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Room: stepWest,
				Position: spatial.Position{X: 5, Y: 2}},
		},
		Endings: []encounter.EndingInput{
			{Key: "found-it", Trigger: encounter.TriggerReachedPosition{Room: stepWest, Position: target}},
		},
	})
	s.Require().NoError(err)

	out, err := enc.Step(&encounter.StepInput{Member: alice, To: stepAbs(stepWestOrigin, target)})
	s.Require().NoError(err)
	s.Require().NotNil(out.Outcome)
	s.Equal("found-it", out.Outcome.Ending)
}

// TestACrossingFiresAReachedPositionEndingOnTheFarSide is the same rule at the
// arrival end of a doorway, which is the half a step verb could plausibly get
// wrong: an ending declared in the room being ENTERED must be evaluated
// against where the member landed, not where they left.
func (s *StepSuite) TestACrossingFiresAReachedPositionEndingOnTheFarSide() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		Field: stepField(),
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Room: stepWest,
				Position: stepDoorWestLocal},
		},
		Endings: []encounter.EndingInput{
			{Key: "through", Trigger: encounter.TriggerReachedPosition{
				Room: stepEast, Position: stepDoorEastLocal}},
		},
	})
	s.Require().NoError(err)

	out, err := enc.Step(&encounter.StepInput{
		Member: alice, To: stepAbs(stepEastOrigin, stepDoorEastLocal)})
	s.Require().NoError(err)
	s.Require().NotNil(out.Outcome)
	s.Equal("through", out.Outcome.Ending)
}

// TestAStepRefusesNilInput pins the caller-defect arm.
func (s *StepSuite) TestAStepRefusesNilInput() {
	_, err := s.enc.Step(nil)
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrNilInput)
}

// TestAStepRefusesAnEmptyMember mirrors Move: an unnamed member is a caller
// defect distinct from a named one who is not here.
func (s *StepSuite) TestAStepRefusesAnEmptyMember() {
	_, err := s.enc.Step(&encounter.StepInput{To: spatial.Position{X: 24, Y: 12}})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrNoMember)
}

// TestAStepRefusesANonMember: somebody who is not in this encounter.
func (s *StepSuite) TestAStepRefusesANonMember() {
	_, err := s.enc.Step(&encounter.StepInput{
		Member: core.EntityID("nobody"), To: spatial.Position{X: 24, Y: 12}})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrNotMember)
}

// TestAStepRefusesAClosedEncounter: a finished encounter does not move.
func (s *StepSuite) TestAStepRefusesAClosedEncounter() {
	_, err := s.enc.End(&encounter.EndInput{Ending: "withdrawn"})
	s.Require().NoError(err)

	_, err = s.enc.Step(&encounter.StepInput{
		Member: alice, To: stepAbs(stepWestOrigin, spatial.Position{X: 4, Y: 2})})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrClosed)
}

// TestAStepRefusesAFightMember pins the world-clock gate Move and Traverse
// both carry: free roam is a world-clock verb, and a member in a bubble acts
// through the fight's own turn structure or not at all.
//
// It is asserted here rather than assumed because the step verb is the one the
// seam will call for every walk — a gate it failed to inherit would let a
// player stroll out of a fight.
func (s *StepSuite) TestAStepRefusesAFightMember() {
	// In sight of each other from first light, which starts the fight.
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		Field: stepField(),
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Room: stepWest,
				Position: spatial.Position{X: 1, Y: 1}},
			{ID: goblin, Kind: encounter.KindMonster, Room: stepWest,
				Position: spatial.Position{X: 4, Y: 4}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	_, err = enc.Step(&encounter.StepInput{
		Member: alice, To: stepAbs(stepWestOrigin, spatial.Position{X: 2, Y: 1})})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrInBubble)
}

// TestGridReportsTheFieldsFamily covers the cheap read the seam needs in place
// of an Atlas call.
//
// Adjacency is grid-family arithmetic, and the family is a FIELD fact by law
// (W1: one family per field). Getting it out of Atlas meant enumerating every
// cell of every room — measured at ~128MB and tens of milliseconds at the
// legal field budget — to read one enum.
func (s *StepSuite) TestGridReportsTheFieldsFamily() {
	shape, err := s.enc.Grid()
	s.Require().NoError(err)
	s.Equal(spatial.GridShapeSquare, shape)
}

// TestGridReportsAHexFieldAsHex is the discriminating half: square is the zero
// value of GridShape, so a verb that returned nothing at all would pass the
// square case.
func (s *StepSuite) TestGridReportsAHexFieldAsHex() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		Field: encounter.FieldInput{Rooms: []encounter.RoomInput{
			{ID: stepWest, Width: 6, Height: 6, Grid: spatial.GridShapeHex,
				Origin: stepWestOrigin},
		}},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Room: stepWest,
				Position: spatial.Position{X: 0, Y: 0}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	shape, err := enc.Grid()
	s.Require().NoError(err)
	s.Equal(spatial.GridShapeHex, shape)
}
