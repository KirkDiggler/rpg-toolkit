// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// sight_test.go is HOW FAR ANYONE CAN SEE (rpg-toolkit#1111), the second half
// of rpg-toolkit#1105.
//
// rpg-toolkit#1106 took the blindfold off: the room-membership filter that had
// been standing in for every wall the composition could not express is gone,
// and sight is decided by geometry. What went with the filter was the only
// thing bounding sight at all — the room label was quietly a range term, and
// removing it left a viewer able to see to the far wall of the dungeon.
//
// The term comes back SUPPLIED and PER MEMBER. The composition asks the
// rulebook how far each member can see and uses the answer; it never learns
// what a torch is. Every test here holds one face of that:
//
//   - THE TERM BITES. A member beyond your range is not in your percept, and
//     one inside it is — at the reference tomb's real scale, where the
//     unobstructed run is 27 cells of 5 feet each.
//   - IT IS ASKED, NOT REMEMBERED. Change the answer mid-scene and the next
//     refresh honours it. A cache would pass every other test in this file.
//   - IT IS SUPPLIED, NEVER DEFAULTED. Both constructors refuse a scene that
//     did not say (rpg-toolkit#1033), and a rulebook that answers badly is
//     refused by name rather than quietly rounded off.
//   - GEOMETRY STILL DECIDES FIRST. A wall inside your range still blinds you.
//     The distance term is a ceiling on sight, not a replacement for it.
//   - AND SURPRISE CAME ALIVE. Two members can now answer differently, so a
//     monster can see a player who cannot see back — the first producible
//     input the spotted arm of trigger detection has ever had.
//
// The fixture is canvas_test.go's tomb, reused deliberately: it is the
// reference tomb's shipping shape (chambers 6, 10 and 12 wide by 8 tall, both
// doorways on one row), which is the geometry rpg-project#227 says this stack
// has to carry.
type SightSuite struct {
	suite.Suite
}

func TestSightSuite(t *testing.T) {
	suite.Run(t, new(SightSuite))
}

// darkvision is the range these scenes hand out when they want a real bound:
// 12 cells, which is the 60 feet 5e gives a dwarf. The conversion is the
// rulebook's, and writing it here rather than in the composition is the whole
// point of the seam.
const (
	darkvision = 12 // 60 ft
	torchlight = 4  // 20 ft of bright light
)

// The three cells this file measures against, on the one row where both
// doorways sit — the tomb's only unobstructed run from end to end.
//
// Projected through the fixture's own anchors, never written absolute, so a
// moved chamber cannot leave a stale literal passing (canvas_test.go's rule).
func aliceCell() spatial.Position { return tombAt(tombEntranceOrigin, 0, tombDoorRow) }
func bobCell() spatial.Position   { return tombAt(tombHallOrigin, 5, tombDoorRow) }
func carolCell() spatial.Position { return tombAt(tombChamberOrigin, 11, tombDoorRow) }

// cells is the map's own distance between two cells: Chebyshev, which is what
// spatial's square grid measures and what 5e means by "a diagonal costs five
// feet".
func cells(from, to spatial.Position) int {
	return int(math.Max(math.Abs(to.X-from.X), math.Abs(to.Y-from.Y)))
}

// theLongRow opens the tomb with alice, bob and carol strung out along the row
// both doorways sit on, all players so that nothing forms a fight and the only
// thing deciding percepts is sight.
func (s *SightSuite) theLongRow(sight encounter.Sight) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: sight, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: tombField(),
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: tombSeat(tombEntranceOrigin, 0, tombDoorRow)},
			{ID: bob, Kind: encounter.KindPlayer, Position: tombSeat(tombHallOrigin, 5, tombDoorRow)},
			{ID: carol, Kind: encounter.KindPlayer, Position: tombSeat(tombChamberOrigin, 11, tombDoorRow)},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	return enc
}

// held reports the observer's holding on a subject, and whether there is one at
// all. A subject that faded out of the percept is still HELD — a ghost of where
// it was last seen — so a test about sight has to read the status, not just the
// presence.
func (s *SightSuite) held(enc *encounter.Encounter, observer, subject core.EntityID) (intel.Holding, bool) {
	view, err := enc.View(&encounter.ViewInput{Member: observer})
	s.Require().NoError(err)
	for _, h := range view {
		if h.Subject == intel.Subject(subject) {
			return h, true
		}
	}

	return intel.Holding{}, false
}

// sees reports whether the observer is CURRENTLY seeing the subject — sustained
// by a channel right now, rather than remembered from earlier.
func (s *SightSuite) sees(enc *encounter.Encounter, observer, subject core.EntityID) bool {
	h, ok := s.held(enc, observer, subject)

	return ok && h.Status == intel.Current
}

// TestAMemberBeyondYourSightIsNotInYourPerceptAndOneInsideItIs is the slice in
// one scene, at the tomb's real scale.
//
// alice stands at the mouth of the entrance. bob is 11 cells away down the
// hall — 55 feet, which is what the longest sighting in this dungeon used to
// be, back when the room label was the range term. carol is 27 cells away at
// the far wall of the tomb chamber: 135 feet, the run rpg-toolkit#1106 opened
// up and left unbounded.
//
// With 60 feet of darkvision supplied, alice sees the first and not the second.
// Nothing about the wall changed — the second half of the test proves it, by
// handing the same scene an unbounded answer and watching carol appear.
func (s *SightSuite) TestAMemberBeyondYourSightIsNotInYourPerceptAndOneInsideItIs() {
	near, far := cells(aliceCell(), bobCell()), cells(aliceCell(), carolCell())
	s.Require().Less(near, darkvision, "the fixture's near pair must be INSIDE the supplied range")
	s.Require().Greater(far, darkvision, "and its far pair must be BEYOND it")

	enc := s.theLongRow(&sightList{fallback: darkvision})

	s.True(s.sees(enc, alice, bob),
		fmt.Sprintf("bob is %d ft away and alice can see %d ft", near*5, darkvision*5))
	_, anyHolding := s.held(enc, alice, carol)
	s.False(anyHolding,
		fmt.Sprintf("carol is %d ft away, and a member beyond your sight is not in your percept", far*5))

	// The same geometry, unbounded: carol is visible, so what excluded her
	// above was the distance term and not a wall.
	unbounded := s.theLongRow(&sightList{fallback: unlimitedSight})
	s.True(s.sees(unbounded, alice, carol),
		"the run down the tomb is unobstructed — only the range was hiding her")
}

// TestTheEdgeOfYourSightIsInsideIt pins the boundary, which is the one thing
// about a range predicate that is worth a test of its own: a member standing at
// EXACTLY the distance you can see is visible, and the first cell past it is
// not.
//
// Written as a sweep across the edge rather than as one assertion, because a
// range term is exactly the kind of code where the off-by-one is the whole bug
// and every other test in this file passes with it.
func (s *SightSuite) TestTheEdgeOfYourSightIsInsideIt() {
	apart := cells(aliceCell(), bobCell())

	for reach, want := range map[int]bool{
		apart + 1: true,  // comfortably inside
		apart:     true,  // exactly at the edge, which is inside it
		apart - 1: false, // one cell short
	} {
		enc := s.theLongRow(&sightList{reach: map[encounter.MemberID]int{alice: reach}, fallback: unlimitedSight})
		s.Equal(want, s.sees(enc, alice, bob),
			fmt.Sprintf("bob is %d cells away and alice can see %d", apart, reach))
	}
}

// TestSightIsStillGeometryFirst. The distance term is a CEILING on sight, not a
// replacement for it: carol and dave are one cell apart with a wall between
// them, which is well inside any range, and they are still blind to each other.
//
// This is canvas_test.go's pair, re-asked with the new term present, because a
// range check written in the wrong place — instead of the ray rather than
// before it — would pass every other test in this file.
func (s *SightSuite) TestSightIsStillGeometryFirst() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: &sightList{fallback: darkvision}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: tombField(),
		Members: []encounter.MemberInput{
			{ID: carol, Kind: encounter.KindPlayer, Position: tombSeat(tombEntranceOrigin, 5, tombDoorRow-1)},
			{ID: dave, Kind: encounter.KindPlayer, Position: tombSeat(tombHallOrigin, 0, tombDoorRow-1)},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	adjacent := cells(tombAt(tombEntranceOrigin, 5, tombDoorRow-1), tombAt(tombHallOrigin, 0, tombDoorRow-1))
	s.Require().Less(adjacent, darkvision, "they are close enough that range cannot be what blinds them")

	_, ok := s.held(enc, carol, dave)
	s.False(ok, "a wall inside your sight range still blinds you")
}

// TestTheConeStillNarrows is rpg-toolkit#1106's measurement, re-run with a
// distance term in the way.
//
// A viewer backing away from an open doorway sees a shrinking slice of the
// chamber beyond it, because the doorway frames the view. That is the effect
// S0 measured (73% -> 44% -> 27% -> 18% of the far chamber at 1, 2, 4 and 8
// cells back) and it is the thing a badly-placed range check would destroy, by
// making distance rather than the doorway decide what is visible.
//
// So the range here is deliberately generous — every mark stays well inside it
// — and the narrowing that remains is the doorway's alone.
func (s *SightSuite) TestTheConeStillNarrows() {
	// A rank of marks down the first column of the tomb chamber, one per row:
	// the slice of the chamber a viewer in the hall can see.
	marks := make([]encounter.MemberInput, 0, 8)
	ids := make([]core.EntityID, 0, 8)
	for y := 0; y < 8; y++ {
		id := core.EntityID(fmt.Sprintf("mark-%d", y))
		ids = append(ids, id)
		marks = append(marks, encounter.MemberInput{
			ID: id, Kind: encounter.KindPlayer, Position: tombSeat(tombChamberOrigin, 1, y),
		})
	}

	// Far enough that the whole rank is inside it from every viewing position
	// below: the doorway is the only thing that can narrow this.
	const generous = 30

	seenFrom := func(back int) int {
		members := append([]encounter.MemberInput{{
			ID: alice, Kind: encounter.KindPlayer, Position: tombSeat(tombHallOrigin, 9-back, tombDoorRow),
		}}, marks...)

		enc, err := encounter.NewEncounter(&encounter.SetupInput{
			Sight: &sightList{fallback: generous}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
			Field:   tombField(),
			Members: members,
			Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		})
		s.Require().NoError(err)

		viewer := tombAt(tombHallOrigin, 9-back, tombDoorRow)
		count := 0
		for i, id := range ids {
			s.Require().LessOrEqual(cells(viewer, tombAt(tombChamberOrigin, 1, i)), generous,
				"the premise: every mark is inside the supplied range")
			if s.sees(enc, alice, id) {
				count++
			}
		}

		return count
	}

	inTheDoorway, oneBack, fourBack := seenFrom(0), seenFrom(1), seenFrom(4)

	s.Positive(inTheDoorway, "standing in the doorway she sees into the chamber")
	s.LessOrEqual(oneBack, inTheDoorway, "and less of it from a step back")
	s.Less(fourBack, inTheDoorway, "and strictly less from four cells back — the cone is still a cone")
	s.LessOrEqual(fourBack, oneBack)
}

// TestHowFarSheCanSeeIsAskedAgainEveryTime is the only test here that can tell
// a pull from a cache.
//
// Every other test in this file would pass against a composition that asked
// once at construction and kept the number. This one puts the torch out
// mid-scene: the rulebook's answer changes, nobody is rebuilt, and the very
// next refresh honours it.
//
// bob does not vanish from alice's view — he becomes a GHOST, held at the cell
// she last saw him in. That is intel's own model of memory and it is the right
// answer: losing sight of somebody is not forgetting where they were.
func (s *SightSuite) TestHowFarSheCanSeeIsAskedAgainEveryTime() {
	rulebook := &sightList{fallback: darkvision}
	enc := s.theLongRow(rulebook)

	s.Require().True(s.sees(enc, alice, bob), "the torch is lit and bob is inside its light")

	// The torch gutters out. Nothing is reconstructed; the rulebook simply
	// answers differently the next time it is asked.
	rulebook.reach = map[encounter.MemberID]int{alice: torchlight}

	// Somebody else moves, which is all it takes for the world to look again.
	_, err := enc.Step(&encounter.StepInput{Member: carol, To: tombAt(tombChamberOrigin, 10, tombDoorRow)})
	s.Require().NoError(err)

	s.Require().Greater(cells(aliceCell(), bobCell()), torchlight,
		"the premise: bob is beyond the guttering torch")
	s.False(s.sees(enc, alice, bob), "she cannot see that far any more")

	ghost, ok := s.held(enc, alice, bob)
	s.True(ok, "but she remembers where he was")
	s.Equal(intel.Held, ghost.Status)

	// And it goes both ways: bob's own answer never changed, so he still sees
	// her. Two members, two ranges, one geometry.
	s.True(s.sees(enc, bob, alice), "bob's eyes are as good as they were")
}

// TestTheConsultRunsAtEveryRefreshAndAsksAboutTheWholeRoster pins the shape of
// the question rather than the answer: sorted, complete, and asked again.
func (s *SightSuite) TestTheConsultRunsAtEveryRefreshAndAsksAboutTheWholeRoster() {
	counter := &countingSight{}
	enc := s.theLongRow(counter)

	first := counter.calls
	s.Require().Positive(first, "first light asks")
	s.Require().Equal([]encounter.MemberID{alice, bob, carol}, counter.asked[0],
		"the whole roster, sorted — a rulebook must not have to cope with a different question each verb")

	_, err := enc.Step(&encounter.StepInput{Member: carol, To: tombAt(tombChamberOrigin, 10, tombDoorRow)})
	s.Require().NoError(err)
	s.Greater(counter.calls, first, "and every refresh asks again")
}

// TestTheFarSightedSpotTheShortSightedAndSurpriseThem is the finding this slice
// did not set out to make.
//
// trigger.go has classified four contact cases since rpg-toolkit#964, and two
// of them — the monster saw you and you did not see back, and its mirror — were
// documented as having no producible input, because every observer saw exactly
// as far as every other. A per-member range is that input. The goblin has
// darkvision; alice has a torch; she walks into the range of the first and not
// of the second, and the fight starts with her surprised.
//
// Not one line of classification changed, which is what its own godoc promised:
// upgrades change how percepts are PRODUCED, never how they are consumed.
//
// This is not rpg-toolkit#1020 and does not stand in for it — no stealth is
// contested here, and the geometry is as mutual as it ever was. What differs is
// reach.
func (s *SightSuite) TestTheFarSightedSpotTheShortSightedAndSurpriseThem() {
	const goblin = core.EntityID("goblin")

	scene := func(sight encounter.Sight) *encounter.Encounter {
		enc, err := encounter.NewEncounter(&encounter.SetupInput{
			Sight: sight, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
			Field: tombField(),
			Members: []encounter.MemberInput{
				{ID: alice, Kind: encounter.KindPlayer, Position: tombSeat(tombEntranceOrigin, 0, tombDoorRow)},
				{ID: goblin, Kind: encounter.KindMonster, Position: tombSeat(tombChamberOrigin, 6, tombDoorRow)},
			},
			Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		})
		s.Require().NoError(err)

		return enc
	}

	goblinCell := tombAt(tombChamberOrigin, 6, tombDoorRow)
	halfway := tombAt(tombHallOrigin, 5, tombDoorRow)
	s.Require().Greater(cells(aliceCell(), goblinCell), darkvision,
		"the premise: at the mouth of the tomb she is beyond even the goblin's eyes")
	s.Require().Less(cells(halfway, goblinCell), darkvision,
		"halfway down the hall she is inside them")
	s.Require().Greater(cells(halfway, goblinCell), torchlight,
		"and the goblin is still outside her torchlight")

	s.Run("she is spotted, and surprised", func() {
		enc := scene(&sightList{reach: map[encounter.MemberID]int{goblin: darkvision}, fallback: torchlight})

		out, err := enc.Step(&encounter.StepInput{Member: alice, To: halfway})
		s.Require().NoError(err)

		s.Require().NotNil(out.Formed, "the goblin saw her, so there is a fight")
		s.ElementsMatch([]encounter.MemberID{alice, goblin}, out.Formed.Order)
		s.Equal([]encounter.MemberID{alice}, out.Formed.Surprised,
			"she walked into a fight she could not see")

		s.True(s.sees(enc, goblin, alice), "the goblin has her")
		s.False(s.sees(enc, alice, goblin), "she does not have the goblin")
	})

	s.Run("with matching eyes nobody is surprised", func() {
		enc := scene(&sightList{fallback: darkvision})

		out, err := enc.Step(&encounter.StepInput{Member: alice, To: halfway})
		s.Require().NoError(err)

		s.Require().NotNil(out.Formed)
		s.Empty(out.Formed.Surprised,
			"the surprise came from the difference in reach, not from the fight")
	})
}

// TestASceneThatDoesNotSayHowFarAnyoneCanSeeIsRefused. Both constructors, by
// name.
//
// There is no default and there cannot be one: a number meaning "everyone sees
// this far" is a rule 5e does not have, and picking one would be this module
// deciding a rule it is not allowed to know — the argument Standing makes about
// itself, and rpg-toolkit#1033's law.
func TestASceneThatDoesNotSayHowFarAnyoneCanSeeIsRefused(t *testing.T) {
	t.Run("setup", func(t *testing.T) {
		_, err := encounter.NewEncounter(&encounter.SetupInput{
			Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
			Field: tombField(),
			Members: []encounter.MemberInput{
				{ID: alice, Kind: encounter.KindPlayer, Position: tombSeat(tombEntranceOrigin, 0, tombDoorRow)},
			},
			Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		})
		require.Error(t, err)
		require.ErrorIs(t, err, encounter.ErrNoSight)
	})

	t.Run("load", func(t *testing.T) {
		enc, err := encounter.NewEncounter(&encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
			Field: tombField(),
			Members: []encounter.MemberInput{
				{ID: alice, Kind: encounter.KindPlayer, Position: tombSeat(tombEntranceOrigin, 0, tombDoorRow)},
			},
			Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		})
		require.NoError(t, err)

		_, err = encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{}, Data: enc.ToData(),
		})
		require.Error(t, err)
		require.ErrorIs(t, err, encounter.ErrNoSight)
	})
}

// TestARulebookThatAnswersBadlyIsRefusedByName.
//
// Three shapes of wrong answer, all of them arriving MID-SCENE so the refusal
// is asserted from an encounter that was already running — a mis-wired
// capability has to look like a mis-wired capability, not like a rule that
// silently never fires.
func (s *SightSuite) TestARulebookThatAnswersBadlyIsRefusedByName() {
	walk := func(enc *encounter.Encounter) error {
		_, err := enc.Step(&encounter.StepInput{Member: carol, To: tombAt(tombChamberOrigin, 10, tombDoorRow)})

		return err
	}

	s.Run("a range for somebody who is not a member", func() {
		rulebook := &sightStrangerWhenTold{}
		enc := s.theLongRow(rulebook)

		rulebook.lying = true
		err := walk(enc)
		s.Require().Error(err)
		s.ErrorIs(err, encounter.ErrNotMember)
		s.Contains(err.Error(), "a-ghost", "the refusal names who")
	})

	s.Run("no range at all for somebody who is", func() {
		rulebook := &sightSkippingWhenTold{}
		enc := s.theLongRow(rulebook)

		rulebook.skip = bob
		err := walk(enc)
		s.Require().Error(err)
		s.ErrorIs(err, encounter.ErrNoSight)
		s.Contains(err.Error(), string(bob), "the refusal names who was skipped")
	})

	s.Run("a range that is not a distance", func() {
		rulebook := &sightBelowZero{who: bob, cells: 0}
		enc := s.theLongRow(rulebook)

		rulebook.cells = -1
		err := walk(enc)
		s.Require().Error(err)
		s.ErrorIs(err, encounter.ErrNoSight)
		s.Contains(err.Error(), string(bob))
	})
}

// TestARulebookThatCannotAnswerAbortsTheVerb is R5: a world that cannot find
// out how far somebody can see does not half-build a percept on a guess.
func (s *SightSuite) TestARulebookThatCannotAnswerAbortsTheVerb() {
	rulebook := &sightBrokenWhenTold{}
	enc := s.theLongRow(rulebook)

	rulebook.broken = true
	_, err := enc.Step(&encounter.StepInput{Member: carol, To: tombAt(tombChamberOrigin, 10, tombDoorRow)})
	s.Require().Error(err)
	s.ErrorIs(err, errRulebookCannotSee, "the rulebook's own error survives the wrapping")
}

// TestBlindIsALegalAnswer. Zero cells is a rulebook saying "this one sees
// nothing" — a creature in magical darkness, or blinded — and it is an answer
// rather than a defect. Nobody is ever asked about the cell underfoot, so zero
// means exactly what it says.
func (s *SightSuite) TestBlindIsALegalAnswer() {
	enc := s.theLongRow(&sightList{reach: map[encounter.MemberID]int{alice: 0}, fallback: unlimitedSight})

	_, ok := s.held(enc, alice, bob)
	s.False(ok, "she sees nothing at all")
	s.True(s.sees(enc, bob, alice), "and the rest of the party is unaffected")
}
