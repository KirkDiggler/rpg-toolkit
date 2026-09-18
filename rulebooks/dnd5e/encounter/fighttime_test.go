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

// FightTimeTestSuite is rpg-toolkit#1725's first walk finding: a fight had no
// time. Every stamp this composition writes reads the world clock's
// high-water, and that high-water only ever moved in [encounter.Encounter.Pump]
// — the free-roam verb. So inside a fight nothing aged: a deed landed in round
// one was exactly as fresh in round nine, a retaliator's patience never ran
// out, and a ghost's last-seen never went stale.
//
// A fight round is one unit of time for everyone in the fight, the same unit a
// pump is for everyone roaming. These are the proofs that it now is.
type FightTimeTestSuite struct {
	suite.Suite
}

func TestFightTimeSuite(t *testing.T) {
	suite.Run(t, new(FightTimeTestSuite))
}

// scene is alice and a goblin alone in an open room, close enough that the
// first sight pass forms the fight — so round 1 is running before the first
// verb and alice holds the first turn (orderAsGiven).
func (s *FightTimeTestSuite) scene(driver encounter.TurnDriver) *encounter.Encounter {
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
	return enc
}

// round reads the fight's own round counter, and insists there is a fight.
func (s *FightTimeTestSuite) round(enc *encounter.Encounter) int {
	out, err := enc.ClockOf(&encounter.ClockOfInput{Member: alice})
	s.Require().NoError(err)
	s.Require().Equal(encounter.ClockTurn, out.Kind, "alice is in a fight")
	return out.Round
}

// at reads the one clock every stamp in this composition reads.
func (s *FightTimeTestSuite) at(enc *encounter.Encounter) int {
	return enc.ToData().Clock.HighWater
}

// endAliceTurn ends alice's turn, which drives the goblin's whole turn in the
// same call (rpg-toolkit#1162) and so closes the round.
func (s *FightTimeTestSuite) endAliceTurn(enc *encounter.Encounter) {
	out, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
	s.Require().True(out.RoundWrapped, "two in the fight, so alice's end closes the round")
}

// holdingOf is one of an observer's holdings, by subject.
func (s *FightTimeTestSuite) holdingOf(
	enc *encounter.Encounter, observer, subject encounter.MemberID,
) (perception.Holding, bool) {
	holdings, err := enc.View(&encounter.ViewInput{Member: observer})
	s.Require().NoError(err)
	for _, h := range holdings {
		if h.Subject == subject {
			return h, true
		}
	}
	return perception.Holding{}, false
}

// TestAFightRoundIsAUnitOfTime: three rounds of fighting move the clock
// twice, and forming the fight moves it not at all.
func (s *FightTimeTestSuite) TestAFightRoundIsAUnitOfTime() {
	enc := s.scene(passDriver{})

	s.Require().Equal(1, s.round(enc), "the fight is on round one")
	before := s.at(enc)
	s.Equal(0, before, "forming a fight is not a round passing")

	s.endAliceTurn(enc)
	s.Equal(2, s.round(enc))
	s.Equal(before+1, s.at(enc), "round one ending is the first unit of time")

	s.endAliceTurn(enc)
	s.Equal(3, s.round(enc))
	s.Equal(before+2, s.at(enc), "and round two ending the second")

	s.Equal(2, s.at(enc)-before, "three rounds of fighting are two units of time")
}

// TestADeedAgesWhileTheFightRuns: alice shoots at the goblin and misses in
// round one; by the goblin's turn in round three the deed it witnessed is two
// units old, which is what lets a mind ever stop holding a grudge.
func (s *FightTimeTestSuite) TestADeedAgesWhileTheFightRuns() {
	driver := &scriptedDriver{}
	enc := s.scene(driver)

	_, err := enc.Record(&encounter.RecordInput{
		Kind: encounter.OutcomeMissed, Actor: alice, Targets: []encounter.MemberID{goblin},
		Values:      map[encounter.OutcomeValue]int{encounter.ValueRoll: 3, encounter.ValueTotal: 8, encounter.ValueAgainst: 15},
		Calculation: attackCalculation(3, 5, 0),
	})
	s.Require().NoError(err)

	landed, ok := s.holdingOf(enc, goblin, deed.Subject(alice))
	s.Require().True(ok, "the goblin saw the shot")
	s.Require().Equal(deed.Channel, landed.Channel)

	s.endAliceTurn(enc) // round 1 -> 2
	s.endAliceTurn(enc) // round 2 -> 3
	s.endAliceTurn(enc) // the goblin's round-3 turn, then 3 -> 4

	s.Require().Len(driver.calls, 3, "the goblin was driven once a round")
	view := driver.calls[2]
	s.Require().Equal(3, view.Round, "this is its round-three view")

	var seen perception.Holding
	found := false
	for _, h := range view.Holdings {
		if h.Subject == deed.Subject(alice) {
			seen, found = h, true
		}
	}
	s.Require().True(found, "the deed is still held")
	s.Equal(landed.Confirmed, seen.Confirmed, "a deed is stamped when it happened and never restamped")
	s.Equal(uint64(2), view.At-seen.Confirmed, "two rounds have passed since alice shot at it")
}

// TestWhatTheGoblinSeesOfAliceAges: a sight holding's Confirmed moves with the
// rounds too.
//
// The sight pass is not run by the round — it runs when somebody MOVES
// ([Encounter.settleWalk] for a driven walk, [Encounter.Step] for a player's).
// So alice steps once per round, and what the goblin holds of her is
// re-confirmed at whatever the clock says then. Before a fight round advanced
// the clock, every one of those re-confirmations wrote the same number.
func (s *FightTimeTestSuite) TestWhatTheGoblinSeesOfAliceAges() {
	enc := s.scene(passDriver{})

	stepAliceTo := func(y int) uint64 {
		_, err := enc.Step(&encounter.StepInput{Member: alice, To: spatial.Position{X: 6, Y: float64(y)}})
		s.Require().NoError(err)
		held, ok := s.holdingOf(enc, goblin, alice)
		s.Require().True(ok, "the goblin can see alice")
		s.Require().True(held.CurrentOn(perception.Sight), "and is looking at her now")
		return held.Confirmed
	}

	firstRound := stepAliceTo(1)
	s.endAliceTurn(enc)

	secondRound := stepAliceTo(0)
	s.endAliceTurn(enc)

	thirdRound := stepAliceTo(1)

	s.Equal(firstRound+1, secondRound, "a round passed between the two sightings")
	s.Equal(secondRound+1, thirdRound, "and another between those two")
}
