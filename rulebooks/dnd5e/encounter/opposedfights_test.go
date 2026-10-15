// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// opposedfights_test.go is FIGHTS FORM BETWEEN OPPOSED MONSTERS, ANY KIND
// (rpg-project#569, the mad king D0): the three-sided fight of "turn the
// garrison", where the party and the turned garrison both fight the
// loyalists and neither fights the other.
//
//	classify pairs every two contact members the graph holds hostile, of any
//	kind; a one-sided monster↔monster contact forms (spotted), and the drop
//	stays a player's;
//	the aggression law's stance turn between two monster factions forms the
//	fight;
//	a fight nobody plays runs at the world's pace, one round per world round,
//	and forming it is not a round passing;
//	a guard who turns on the thugs joins the party's fight against them and
//	never swings at the party.
//
// THE YARD is the both-ways yard's shape: open air, everybody in sight of
// everybody unless the scene dims somebody. Every monster faction here is
// declared NEUTRAL to the party, so a member of the party who can see the
// whole fight is in none of it — the graph, not the kind, decides who fights.

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	ofGarrison  encounter.FactionID = "garrison"
	ofLoyalists encounter.FactionID = "loyalists"
	ofThugs     encounter.FactionID = "thugs"
	ofTurned                        = "guards-turned"
	ofOrders                        = encounter.PropID("captains-orders")
)

var (
	ofGuard    = core.EntityID("guard")
	ofLoyalist = core.EntityID("loyalist")
	ofCaptain  = core.EntityID("captain")
	ofThug     = core.EntityID("thug")
	ofClub     = core.Ref{Module: "dnd5e", Type: "weapons", ID: "club"}
)

type OpposedFightsSuite struct {
	suite.Suite
}

func TestOpposedFightsSuite(t *testing.T) { suite.Run(t, new(OpposedFightsSuite)) }

// yard is one open region with the named factions, each neutral to the party
// unless the scene's own dispositions say otherwise.
func (s *OpposedFightsSuite) yard(
	factions []encounter.FactionInput, dispositions ...encounter.DispositionInput,
) encounter.FieldInput {
	for _, f := range factions {
		dispositions = append(dispositions, encounter.DispositionInput{
			Between: [2]encounter.FactionID{f.ID, encounter.FactionParty}, Stance: encounter.StanceNeutral,
		})
	}
	return encounter.FieldInput{
		Canvas:       openAir(),
		Regions:      []encounter.RegionInput{rectRegion("yard", 0, 0, 12, 6)},
		Factions:     factions,
		Dispositions: dispositions,
	}
}

// openWith builds the scene with a driver, a striker, a sight model and
// sheets — the four things that differ between a scene about WHO fights and a
// scene about what they DO.
func (s *OpposedFightsSuite) openWith(
	field encounter.FieldInput, members []encounter.MemberInput,
	sight encounter.Sight, sheets encounter.Sheets, driver encounter.Driver, striker encounter.Striker,
) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field:     field,
		Members:   members,
		Endings:   []encounter.EndingInput{withdrawn()},
		Retention: encounter.RetentionUnbounded,
		Capabilities: encounter.Capabilities{
			Sight:         sight,
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        sheets,
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        driver,
			Roller:        rollsLowest{},
			CheckResolver: findsNothing{},
			Witness:       nobodyPerceives{},
			Actors: encounter.Actors{
				Striker:   striker,
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	s.Require().NoError(err)
	return enc
}

// open is a scene about who fights: nobody acts.
func (s *OpposedFightsSuite) open(
	field encounter.FieldInput, members []encounter.MemberInput, sight encounter.Sight,
) *encounter.Encounter {
	return s.openWith(field, members, sight, zeroSheets{}, passDriver{}, passStriker{})
}

func (s *OpposedFightsSuite) clockOf(enc *encounter.Encounter, member core.EntityID) encounter.ClockKind {
	out, err := enc.ClockOf(&encounter.ClockOfInput{Member: member})
	s.Require().NoError(err)
	return out.Kind
}

// formed is the one bubble-formed beat the audience was told, nil if none.
func (s *OpposedFightsSuite) formed(enc *encounter.Encounter, audience core.EntityID) map[string]any {
	story, err := enc.Story(&encounter.StoryInput{Audience: audience})
	s.Require().NoError(err)
	var found map[string]any
	for _, beat := range story {
		var body map[string]any
		s.Require().NoError(json.Unmarshal(beat.Payload, &body))
		if body["beat"] != "bubble-formed" {
			continue
		}
		s.Require().Nil(found, "two fights formed where the scene expects one")
		found = body
	}
	return found
}

// order is the running fight's initiative order, read from one member's clock.
func (s *OpposedFightsSuite) order(enc *encounter.Encounter, member core.EntityID) []encounter.MemberID {
	out, err := enc.ClockOf(&encounter.ClockOfInput{Member: member})
	s.Require().NoError(err)
	s.Require().Equal(encounter.ClockTurn, out.Kind, "%s is in a fight", member)
	return out.Order
}

func (s *OpposedFightsSuite) highWater(enc *encounter.Encounter) int {
	return enc.ToData().Clock.HighWater
}

// sworn is the declared war between the garrison and the loyalists.
func sworn() encounter.DispositionInput {
	return encounter.DispositionInput{
		Between: [2]encounter.FactionID{ofGarrison, ofLoyalists}, Stance: encounter.StanceHostile,
	}
}

// TestTwoHostileMonsterFactionsFightOnSight is classify's new table: two
// monster factions declared hostile, in sight of each other at first light,
// go to initiative with each other and nobody else.
func (s *OpposedFightsSuite) TestTwoHostileMonsterFactionsFightOnSight() {
	factions := []encounter.FactionInput{{ID: ofGarrison}, {ID: ofLoyalists}}
	members := []encounter.MemberInput{
		player(alice, 0, 0),
		monster(ofGuard, ofGarrison, 4, 1),
		monster(ofLoyalist, ofLoyalists, 6, 1),
	}

	s.Run("control: neutral to each other, as undeclared factions are, nobody fights", func() {
		enc := s.open(s.yard(factions), members, everyoneSeesTheWholeMap{})
		s.Equal(encounter.ClockWorld, s.clockOf(enc, ofGuard))
		s.Equal(encounter.ClockWorld, s.clockOf(enc, ofLoyalist))
	})

	s.Run("mutual: both enter, nobody surprised, the party watches", func() {
		enc := s.open(s.yard(factions, sworn()), members, everyoneSeesTheWholeMap{})

		form := s.formed(enc, ofGuard)
		s.Require().NotNil(form, "two hostile factions in plain sight go to initiative")
		s.ElementsMatch([]string{string(ofGuard), string(ofLoyalist)}, engaged(form),
			"the pair the graph holds hostile, and only that pair")
		s.NotContains(form, "surprised", "each saw the other")
		s.Equal(encounter.ClockWorld, s.clockOf(enc, alice),
			"alice sees the whole fight and is opposed to neither side")
	})

	s.Run("spotted: a guard who sees a loyalist that cannot see him starts it", func() {
		blind := &sightList{reach: map[encounter.MemberID]int{ofLoyalist: 0}, fallback: unlimitedSight}
		enc := s.open(s.yard(factions, sworn()), members, blind)

		form := s.formed(enc, ofGuard)
		s.Require().NotNil(form, "a one-sided monster contact forms: nobody has a choice to offer")
		s.ElementsMatch([]string{string(ofGuard), string(ofLoyalist)}, engaged(form))
		s.Equal([]any{string(ofLoyalist)}, form["surprised"], "the one who saw nothing enters unaware")
	})

	s.Run("the drop stays a player's: alice unseen by a hostile camp starts nothing", func() {
		// Declared with nothing toward the party, the loyalists are hostile
		// to it — the default for an authored camp.
		field := s.yard(nil)
		field.Factions = []encounter.FactionInput{{ID: ofLoyalists}}
		blind := &sightList{reach: map[encounter.MemberID]int{ofLoyalist: 0}, fallback: unlimitedSight}
		enc := s.open(field,
			[]encounter.MemberInput{player(alice, 0, 0), monster(ofLoyalist, ofLoyalists, 6, 1)}, blind)

		s.Equal(encounter.ClockWorld, s.clockOf(enc, alice), "a player who gets the drop is offered a choice")
		s.Equal(encounter.ClockWorld, s.clockOf(enc, ofLoyalist))
	})
}

// TestTheAggressionLawBetweenTwoMonsterFactionsFormsTheFight is the turn as
// a transition between two monster factions: a guard strikes a loyalist of a
// neutral faction, the pair is hostile from that moment, and the fight forms
// around everyone of either side who could see it — with no player.
func (s *OpposedFightsSuite) TestTheAggressionLawBetweenTwoMonsterFactionsFormsTheFight() {
	enc := s.open(
		s.yard([]encounter.FactionInput{{ID: ofGarrison}, {ID: ofLoyalists}}),
		[]encounter.MemberInput{
			player(alice, 0, 0),
			monster(ofGuard, ofGarrison, 4, 1),
			monster(ofLoyalist, ofLoyalists, 5, 1),
			monster(ofCaptain, ofGarrison, 8, 4),
		},
		everyoneSeesTheWholeMap{},
	)
	s.Require().Equal(encounter.ClockWorld, s.clockOf(enc, ofGuard), "precondition: two civil factions")

	_, err := recordOne(enc, &encounter.RecordInput{
		Kind: encounter.OutcomeStruck, Actor: ofGuard, Targets: []encounter.MemberID{ofLoyalist},
		Values: map[encounter.OutcomeValue]int{encounter.ValueAmount: 3},
	})
	s.Require().NoError(err)

	stance, err := enc.Stance(ofGarrison, ofLoyalists)
	s.Require().NoError(err)
	s.Equal(encounter.StanceHostile, stance, "the law turned the pair")

	form := s.formed(enc, ofGuard)
	s.Require().NotNil(form, "the turn is the first contact with an enemy, and the fight forms")
	s.ElementsMatch([]string{string(ofGuard), string(ofLoyalist), string(ofCaptain)}, engaged(form),
		"the striker, the struck, and the captain across the yard — a pair is a pair")
	s.NotContains(form, "surprised", "everyone was already watching everyone")
	s.Equal(encounter.ClockWorld, s.clockOf(enc, alice), "the party is in no side of it")
}

// TestAFightNobodyPlaysRunsAtTheWorldsPace is the driver half: a fight of
// creatures alone takes no turn when it forms, and one round of turns — each
// side with an attack — for every round the party lives.
func (s *OpposedFightsSuite) TestAFightNobodyPlaysRunsAtTheWorldsPace() {
	swings := encounter.Table{encounter.AnswerTime: {
		{Weight: 1, When: []encounter.When{{Enemy: encounter.EnemyReach}},
			Attack: &encounter.Selector{Word: encounter.SelectorEnemy}},
		{Weight: 1, Hold: true},
	}}
	guard := monster(ofGuard, ofGarrison, 4, 1)
	guard.Table = swings
	loyalist := monster(ofLoyalist, ofLoyalists, 5, 1)
	loyalist.Table = swings
	melee := encounter.SheetFacts{SpeedFeet: 30, Actions: []encounter.ActionView{{Ref: ofClub, RangeFeet: 5}}}
	striker := &scriptedStriker{kind: encounter.OutcomeMissed}

	enc := s.openWith(
		s.yard([]encounter.FactionInput{{ID: ofGarrison}, {ID: ofLoyalists}}, sworn()),
		[]encounter.MemberInput{player(alice, 0, 0), guard, loyalist},
		everyoneSeesTheWholeMap{},
		sheetFacts{alice: {SpeedFeet: 30}, ofGuard: melee, ofLoyalist: melee},
		tableDriver(), striker,
	)
	s.Require().Equal(encounter.ClockTurn, s.clockOf(enc, ofGuard), "precondition: they fight on sight")
	s.Require().Equal(encounter.ClockWorld, s.clockOf(enc, alice))

	s.Run("forming it took nobody's turn", func() {
		s.Empty(striker.calls, "no round has passed")
	})

	before := s.highWater(enc)
	_, err := aRound(enc)
	s.Require().NoError(err)

	s.Run("one round of the world is one round of the fight, each side swinging", func() {
		s.Require().Len(striker.calls, 2, "the guard's turn and the loyalist's: one round, not the fight")
		s.Equal(ofGuard, striker.calls[0].Attacker)
		s.Equal(ofLoyalist, striker.calls[0].Target)
		s.Equal(ofLoyalist, striker.calls[1].Attacker)
		s.Equal(ofGuard, striker.calls[1].Target)
	})
	s.Run("and the round cost the world the one round the party paid", func() {
		s.Equal(before+1, s.highWater(enc))
	})

	_, err = aRound(enc)
	s.Require().NoError(err)
	s.Run("the next round of the world is the fight's next round", func() {
		s.Len(striker.calls, 4)
		s.Equal(before+2, s.highWater(enc))
	})
}

// TestAGuardWhoTurnedOnTheThugsJoinsThePartysFight is "turn the garrison" in
// miniature: the party is fighting a thug; the captain learns the guards have
// turned, the garrison is hostile to the thugs from that moment, and the
// captain joins the party's fight — against the thug, and never the party.
func (s *OpposedFightsSuite) TestAGuardWhoTurnedOnTheThugsJoinsThePartysFight() {
	swings := encounter.Table{encounter.AnswerTime: {
		{Weight: 1, When: []encounter.When{{Enemy: encounter.EnemyReach}},
			Attack: &encounter.Selector{Word: encounter.SelectorEnemy}},
		{Weight: 1, Hold: true},
	}}
	captain := monster(ofCaptain, ofGarrison, 3, 1)
	captain.Table = swings
	thug := monster(ofThug, ofThugs, 2, 1)
	thug.Table = swings
	melee := encounter.SheetFacts{SpeedFeet: 30, Actions: []encounter.ActionView{{Ref: ofClub, RangeFeet: 5}}}
	striker := &scriptedStriker{kind: encounter.OutcomeMissed}

	field := s.yard(
		[]encounter.FactionInput{{ID: ofGarrison, Mind: ofCaptain}},
		encounter.DispositionInput{
			Between: [2]encounter.FactionID{ofGarrison, ofThugs},
			Stance:  encounter.StanceNeutral, Until: encounter.TriggerFact{Fact: ofTurned},
		},
	)
	// The thugs are declared with nothing toward the party, so they are
	// hostile to it — the camp the party walked in on.
	field.Factions = append(field.Factions, encounter.FactionInput{ID: ofThugs})
	field.Intel = []encounter.IntelRecord{{ID: "orders", Reveals: encounter.RevealTargets{Fact: ofTurned}}}
	orders := holdableProp(ofOrders, "dnd5e:props:scroll", spatial.Position{X: 4, Y: 1})
	orders.Holds = []encounter.IntelID{"orders"}
	field.Props = []encounter.PropInput{orders}

	enc := s.openWith(field,
		[]encounter.MemberInput{player(alice, 1, 1), thug, captain},
		everyoneSeesTheWholeMap{},
		sheetFacts{alice: melee, ofThug: melee, ofCaptain: melee},
		tableDriver(), striker,
	)
	s.Require().Equal([]encounter.MemberID{alice, ofThug}, s.order(enc, alice),
		"precondition: the party is fighting the thug, and the captain is civil to both")
	s.Require().Equal(encounter.ClockWorld, s.clockOf(enc, ofCaptain))

	_, err := enc.Hold(&encounter.HoldInput{Member: ofCaptain, Target: ofOrders})
	s.Require().NoError(err)

	s.Run("the captain joined the running fight, last in the order", func() {
		s.Equal([]encounter.MemberID{alice, ofThug, ofCaptain}, s.order(enc, alice),
			"joined as a straggler, behind the two already trading blows")
		form := s.formed(enc, alice)
		s.Require().NotNil(form)
		s.ElementsMatch([]string{string(alice), string(ofThug)}, engaged(form),
			"the only fight that ever formed is the one the party started")
	})
	s.Run("and he is on the party's side of it", func() {
		hostile, known := enc.IsHostile(ofCaptain, alice)
		s.True(known)
		s.False(hostile, "the captain and alice are not enemies")
		hostile, _ = enc.IsHostile(ofCaptain, ofThug)
		s.True(hostile)
	})

	// Alice ends her turn: the thug's turn and the captain's are driven.
	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)

	s.Run("the captain swings at the thug, never at the party", func() {
		var captains []encounter.MemberID
		for _, call := range striker.calls {
			if call.Attacker == ofCaptain {
				captains = append(captains, call.Target)
			}
		}
		s.Require().NotEmpty(captains, "the captain had a turn and an enemy in reach")
		for _, target := range captains {
			s.Equal(ofThug, target)
		}
	})
}
