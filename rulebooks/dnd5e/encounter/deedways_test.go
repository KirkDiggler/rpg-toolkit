// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// deedways_test.go holds the record of how a deed reaches a creature
// ([encounter.DeedWays]) to the landing code (rpg-project#987): each verb is
// done on a real board, and every member's holding is checked against the
// record for its relation to the deed. A change to who a deed lands on, or
// to the record, fails here.

import (
	"context"
	"slices"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/deed"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// The board's members. Above the wall: alice (a player, the doer), goblin
// (a creature with lines, spoken to) and carl (a creature beside them who
// sees both). Below it: billy (a player) and lurker (a creature), who see
// each other and nothing above.
const (
	dwCarl   encounter.MemberID = "carl"
	dwLurker encounter.MemberID = "lurker"
	dwLever  encounter.PropID   = "lever"
)

// DeedWaysSuite is the record of how a deed reaches a creature, driven.
type DeedWaysSuite struct {
	suite.Suite
	ctx context.Context
}

func TestDeedWaysSuite(t *testing.T) { suite.Run(t, new(DeedWaysSuite)) }

func (s *DeedWaysSuite) SetupTest() { s.ctx = context.Background() }

var dwRoster = []encounter.MemberID{alice, goblin, dwCarl, billy, dwLurker}

// board is the scene; down is who the rulebook says is down.
func (s *DeedWaysSuite) board(down *downList) *encounter.Encounter {
	lever := encounter.PropInput{
		ID: dwLever, Ref: "test:props:lever", At: spatial.Position{X: 5, Y: 2},
		BlocksMovement: no(), BlocksLineOfSight: no(),
		Options: []encounter.Option{{ID: "pull", Do: "Pull the lever.", Seen: "pulls the lever",
			Success: []encounter.Answer{{Weight: 1}}}},
	}
	var standing encounter.Participation = everyoneStanding{}
	if down != nil {
		standing = down
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion(outcomeRoom, 0, 0, 12, 12)},
			Props:   append(wallRow(6, 0, 11), lever, holdableProp(thePurse, "test:props:purse", spatial.Position{X: 4, Y: 2})),
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 2}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 6, Y: 4}, Options: goblinOptions()},
			{ID: dwCarl, Kind: encounter.KindMonster, Position: spatial.Position{X: 7, Y: 3}},
			{ID: billy, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 10}},
			{ID: dwLurker, Kind: encounter.KindMonster, Position: spatial.Position{X: 5, Y: 10}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sight:         everyoneSeesTheWholeMap{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        zeroSheets{},
			Participation: standing,
			Initiative:    orderAsGiven{},
			Driver:        passDriver{},
			Actors: encounter.Actors{
				Striker:   passStriker{},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	s.Require().NoError(err)
	return enc
}

// held is the deed observer holds of verb done by actor (or by someone it
// did not see) to target (or to someone it did not see), as it holds it.
func (s *DeedWaysSuite) held(enc *encounter.Encounter, observer, actor, target encounter.MemberID, verb string) (deed.Deed, bool) {
	holdings, err := enc.View(&encounter.ViewInput{Member: observer})
	s.Require().NoError(err)
	for _, h := range holdings {
		if h.Channel != deed.Channel {
			continue
		}
		d, err := deed.Decode(h.Payload)
		if err != nil || d.Verb != verb {
			continue
		}
		if (d.Actor == actor || d.Actor == "") && (d.Target == target || d.Target == "") {
			return d, true
		}
	}
	return deed.Deed{}, false
}

// way is the record's entry for verb.
func (s *DeedWaysSuite) way(verb string) encounter.DeedWay {
	for _, w := range encounter.DeedWays() {
		if w.Verb == verb {
			return w
		}
	}
	s.FailNow("no way recorded", verb)
	return encounter.DeedWay{}
}

// dwSeen is who saw what of one deed done on the board.
type dwSeen struct {
	// it is who saw the deed done (for a fall, who saw the one who fell).
	it []encounter.MemberID
	// doer and target are who saw each party.
	doer, target []encounter.MemberID
}

// check holds every member to the record: who holds the deed is exactly
// who its relation's reach says, and each holder knows a party only if it
// is that party or saw it ([encounter.DeedReach]).
func (s *DeedWaysSuite) check(enc *encounter.Encounter, verb string, doer, target encounter.MemberID, saw dwSeen) {
	w := s.way(verb)
	for _, m := range dwRoster {
		rel := encounter.RelationOther
		switch m {
		case doer:
			rel = encounter.RelationDoer
		case target:
			rel = encounter.RelationTarget
		}
		var want bool
		switch w.Reach(rel) {
		case encounter.ReachSaw:
			want = slices.Contains(saw.it, m)
		case encounter.ReachDid, encounter.ReachOwn:
			want = true
		}
		d, got := s.held(enc, m, doer, target, verb)
		s.Equal(want, got, "%s: %s is the %s, whose reach is %q", verb, m, rel, w.Reach(rel))
		if !got {
			continue
		}
		s.Equal(m == doer || slices.Contains(saw.doer, m), d.Actor == doer, "%s: %s knows who did it only if it saw them", verb, m)
		if target != "" {
			s.Equal(m == target || slices.Contains(saw.target, m), d.Target == target,
				"%s: %s knows who it was done to only if it saw them", verb, m)
		}
	}
}

// dwAbove is who stands above the wall, who all see each other.
var dwAbove = []encounter.MemberID{alice, goblin, dwCarl}

// dwBelow is who stands below it.
var dwBelow = []encounter.MemberID{billy, dwLurker}

// TestAnAttack: alice strikes lurker below the wall. Who sees alice holds
// it; lurker, who cannot see her, does not.
func (s *DeedWaysSuite) TestAnAttack() {
	enc := s.board(nil)
	_, err := recordOne(enc, &encounter.RecordInput{
		Kind: encounter.OutcomeStruck, Actor: alice, Targets: []encounter.MemberID{dwLurker},
		Values: map[encounter.OutcomeValue]int{encounter.ValueAmount: 1},
	})
	s.Require().NoError(err)
	s.check(enc, encounter.DeedAttack, alice, dwLurker, dwSeen{it: dwAbove, doer: dwAbove, target: dwBelow})
}

// TestAFallWithAnUnseenCause: alice's blow drops lurker, out of her sight.
// Who sees lurker holds the fall, lurker holds its own, and alice holds it
// unseen as its cause: as a fall she caused, not knowing whose.
func (s *DeedWaysSuite) TestAFallWithAnUnseenCause() {
	down := &downList{}
	enc := s.board(down)
	down.down = []encounter.MemberID{dwLurker}
	_, err := recordOne(enc, &encounter.RecordInput{
		Kind: encounter.OutcomeStruck, Actor: alice, Targets: []encounter.MemberID{dwLurker},
		Values: map[encounter.OutcomeValue]int{encounter.ValueAmount: 9},
	})
	s.Require().NoError(err)
	s.check(enc, encounter.DeedFell, alice, dwLurker, dwSeen{it: dwBelow, doer: dwAbove, target: dwBelow})
}

// TestAnIntimidationAndAFlight: alice beats goblin's threaten line, and its
// success row flees. The intimidation lands on who caught the words and saw
// alice (billy and lurker catch the words, unseeing); the flight lands on
// goblin alone, naming alice.
func (s *DeedWaysSuite) TestAnIntimidationAndAFlight() {
	enc := s.board(nil)
	out, err := s.speak(enc, alice, "threaten", "intimidation")
	s.Require().NoError(err)
	s.Require().NotNil(out.Answered)
	s.Require().True(out.Answered.Answer.Flee)
	s.check(enc, encounter.DeedIntimidate, alice, goblin, dwSeen{it: dwAbove, doer: dwAbove, target: dwAbove})
	s.check(enc, encounter.DeedFled, alice, goblin, dwSeen{it: dwAbove, doer: dwAbove, target: dwAbove})
}

// TestAPersuasion: the same for a beaten persuasion.
func (s *DeedWaysSuite) TestAPersuasion() {
	enc := s.board(nil)
	_, err := s.speak(enc, alice, "gossip", "persuasion")
	s.Require().NoError(err)
	s.check(enc, encounter.DeedPersuade, alice, goblin, dwSeen{it: dwAbove, doer: dwAbove, target: dwAbove})
}

// TestACreatureWithATableCanBeTheSpeaker: carl, a creature, beats goblin's
// threaten line and holds the intimidation as its doer, so `intimidated`
// with `as: actor` can be true for a creature that runs a table, and is not
// refused.
func (s *DeedWaysSuite) TestACreatureWithATableCanBeTheSpeaker() {
	enc := s.board(nil)
	_, err := s.speak(enc, dwCarl, "threaten", "intimidation")
	s.Require().NoError(err)
	s.check(enc, encounter.DeedIntimidate, dwCarl, goblin, dwSeen{it: dwAbove, doer: dwAbove, target: dwAbove})
}

// TestAUse: alice pulls the lever. Who sees her holds it; it is done to
// no one.
func (s *DeedWaysSuite) TestAUse() {
	enc := s.board(nil)
	_, err := enc.Interact(&encounter.InteractInput{
		Actor: alice, Prop: dwLever, OptionID: "pull", Range: speakReach, Roller: rollsLowest{},
	})
	s.Require().NoError(err)
	s.check(enc, encounter.DeedUse, alice, "", dwSeen{it: dwAbove, doer: dwAbove})
}

// TestEveryVerbIsDriven: every verb the record holds is driven above, and
// every `when` word has a record.
func (s *DeedWaysSuite) TestEveryVerbIsDriven() {
	driven := []string{encounter.DeedAttack, encounter.DeedFell, encounter.DeedIntimidate,
		encounter.DeedFled, encounter.DeedPersuade, encounter.DeedUse}
	var recorded []string
	for _, w := range encounter.DeedWays() {
		recorded = append(recorded, w.Verb)
	}
	s.ElementsMatch(driven, recorded, "a verb in the record is driven by a test here")
	for _, word := range encounter.WhenDeeds {
		_, ok := encounter.DeedWayOf(word)
		s.True(ok, "the `when` word %q has a record of how it reaches a creature", word)
	}
}

func (s *DeedWaysSuite) speak(enc *encounter.Encounter, actor encounter.MemberID, option, skill string) (*encounter.SpeakOutput, error) {
	return enc.Speak(s.ctx, &encounter.SpeakInput{
		Range: speakReach, Actor: actor, Target: goblin, OptionID: option, Skill: skill,
		Beaten: true, DC: 12, Total: 15, Roller: rollsLowest{},
	})
}
