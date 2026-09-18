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
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 6, Y: 4},
				Temper: encounter.Temper{Word: "coward", Profile: encounter.TemperProfile{
					Attack: 50, Toward: 50, Away: 300, Flee: 300, Hold: 100}}},
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

// TestTheTemperCrossesLikeTargeting: the word and the profile the caller
// filled for it reach the member record verbatim, and round-trip through Data
// (rpg-project#465 — this was the `mind` word, and a mind's numbers lived in
// Go where nobody could see them).
func (s *DeedTestSuite) TestTheTemperCrossesLikeTargeting() {
	enc := s.scene()
	s.Equal("coward", s.temperOf(enc, goblin).Word)
	s.Equal(300, s.temperOf(enc, goblin).Profile.Away, "the numbers cross too — a temper IS its profile")

	reloaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: enc.ToData(), Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{}, Standing: everyoneStanding{},
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{},
		Announcer: quietAnnouncer{},
	})
	s.Require().NoError(err)
	s.Equal("coward", s.temperOf(reloaded, goblin).Word, "and survives a reload")
	s.Equal(300, s.temperOf(reloaded, goblin).Profile.Away, "profile and all")
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

func (s *DeedTestSuite) temperOf(enc *encounter.Encounter, id encounter.MemberID) encounter.Temper {
	members, err := enc.Members()
	s.Require().NoError(err)
	for _, m := range members {
		if m.ID == id {
			return m.Temper
		}
	}
	s.Require().Failf("not a member", "%s", id)
	return encounter.Temper{}
}

// TestTheViewCarriesWhatADriverNeeds: rule A2. The goblin's view names its
// own policy, carries every holding as values — the sight of alice and the deed
// she just did — stamps when, and precomputes the step away from her, so a
// driver decides without reaching the canvas.
func (s *DeedTestSuite) TestTheViewCarriesWhatADriverNeeds() {
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
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 6, Y: 4}, SpeedFeet: 30, Actions: []encounter.ActionView{{Ref: testMeleeAction, RangeFeet: 5}}},
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
	s.Equal(encounter.Temper{}, view.Temper, "no temperament is a soldier, and stores nothing")
	s.Require().Len(view.Deeds, 1, "the deed done TO it is decoded onto the view, not left as bytes")
	s.Equal(encounter.DeedAttack, view.Deeds[0].Kind)
	s.Equal(alice, view.Deeds[0].Actor, "and it names who did it, which is what `attack: attacker` reads")
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
