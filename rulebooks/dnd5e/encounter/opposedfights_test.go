// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// opposedfights_test.go is FIGHTS FORM BETWEEN OPPOSED MONSTERS, ANY KIND
// (rpg-project#569, the mad king D0): the three-sided fight of "turn the
// garrison", where the party and the turned garrison both fight the
// loyalists and neither fights the other.
//
//	classify pairs every two contact members the graph holds hostile, of any
//	kind, and forms only a pair a player anchors — a player, or a member
//	already in the player's fight;
//	two monsters making contact with no player in it is no formation, not an
//	error, and the pair comes into the fight when a player makes contact
//	with either side;
//	the aggression law's turn between two monster factions forms the fight
//	when one side is already in the party's;
//	a guard who turns on the thugs joins the party's fight against them and
//	never swings at the party.
//
// THE YARD is the both-ways yard's shape: open air, everybody in sight of
// everybody unless the scene dims somebody. The graph, not the kind, decides
// who fights.

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

// sworn is the declared war between the garrison and the loyalists.
func sworn() encounter.DispositionInput {
	return encounter.DispositionInput{
		Between: [2]encounter.FactionID{ofGarrison, ofLoyalists}, Stance: encounter.StanceHostile,
	}
}

// TestTwoHostileMonsterFactionsComeIntoThePartysFight is classify's new
// table: two monster factions declared hostile and watching each other form
// nothing on their own, and both come into the fight the moment the party
// makes contact with one of them.
func (s *OpposedFightsSuite) TestTwoHostileMonsterFactionsComeIntoThePartysFight() {
	factions := []encounter.FactionInput{{ID: ofGarrison}, {ID: ofLoyalists}}
	members := []encounter.MemberInput{
		player(alice, 11, 4),
		monster(ofGuard, ofGarrison, 4, 1),
		monster(ofLoyalist, ofLoyalists, 5, 1),
	}

	s.Run("with no player in the contact it is no formation, and no error", func() {
		enc := s.open(s.yard(factions, sworn()), members, everyoneSeesTheWholeMap{})
		s.Equal(encounter.ClockWorld, s.clockOf(enc, ofGuard), "a fight nobody plays does not form")
		s.Equal(encounter.ClockWorld, s.clockOf(enc, ofLoyalist))
		hostile, known := enc.IsHostile(ofGuard, ofLoyalist)
		s.True(known)
		s.True(hostile, "they are still at war: the graph says so and `enemy:` answers so")
	})

	s.Run("the party making contact with the guard brings both sides in", func() {
		// The garrison is hostile to the party (declared with nothing toward
		// it); the loyalists are neutral to it. Only the pair can see at
		// first light: alice and the guard are dark until the scene lights
		// them.
		field := s.yard([]encounter.FactionInput{{ID: ofLoyalists}}, sworn())
		field.Factions = append(field.Factions, encounter.FactionInput{ID: ofGarrison})
		light := &sightList{reach: map[encounter.MemberID]int{alice: 0, ofGuard: 2, ofLoyalist: 2}}
		enc := s.open(field, members, light)
		s.Require().Equal(encounter.ClockWorld, s.clockOf(enc, ofGuard), "precondition: no player, no fight")

		light.reach[alice], light.reach[ofGuard] = unlimitedSight, unlimitedSight
		_, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(11, 3)})
		s.Require().NoError(err)

		form := s.formed(enc, alice)
		s.Require().NotNil(form, "the guard and alice saw each other: a fight with a player in it")
		s.ElementsMatch([]string{string(alice), string(ofGuard), string(ofLoyalist)}, engaged(form),
			"and the loyalist who has been watching the guard comes in with him")
		hostile, _ := enc.IsHostile(ofLoyalist, alice)
		s.False(hostile, "the loyalist is in the party's fight, and not against the party")
	})

	s.Run("spotted: a loyalist who cannot see the guard comes in unaware", func() {
		field := s.yard([]encounter.FactionInput{{ID: ofLoyalists}}, sworn())
		field.Factions = append(field.Factions, encounter.FactionInput{ID: ofGarrison})
		blind := &sightList{reach: map[encounter.MemberID]int{ofLoyalist: 0}, fallback: unlimitedSight}
		enc := s.open(field, members, blind)

		form := s.formed(enc, alice)
		s.Require().NotNil(form, "alice and the guard saw each other")
		s.ElementsMatch([]string{string(alice), string(ofGuard), string(ofLoyalist)}, engaged(form),
			"the guard sees the loyalist, who sees nothing: one-sided, and in")
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

// TestAPlayerWithTheDropIsNeverPulledIn guards pullInWatchers's one
// documented exclusion: it admits creatures, never players. Alice has the
// drop on the guard — she sees him, he does not see her — when bob's contact
// with the guard forms a fight. Alice watching an engaged enemy is exactly
// the link a creature is pulled in by, and she keeps her drop
// (rpg-toolkit#1020) until the guard notices her.
func (s *OpposedFightsSuite) TestAPlayerWithTheDropIsNeverPulledIn() {
	field := s.yard(nil)
	field.Factions = []encounter.FactionInput{{ID: ofGarrison}} // hostile to the party
	light := &sightList{reach: map[encounter.MemberID]int{ofGuard: 0}, fallback: unlimitedSight}
	enc := s.open(field, []encounter.MemberInput{
		player(alice, 11, 4), player(bob, 4, 2), monster(ofGuard, ofGarrison, 4, 1),
	}, light)
	s.Require().Equal(encounter.ClockWorld, s.clockOf(enc, alice), "precondition: alice has the drop")

	light.reach[ofGuard] = 2 // the guard can now see bob beside him, and not alice across the yard
	_, err := enc.Step(&encounter.StepInput{Member: bob, To: cellAt(4, 3)})
	s.Require().NoError(err)

	form := s.formed(enc, bob)
	s.Require().NotNil(form, "the guard noticed bob: a fight")
	s.ElementsMatch([]string{string(bob), string(ofGuard)}, engaged(form))
	s.Equal(encounter.ClockWorld, s.clockOf(enc, alice), "alice watching the guard is not pulled in")
}

// TestTheAggressionLawBetweenTwoMonsterFactionsFormsTheFight is the turn as
// a transition between two monster factions: a loyalist the party is
// fighting strikes a guard of a neutral faction, the pair is hostile from
// that moment, and the guard comes into the fight — while the same swing
// with no player anywhere turns the pair and forms nothing.
func (s *OpposedFightsSuite) TestTheAggressionLawBetweenTwoMonsterFactionsFormsTheFight() {
	strike := func(enc *encounter.Encounter, actor, target core.EntityID) {
		_, err := recordOne(enc, &encounter.RecordInput{
			Kind: encounter.OutcomeStruck, Actor: actor, Targets: []encounter.MemberID{target},
			Values: map[encounter.OutcomeValue]int{encounter.ValueAmount: 3},
		})
		s.Require().NoError(err)
	}
	stance := func(enc *encounter.Encounter) encounter.Stance {
		out, err := enc.Stance(ofGarrison, ofLoyalists)
		s.Require().NoError(err)
		return out
	}

	s.Run("in the party's fight: the guard joins it", func() {
		// The loyalists are hostile to the party; the garrison is neutral to
		// everyone until somebody swings at it.
		field := s.yard([]encounter.FactionInput{{ID: ofGarrison}})
		field.Factions = append(field.Factions, encounter.FactionInput{ID: ofLoyalists})
		enc := s.open(field, []encounter.MemberInput{
			player(alice, 4, 2),
			monster(ofLoyalist, ofLoyalists, 5, 1),
			monster(ofGuard, ofGarrison, 8, 4),
		}, everyoneSeesTheWholeMap{})
		s.Require().Equal([]encounter.MemberID{alice, ofLoyalist}, s.order(enc, alice),
			"precondition: the party is fighting the loyalist, and the guard is civil")

		strike(enc, ofLoyalist, ofGuard)

		s.Equal(encounter.StanceHostile, stance(enc), "the law turned the pair")
		s.Equal([]encounter.MemberID{alice, ofLoyalist, ofGuard}, s.order(enc, alice),
			"the guard joined the running fight, last in the order")
		hostile, _ := enc.IsHostile(ofGuard, alice)
		s.False(hostile, "and is not the party's enemy")
	})

	s.Run("with no player in it: the pair turns, and nothing forms", func() {
		enc := s.open(s.yard([]encounter.FactionInput{{ID: ofGarrison}, {ID: ofLoyalists}}),
			[]encounter.MemberInput{
				player(alice, 0, 0),
				monster(ofGuard, ofGarrison, 4, 1),
				monster(ofLoyalist, ofLoyalists, 5, 1),
			}, everyoneSeesTheWholeMap{})

		strike(enc, ofGuard, ofLoyalist)

		s.Equal(encounter.StanceHostile, stance(enc), "the law turned the pair")
		s.Equal(encounter.ClockWorld, s.clockOf(enc, ofGuard), "a fight nobody plays does not form")
		s.Equal(encounter.ClockWorld, s.clockOf(enc, ofLoyalist))
	})
}

// TestAGuardWhoTurnedOnTheThugsJoinsThePartysFight is "turn the garrison" in
// miniature: the party is fighting a thug; the captain learns the guards have
// turned, the garrison is hostile to the thugs from that moment, and the
// captain joins the party's fight — against the thug, and never the party.
func (s *OpposedFightsSuite) TestAGuardWhoTurnedOnTheThugsJoinsThePartysFight() {
	swings := encounter.Table{encounter.AnswerTime: {
		{Weight: 1, When: []encounter.When{{Enemy: encounter.EnemyReach}},
			Attack: &encounter.Selector{Word: encounter.SelectorEnemy, Target: encounter.TargetClosest}},
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

// walkAsTheSessionDoes walks a member down column 4 to the given row, one
// cell at a time, asking AdmitWalk before every step exactly as the session's
// Move does (session/move.go) — the shape the castle walk found the bard
// pulled into the fighter's bubble by.
func (s *OpposedFightsSuite) walkAsTheSessionDoes(enc *encounter.Encounter, who core.EntityID, fromRow, toRow int) {
	for row := fromRow - 1; row >= toRow; row-- {
		admitted, err := enc.AdmitWalk(&encounter.AdmitWalkInput{Member: who})
		s.Require().NoError(err)
		if admitted.Joined {
			return
		}
		_, err = enc.Step(&encounter.StepInput{Member: who, To: cellAt(4, row), EndWalk: row == toRow})
		s.Require().NoError(err)
		if s.clockOf(enc, who) == encounter.ClockTurn {
			return // the step made contact: the walk ends in the fight, as the session's does
		}
	}
}

// TestSeeingAnAllyInAFightIsNotContact is the castle walk's finding
// (rpg-project#569, D0): alice is fighting a goblin; bob walks up until he
// can see alice and then the goblin, and the goblin never sees him. Bob is in
// contact with nobody — sight of an ally is not contact, and his sight of the
// goblin is a drop — so he stays on the world clock. Before the fix the walk
// admitted him the step after alice came into view.
func (s *OpposedFightsSuite) TestSeeingAnAllyInAFightIsNotContact() {
	open := func(goblinReach int, goblinFaction encounter.FactionID) *encounter.Encounter {
		field := s.yard(nil)
		field.Regions = []encounter.RegionInput{rectRegion("yard", 0, 0, 12, 12)}
		if goblinFaction != "" {
			field.Factions = []encounter.FactionInput{{ID: goblinFaction}}
			field.Dispositions = []encounter.DispositionInput{{
				Between: [2]encounter.FactionID{goblinFaction, encounter.FactionParty}, Stance: encounter.StanceNeutral,
			}}
		}
		light := &sightList{reach: map[encounter.MemberID]int{alice: 2, bob: 3, goblin: goblinReach}}
		gob := encounter.MemberInput{ID: goblin, Kind: encounter.KindMonster, Faction: goblinFaction, Position: spatial.Position{X: 4, Y: 1}}
		return s.openWith(field, []encounter.MemberInput{player(alice, 4, 2), player(bob, 4, 10), gob},
			light, sheetFacts{alice: {SpeedFeet: 30}, bob: {SpeedFeet: 30}, goblin: {SpeedFeet: 30}},
			passDriver{}, passStriker{})
	}

	s.Run("bob walking into sight of alice only stays on the world clock", func() {
		enc := open(2, "")
		s.Require().Equal(encounter.ClockTurn, s.clockOf(enc, alice), "precondition: alice is fighting the goblin")
		s.walkAsTheSessionDoes(enc, bob, 10, 4)
		s.Equal(encounter.ClockWorld, s.clockOf(enc, bob), "seeing his teammate fight is not contact")
		// From the last cell he sees the goblin, which does not see him: the
		// drop is his (rpg-toolkit#1020), and the next step's admission ask
		// says so.
		admitted, err := enc.AdmitWalk(&encounter.AdmitWalkInput{Member: bob})
		s.Require().NoError(err)
		s.False(admitted.Joined, "bob sees the goblin unseen: the drop is his, not admission")
		s.Equal([]encounter.MemberID{alice, goblin}, s.order(enc, alice))
	})

	s.Run("bob walking into the goblin's sight joins: the goblin spotted him", func() {
		enc := open(4, "")
		s.walkAsTheSessionDoes(enc, bob, 10, 4)
		s.Equal(encounter.ClockTurn, s.clockOf(enc, bob), "a hostile seeing him is contact")
		s.Equal([]encounter.MemberID{alice, goblin, bob}, s.order(enc, alice), "joined last, as a straggler")
	})

	s.Run("a neutral creature seen by a player forms nothing", func() {
		enc := open(4, "crowd")
		s.Equal(encounter.ClockWorld, s.clockOf(enc, alice), "alice beside a neutral is in no fight")
		s.walkAsTheSessionDoes(enc, bob, 10, 4)
		s.Equal(encounter.ClockWorld, s.clockOf(enc, bob))
		s.Equal(encounter.ClockWorld, s.clockOf(enc, goblin))
	})
}
