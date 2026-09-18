// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
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

	mix := encounter.Temper{
		Mix:      map[string]int{"coward": 1, "soldier": 2, "aggressive": 1},
		Profiles: profiles,
	}
	enc := s.hallWith(
		[]encounter.DispositionInput{{
			Between: [2]encounter.FactionID{"vendors", encounter.FactionParty},
			Stance:  encounter.StanceHostile,
		}},
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer,
			Position: spatial.Position{X: 1, Y: 1}, SpeedFeet: 30},
		encounter.MemberInput{ID: goblin, Kind: encounter.KindMonster, Faction: "vendors",
			Position: spatial.Position{X: 20, Y: 6}, SpeedFeet: 30, Temper: mix},
		// One that named no side, to pin that the beat carries the RESOLVED
		// faction rather than the authored blank.
		encounter.MemberInput{ID: "straggler", Kind: encounter.KindMonster,
			Position: spatial.Position{X: 28, Y: 7}, SpeedFeet: 30, Temper: mix},
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

	// THE FACTION IS THE DIE'S ENTITY (rpg-project#463): the spread belongs to
	// the group, and the deal is one roll out of it — so the beat names the
	// faction that threw the die and not the creature that came out of it.
	s.Equal("vendors", beat["faction"], "the side whose orders this creature came in under")

	stragglerBeat := s.beatOf(enc, encounter.BeatTempered, "straggler")
	s.Require().NotNil(stragglerBeat)
	s.Equal(string(encounter.FactionMonsters), stragglerBeat["faction"],
		"and a monster that named no side is in the reserved one, resolved rather than left blank")

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

// --- the band that made the default work -------------------------------------

// swingRecorder is a Striker that remembers who swung at whom. It records
// rather than resolves: what this scene is about is whether the table ever got
// the creature close enough to swing at all.
type swingRecorder struct {
	swings []encounter.MemberID
}

func (r *swingRecorder) Strike(
	_ context.Context, _ *encounter.Encounter, _, target encounter.MemberID, _ core.Ref,
) error {
	r.swings = append(r.swings, target)

	return nil
}

// TestTheDefaultTableClosesAndThenSwings is the session builder's finding,
// fixed and pinned end to end.
//
// THE SHIPPED DEFAULT COULD NOT CLOSE. `enemy: seen` fired `attack: enemy`,
// and an attack on somebody out of reach is a pass — so a thug that could see
// the party stood across the room swinging at nothing, every round, forever.
// Sight and reach are two different questions and the table had a word for
// only one of them.
//
// With the band: `seen` (in sight, out of reach) walks, `reach` swings. One
// driven turn does both, because the driver is asked again after every
// executed intent and the view is rebuilt each time.
func (s *WorldTimeSuite) TestTheDefaultTableClosesAndThenSwings() {
	striker := &swingRecorder{}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: tableDriver(), Roller: rollsLowest{},
		Striker: striker, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: encounter.FieldInput{
			Canvas:  openAir(),
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 30, 8)},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}, SpeedFeet: 30},
			// Four cells away with thirty feet of speed and five of reach: it
			// cannot touch her where it stands, and it can reach her if it
			// walks. That gap is the whole scene.
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 5, Y: 1},
				SpeedFeet: 30,
				Actions:   []encounter.ActionView{{Ref: testMeleeAction, RangeFeet: 5}},
				Table:     theDefaultThugTable()},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	on, err := enc.ClockOf(&encounter.ClockOfInput{Member: alice})
	s.Require().NoError(err)
	s.Require().Equal(encounter.ClockTurn, on.Kind, "precondition: seeing each other started the fight")

	started := whereIs(s.T(), enc, goblin)

	// Alice ends her turn; the goblin's is driven inside the same call.
	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: on.Active})
	s.Require().NoError(err)

	s.NotEqual(started, whereIs(s.T(), enc, goblin), "it closed, which `enemy: seen` is for")
	s.Require().Len(striker.swings, 1, "and then it swung, which `enemy: reach` is for")
	s.Equal(alice, striker.swings[0])
}

// theDefaultThugTable is the rulebook's default, as rulebooks/dnd5e ships it
// and as dungeonspec's own test compiles it from the design's §2 text. Written
// out in Go here because this scene is about what the table DOES, and a scene
// that also had to parse it would be two claims in one test.
func theDefaultThugTable() encounter.Table {
	return encounter.Table{
		encounter.AnswerTime: {
			{Weight: 3, When: &encounter.When{Deed: "attacked", Within: 3},
				Attack: &encounter.Selector{Word: encounter.SelectorAttacker}},
			{Weight: 1, When: &encounter.When{Enemy: encounter.EnemyReach},
				Attack: &encounter.Selector{Word: encounter.SelectorEnemy}},
			{Weight: 1, When: &encounter.When{Enemy: encounter.EnemySeen},
				Toward: &encounter.Selector{Word: encounter.SelectorEnemy}},
			{Weight: 1, When: &encounter.When{Enemy: encounter.EnemyRemembered},
				Toward: &encounter.Selector{Word: encounter.SelectorEnemy}},
			{Weight: 1, Hold: true},
		},
	}
}

// TestASpentCreatureIsNotAskedToSwingAgain is the api builder's finding on the
// live stack, fixed and pinned.
//
// WHAT HAPPENED. The turn loop asks again after every executed intent — an
// attack that lands still leaves movement to spend — so a creature whose table
// said `attack` was handed `attack` a SECOND time with no attack left, a
// second identical `answered` beat went down the log, and nothing happened. A
// pick that cannot act is noise, and two accounts of one swing is a story
// nobody can read.
//
// AFFORDABILITY IS ELIGIBILITY now. The second consult finds the `attack` row
// off the table, rolls the `hold` beneath it, and the turn ends. The log
// carries one account of the swing.
func (s *WorldTimeSuite) TestASpentCreatureIsNotAskedToSwingAgain() {
	striker := &swingRecorder{}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: tableDriver(), Roller: rollsLowest{},
		Striker: striker, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: encounter.FieldInput{
			Canvas:  openAir(),
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 30, 8)},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}, SpeedFeet: 30},
			// Already in reach, so its first roll is the swing and nothing
			// has to walk first.
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 2, Y: 1},
				SpeedFeet: 30,
				Actions:   []encounter.ActionView{{Ref: testMeleeAction, RangeFeet: 5}},
				// ONLY `attack`, and nothing else: with no `hold` beneath it
				// the second consult has nothing at all on the table, which
				// is the empty pick this rule is really about.
				Table: encounter.Table{encounter.AnswerTime: {
					{Weight: 1, When: &encounter.When{Enemy: encounter.EnemyReach},
						Attack: &encounter.Selector{Word: encounter.SelectorEnemy}},
				}},
			},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	on, err := enc.ClockOf(&encounter.ClockOfInput{Member: alice})
	s.Require().NoError(err)
	s.Require().Equal(encounter.ClockTurn, on.Kind, "precondition: they are in a fight")

	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: on.Active})
	s.Require().NoError(err)

	s.Require().Len(striker.swings, 1, "it swung once, which is all it could pay for")

	picks := s.picksOf(enc, goblin)
	s.Require().Len(picks, 1, "and the log carries ONE account of that swing, not two")
	s.Equal("attack", picks[0]["word"])
}

// picksOf is every `answered` beat naming one creature, decoded, in order.
func (s *WorldTimeSuite) picksOf(enc *encounter.Encounter, creature encounter.MemberID) []map[string]any {
	s.T().Helper()

	story, err := enc.Story(&encounter.StoryInput{Audience: alice})
	s.Require().NoError(err)

	var out []map[string]any
	for _, entry := range story {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		if beat["beat"] == encounter.BeatAnswered && beat["creature"] == string(creature) {
			out = append(out, beat)
		}
	}

	return out
}

// --- Kirk's walk: the bandits who never left the hall -------------------------

// TestAnOrderedCellSomebodyIsStandingOnIsStillWalkedToward is Kirk's walk
// finding, reproduced and fixed (rpg-project#465).
//
// WHAT HE SAW. Ten rounds of the world went by, the bandits picked their one
// entry — `toward: { at: [3,3] }`, the front room — on every one of them, and
// their budgets were spent. Not one `moved` beat existed for either of them.
// They never left the cells they arrived on.
//
// WHY. A goblin was standing on (3,3). The exact-cell search that makes
// `toward: { at: … }` mean "go THERE" needs its goal to be STANDABLE, and a
// cell with a creature on it is not — so it found no route at all, and the
// bandits took none of the twelve steps they could have walked. An order that
// cannot be completed is not an order to stand still.
//
// The exact cell is still preferred; a cell nobody can stop on falls through
// to the approach policy, whose answer is "as near as I can get".
func (s *WorldTimeSuite) TestAnOrderedCellSomebodyIsStandingOnIsStillWalkedToward() {
	target := cellAt(3, 3)

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: tableDriver(), Roller: rollsLowest{},
		Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: encounter.FieldInput{
			Canvas:  openAir(),
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 30, 8)},
			Factions: []encounter.FactionInput{
				{ID: "bandits", Mind: "bandit"},
				{ID: "squatters", Mind: "squatter"},
			},
			// Nobody is against anybody, so no fight forms and the bandit
			// stays the world's to think for — the state the walk was in.
			Dispositions: []encounter.DispositionInput{
				{Between: [2]encounter.FactionID{"bandits", encounter.FactionParty}, Stance: encounter.StanceNeutral},
				{Between: [2]encounter.FactionID{"squatters", encounter.FactionParty}, Stance: encounter.StanceNeutral},
				{Between: [2]encounter.FactionID{"bandits", "squatters"}, Stance: encounter.StanceNeutral},
			},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 20, Y: 6}, SpeedFeet: 30},
			// THE CELL IS TAKEN, which is the whole scene.
			{ID: "squatter", Kind: encounter.KindMonster, Faction: "squatters",
				Position: spatial.Position{X: 3, Y: 3}, SpeedFeet: 30},
			{ID: "bandit", Kind: encounter.KindMonster, Faction: "bandits",
				Position: spatial.Position{X: 15, Y: 3}, SpeedFeet: 30,
				Table: walksTo(target)},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	s.Require().Equal(target, whereIs(s.T(), enc, "squatter"), "precondition: somebody is on the ordered cell")
	started := whereIs(s.T(), enc, "bandit")

	_, err = enc.Search(&encounter.SearchInput{Member: alice, Region: "hall"})
	s.Require().NoError(err)

	moved := whereIs(s.T(), enc, "bandit")
	s.NotEqual(started, moved, "it walked, which on the walk it never did")
	s.Less(enc.Distance(moved, target), enc.Distance(started, target), "and it walked TOWARD the cell it was sent at")
	s.Equal(float64(6), enc.Distance(started, moved), "its whole speed: six cells of a thirty-foot walker")
}

// TestAWalkThatMovesNobodySaysSo is the second half of the same finding: the
// bandits' rounds were being spent and the story said nothing, so a reader
// could not tell a creature that refused from one nobody asked from one sent
// somewhere it could not reach.
func (s *WorldTimeSuite) TestAWalkThatMovesNobodySaysSo() {
	// Ordered at the cell it is already standing on: an order it has already
	// obeyed, and the shortest way to a route with nowhere to go.
	standing := cellAt(25, 6)

	enc := s.hallWith(
		[]encounter.DispositionInput{{
			Between: [2]encounter.FactionID{"vendors", encounter.FactionParty},
			Stance:  encounter.StanceNeutral,
		}},
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer,
			Position: spatial.Position{X: 1, Y: 1}, SpeedFeet: 30},
		encounter.MemberInput{ID: goblin, Kind: encounter.KindMonster, Faction: "vendors",
			Position: spatial.Position{X: 25, Y: 6}, SpeedFeet: 30,
			Table: walksTo(standing)},
	)

	s.Require().Equal(standing, whereIs(s.T(), enc, goblin), "precondition: it is already there")

	_, err := enc.Search(&encounter.SearchInput{Member: alice, Region: "hall"})
	s.Require().NoError(err)

	s.Equal(standing, whereIs(s.T(), enc, goblin), "it is already where it was sent, so it goes nowhere")

	beat := s.beatOf(enc, encounter.BeatStayed, goblin)
	s.Require().NotNil(beat, "and the round it spent going nowhere is in the story")
	s.Equal("encounter:table:away", beat["cause"], "naming what routed it")
}
