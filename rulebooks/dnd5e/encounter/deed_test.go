// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/deed"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// DeedTestSuite is mind/behavior rule A4 on a real board: a deed is landed
// where the fact is known — Record — on every member whose senses reach the
// actor's cell, and on nobody else.
type DeedTestSuite struct {
	suite.Suite
}

func TestDeedSuite(t *testing.T) {
	suite.Run(t, new(DeedTestSuite))
}

// scene is the outcome suite's room with one more member: billy stands on
// the far side of the wall row, where he can see neither alice nor goblin.
func (s *DeedTestSuite) scene() *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion(outcomeRoom, 0, 0, 12, 12)},
			Props:   wallRow(6, 4, 8),
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 2}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 6, Y: 4}, Mind: "retaliator"},
			{ID: billy, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 10}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	return enc
}

func (s *DeedTestSuite) holdingOf(enc *encounter.Encounter, observer, subject encounter.MemberID) (perception.Holding, bool) {
	holdings, err := enc.View(&encounter.ViewInput{Member: observer})
	s.Require().NoError(err)
	for _, h := range holdings {
		if h.Subject == subject {
			return h, true
		}
	}
	return perception.Holding{}, false
}

// TestAStrikeLandsADeedOnWhoeverSawIt: alice strikes goblin. Goblin, who
// can see alice, holds the deed with alice named as its actor and itself as
// the target; it is on the deeds channel, under alice's qualified subject,
// and current on nothing. Billy, behind the wall, holds nothing of it.
func (s *DeedTestSuite) TestAStrikeLandsADeedOnWhoeverSawIt() {
	enc := s.scene()

	_, err := enc.Record(&encounter.RecordInput{
		Kind:    encounter.OutcomeStruck,
		Actor:   alice,
		Targets: []encounter.MemberID{goblin},
		Values: map[encounter.OutcomeValue]int{
			encounter.ValueRoll: 17, encounter.ValueTotal: 22, encounter.ValueAgainst: 15, encounter.ValueAmount: 9,
		},
		Calculation: attackCalculation(17, 5, 0),
	})
	s.Require().NoError(err)

	held, ok := s.holdingOf(enc, goblin, deed.Subject(alice))
	s.Require().True(ok, "the goblin saw alice attack")
	s.Equal(deed.Channel, held.Channel, "on the deeds channel")
	s.Empty(held.CurrentVia, "a deed is in the past the moment it exists")

	saw, err := deed.Decode(held.Payload)
	s.Require().NoError(err)
	s.Equal(encounter.DeedAttack, saw.Verb)
	s.Equal(alice, saw.Actor, "the goblin can see alice, so it knows who")
	s.Equal(goblin, saw.Target, "and that it was the one attacked")
	s.Equal(s.cellOf(enc, alice).String(), saw.Where, "where alice stood, as the board has her")

	_, ok = s.holdingOf(enc, billy, deed.Subject(alice))
	s.False(ok, "billy, behind the wall, never learned a shot happened")
}

// TestAMissIsStillAShotAtYou: the same deed lands on a miss.
func (s *DeedTestSuite) TestAMissIsStillAShotAtYou() {
	enc := s.scene()

	_, err := enc.Record(&encounter.RecordInput{
		Kind:    encounter.OutcomeMissed,
		Actor:   alice,
		Targets: []encounter.MemberID{goblin},
		Values: map[encounter.OutcomeValue]int{
			encounter.ValueRoll: 3, encounter.ValueTotal: 8, encounter.ValueAgainst: 15,
		},
		Calculation: attackCalculation(3, 5, 0),
	})
	s.Require().NoError(err)

	held, ok := s.holdingOf(enc, goblin, deed.Subject(alice))
	s.Require().True(ok)
	saw, err := deed.Decode(held.Payload)
	s.Require().NoError(err)
	s.Equal(encounter.DeedAttack, saw.Verb)
}

// TestTheMindCrossesLikeTargeting: the sheet's word reaches the member
// record verbatim, and round-trips through Data.
func (s *DeedTestSuite) TestTheMindCrossesLikeTargeting() {
	enc := s.scene()
	s.Equal("retaliator", s.mindOf(enc, goblin))

	reloaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: enc.ToData(), Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{}, Standing: everyoneStanding{},
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{},
		Announcer: quietAnnouncer{},
	})
	s.Require().NoError(err)
	s.Equal("retaliator", s.mindOf(reloaded, goblin), "and survives a reload")
}

func (s *DeedTestSuite) cellOf(enc *encounter.Encounter, id encounter.MemberID) spatial.Position {
	members, err := enc.Members()
	s.Require().NoError(err)
	for _, m := range members {
		if m.ID == id {
			return m.Position
		}
	}
	s.Require().Failf("not a member", "%s", id)
	return spatial.Position{}
}

func (s *DeedTestSuite) mindOf(enc *encounter.Encounter, id encounter.MemberID) string {
	members, err := enc.Members()
	s.Require().NoError(err)
	for _, m := range members {
		if m.ID == id {
			return m.Mind
		}
	}
	s.Require().Failf("not a member", "%s", id)
	return ""
}

// TestTheViewCarriesWhatAMindNeeds: rule A2. The goblin's view names its
// mind, carries every holding as values — the sight of alice and the deed
// she just did — stamps when, and precomputes the step away from her, so a
// driver decides without reaching the canvas.
func (s *DeedTestSuite) TestTheViewCarriesWhatAMindNeeds() {
	driver := &scriptedDriver{}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: driver, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion(outcomeRoom, 0, 0, 12, 12)},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 2}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 6, Y: 4}, Mind: "retaliator",
				SpeedFeet: 30, Actions: []encounter.ActionView{{Ref: testMeleeAction, RangeFeet: 5}}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	_, err = enc.Record(&encounter.RecordInput{
		Kind: encounter.OutcomeMissed, Actor: alice, Targets: []encounter.MemberID{goblin},
		Values:      map[encounter.OutcomeValue]int{encounter.ValueRoll: 3, encounter.ValueTotal: 8, encounter.ValueAgainst: 15},
		Calculation: attackCalculation(3, 5, 0),
	})
	s.Require().NoError(err)

	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
	s.Require().NotEmpty(driver.calls, "the goblin's turn was driven")

	view := driver.calls[0]
	s.Equal("retaliator", view.Mind, "the sheet's word, verbatim")
	for _, h := range view.Holdings {
		s.LessOrEqual(h.Confirmed, view.At, "nothing held is stamped later than the view")
	}

	var sawAlice, sawDeed bool
	for _, h := range view.Holdings {
		switch h.Subject {
		case alice:
			sawAlice = h.CurrentOn(perception.Sight)
		case deed.Subject(alice):
			sawDeed = h.Channel == deed.Channel
		}
	}
	s.True(sawAlice, "the sight holding alice's Seen entry was decoded from")
	s.True(sawDeed, "and the deed Seen drops")

	s.Require().Len(view.Seen, 1)
	s.Equal(alice, view.Seen[0].ID)
	s.NotEmpty(view.Seen[0].AwayPath, "one step that puts more board between them")
	s.NotEqual(view.Seen[0].Position, view.Seen[0].AwayPath[0])
}
