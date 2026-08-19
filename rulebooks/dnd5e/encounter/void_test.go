// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// void_test.go is WHAT THE SPACE BETWEEN THE CHAMBERS IS MADE OF
// (rpg-toolkit#1116).
//
// The canvas spans the field's whole bounding box, so a field of two chambers
// with a gap between them has cells no chamber owns. Until this slice those
// cells were unwalkable but TRANSPARENT — Step refused them and a sight ray
// sailed straight through — which is neither rock nor sky, but what fell out of
// nobody deciding. Kirk ruled that the canvas DECLARES it, as authored data,
// required and never defaulted: "seems like we have some very specific choices
// and these choices could be configured on the canvas."
//
// The fixture is the smallest thing that can tell the two answers apart: two
// four-by-four chambers with ONE COLUMN OF VOID between them, and a member on
// each side of it standing on the same row. Two cells apart, in plain sight of
// each other across a strip of nothing.
//
//	x: 0  1  2  3  |  4  |  5  6  7  8
//	         west  | void|  east
//	west's brenna at (3,1)      east's kade at (5,1)
//
// Nothing about the two declarations differs except the one word each field
// says about that column.
type VoidSuite struct {
	suite.Suite
}

func TestVoidSuite(t *testing.T) {
	suite.Run(t, new(VoidSuite))
}

const (
	voidWest = "west"
	voidEast = "east"

	brenna = core.EntityID("brenna")
	kade   = core.EntityID("kade")
)

var (
	voidWestOrigin = spatial.Position{X: 0, Y: 0} // cells x 0..3
	voidEastOrigin = spatial.Position{X: 5, Y: 0} // cells x 5..8, leaving column 4 void

	brennaCell = spatial.Position{X: 3, Y: 1}
	kadeCell   = spatial.Position{X: 5, Y: 1}
	theGapCell = spatial.Position{X: 4, Y: 1}
)

// gappedField is two chambers with a column of void between them, declaring
// what that column is made of.
func gappedField(void encounter.Void) encounter.FieldInput {
	return encounter.FieldInput{
		Canvas: encounter.CanvasInput{Void: void},
		Rooms: []encounter.RoomInput{
			{ID: voidWest, Width: 4, Height: 4, Origin: voidWestOrigin},
			{ID: voidEast, Width: 4, Height: 4, Origin: voidEastOrigin},
		},
	}
}

// gapped opens the fixture with one member in each chamber, both players so
// that nothing forms a fight and sight is the only thing under test.
func (s *VoidSuite) gapped(void encounter.Void) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		Field: gappedField(void),
		Members: []encounter.MemberInput{
			{ID: brenna, Kind: encounter.KindPlayer, Room: voidWest,
				Position: spatial.Position{X: 3, Y: 1}},
			{ID: kade, Kind: encounter.KindPlayer, Room: voidEast,
				Position: spatial.Position{X: 0, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	return enc
}

// sees reports whether an observer holds anything at all on a subject.
func (s *VoidSuite) sees(enc *encounter.Encounter, observer, subject core.EntityID) bool {
	view, err := enc.View(&encounter.ViewInput{Member: observer})
	s.Require().NoError(err)
	for _, h := range view {
		if h.Subject == intel.Subject(subject) {
			return true
		}
	}

	return false
}

// TestRockVoidBlocksSightBetweenChambers is the slice, from the tomb's side.
//
// One column of rock stands between brenna and kade, and it is not a wall
// anybody drew: it is the space the authored chambers left, which this field
// declared is stone. Neither of them sees the other, and that is the same
// answer a wall would give — because it IS the same answer. Rock is rock.
func (s *VoidSuite) TestRockVoidBlocksSightBetweenChambers() {
	enc := s.gapped(encounter.VoidIsRock())

	s.False(s.sees(enc, brenna, kade), "a column of rock stands between them")
	s.False(s.sees(enc, kade, brenna), "from either side — geometry is mutual")
}

// TestOpenVoidDoesNotBlockSightBetweenChambers is the same slice from the
// ruined-courtyard side, and it is the half that shows the declaration is a
// CHOICE rather than a new rule.
//
// Identical geometry, identical members, identical distance. The only thing
// that changed is the word the field says about that column, and the answer
// inverts.
func (s *VoidSuite) TestOpenVoidDoesNotBlockSightBetweenChambers() {
	enc := s.gapped(encounter.VoidIsOpen())

	s.True(s.sees(enc, brenna, kade), "the gap is open sky; they are two cells apart in plain view")
	s.True(s.sees(enc, kade, brenna), "from either side")
}

// TestOpenVoidIsStillNotFloor pins the half of the ruling that does NOT vary:
// both declarations are unwalkable. Open means you can SEE across the gap, not
// that you can walk out over it.
func (s *VoidSuite) TestOpenVoidIsStillNotFloor() {
	for _, tc := range []struct {
		name string
		void encounter.Void
	}{
		{"rock", encounter.VoidIsRock()},
		{"open", encounter.VoidIsOpen()},
	} {
		s.Run(tc.name, func() {
			enc := s.gapped(tc.void)
			_, err := enc.Step(&encounter.StepInput{Member: brenna, To: theGapCell})
			s.Require().ErrorIs(err, encounter.ErrBadPlacement)
			s.Contains(err.Error(), "not floor")
		})
	}
}

// TestRockVoidDoesNotBlockSightWithinAChamber is the false-positive guard.
//
// A rule that blocked every ray would pass the rock test above and be useless.
// Two members in the SAME chamber, on a ray that never leaves its floor, see
// each other under exactly the declaration that blinded the pair across the
// gap.
func (s *VoidSuite) TestRockVoidDoesNotBlockSightWithinAChamber() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		Field: gappedField(encounter.VoidIsRock()),
		Members: []encounter.MemberInput{
			{ID: brenna, Kind: encounter.KindPlayer, Room: voidWest,
				Position: spatial.Position{X: 0, Y: 0}},
			{ID: kade, Kind: encounter.KindPlayer, Room: voidWest,
				Position: spatial.Position{X: 3, Y: 3}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	s.True(s.sees(enc, brenna, kade), "corner to corner of one chamber, over its own floor")
	s.True(s.sees(enc, kade, brenna))
}

// TestTheHandedOutCanvasAnswersTheSameWay is the dual-state guard, and it is
// the reason this rule lives on the map rather than in the sight loop.
//
// rpg-toolkit#1118 hands the canvas out to be READ, so a rule installed on it
// asks it about line of sight directly. If void opacity were a filter inside
// rebuildPercepts, that caller would be told nothing stands between two cells
// while this module refuses to let them see each other — which is exactly the
// defect #1118 exists to end, reintroduced one layer down.
func (s *VoidSuite) TestTheHandedOutCanvasAnswersTheSameWay() {
	rock, err := s.gapped(encounter.VoidIsRock()).Canvas()
	s.Require().NoError(err)
	s.True(rock.IsLineOfSightBlocked(brennaCell, kadeCell), "the map itself says rock is in the way")

	open, err := s.gapped(encounter.VoidIsOpen()).Canvas()
	s.Require().NoError(err)
	s.False(open.IsLineOfSightBlocked(brennaCell, kadeCell), "and that open sky is not")
}

// TestAFieldMustSayWhatItsVoidIs is the ruling's "required, never defaulted".
//
// There is no answer this module is allowed to pick. A tomb's void is rock and
// an open-air ruin's is sky, and which one THIS world is made of is not a 5e
// rule the composition could derive — it is a fact about the world, which makes
// it construction data (rpg-toolkit#1033's law, applied to the map).
func (s *VoidSuite) TestAFieldMustSayWhatItsVoidIs() {
	_, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{{ID: voidWest, Width: 4, Height: 4}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().ErrorIs(err, encounter.ErrNoField)
	s.Contains(err.Error(), "Canvas.Void", "the refusal names the declaration that is missing")
}

// TestTheDeclarationSurvivesASave round-trips both answers.
//
// It is construction truth, so it persists with the rooms it is a fact about —
// and a reloaded encounter that guessed instead would be a different dungeon
// from the one that was saved.
func (s *VoidSuite) TestTheDeclarationSurvivesASave() {
	for _, tc := range []struct {
		name    string
		void    encounter.Void
		kind    encounter.VoidKind
		blocked bool
	}{
		{"rock", encounter.VoidIsRock(), encounter.VoidRock, true},
		{"open", encounter.VoidIsOpen(), encounter.VoidOpen, false},
	} {
		s.Run(tc.name, func() {
			data := s.gapped(tc.void).ToData()
			s.Equal(string(tc.kind), data.Field.Canvas.Void, "the blob carries the word")

			back, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
				Data:  data,
				Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
			})
			s.Require().NoError(err)

			canvas, err := back.Canvas()
			s.Require().NoError(err)
			s.Equal(tc.blocked, canvas.IsLineOfSightBlocked(brennaCell, kadeCell),
				"the reloaded map is made of what the saved one was")
		})
	}
}

// TestAnOldBlobIsRefusedByName is the standing precedent (#1053/#1068): fail
// loudly, no migration, no default.
//
// A blob written before this slice has no canvas declaration at all. Loading it
// under a guess would put a party in a dungeon whose walls the host never
// authored, so it is refused, and the refusal names what is missing.
func (s *VoidSuite) TestAnOldBlobIsRefusedByName() {
	data := s.gapped(encounter.VoidIsRock()).ToData()

	raw, err := json.Marshal(data)
	s.Require().NoError(err)

	var blob map[string]any
	s.Require().NoError(json.Unmarshal(raw, &blob))
	field, _ := blob["field"].(map[string]any)
	s.Require().NotNil(field)
	delete(field, "canvas")

	raw, err = json.Marshal(blob)
	s.Require().NoError(err)

	var old encounter.EncounterData
	s.Require().NoError(json.Unmarshal(raw, &old))

	_, err = encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data:  old,
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
	})
	s.Require().ErrorIs(err, encounter.ErrNoField)
	s.Contains(err.Error(), "canvas.void", "the refusal names the field the blob does not carry")
	s.Contains(err.Error(), "does not say what its void is",
		"and says the word is ABSENT — a blob that never declared is a different story from one "+
			"declaring a word this build does not know, and the two must not read the same")
}

// TestAnUnknownVoidIsRefusedByName closes the other end of the wire: a word
// this module does not know is not a third kind of void, it is a blob from a
// dialect this build does not speak.
func (s *VoidSuite) TestAnUnknownVoidIsRefusedByName() {
	data := s.gapped(encounter.VoidIsRock()).ToData()
	data.Field.Canvas.Void = "lava"

	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data:  data,
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
	})
	s.Require().ErrorIs(err, encounter.ErrNoField)
	s.Contains(err.Error(), "lava", "the refusal quotes the word it does not know")
	s.Contains(err.Error(), "does not know", "and says it is UNKNOWN rather than absent")
}

// TestTheCanvasKeepsItsOwnCopyOfTheChambers is the alias guard this slice makes
// necessary.
//
// The canvas now asks which cells are FLOOR, and the chambers are what answer.
// Handed the caller's own slice it would answer out of a thing the caller can
// still edit, while [Encounter.RegionAt] answered out of the deep copy Setup
// takes — two answers to one question, diverging silently the moment somebody
// reused their RoomInputs to build a second encounter.
//
// SHIFTING west one cell east is the way to see it, and the choice of field
// matters: [regionAt] asks each room's constructed GRID whether a local cell is
// in bounds, so Width and Height are spent at construction and editing them
// afterwards changes nothing. ORIGIN is the one it still reads on every call —
// measured, not assumed, by mutating this call site back to the caller's slice
// and watching a Width-based version of this test go on passing. Shifted, west
// claims the gap column, the column stops being void, and the rock stops
// blocking.
func (s *VoidSuite) TestTheCanvasKeepsItsOwnCopyOfTheChambers() {
	field := gappedField(encounter.VoidIsRock())
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		Field: field,
		Members: []encounter.MemberInput{
			{ID: brenna, Kind: encounter.KindPlayer, Room: voidWest,
				Position: spatial.Position{X: 3, Y: 1}},
			{ID: kade, Kind: encounter.KindPlayer, Room: voidEast,
				Position: spatial.Position{X: 0, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	canvas, err := enc.Canvas()
	s.Require().NoError(err)
	s.Require().True(canvas.IsLineOfSightBlocked(brennaCell, kadeCell), "rock, before anybody touches anything")

	field.Rooms[0].Origin = spatial.Position{X: 1, Y: 0} // west now covers x 1..4 — in the CALLER's slice

	s.True(canvas.IsLineOfSightBlocked(brennaCell, kadeCell),
		"the encounter's map is not the caller's to redraw after the fact")

	_, floor := enc.RegionAt(theGapCell)
	s.False(floor, "and RegionAt says the same thing the canvas does")
}

// TestNobodySeesOutOfSolidRock is the endpoint half of the rule, and the reason
// the ray is walked from end to end rather than through its middle.
//
// A member is never on a void cell — [Encounter.Step] and [Encounter.Join]
// refuse one — but the canvas rpg-toolkit#1118 hands out takes any two cells a
// caller cares to name, and the honest answer for a cell inside the rock is
// that nothing sees out of it. Under an open sky the same cell is just a place
// nobody can stand, and the sightline across it is clear.
func (s *VoidSuite) TestNobodySeesOutOfSolidRock() {
	rock, err := s.gapped(encounter.VoidIsRock()).Canvas()
	s.Require().NoError(err)
	s.True(rock.IsLineOfSightBlocked(theGapCell, brennaCell), "the gap cell IS the rock")
	s.True(rock.IsLineOfSightBlocked(brennaCell, theGapCell), "from either end")

	open, err := s.gapped(encounter.VoidIsOpen()).Canvas()
	s.Require().NoError(err)
	s.False(open.IsLineOfSightBlocked(theGapCell, brennaCell), "open air is nothing to see through")
	s.False(open.IsLineOfSightBlocked(brennaCell, theGapCell))
}
