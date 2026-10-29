// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// fell_test.go is a fall as a deed (rpg-project#716, lane rpg-project#722
// D1): a member going down lands [encounter.DeedFell] on every member whose
// sight reaches it, naming the one who dropped it as the actor and the
// fallen as the target. Testimony, never a world flag: a member behind the
// wall holds nothing.

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/deed"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// fellWatcher stands on alice's side of the wall, where it sees the goblin
// fall; billy stands behind the wall, where it sees nothing.
const fellWatcher = core.EntityID("watcher")

type FellTestSuite struct {
	suite.Suite

	standing *downList
}

func TestFellSuite(t *testing.T) {
	suite.Run(t, new(FellTestSuite))
}

// scene is the deed suite's room: alice and the goblin north of the wall
// row, the watcher beside them, billy south of it.
func (s *FellTestSuite) scene() *encounter.Encounter {
	s.standing = &downList{}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion(outcomeRoom, 0, 0, 12, 12)},
			Props:   wallRow(6, 4, 8),
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 2}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 6, Y: 4}},
			{ID: fellWatcher, Kind: encounter.KindMonster, Position: spatial.Position{X: 9, Y: 3}},
			{ID: billy, Kind: encounter.KindMonster, Position: spatial.Position{X: 6, Y: 10}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sight:         everyoneSeesTheWholeMap{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        zeroSheets{},
			Participation: s.standing,
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

// strike records alice hitting the goblin, which drops it when drops says so.
func (s *FellTestSuite) strike(enc *encounter.Encounter, drops bool) {
	if drops {
		s.standing.down = []encounter.MemberID{goblin}
	}
	_, err := recordOne(enc, &encounter.RecordInput{
		Kind: encounter.OutcomeStruck, Actor: alice, Targets: []encounter.MemberID{goblin},
		Values: map[encounter.OutcomeValue]int{
			encounter.ValueRoll: 17, encounter.ValueTotal: 22, encounter.ValueAgainst: 15, encounter.ValueAmount: 9,
		},
		Calculation: attackCalculation(17, 5, 0),
	})
	s.Require().NoError(err)
}

// fall is the fall observer holds under actor's subject, decoded, with the
// stamp it is held at.
func (s *FellTestSuite) fall(enc *encounter.Encounter, observer, actor encounter.MemberID) (deed.Deed, uint64, bool) {
	holdings, err := enc.View(&encounter.ViewInput{Member: observer})
	s.Require().NoError(err)
	for _, h := range holdings {
		if h.Channel != deed.Channel || h.Subject != deed.Subject(actor, encounter.DeedFell) {
			continue
		}
		saw, derr := deed.Decode(h.Payload)
		s.Require().NoError(derr)

		return saw, h.Confirmed, true
	}

	return deed.Deed{}, 0, false
}

// anyFall reports whether observer holds a fall at all, under any subject.
func (s *FellTestSuite) anyFall(enc *encounter.Encounter, observer encounter.MemberID) bool {
	holdings, err := enc.View(&encounter.ViewInput{Member: observer})
	s.Require().NoError(err)
	for _, h := range holdings {
		if h.Channel != deed.Channel {
			continue
		}
		if saw, derr := deed.Decode(h.Payload); derr == nil && saw.Verb == encounter.DeedFell {
			return true
		}
	}

	return false
}

// stepAlice walks alice one cell, which refreshes sight and so asks who is
// down: a participation pass no blow is part of.
func (s *FellTestSuite) stepAlice(enc *encounter.Encounter, to spatial.Position) {
	_, err := enc.Step(&encounter.StepInput{Member: alice, To: to, EndWalk: true})
	s.Require().NoError(err)
}

// TestAFallLandsOnWhoeverSawIt is the slice's bullet: the goblin goes down
// in sight of the watcher and out of sight of billy. The watcher holds
// `fell`, with alice (who dropped it) as the actor and the goblin as the
// target; billy holds nothing; alice holds what she did, and the goblin
// knows it fell.
func (s *FellTestSuite) TestAFallLandsOnWhoeverSawIt() {
	enc := s.scene()
	s.strike(enc, true)

	saw, _, ok := s.fall(enc, fellWatcher, alice)
	s.Require().True(ok, "the watcher saw the goblin fall")
	s.Equal(deed.Deed{Verb: encounter.DeedFell, Actor: alice, Target: goblin, Where: saw.Where}, saw,
		"the actor is who dropped it, the target is who fell")
	s.NotEmpty(saw.Where, "where the goblin fell, as the board has it")

	s.False(s.anyFall(enc, billy), "billy, behind the wall, holds no fall at all")

	_, _, ok = s.fall(enc, alice, alice)
	s.True(ok, "the one who dropped it knows it did")
	_, _, ok = s.fall(enc, goblin, alice)
	s.True(ok, "and the fallen knows it fell")
}

// TestAFallIsWitnessedOnce: the story's ledger that keeps a death from being
// told twice keeps it from being witnessed twice. A later pass that asks
// who is down again restamps nothing.
func (s *FellTestSuite) TestAFallIsWitnessedOnce() {
	enc := s.scene()
	s.strike(enc, true)
	_, first, ok := s.fall(enc, fellWatcher, alice)
	s.Require().True(ok)

	s.stepAlice(enc, spatial.Position{X: 7, Y: 2})

	_, again, ok := s.fall(enc, fellWatcher, alice)
	s.Require().True(ok)
	s.Equal(first, again, "the fall is stamped when it happened and never again")
}

// TestAFallNoBlowExplainsIsTheFallensOwn: the goblin is reported down by a
// pass no attack is part of (a death save, a hazard, a turn boundary). There
// is no doer this composition knows, so the fall is filed as the goblin's
// own, and still witnessed.
func (s *FellTestSuite) TestAFallNoBlowExplainsIsTheFallensOwn() {
	enc := s.scene()
	s.standing.down = []encounter.MemberID{goblin}
	s.stepAlice(enc, spatial.Position{X: 7, Y: 2})

	saw, _, ok := s.fall(enc, fellWatcher, goblin)
	s.Require().True(ok, "the watcher saw the goblin fall")
	s.Equal(goblin, saw.Actor, "nobody dropped it that the record knows")
	s.Equal(goblin, saw.Target)
	_, _, ok = s.fall(enc, fellWatcher, alice)
	s.False(ok, "alice is not named for a fall she did not cause")
}

// TestAnEarlierBlowDoesNotExplainALaterFall: alice hits the goblin and it
// stays up; it falls later, to a pass that lands no blow. The blow belonged
// to its own verb, so the fall is the goblin's own, not alice's.
func (s *FellTestSuite) TestAnEarlierBlowDoesNotExplainALaterFall() {
	enc := s.scene()
	s.strike(enc, false)
	s.standing.down = []encounter.MemberID{goblin}
	s.stepAlice(enc, spatial.Position{X: 7, Y: 2})

	_, _, ok := s.fall(enc, fellWatcher, alice)
	s.False(ok, "the earlier blow is not this fall's cause")
	_, _, ok = s.fall(enc, fellWatcher, goblin)
	s.True(ok, "the fall is the goblin's own")
}
