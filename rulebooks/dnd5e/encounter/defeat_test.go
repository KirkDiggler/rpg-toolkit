// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// defeat_test.go is death lane slice D2 (rpg-toolkit#1078): the last skeleton
// drops and the fight ends with no caller.
//
// D1 taught the world to notice who is down. It stopped one step short, on
// purpose: a bubble holding one live side kept running, and a bubble whose last
// member went down was pruned as an empty husk by machinery that knows nothing
// about death. Both are replaced here, and ruled fork (c) on rpg-toolkit#959 is
// what they are replaced with — all-hostiles-down ends the FIGHT, not the
// dungeon.
//
// Nothing below calls Dissolve. That is the point: the ending is a fact the
// world notices at the same choke point sight already starts fights at, which is
// rpg-toolkit#964's mirror. The one test that DOES call it is here to pin the
// other cause, so the two endings can be told apart in the story.

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/clock"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// orc is D2's newcomer: the monster that arrives AFTER a fight has ended, to
// prove the survivors are free rather than merely un-bubbled.
const orc = encounter.MemberID("orc")

type DefeatSuite struct {
	deathScene
}

func TestDefeatSuite(t *testing.T) {
	suite.Run(t, new(DefeatSuite))
}

// quartet is two players and two monsters in one open room, all in plain sight.
// First light puts all four in ONE bubble; the multi-bubble scene below splits
// that blob by hand, because form's policy is one fight per encounter and the
// only door to a second is a loaded shape.
func (s *DefeatSuite) quartet(standing encounter.Standing) *encounter.Encounter {
	s.T().Helper()

	return s.scene(standing,
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Room: cryptID,
			Position: spatial.Position{X: 0, Y: 2}},
		encounter.MemberInput{ID: bob, Kind: encounter.KindPlayer, Room: cryptID,
			Position: spatial.Position{X: 1, Y: 2}},
		encounter.MemberInput{ID: goblin, Kind: encounter.KindMonster, Room: cryptID,
			Position: spatial.Position{X: 0, Y: 10}},
		encounter.MemberInput{ID: wolf, Kind: encounter.KindMonster, Room: cryptID,
			Position: spatial.Position{X: 2, Y: 10}},
	)
}

// endingBeat returns the one bubble-dissolved beat in an audience's story, and
// fails if there is not exactly one — "the fight ended twice" and "the fight
// never ended" are both defects this suite must not read past.
func (s *DefeatSuite) endingBeat(enc *encounter.Encounter, audience encounter.MemberID) map[string]any {
	s.T().Helper()

	var found []map[string]any
	for _, beat := range s.beatsOf(enc, audience) {
		if beat["beat"] == "bubble-dissolved" {
			found = append(found, beat)
		}
	}
	s.Require().Len(found, 1, "exactly one ending")

	return found[0]
}

// membersOf reads a beat's member list back as IDs.
func (s *DefeatSuite) membersOf(beat map[string]any) []encounter.MemberID {
	s.T().Helper()

	raw, ok := beat["members"].([]any)
	s.Require().True(ok, "the ending names who it held")

	ids := make([]encounter.MemberID, 0, len(raw))
	for _, entry := range raw {
		name, isString := entry.(string)
		s.Require().True(isString)
		ids = append(ids, encounter.MemberID(name))
	}

	return ids
}

// --- the fight ends -------------------------------------------------------

// TestTheLastOfASideDownEndsTheFight is the slice, whole.
//
// Two monsters and a character are trading blows; both monsters go down in the
// same pass; nothing calls anything. The fight is over when the pass returns,
// and the story tells it in the order it happened — the tick that caused the
// consult, the two bodies, then the ending those bodies explain.
func (s *DefeatSuite) TestTheLastOfASideDownEndsTheFight() {
	down := &downList{}
	enc := s.trio(down)
	s.Require().Equal(encounter.ClockTurn, s.clockOf(enc, alice), "control: a fight is running")

	down.down = []encounter.MemberID{goblin, wolf}
	_, err := enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)

	s.Equal(encounter.ClockWorld, s.clockOf(enc, alice),
		"the last monster is down, so the fight is over and nobody had to say so")
	s.Empty(enc.ToData().Bubbles, "a bubble exists only while a fight does")

	s.Equal([]string{"scene-opened", "bubble-formed", "tick", "down", "down", "bubble-dissolved"},
		s.beatKindsOf(enc, alice),
		"cause before effect: both bodies are news before the ending they explain")
}

// TestTheEndingSaysItWasDefeat is why ByDefeat exists rather than a second
// bubble-dissolved beat. A client reading the story has to be able to tell a
// party walking away from a party winning, and the beat is the only channel a
// self-dissolve has — nothing returned to a caller, because there was no caller.
func (s *DefeatSuite) TestTheEndingSaysItWasDefeat() {
	down := &downList{}
	enc := s.trio(down)

	down.down = []encounter.MemberID{goblin, wolf}
	_, err := enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)

	ending := s.endingBeat(enc, alice)
	s.Equal(string(encounter.DissolveByDefeat), ending["cause"])
	s.Equal([]encounter.MemberID{alice, goblin, wolf}, s.membersOf(ending),
		"and it names everyone the fight held, bodies included")
}

// TestACallerEndingAFightStillSaysDecision is the control that gives the test
// above its meaning. The same beat carries both endings, so a cause that were
// always "defeat" would pass every assertion in this file and lie about half of
// them.
func (s *DefeatSuite) TestACallerEndingAFightStillSaysDecision() {
	enc := s.trio(&downList{})
	s.Require().Equal(encounter.ClockTurn, s.clockOf(enc, alice))

	out, err := enc.Dissolve(&encounter.DissolveInput{Member: alice})
	s.Require().NoError(err)

	s.Equal(encounter.ByDecision(), out.Cause, "the verb IS the decision")
	s.Equal(string(encounter.DissolveByDecision), s.endingBeat(enc, alice)["cause"])
}

// TestTheCauseSetIsSealed pins the discipline rather than the type, exactly as
// session's DissolveCause and saves' DCSource do.
//
// A bare string would let anything write a cause into the story — a fight that
// ended because somebody fled, or was banished, or whatever a later author
// needed that afternoon — with no ruling behind it. The unexported method makes
// a third case impossible to declare outside this package, so adding one means
// editing dissolve.go, and editing dissolve.go means having the caller in hand.
func (s *DefeatSuite) TestTheCauseSetIsSealed() {
	iface := reflect.TypeOf((*encounter.DissolveCause)(nil)).Elem()

	var sealed []string
	for i := 0; i < iface.NumMethod(); i++ {
		if m := iface.Method(i); !m.IsExported() {
			sealed = append(sealed, m.Name)
		}
	}

	s.NotEmpty(sealed,
		"DissolveCause must keep an unexported method: without it any caller can declare "+
			"a cause nothing ruled on, and defeat stops being earned by the caller that forces it")
}

// --- and only when it is actually over ------------------------------------

// TestAFightWithBothSidesStandingNeverSelfDissolves is the rule's other half,
// and the one a naive implementation gets wrong: the trigger is a SIDE being
// gone, not a body being noticed. One of two monsters down leaves a fight.
func (s *DefeatSuite) TestAFightWithBothSidesStandingNeverSelfDissolves() {
	down := &downList{}
	enc := s.trio(down)

	down.down = []encounter.MemberID{goblin}
	_, err := enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)

	s.Equal(encounter.ClockTurn, s.clockOf(enc, alice), "the wolf is still standing")
	s.Equal([]encounter.MemberID{alice, wolf}, s.orderOf(enc, alice))
	s.NotContains(s.beatKindsOf(enc, alice), "bubble-dissolved")
}

// TestAFightEmptiedByItsOwnMembersIsNotADefeat holds the check to its cause. A
// bubble can go one-sided without anybody dying — a caller transfers the last
// monster out — and calling that a defeat would put an ending in the story that
// nothing in the fiction earned.
//
// The bubble is left running, which is D1's state and is deliberately not this
// slice's business: ending a fight nobody is fighting is a different ruling with
// a different cause.
func (s *DefeatSuite) TestAFightEmptiedByItsOwnMembersIsNotADefeat() {
	enc := s.trio(&downList{})

	_, err := enc.Transfer(&encounter.TransferInput{Member: goblin, To: encounter.ClockWorld})
	s.Require().NoError(err)
	_, err = enc.Transfer(&encounter.TransferInput{Member: wolf, To: encounter.ClockWorld})
	s.Require().NoError(err)
	s.Require().Equal([]encounter.MemberID{alice}, s.orderOf(enc, alice),
		"one-sided, and nobody is down")

	_, err = enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)

	s.Equal(encounter.ClockTurn, s.clockOf(enc, alice))
	s.NotContains(s.beatKindsOf(enc, alice), "bubble-dissolved")
}

// --- the husk edge, told honestly -----------------------------------------

// TestEveryMemberDownEndsWithAnHonestCause replaces D1's emergent edge.
//
// When every member of a bubble went down, D1 spliced them out one at a time
// and dropBubbleIfIdle deleted what was left. The fight ended — correctly, by
// accident, and silently: the pruner knows nothing about death, so the story
// showed bodies and then a fight that simply stopped being mentioned. It now
// ends the same way every other defeat does, with the beat that says so.
func (s *DefeatSuite) TestEveryMemberDownEndsWithAnHonestCause() {
	down := &downList{}
	enc := s.trio(down)

	down.down = []encounter.MemberID{alice, goblin, wolf}
	_, err := enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)

	s.Empty(enc.ToData().Bubbles, "no husk left behind")
	ending := s.endingBeat(enc, alice)
	s.Equal(string(encounter.DissolveByDefeat), ending["cause"], "an ending, not a side effect")
	s.Equal([]encounter.MemberID{alice, goblin, wolf}, s.membersOf(ending))

	for _, id := range []encounter.MemberID{alice, goblin, wolf} {
		s.Equal(encounter.ClockWorld, s.clockOf(enc, id), "%q is home, not on no clock at all", id)
	}
}

// --- dissolving removes nobody, and closes nothing -------------------------

// TestTheFightEndsAndNobodyLeaves is ruled fork (a) surviving fork (c). The
// bubble is what ended; the roster, the map and the exit path are untouched, so
// the killing blow is still recordable and the bodies are still carried out.
func (s *DefeatSuite) TestTheFightEndsAndNobodyLeaves() {
	down := &downList{}
	enc := s.trio(down)
	fell := s.positionOf(enc, goblin)

	down.down = []encounter.MemberID{goblin, wolf}
	_, err := enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)

	members, err := enc.Members()
	s.Require().NoError(err)
	s.Len(members, 3, "the fight ended; the roster did not")
	s.Equal(fell, s.positionOf(enc, goblin), "still where it fell")

	_, err = enc.Record(&encounter.RecordInput{
		Kind:    encounter.OutcomeStruck,
		Actor:   alice,
		Targets: []encounter.MemberID{goblin},
		Values:  map[encounter.OutcomeValue]int{encounter.ValueAmount: 7},
	})
	s.Require().NoError(err, "the killing blow is recordable after the fight it ended")

	out, err := enc.Exit(&encounter.ExitInput{Member: goblin})
	s.Require().NoError(err)
	s.Equal(goblin, out.Outcome.ID, "carried out, not vanished")
}

// TestTheEncounterOutlivesTheFight is the distinction the old stack conflated
// and fork (c) rules against: all_hostiles_defeated used to close the whole
// encounter, which in a multi-room dungeon means clearing room one ends the
// dungeon. Only the bubble ends here.
func (s *DefeatSuite) TestTheEncounterOutlivesTheFight() {
	down := &downList{}
	enc := s.trio(down)

	down.down = []encounter.MemberID{goblin, wolf}
	_, err := enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)

	status, err := enc.Status()
	s.Require().NoError(err)
	s.True(status.Open, "the fight ended; the dungeon is still in front of them")
	s.Nil(status.Outcome)
}

// --- survivors keep playing ------------------------------------------------

// TestSurvivorsExploreAgain is what the world clock is FOR. A fight member is
// refused every free-roam verb (ErrInBubble), correctly and forever, so an
// ending that left alice in the bubble would leave her unable to take a step
// with nothing on the map to fight.
func (s *DefeatSuite) TestSurvivorsExploreAgain() {
	down := &downList{}
	enc := s.trio(down)

	_, err := enc.Move(&encounter.MoveInput{Member: alice, To: spatial.Position{X: 1, Y: 2}})
	s.Require().ErrorIs(err, encounter.ErrInBubble, "control: mid-fight, she is not free to walk")

	down.down = []encounter.MemberID{goblin, wolf}
	_, err = enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)

	_, err = enc.Move(&encounter.MoveInput{Member: alice, To: spatial.Position{X: 1, Y: 2}})
	s.Require().NoError(err, "the fight is over, so the walk is hers again")
	s.Equal(spatial.Position{X: 1, Y: 2}, s.positionOf(enc, alice))
}

// TestSurvivorsFightAgain closes the loop. An ending that quietly broke trigger
// detection would pass every assertion above — the party would explore a dungeon
// that could never threaten them again.
func (s *DefeatSuite) TestSurvivorsFightAgain() {
	down := &downList{}
	enc := s.trio(down)

	down.down = []encounter.MemberID{goblin, wolf}
	_, err := enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)
	s.Require().Equal(encounter.ClockWorld, s.clockOf(enc, alice))

	_, err = enc.Join(&encounter.JoinInput{Member: encounter.MemberInput{
		ID: orc, Kind: encounter.KindMonster, Room: cryptID,
		Position: spatial.Position{X: 0, Y: 6},
	}})
	s.Require().NoError(err)

	s.Equal(encounter.ClockTurn, s.clockOf(enc, alice), "something new walked in, and it is a fight")
	s.Equal([]encounter.MemberID{alice, orc}, s.orderOf(enc, alice),
		"the bodies are not in it — they are on neither side")
}

// --- a fight ends, not every fight -----------------------------------------

// TestOnlyTheDecidedFightEnds is why the check is reached through a member
// rather than run over the bubble list. Two fights are running; one side of one
// of them goes down; the other fight does not notice.
//
// The two bubbles are written by hand because form's policy is one per
// encounter — a loaded shape is the only door to a second, and it is the same
// door clocks_test.go uses.
func (s *DefeatSuite) TestOnlyTheDecidedFightEnds() {
	built := s.quartet(&downList{})
	data := built.ToData()
	s.Require().Len(data.Bubbles, 1, "first light put all four in one fight")

	// Split it: alice fights the goblin, bob fights the wolf.
	data.Bubbles = []clock.TurnData{
		{Order: []core.EntityID{alice, goblin}, ActiveIdx: 0, Round: 1},
		{Order: []core.EntityID{bob, wolf}, ActiveIdx: 0, Round: 1},
	}

	enc, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data:       data,
		Initiative: orderAsGiven{},
		Standing:   &downList{down: []encounter.MemberID{goblin}},
	})
	s.Require().NoError(err)

	_, err = enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)

	s.Equal(encounter.ClockWorld, s.clockOf(enc, alice), "her fight is over")
	s.Equal(encounter.ClockTurn, s.clockOf(enc, bob), "his is not")
	s.Equal([]encounter.MemberID{bob, wolf}, s.orderOf(enc, bob), "and it is untouched")
	s.Equal([]encounter.MemberID{alice, goblin}, s.membersOf(s.endingBeat(enc, bob)),
		"the ending names one fight's members, not the room's")
}
