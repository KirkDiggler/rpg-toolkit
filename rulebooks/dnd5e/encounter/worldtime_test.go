// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// worldtime_test.go is TIME ON THE WORLD CLOCK, pinned (rpg-project#465,
// design §5): what each verb costs, who it names as the driver, and what the
// world does with the round it raised.

type WorldTimeSuite struct {
	suite.Suite
	ctx context.Context
}

func TestWorldTimeSuite(t *testing.T) { suite.Run(t, new(WorldTimeSuite)) }

func (s *WorldTimeSuite) SetupTest() { s.ctx = context.Background() }

// hall is a long open room with nobody in it but the members a test names —
// no walls to route around and no props, so a claim about TIME is not quietly
// a claim about geometry.
func (s *WorldTimeSuite) hall(members ...encounter.MemberInput) *encounter.Encounter {
	return s.hallWith(nil, members...)
}

// hallWith is hall with the sides declared — for the one test whose creature
// has to be a MONSTER (so the world thinks for it) and opposed to nobody (so
// its table's `enemy: none` entry is the one that fires).
func (s *WorldTimeSuite) hallWith(
	dispositions []encounter.DispositionInput, members ...encounter.MemberInput,
) *encounter.Encounter {
	s.T().Helper()

	var factions []encounter.FactionInput
	if len(dispositions) > 0 {
		factions = []encounter.FactionInput{{ID: "vendors", Mind: goblin}}
	}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: tableDriver(), Roller: rollsLowest{},
		Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: encounter.FieldInput{
			Canvas:       openAir(),
			Regions:      []encounter.RegionInput{rectRegion("hall", 0, 0, 30, 8)},
			Factions:     factions,
			Dispositions: dispositions,
		},
		Members: members,
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	return enc
}

// reading is the world clock's high-water right now.
func (s *WorldTimeSuite) reading(enc *encounter.Encounter) int {
	return enc.ToData().Clock.HighWater
}

// budget is what one member has left to spend on the world clock.
func (s *WorldTimeSuite) budget(enc *encounter.Encounter, who encounter.MemberID) int {
	return enc.ToData().Clock.Budgets[who]
}

// walk steps a member cell by cell along a row.
func (s *WorldTimeSuite) walk(enc *encounter.Encounter, who encounter.MemberID, from, cells int, row int) {
	s.T().Helper()
	for i := 1; i <= cells; i++ {
		_, err := enc.Step(&encounter.StepInput{Member: who, To: cellAt(from+i, row)})
		s.Require().NoError(err, "step %d", i)
	}
}

// --- pace --------------------------------------------------------------------

// TestAPaceIsTheMoversOwnSpeed: a thirty-foot walker pays the world a round
// every SIXTH cell, and five cells pay nothing. The remainder is not lost —
// it carries on the member — which is what makes "the party walks and the
// world moves with it" continuous rather than stepwise per verb.
func (s *WorldTimeSuite) TestAPaceIsTheMoversOwnSpeed() {
	s.Run("five cells is no round", func() {
		enc := s.hall(encounter.MemberInput{
			ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}, SpeedFeet: 30,
		})
		s.walk(enc, alice, 1, 5, 1)
		s.Equal(0, s.reading(enc), "five cells of a thirty-foot walker is most of a round, and most is none")
	})

	s.Run("six cells is one", func() {
		enc := s.hall(encounter.MemberInput{
			ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}, SpeedFeet: 30,
		})
		s.walk(enc, alice, 1, 6, 1)
		s.Equal(1, s.reading(enc), "the sixth cell is the pace, and the pace is the round")
	})

	s.Run("and the remainder carries", func() {
		enc := s.hall(encounter.MemberInput{
			ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}, SpeedFeet: 30,
		})
		s.walk(enc, alice, 1, 11, 1)
		s.Equal(1, s.reading(enc), "eleven cells is one round and five over")
		s.walk(enc, alice, 12, 1, 1)
		s.Equal(2, s.reading(enc), "and the twelfth completes the second, which a per-verb count could not")
	})
}

// TestTwoMoversWalkingTogetherAdvanceItOnce is why every advance names its own
// member: the leaf accrues by driver as MAX, not sum. A party of four crossing
// a hall together is one round of the world, not four — and it is the driver
// name that makes that true.
func (s *WorldTimeSuite) TestTwoMoversWalkingTogetherAdvanceItOnce() {
	enc := s.hall(
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer,
			Position: spatial.Position{X: 1, Y: 1}, SpeedFeet: 30},
		encounter.MemberInput{ID: billy, Kind: encounter.KindPlayer,
			Position: spatial.Position{X: 1, Y: 3}, SpeedFeet: 30},
	)

	s.walk(enc, alice, 1, 6, 1)
	s.Require().Equal(1, s.reading(enc), "the first of them to finish a pace raises it")

	s.walk(enc, billy, 1, 6, 3)
	s.Equal(1, s.reading(enc), "and the second walking the same distance raises nothing — max, not sum")

	s.walk(enc, alice, 7, 6, 1)
	s.Equal(2, s.reading(enc), "the round after is the next one either of them completes")
}

// TestAMemberWithNoSpeedPacesNothing: zero speed is a roster row that carried
// no number, not the slowest creature in the world. Dividing by it would make
// every cell a round.
func (s *WorldTimeSuite) TestAMemberWithNoSpeedPacesNothing() {
	enc := s.hall(encounter.MemberInput{
		ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1},
	})

	s.walk(enc, alice, 1, 8, 1)
	s.Zero(s.reading(enc), "nobody said how fast this member is, so nothing says how long its walk took")
}

// --- an action ---------------------------------------------------------------

// TestAnActionCostsOneRoundForItsActor is the second row of design §5's table.
// Search is the cheapest verb to prove it with: no target, no die, no
// concealment to sweep.
func (s *WorldTimeSuite) TestAnActionCostsOneRoundForItsActor() {
	enc := s.hall(encounter.MemberInput{
		ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}, SpeedFeet: 30,
	})

	_, err := enc.Search(&encounter.SearchInput{Member: alice, Region: "hall"})
	s.Require().NoError(err)
	s.Equal(1, s.reading(enc), "an action is a round, whole, however little walking went with it")

	_, err = enc.Search(&encounter.SearchInput{Member: alice, Region: "hall"})
	s.Require().NoError(err)
	s.Equal(2, s.reading(enc), "and the next one is the next round")
}

// TestAThreatCostsItsActorARound is the same rule at the verb the whole
// shenanigan rests on, and it is what makes a cowed creature able to keep
// running: the verb that scared it pays the round its own table then spends.
func (s *WorldTimeSuite) TestAThreatCostsItsActorARound() {
	enc := s.hall(
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer,
			Position: spatial.Position{X: 1, Y: 1}, SpeedFeet: 30},
		encounter.MemberInput{ID: goblin, Kind: encounter.KindWorld,
			Position: spatial.Position{X: 3, Y: 1}, SpeedFeet: 30},
	)

	_, err := enc.Intimidate(s.ctx, &encounter.IntimidateInput{
		Actor: alice, Target: goblin, Beaten: true, DC: 9, Total: 14, Roller: rollsLowest{},
	})
	s.Require().NoError(err)
	s.Equal(1, s.reading(enc), "the threat landed, and then the world moved on")
}

// --- a fight round -----------------------------------------------------------

// TestAFightRoundIsARoundForEveryFighter is the third row, and the correction
// the shipped site needed: the round used to advance once under the literal
// driver "world", which — the accrual being max-by-driver — would have put
// that phantom ten rounds ahead of everybody after ten rounds of fighting, so
// a player's first walk afterwards raised nothing until they had caught up.
//
// AND A CREATURE OUTSIDE THE FIGHT GAINS THE ROUND, which is the primitive
// Alarm needs: creatures elsewhere close one round at a time while a fight
// runs.
func (s *WorldTimeSuite) TestAFightRoundIsARoundForEveryFighter() {
	enc := s.hall(
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer,
			Position: spatial.Position{X: 1, Y: 1}, SpeedFeet: 30},
		encounter.MemberInput{ID: goblin, Kind: encounter.KindMonster,
			Position: spatial.Position{X: 3, Y: 1}, SpeedFeet: 30},
		// A world NPC: on no side, so the fight forms without it and it stays
		// on the world clock, which is what this half is about.
		encounter.MemberInput{ID: "straggler", Kind: encounter.KindWorld,
			Position: spatial.Position{X: 28, Y: 7}, SpeedFeet: 30},
	)

	on, err := enc.ClockOf(&encounter.ClockOfInput{Member: alice})
	s.Require().NoError(err)
	s.Require().Equal(encounter.ClockTurn, on.Kind, "precondition: seeing each other started a fight")

	before := s.reading(enc)

	// One whole round of it: alice ends her turn, the goblin's is driven for
	// it inside the same call, and the last of the order wraps.
	active, err := enc.ClockOf(&encounter.ClockOfInput{Member: alice})
	s.Require().NoError(err)
	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: active.Active})
	s.Require().NoError(err)

	s.Equal(before+1, s.reading(enc), "the round wrapped, and the world got one round for it")

	progress := enc.ToData().Clock.DriverProgress
	s.Equal(1, progress[alice], "advanced under alice's own name, because she lived the round")
	s.Equal(1, progress[goblin], "and under the goblin's, because it did too")
	s.NotContains(progress, encounter.MemberID("world"), "and never under a driver nobody is")

	s.Equal(1, s.budget(enc, "straggler"),
		"a creature outside the fight gains the round the fight paid for — the primitive Alarm needs")
}

// --- what the world does with it ---------------------------------------------

// TestACreatureWithNothingOpposedHoldsAndTheBeatSaysSo: a neutral creature is
// not a target-seeking one with nothing in range. Its table's `enemy: none`
// entry is what fires, the world records that it was asked, and the pick says
// `hold` — which is the difference between a creature standing there and a
// creature nobody consulted.
func (s *WorldTimeSuite) TestACreatureWithNothingOpposedHoldsAndTheBeatSaysSo() {
	enc := s.hallWith(
		[]encounter.DispositionInput{{
			Between: [2]encounter.FactionID{"vendors", encounter.FactionParty},
			Stance:  encounter.StanceNeutral,
		}},
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer,
			Position: spatial.Position{X: 1, Y: 1}, SpeedFeet: 30},
		// A monster, so the world thinks for it — and on a side nobody is
		// against, so it is opposed to nobody however crowded the hall is.
		encounter.MemberInput{ID: goblin, Kind: encounter.KindMonster, Faction: "vendors",
			Position: spatial.Position{X: 20, Y: 6}, SpeedFeet: 30, Table: hunts()},
	)

	started := whereIs(s.T(), enc, goblin)

	_, err := enc.Search(&encounter.SearchInput{Member: alice, Region: "hall"})
	s.Require().NoError(err)

	s.Equal(started, whereIs(s.T(), enc, goblin), "it is opposed to nobody it can see, so it stands there")

	pick := s.lastPick(enc, goblin)
	s.Require().NotNil(pick, "and it WAS asked — a beat is the only account of that")
	s.Equal("hold", pick["word"])
	s.Equal(string(encounter.AnswerTime), pick["key"])
}

// TestAnAttackOffTheTurnClockIsAnError: an enemy in reach on the world clock
// is a fight sight that has already formed, so a driver that swings there is
// describing a world that cannot happen. An error rather than a skipped
// intent, because passing quietly would leave the decision looking like it
// fired and did nothing.
//
// A HAND-ROLLED DRIVER IS WHAT REACHES IT, and that is the honest way to pin a
// guard: [encounter.TableDriver] cannot produce this, because `attack: enemy`
// selects off the view's own opposed sightings and a creature that has one is
// in a fight already. The refusal exists for the day somebody writes a second
// driver, which is exactly when a silent pass would cost the most.
func (s *WorldTimeSuite) TestAnAttackOffTheTurnClockIsAnError() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: alwaysSwings{}, Roller: rollsLowest{},
		Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: encounter.FieldInput{
			Canvas:   openAir(),
			Regions:  []encounter.RegionInput{rectRegion("hall", 0, 0, 30, 8)},
			Factions: []encounter.FactionInput{{ID: "vendors", Mind: goblin}},
			Dispositions: []encounter.DispositionInput{{
				Between: [2]encounter.FactionID{"vendors", encounter.FactionParty},
				Stance:  encounter.StanceNeutral,
			}},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}, SpeedFeet: 30},
			// Neutral, so no fight forms and it stays the world's to think
			// for — and carrying a table, because a creature with none is
			// never consulted at all.
			{ID: goblin, Kind: encounter.KindMonster, Faction: "vendors",
				Position: spatial.Position{X: 20, Y: 6}, SpeedFeet: 30,
				Table: encounter.Table{encounter.AnswerTime: {{Weight: 1, Hold: true}}}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	_, err = enc.Search(&encounter.SearchInput{Member: alice, Region: "hall"})
	s.Require().Error(err, "the world asked, the driver said `attack`, and the world clock has no attacks in it")
	s.ErrorIs(err, encounter.ErrAttackOffTurn)
}

// alwaysSwings is the driver the guard above exists for: one that answers
// [encounter.Attack] whatever the view says, including on a clock where no
// attack can land.
type alwaysSwings struct{}

func (alwaysSwings) Act(view encounter.MonsterView) (encounter.Decision, error) {
	return encounter.Decision{Intent: encounter.Attack{Target: alice, Action: testMeleeAction}}, nil
}

// lastPick is the most recent `answered` beat naming one creature, decoded —
// the only account of a pick there is.
func (s *WorldTimeSuite) lastPick(enc *encounter.Encounter, creature encounter.MemberID) map[string]any {
	s.T().Helper()

	story, err := enc.Story(&encounter.StoryInput{Audience: alice})
	s.Require().NoError(err)

	var found map[string]any
	for _, entry := range story {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		if beat["beat"] == encounter.BeatAnswered && beat["creature"] == string(creature) {
			found = beat
		}
	}

	return found
}

// --- the deal, as the table sees it ------------------------------------------

// TestAFactionsMixIsDealtAtTheDoorAndSaysSo is design §3 at the seam: "so the
// streamer sees which goblin came out the coward". The deal happens once, when
// the creature enters the run, and the beat carries the face and the die it
// was rolled on.
func (s *WorldTimeSuite) TestAFactionsMixIsDealtAtTheDoorAndSaysSo() {
	profiles := map[string]encounter.TemperProfile{
		"coward":     {Attack: 50, Toward: 50, Away: 300, Flee: 300, Hold: 100},
		"soldier":    {Attack: 100, Toward: 100, Away: 100, Flee: 100, Hold: 100},
		"aggressive": {Attack: 300, Toward: 300, Away: 25, Flee: 25, Hold: 50},
	}

	enc := s.hall(
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer,
			Position: spatial.Position{X: 1, Y: 1}, SpeedFeet: 30},
		encounter.MemberInput{ID: goblin, Kind: encounter.KindMonster,
			Position: spatial.Position{X: 20, Y: 6}, SpeedFeet: 30,
			Temper: encounter.Temper{
				Mix:      map[string]int{"coward": 1, "soldier": 2, "aggressive": 1},
				Profiles: profiles,
			}},
	)

	// AND THE SCENE OPENED FIRST. A deal is a beat, and a beat inside a scene
	// that has not opened yet is one nobody can follow — the law every other
	// beat this constructor writes keeps.
	kinds := s.beatKinds(enc)
	s.Require().Equal("scene-opened", kinds[0])
	s.Contains(kinds[1:], encounter.BeatTempered)

	beat := s.beatOf(enc, encounter.BeatTempered, goblin)
	s.Require().NotNil(beat, "the deal is a beat, or the streamer never learns which goblin this is")
	// rollsLowest answers 1, which is the first share of the sorted mix.
	s.Equal("aggressive", beat["temper"])
	s.EqualValues(1, beat["roll"])
	s.EqualValues(4, beat["of"], "the die is the sum of the shares the author wrote")

	members, err := enc.Members()
	s.Require().NoError(err)
	for _, m := range members {
		if m.ID != goblin {
			continue
		}
		s.Equal("aggressive", m.Temper.Word, "and what was dealt is what the creature now IS")
		s.Equal(profiles["aggressive"], m.Temper.Profile, "profile and all")
		s.Empty(m.Temper.Mix, "the spread is spent")
	}
}

// TestAnAuthoredWordDealsNothingAndSaysNothing: the author has already
// answered the question the mix exists to ask, and a beat claiming a die chose
// this creature's nerve would be the composition inventing a roll.
func (s *WorldTimeSuite) TestAnAuthoredWordDealsNothingAndSaysNothing() {
	enc := s.hall(
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer,
			Position: spatial.Position{X: 1, Y: 1}, SpeedFeet: 30},
		encounter.MemberInput{ID: goblin, Kind: encounter.KindMonster,
			Position: spatial.Position{X: 20, Y: 6}, SpeedFeet: 30,
			Temper: encounter.Temper{Word: "coward",
				Profile: encounter.TemperProfile{Attack: 50, Toward: 50, Away: 300, Flee: 300, Hold: 100}}},
	)

	s.Nil(s.beatOf(enc, encounter.BeatTempered, goblin), "nothing was rolled, so nothing is narrated")

	members, err := enc.Members()
	s.Require().NoError(err)
	for _, m := range members {
		if m.ID == goblin {
			s.Equal("coward", m.Temper.Word, "and the word the author wrote is the word it has")
		}
	}
}

// beatKinds is every beat the story tells, in order, by kind.
func (s *WorldTimeSuite) beatKinds(enc *encounter.Encounter) []string {
	s.T().Helper()

	story, err := enc.Story(&encounter.StoryInput{Audience: alice})
	s.Require().NoError(err)
	out := make([]string, 0, len(story))
	for _, entry := range story {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		out = append(out, beat["beat"].(string))
	}

	return out
}

// beatOf is the first beat of a kind naming one member, decoded.
func (s *WorldTimeSuite) beatOf(
	enc *encounter.Encounter, kind string, member encounter.MemberID,
) map[string]any {
	s.T().Helper()

	story, err := enc.Story(&encounter.StoryInput{Audience: alice})
	s.Require().NoError(err)
	for _, entry := range story {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		if beat["beat"] == kind && beat["member"] == string(member) {
			return beat
		}
	}

	return nil
}
