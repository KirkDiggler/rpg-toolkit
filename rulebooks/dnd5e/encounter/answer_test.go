// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// answer_test.go is THE AUTHOR'S TABLE ON A REAL BOARD (rpg-project#458,
// ideas/shenanigans/front-room-goblin.md): the world's die picks one entry,
// a `fact` reaches every witness and turns a disposition, a `flee` walks the
// creature away without a fight existing, and a failed check reads its own
// half of the table.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

type AnswerTestSuite struct {
	suite.Suite
	ctx context.Context
}

func TestAnswerSuite(t *testing.T) {
	suite.Run(t, new(AnswerTestSuite))
}

func (s *AnswerTestSuite) SetupTest() {
	s.ctx = context.Background()
}

// front is the goblin's own room: alice at one end, the goblin at the other,
// no wall between them and — deliberately — NO FIGHT. Nothing here is
// hostile, nobody rolled initiative, and the goblin still gets to act.
func (s *AnswerTestSuite) front(table encounter.Table) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: encounter.FieldInput{
			Canvas:   openAir(),
			Regions:  []encounter.RegionInput{rectRegion("front", 0, 0, 12, 6)},
			Factions: []encounter.FactionInput{{ID: campFaction, Mind: core.EntityID(goblin)}},
			Dispositions: []encounter.DispositionInput{{
				Between: [2]encounter.FactionID{campFaction, encounter.FactionParty},
				Stance:  encounter.StanceHostile, Until: encounter.TriggerFact{Fact: campFact},
			}},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 3, Y: 1},
				Faction: campFaction, SpeedFeet: 30, Table: table},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	return enc
}

func (s *AnswerTestSuite) beatsOf(
	enc *encounter.Encounter, who core.EntityID, kind string,
) []map[string]any {
	story, err := enc.Story(&encounter.StoryInput{Audience: who})
	s.Require().NoError(err)
	out := make([]map[string]any, 0)
	for _, entry := range story {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		if beat["beat"] == kind {
			out = append(out, beat)
		}
	}

	return out
}

func (s *AnswerTestSuite) answeredBeats(enc *encounter.Encounter, who core.EntityID) []map[string]any {
	story, err := enc.Story(&encounter.StoryInput{Audience: who})
	s.Require().NoError(err)
	out := make([]map[string]any, 0)
	for _, entry := range story {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		if beat["beat"] == encounter.BeatAnswered {
			out = append(out, beat)
		}
	}

	return out
}

func (s *AnswerTestSuite) cellOf(enc *encounter.Encounter, who encounter.MemberID) spatial.Position {
	members, err := enc.Members()
	s.Require().NoError(err)
	for _, m := range members {
		if m.ID == who {
			return m.Position
		}
	}
	s.Require().Fail("not on the roster", string(who))

	return spatial.Position{}
}

// A seeded die picks the entry its face lands in, and the die it rolls is the
// sum of every eligible entry's LOADED share — its authored weight times its
// temperament's percent factor (rpg-project#465). A soldier's factor is 100,
// so 70/30 is a d10000 and face 7001 is the first face of the second entry.
//
// HUNDREDTHS OF A WEIGHT, so a coward's half and an aggressive one's triple
// are expressible without fractions and the whole arithmetic on the beat stays
// in integers a reader can add up.
func (s *AnswerTestSuite) TestTheDieIsTheSumOfTheWeightsAndPicksByFace() {
	table := encounter.Table{
		encounter.AnswerIntimidated: {
			{Weight: 70, Say: "Fine, fine!", Fact: campFact},
			{Weight: 30, Say: "BOSS!"},
		},
	}

	for _, tc := range []struct {
		face  int
		entry int
		say   string
	}{
		{face: 1, entry: 0, say: "Fine, fine!"},
		{face: 7000, entry: 0, say: "Fine, fine!"},
		{face: 7001, entry: 1, say: "BOSS!"},
		{face: 10000, entry: 1, say: "BOSS!"},
	} {
		s.Run(tc.say, func() {
			enc := s.front(table)
			var of int
			_, err := enc.Intimidate(s.ctx, &encounter.IntimidateInput{
				Actor: alice, Target: goblin, Beaten: true, DC: 9, Total: 14,
				Roller: rollsFace{face: tc.face, of: &of},
			})
			s.Require().NoError(err)
			s.Equal(10000, of, "one die the size of the summed loaded shares")

			beats := s.answeredBeats(enc, goblin)
			s.Require().Len(beats, 1)
			s.EqualValues(tc.entry, beats[0]["entry"])
			s.Equal(tc.say, beats[0]["say"], "the author's line, verbatim")
			s.EqualValues(tc.face, beats[0]["roll"])
			s.EqualValues(10000, beats[0]["of"], "R1: the roll is in the log with what it was rolled against")
		})
	}
}

// A `fact` entry teaches EVERY witness and the disposition waiting on it
// fires: the camp turns, which is the proof the fact arrived rather than the
// beat merely claiming it did.
func (s *AnswerTestSuite) TestAFactEntryTeachesEveryWitnessAndTurnsTheCamp() {
	enc := s.front(encounter.Table{
		encounter.AnswerIntimidated: {{Weight: 1, Say: "Fine.", Fact: campFact}},
	})

	before, err := enc.Stance(campFaction, encounter.FactionParty)
	s.Require().NoError(err)
	s.Require().Equal(encounter.StanceHostile, before, "precondition: nobody knows yet")

	_, err = enc.Intimidate(s.ctx, &encounter.IntimidateInput{
		Actor: alice, Target: goblin, Beaten: true, DC: 9, Total: 14, Roller: rollsLowest{},
	})
	s.Require().NoError(err)

	after, err := enc.Stance(campFaction, encounter.FactionParty)
	s.Require().NoError(err)
	s.Equal(encounter.StanceNeutral, after, "the until fired on the fact the entry planted")

	beats := s.answeredBeats(enc, alice)
	s.Require().Len(beats, 1)
	s.Equal("fact", beats[0]["word"])
	s.Equal(string(campFact), beats[0]["fact"])
	s.Equal(string(goblin), beats[0]["creature"])
	s.Equal(encounter.DeedIntimidate, beats[0]["verb"])
	s.Equal(true, beats[0]["beaten"])
}

// neutralFront is THE FRONT ROOM AS THE DESIGN DESCRIBES IT: a declared
// faction the party is NOT hostile to, so no bubble forms, nobody rolls
// initiative, and the goblin is on the world clock — the room the whole slice
// exists for.
func (s *AnswerTestSuite) neutralFront(table encounter.Table) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: tableDriver(), Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: encounter.FieldInput{
			Canvas:   openAir(),
			Regions:  []encounter.RegionInput{rectRegion("front", 0, 0, 30, 6)},
			Factions: []encounter.FactionInput{{ID: "goblins", Mind: core.EntityID(goblin)}},
			Dispositions: []encounter.DispositionInput{{
				Between: [2]encounter.FactionID{"goblins", encounter.FactionParty},
				Stance:  encounter.StanceNeutral,
			}},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 3, Y: 1},
				Faction: "goblins", SpeedFeet: 30, Table: table},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	return enc
}

// A `flee` entry LANDS A DEED AND NOTHING ELSE, and the creature's own `time`
// table is what does the running — for as many rounds as the author wrote
// (rpg-project#465, design §2).
//
// THE ONE-SHOT WALK IS GONE. `flee` used to route the creature away once,
// inside the verb, which could never express "it keeps running while the party
// walks after it". Now the verb that scared it pays a round of the world clock,
// the creature reads `fled: { within: 3 }` on its own table, and it runs on
// that round and the two after it — then holds, because the deed has aged out.
func (s *AnswerTestSuite) TestAFleeEntryLandsTheDeedAndTheTableDoesTheRunning() {
	enc := s.neutralFront(encounter.Table{
		encounter.AnswerIntimidated: {{Weight: 1, Say: "BOSS!", Flee: true}},
		encounter.AnswerTime: {
			{Weight: 1, When: &encounter.When{Deed: "fled", Within: 3},
				Away: &encounter.Selector{Word: encounter.SelectorActor}},
			{Weight: 1, Hold: true},
		},
	})

	clock, err := enc.ClockOf(&encounter.ClockOfInput{Member: goblin})
	s.Require().NoError(err)
	s.Require().Equal(encounter.ClockWorld, clock.Kind, "precondition: nothing here is fighting")

	start := s.cellOf(enc, goblin)
	s.Require().Equal(float64(3), start.X)

	// The threat, which pays the world a round on its way out — so the goblin
	// gets that round and spends it running.
	_, err = enc.Intimidate(s.ctx, &encounter.IntimidateInput{
		Actor: alice, Target: goblin, Beaten: true, DC: 9, Total: 14, Roller: rollsLowest{},
	})
	s.Require().NoError(err)

	first := s.cellOf(enc, goblin)
	s.Greater(enc.Distance(s.cellOf(enc, alice), first), enc.Distance(s.cellOf(enc, alice), start),
		"it ran away from her on the round the threat paid for, by the ruler")

	after, err := enc.ClockOf(&encounter.ClockOfInput{Member: goblin})
	s.Require().NoError(err)
	s.Equal(encounter.ClockWorld, after.Kind, "a rout is not a turn, and did not start one")

	// AND IT KEEPS RUNNING while the party acts — rounds two and three of the
	// span the author wrote.
	previous := first
	for round := 2; round <= 3; round++ {
		_, err = aRound(enc)
		s.Require().NoError(err)
		now := s.cellOf(enc, goblin)
		s.Greater(enc.Distance(s.cellOf(enc, alice), now), enc.Distance(s.cellOf(enc, alice), previous),
			"round %d of `fled: { within: 3 }` is still a run", round)
		previous = now
	}

	// THE FOURTH ROUND IS WHERE IT STOPS. The deed is four rounds old, the
	// condition no longer holds, and the entry under it is `hold`.
	_, err = aRound(enc)
	s.Require().NoError(err)
	s.Equal(previous, s.cellOf(enc, goblin), "the span ran out, so the running did")

	// EVERY CELL IS A MOVEMENT BEAT THAT NAMES WHAT MOVED IT. A table's own
	// `away` names the table as the cause, which is how an observer tells a
	// creature obeying its orders from one being shoved.
	moved := s.beatsOf(enc, alice, "moved")
	s.Require().NotEmpty(moved, "the walk went down the log a cell at a time")
	for _, beat := range moved {
		s.Equal(string(goblin), beat["member"])
		s.Equal("encounter:table:away", beat["cause"],
			"a step nobody caused would leave this key absent")
	}

	last := moved[len(moved)-1]["position"].(map[string]any)
	s.EqualValues(previous.X, last["x"], "the last beat is where it stopped")
	s.EqualValues(previous.Y, last["y"])

	beats := s.answeredBeats(enc, alice)
	s.Require().NotEmpty(beats)
	s.Equal("flee", beats[0]["word"], "the social answer that started it all")
	s.Empty(beats[0]["fact"], "a flee teaches nothing")
}

// A creature with nowhere to go stays put, and the beat still says the entry
// fired. Not going anywhere is an outcome, not an error.
func (s *AnswerTestSuite) TestAPinnedCreatureStaysAndTheBeatStillFires() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: encounter.FieldInput{
			Canvas:  openAir(),
			Regions: []encounter.RegionInput{rectRegion("cell", 0, 0, 2, 1)},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 0}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 1, Y: 0}, SpeedFeet: 30,
				Table: encounter.Table{
					encounter.AnswerIntimidated: {{Weight: 1, Flee: true}},
				}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	_, err = enc.Intimidate(s.ctx, &encounter.IntimidateInput{
		Actor: alice, Target: goblin, Beaten: true, DC: 9, Total: 14, Roller: rollsLowest{},
	})
	s.Require().NoError(err)

	s.Equal(float64(1), s.cellOf(enc, goblin).X, "there was nowhere farther to stand")
	s.Require().Len(s.answeredBeats(enc, alice), 1, "the entry fired even though nothing moved")
}

// A failed check reads the FAILED half of the table, which is what makes
// attempting worse than not attempting.
func (s *AnswerTestSuite) TestAFailedCheckReadsItsOwnTable() {
	enc := s.front(encounter.Table{
		encounter.AnswerIntimidated:      {{Weight: 1, Say: "Fine."}},
		encounter.AnswerIntimidateFailed: {{Weight: 1, Say: "Big talk."}},
	})

	_, err := enc.Intimidate(s.ctx, &encounter.IntimidateInput{
		Actor: alice, Target: goblin, Beaten: false, DC: 9, Total: 4, Roller: rollsLowest{},
	})
	s.Require().NoError(err)

	beats := s.answeredBeats(enc, alice)
	s.Require().Len(beats, 1)
	s.Equal("Big talk.", beats[0]["say"])
	s.Equal(false, beats[0]["beaten"])
	s.Empty(beats[0]["word"], "an entry that only speaks carries no word")
}

// No table for the outcome that happened is SILENCE: no die, no beat. Absent
// means absent, and it is distinguishable from an entry that fired and did
// nothing.
func (s *AnswerTestSuite) TestNoTableMeansNoRollAndNoBeat() {
	s.Run("nothing authored at all", func() {
		enc := s.front(nil)
		_, err := enc.Intimidate(s.ctx, &encounter.IntimidateInput{
			Actor: alice, Target: goblin, Beaten: true, DC: 9, Total: 14, Roller: refusesToRoll{fail: func(why string) { s.Fail(why) }},
		})
		s.Require().NoError(err)
		s.Empty(s.answeredBeats(enc, alice))
	})

	s.Run("a table for the other verdict only", func() {
		enc := s.front(encounter.Table{
			encounter.AnswerIntimidated: {{Weight: 1, Say: "Fine."}},
		})
		_, err := enc.Intimidate(s.ctx, &encounter.IntimidateInput{
			Actor: alice, Target: goblin, Beaten: false, DC: 9, Total: 2, Roller: refusesToRoll{fail: func(why string) { s.Fail(why) }},
		})
		s.Require().NoError(err)
		s.Empty(s.answeredBeats(enc, alice), "the failed half was never authored")
	})
}

// The die is supplied, never defaulted: a verb handed no roller is refused
// before anything is written.
func (s *AnswerTestSuite) TestAVerbWithNoDieIsRefused() {
	enc := s.front(nil)

	_, err := enc.Intimidate(s.ctx, &encounter.IntimidateInput{Actor: alice, Target: goblin})
	s.ErrorIs(err, encounter.ErrNoRoller)

	_, err = enc.Persuade(s.ctx, &encounter.PersuadeInput{Actor: alice, Target: goblin})
	s.ErrorIs(err, encounter.ErrNoRoller)
}

// A table this composition could not roll is refused at the door the member
// came in through, never at the roll.
func (s *AnswerTestSuite) TestAnUnrollableTableIsRefusedAtTheDoor() {
	s.Run("an outcome key this build does not land", func() {
		_, err := encounter.NewEncounter(s.setupWith(encounter.Table{
			"bribed": {{Weight: 1, Say: "ok"}},
		}))
		s.Require().ErrorIs(err, encounter.ErrBadAnswer)
	})

	s.Run("an entry that can never fire", func() {
		_, err := encounter.NewEncounter(s.setupWith(encounter.Table{
			encounter.AnswerPersuaded: {{Weight: 0, Say: "never"}},
		}))
		s.Require().ErrorIs(err, encounter.ErrBadAnswer)
	})
}

// setupWith is the front room with one table, for the refusal scenes.
func (s *AnswerTestSuite) setupWith(table encounter.Table) *encounter.SetupInput {
	return &encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: encounter.FieldInput{
			Canvas:  openAir(),
			Regions: []encounter.RegionInput{rectRegion("front", 0, 0, 6, 6)},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 1}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 3, Y: 1}, Table: table},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	}
}
