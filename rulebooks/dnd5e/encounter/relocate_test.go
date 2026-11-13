// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// relocate_test.go is A MEMBER MOVED TO A PLACE (rpg-project#925) on the
// noise hall: three rooms in a row, a | b | c, both doors SHUT, so sight stops
// at every seam. Alice is set down in room c from room a, across two shut
// doors and two walls. Bob stands beside her in a, Carol in c, Dan in b, who
// sees neither end.

const (
	relAlice encounter.MemberID = "alice"
	relBob   encounter.MemberID = "bob"
	relCarol encounter.MemberID = "carol"
	relDan   encounter.MemberID = "dan"
	relOrc   encounter.MemberID = "orc"
)

// relCause is the cause every relocation here names.
var relCause = encounter.RelocationCause{
	Ref:   core.Ref{Module: "dnd5e", Type: "consequences", ID: "test-cut"},
	Words: "The floor tilts, and you are somewhere else.",
}

type RelocateSuite struct {
	suite.Suite
}

func TestRelocateSuite(t *testing.T) { suite.Run(t, new(RelocateSuite)) }

// hall is the shut hall with the given members, every one with a sheet.
func (s *RelocateSuite) hall(members ...encounter.MemberInput) *encounter.Encounter {
	sheets := sheetFacts{}
	for _, m := range members {
		sheets[m.ID] = encounter.SheetFacts{SpeedFeet: 30}
	}
	return noiseHall(s.T(), encounter.DoorIsClosed(), encounter.DoorIsClosed(), sheets, members...)
}

// party is Alice and Bob in a, Dan in b, Carol in c.
func (s *RelocateSuite) party() *encounter.Encounter {
	return s.hall(
		at(relAlice, encounter.KindPlayer, 2, 5), at(relBob, encounter.KindPlayer, 4, 5),
		at(relDan, encounter.KindPlayer, 15, 5), at(relCarol, encounter.KindPlayer, 25, 5),
	)
}

// relocated is every relocated beat in member's story.
func (s *RelocateSuite) relocated(enc *encounter.Encounter, member encounter.MemberID) []map[string]any {
	var out []map[string]any
	for _, b := range storyBeats(s.T(), enc, member) {
		if b["beat"] == encounter.BeatRelocated {
			out = append(out, b)
		}
	}
	return out
}

func (s *RelocateSuite) where(enc *encounter.Encounter, id encounter.MemberID) spatial.Position {
	members, err := enc.Members()
	s.Require().NoError(err)
	for _, m := range members {
		if m.ID == id {
			return m.Position
		}
	}
	s.FailNow("no such member", string(id))
	return spatial.Position{}
}

// TestEachEndIsToldOnlyWhatItSaw: Alice is set down in room c across two
// shut doors. She is told both ends and the words; Bob, beside her in a,
// is told she left and never where to; Carol, in c, is told she arrived and
// never from where; Dan, in b, who saw neither, is told nothing. No observer
// is told the words.
func (s *RelocateSuite) TestEachEndIsToldOnlyWhatItSaw() {
	enc := s.party()
	from, to := cellAt(2, 5), cellAt(26, 5)
	s.Require().Equal(from, s.where(enc, relAlice))

	out, err := enc.Relocate(&encounter.RelocateInput{Moves: []encounter.Relocation{{Member: relAlice, To: to}}, Cause: relCause})
	s.Require().NoError(err)
	s.Equal([]encounter.Relocated{{Member: relAlice, From: from, To: to}}, out.Moved)
	s.Equal(to, s.where(enc, relAlice), "set down across two shut doors and two walls")

	alice := s.relocated(enc, relAlice)
	s.Require().Len(alice, 1)
	s.Equal(encounter.RelocatedBoth, alice[0]["end"])
	s.NotNil(alice[0]["from"])
	s.NotNil(alice[0]["to"])
	s.Equal(relCause.Words, alice[0]["words"], "the member is told the words")
	s.Equal(relCause.Ref.String(), alice[0]["cause"])

	bob := s.relocated(enc, relBob)
	s.Require().Len(bob, 1)
	s.Equal(encounter.RelocatedLeft, bob[0]["end"])
	s.NotNil(bob[0]["from"])
	s.NotContains(bob[0], "to", "who saw her leave is never told where she went")
	s.NotContains(bob[0], "words", "nor the words")
	s.Equal(relCause.Ref.String(), bob[0]["cause"], "only the cause")

	carol := s.relocated(enc, relCarol)
	s.Require().Len(carol, 1)
	s.Equal(encounter.RelocatedArrived, carol[0]["end"])
	s.NotNil(carol[0]["to"])
	s.NotContains(carol[0], "from", "who saw her arrive is never told where from")
	s.NotContains(carol[0], "words")

	s.Empty(s.relocated(enc, relDan), "who saw neither end is told nothing")
}

// TestWhoSawBothEndsIsToldBoth: set down across room a, Alice is seen at
// both ends by Bob, who is told both and still not the words.
func (s *RelocateSuite) TestWhoSawBothEndsIsToldBoth() {
	enc := s.party()
	_, err := enc.Relocate(&encounter.RelocateInput{Moves: []encounter.Relocation{{Member: relAlice, To: cellAt(7, 8)}}, Cause: relCause})
	s.Require().NoError(err)
	bob := s.relocated(enc, relBob)
	s.Require().Len(bob, 1)
	s.Equal(encounter.RelocatedBoth, bob[0]["end"])
	s.NotNil(bob[0]["from"])
	s.NotNil(bob[0]["to"])
	s.NotContains(bob[0], "words", "the words are the member's alone")
	s.Empty(s.relocated(enc, relCarol), "Carol, behind two shut doors, saw neither end")
}

// TestWhoSawHerLeaveHoldsAGhost: after the move Bob no longer sees Alice; his
// sighting of her is a ghost, held and not current. Carol now sees her.
func (s *RelocateSuite) TestWhoSawHerLeaveHoldsAGhost() {
	enc := s.party()
	s.Require().Equal(holding{held: true, current: true}, s.holdingOf(enc, relBob, relAlice), "precondition: Bob sees her")
	_, err := enc.Relocate(&encounter.RelocateInput{Moves: []encounter.Relocation{{Member: relAlice, To: cellAt(26, 5)}}, Cause: relCause})
	s.Require().NoError(err)

	s.Equal(holding{held: true, current: false}, s.holdingOf(enc, relBob, relAlice), "a ghost at the old cell")
	s.Equal(holding{held: true, current: true}, s.holdingOf(enc, relCarol, relAlice), "seen where she stands now")
}

type holding struct{ held, current bool }

func (s *RelocateSuite) holdingOf(enc *encounter.Encounter, observer, subject encounter.MemberID) holding {
	held, err := enc.View(&encounter.ViewInput{Member: observer})
	s.Require().NoError(err)
	for _, h := range held {
		if h.Channel == perception.Sight && encounter.MemberID(h.Subject) == subject {
			return holding{held: true, current: len(h.CurrentVia) > 0}
		}
	}
	return holding{}
}

// TestAGroupIsOneFactAppliedWhole: two members move in one call, onto the
// cells FreeCellsIn chose; a refused second move leaves the first unmoved.
func (s *RelocateSuite) TestAGroupIsOneFactAppliedWhole() {
	enc := s.party()
	cells, err := enc.FreeCellsIn("room-c", 2)
	s.Require().NoError(err)

	_, err = enc.Relocate(&encounter.RelocateInput{Moves: []encounter.Relocation{
		{Member: relAlice, To: cells[0]}, {Member: relBob, To: cellAt(15, 5)},
	}, Cause: relCause})
	s.Require().ErrorIs(err, encounter.ErrBadPlacement, "Dan stands on Bob's cell")
	s.Equal(cellAt(2, 5), s.where(enc, relAlice), "a refused group moves nobody")
	s.Empty(s.relocated(enc, relAlice))

	out, err := enc.Relocate(&encounter.RelocateInput{Moves: []encounter.Relocation{
		{Member: relAlice, To: cells[0]}, {Member: relBob, To: cells[1]},
	}, Cause: relCause})
	s.Require().NoError(err)
	s.Len(out.Moved, 2)
	s.Equal(cells[0], s.where(enc, relAlice))
	s.Equal(cells[1], s.where(enc, relBob))
	next, err := enc.FreeCellsIn("room-c", 2)
	s.Require().NoError(err)
	s.NotContains(next, cells[0], "an occupied cell is no longer free")
	s.NotContains(next, cells[1])
}

// TestFreeCellsAreTheSameEveryRun: two runs of one setup choose the same
// cells, so a replay sets the members down where the record says.
func (s *RelocateSuite) TestFreeCellsAreTheSameEveryRun() {
	a, err := s.party().FreeCellsIn("room-c", 3)
	s.Require().NoError(err)
	b, err := s.party().FreeCellsIn("room-c", 3)
	s.Require().NoError(err)
	s.Equal(a, b)
	s.NotContains(a, cellAt(25, 5), "Carol's cell is not free")

	_, err = s.party().FreeCellsIn("room-z", 1)
	s.ErrorIs(err, encounter.ErrBadPlacement, "no such region")
	_, err = s.party().FreeCellsIn("room-c", 10000)
	s.ErrorIs(err, encounter.ErrBadPlacement, "not that many free cells")
	_, err = s.party().FreeCellsIn("room-c", 0)
	s.ErrorIs(err, encounter.ErrInvalidData)
}

// TestReloadNeverRepeatsIt: the run saved after a relocation loads with
// Alice where she was set down, the journal fact standing, and the story
// exactly as long — nothing fires again.
func (s *RelocateSuite) TestReloadNeverRepeatsIt() {
	enc := s.party()
	to := cellAt(26, 5)
	_, err := enc.Relocate(&encounter.RelocateInput{Moves: []encounter.Relocation{{Member: relAlice, To: to}}, Cause: relCause})
	s.Require().NoError(err)
	before := len(storyBeats(s.T(), enc, relAlice))

	blob, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var data encounter.EncounterData
	s.Require().NoError(json.Unmarshal(blob, &data))
	facts := 0
	for _, f := range data.Journal.Facts {
		if strings.HasPrefix(f.Kind, "relocated:alice@") {
			facts++
		}
	}
	s.Equal(1, facts, "one fact records where she was set down")

	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data, Capabilities: noiseHallSetup(
		encounter.DoorIsClosed(), encounter.DoorIsClosed(), sheetFacts{
			relAlice: {SpeedFeet: 30}, relBob: {SpeedFeet: 30}, relCarol: {SpeedFeet: 30}, relDan: {SpeedFeet: 30},
		}, passDriver{}).Capabilities})
	s.Require().NoError(err)
	s.Equal(to, s.where(loaded, relAlice))
	s.Len(storyBeats(s.T(), loaded, relAlice), before, "nothing fires again")
}

// TestRoundsPassWithTheMove: a cause of two rounds raises the world clock by
// two.
func (s *RelocateSuite) TestRoundsPassWithTheMove() {
	enc := s.party()
	before := enc.ToData().Clock.HighWater
	cause := relCause
	cause.Rounds = 2
	_, err := enc.Relocate(&encounter.RelocateInput{Moves: []encounter.Relocation{{Member: relAlice, To: cellAt(26, 5)}}, Cause: cause})
	s.Require().NoError(err)
	s.Equal(before+2, enc.ToData().Clock.HighWater)
}

// TestArrivingInHostileSightFormsAFight: an orc alone in room c sees nobody
// until Alice is set down beside it; the one refresh forms the fight.
func (s *RelocateSuite) TestArrivingInHostileSightFormsAFight() {
	orc := encounter.MemberInput{ID: relOrc, Kind: encounter.KindMonster, Position: spatial.Position{X: 27, Y: 5}}
	enc := s.hall(at(relAlice, encounter.KindPlayer, 2, 5), orc)
	clock, err := enc.ClockOf(&encounter.ClockOfInput{Member: relAlice})
	s.Require().NoError(err)
	s.Require().Equal(encounter.ClockWorld, clock.Kind, "precondition: the walls keep them apart")

	out, err := enc.Relocate(&encounter.RelocateInput{Moves: []encounter.Relocation{{Member: relAlice, To: cellAt(25, 5)}}, Cause: relCause})
	s.Require().NoError(err)
	s.NotNil(out.Formed, "the arrival in hostile sight forms the fight")
}

// TestItRefuses: what a relocation refuses, each before anything changes.
func (s *RelocateSuite) TestItRefuses() {
	enc := s.party()
	move := func(m encounter.MemberID, to spatial.Position) []encounter.Relocation {
		return []encounter.Relocation{{Member: m, To: to}}
	}
	for name, tc := range map[string]struct {
		in   *encounter.RelocateInput
		want error
	}{
		"nil":              {nil, encounter.ErrNilInput},
		"no moves":         {&encounter.RelocateInput{Cause: relCause}, encounter.ErrNoMember},
		"no words":         {&encounter.RelocateInput{Moves: move(relAlice, cellAt(26, 5)), Cause: encounter.RelocationCause{Ref: relCause.Ref}}, encounter.ErrInvalidData},
		"no cause":         {&encounter.RelocateInput{Moves: move(relAlice, cellAt(26, 5)), Cause: encounter.RelocationCause{Words: "x"}}, encounter.ErrInvalidData},
		"negative rounds":  {&encounter.RelocateInput{Moves: move(relAlice, cellAt(26, 5)), Cause: encounter.RelocationCause{Ref: relCause.Ref, Words: "x", Rounds: -1}}, encounter.ErrInvalidData},
		"not a member":     {&encounter.RelocateInput{Moves: move("ghost", cellAt(26, 5)), Cause: relCause}, encounter.ErrNotMember},
		"own cell":         {&encounter.RelocateInput{Moves: move(relAlice, cellAt(2, 5)), Cause: relCause}, encounter.ErrBadPlacement},
		"off the floor":    {&encounter.RelocateInput{Moves: move(relAlice, cellAt(60, 60)), Cause: relCause}, encounter.ErrBadPlacement},
		"occupied":         {&encounter.RelocateInput{Moves: move(relAlice, cellAt(25, 5)), Cause: relCause}, encounter.ErrBadPlacement},
		"one cell twice":   {&encounter.RelocateInput{Moves: append(move(relAlice, cellAt(26, 5)), move(relBob, cellAt(26, 5))...), Cause: relCause}, encounter.ErrBadPlacement},
		"one member twice": {&encounter.RelocateInput{Moves: append(move(relAlice, cellAt(26, 5)), move(relAlice, cellAt(27, 5))...), Cause: relCause}, encounter.ErrInvalidData},
		"negative reach":   {&encounter.RelocateInput{Moves: move(relAlice, cellAt(26, 5)), Cause: relCause, Reach: -1}, encounter.ErrInvalidData},
	} {
		s.Run(name, func() {
			_, err := enc.Relocate(tc.in)
			s.ErrorIs(err, tc.want)
			s.Equal(cellAt(2, 5), s.where(enc, relAlice), "nothing moved")
			s.Empty(s.relocated(enc, relAlice), "nothing told")
		})
	}
}

// TestNoRelocatingInAFight: a member in a fight is refused (the lead's call
// on rpg-project#925).
func (s *RelocateSuite) TestNoRelocatingInAFight() {
	orc := encounter.MemberInput{ID: relOrc, Kind: encounter.KindMonster, Position: spatial.Position{X: 4, Y: 5}}
	enc := s.hall(at(relAlice, encounter.KindPlayer, 2, 5), orc)
	clock, err := enc.ClockOf(&encounter.ClockOfInput{Member: relAlice})
	s.Require().NoError(err)
	s.Require().Equal(encounter.ClockTurn, clock.Kind, "precondition: the orc beside her opened a fight")

	_, err = enc.Relocate(&encounter.RelocateInput{Moves: []encounter.Relocation{{Member: relAlice, To: cellAt(26, 5)}}, Cause: relCause})
	s.ErrorIs(err, encounter.ErrInBubble)
	s.Equal(cellAt(2, 5), s.where(enc, relAlice))
}

// TestTheReadOnlyCanvasRefusesARelocation: the handed-out canvas refuses the
// new mutator as it refuses the other three.
func (s *RelocateSuite) TestTheReadOnlyCanvasRefusesARelocation() {
	enc := s.party()
	canvas, err := enc.Canvas()
	s.Require().NoError(err)
	s.ErrorIs(canvas.RelocateEntity(string(relAlice), cellAt(26, 5)), encounter.ErrReadOnly)
	s.Equal(cellAt(2, 5), s.where(enc, relAlice))
}
