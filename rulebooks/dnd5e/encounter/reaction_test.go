// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// reaction_test.go is THE AUTHOR'S TABLE ON A REAL BOARD (rpg-project#458,
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

type ReactionTestSuite struct {
	suite.Suite
	ctx context.Context
}

func TestReactionSuite(t *testing.T) {
	suite.Run(t, new(ReactionTestSuite))
}

func (s *ReactionTestSuite) SetupTest() {
	s.ctx = context.Background()
}

// front is the goblin's own room: alice at one end, the goblin at the other,
// no wall between them and — deliberately — NO FIGHT. Nothing here is
// hostile, nobody rolled initiative, and the goblin still gets to act.
func (s *ReactionTestSuite) front(table map[string][]encounter.Reaction) *encounter.Encounter {
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
				Faction: campFaction, SpeedFeet: 30, Reactions: table},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	return enc
}

func (s *ReactionTestSuite) reactedBeats(enc *encounter.Encounter, who core.EntityID) []map[string]any {
	story, err := enc.Story(&encounter.StoryInput{Audience: who})
	s.Require().NoError(err)
	out := make([]map[string]any, 0)
	for _, entry := range story {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		if beat["beat"] == encounter.BeatReacted {
			out = append(out, beat)
		}
	}

	return out
}

func (s *ReactionTestSuite) cellOf(enc *encounter.Encounter, who encounter.MemberID) spatial.Position {
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
// size of the SUM of the weights — not the count of the entries. 70/30 is a
// d100, and face 71 is the first face of the second entry.
func (s *ReactionTestSuite) TestTheDieIsTheSumOfTheWeightsAndPicksByFace() {
	table := map[string][]encounter.Reaction{
		encounter.ReactionIntimidated: {
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
		{face: 70, entry: 0, say: "Fine, fine!"},
		{face: 71, entry: 1, say: "BOSS!"},
		{face: 100, entry: 1, say: "BOSS!"},
	} {
		s.Run(tc.say, func() {
			enc := s.front(table)
			var of int
			_, err := enc.Intimidate(s.ctx, &encounter.IntimidateInput{
				Actor: alice, Target: goblin, Beaten: true, DC: 9, Total: 14,
				Roller: rollsFace{face: tc.face, of: &of},
			})
			s.Require().NoError(err)
			s.Equal(100, of, "one die the size of the summed weights")

			beats := s.reactedBeats(enc, goblin)
			s.Require().Len(beats, 1)
			s.EqualValues(tc.entry, beats[0]["entry"])
			s.Equal(tc.say, beats[0]["say"], "the author's line, verbatim")
			s.EqualValues(tc.face, beats[0]["roll"])
			s.EqualValues(100, beats[0]["of"], "R1: the roll is in the log with what it was rolled against")
		})
	}
}

// A `fact` entry teaches EVERY witness and the disposition waiting on it
// fires: the camp turns, which is the proof the fact arrived rather than the
// beat merely claiming it did.
func (s *ReactionTestSuite) TestAFactEntryTeachesEveryWitnessAndTurnsTheCamp() {
	enc := s.front(map[string][]encounter.Reaction{
		encounter.ReactionIntimidated: {{Weight: 1, Say: "Fine.", Fact: campFact}},
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

	beats := s.reactedBeats(enc, alice)
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
func (s *ReactionTestSuite) neutralFront(table map[string][]encounter.Reaction) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: encounter.FieldInput{
			Canvas:   openAir(),
			Regions:  []encounter.RegionInput{rectRegion("front", 0, 0, 12, 6)},
			Factions: []encounter.FactionInput{{ID: "goblins", Mind: core.EntityID(goblin)}},
			Dispositions: []encounter.DispositionInput{{
				Between: [2]encounter.FactionID{"goblins", encounter.FactionParty},
				Stance:  encounter.StanceNeutral,
			}},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 3, Y: 1},
				Faction: "goblins", SpeedFeet: 30, Reactions: table},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	return enc
}

// A `flee` entry WALKS THE CREATURE, with no fight in the room and nobody's
// turn being spent — the primitive this slice buys. It ends farther from the
// actor than it started, and no turn is consumed because there is no turn.
func (s *ReactionTestSuite) TestAFleeEntryWalksTheCreatureWithNoFightRunning() {
	enc := s.neutralFront(map[string][]encounter.Reaction{
		encounter.ReactionIntimidated: {{Weight: 1, Say: "BOSS!", Flee: true}},
	})

	clock, err := enc.ClockOf(&encounter.ClockOfInput{Member: goblin})
	s.Require().NoError(err)
	s.Require().Equal(encounter.ClockWorld, clock.Kind, "precondition: nothing here is fighting")

	start := s.cellOf(enc, goblin)
	s.Require().Equal(float64(3), start.X)

	_, err = enc.Intimidate(s.ctx, &encounter.IntimidateInput{
		Actor: alice, Target: goblin, Beaten: true, DC: 9, Total: 14, Roller: rollsLowest{},
	})
	s.Require().NoError(err)

	end := s.cellOf(enc, goblin)
	s.Greater(enc.Distance(s.cellOf(enc, alice), end), enc.Distance(s.cellOf(enc, alice), start),
		"it ran away from her, by the ruler")

	after, err := enc.ClockOf(&encounter.ClockOfInput{Member: goblin})
	s.Require().NoError(err)
	s.Equal(encounter.ClockWorld, after.Kind, "a rout is not a turn, and did not start one")

	beats := s.reactedBeats(enc, alice)
	s.Require().Len(beats, 1)
	s.Equal("flee", beats[0]["word"])
	s.Empty(beats[0]["fact"], "a flee teaches nothing")
}

// A creature with nowhere to go stays put, and the beat still says the entry
// fired. Not going anywhere is an outcome, not an error.
func (s *ReactionTestSuite) TestAPinnedCreatureStaysAndTheBeatStillFires() {
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
				Reactions: map[string][]encounter.Reaction{
					encounter.ReactionIntimidated: {{Weight: 1, Flee: true}},
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
	s.Require().Len(s.reactedBeats(enc, alice), 1, "the entry fired even though nothing moved")
}

// A failed check reads the FAILED half of the table, which is what makes
// attempting worse than not attempting.
func (s *ReactionTestSuite) TestAFailedCheckReadsItsOwnTable() {
	enc := s.front(map[string][]encounter.Reaction{
		encounter.ReactionIntimidated:      {{Weight: 1, Say: "Fine."}},
		encounter.ReactionIntimidateFailed: {{Weight: 1, Say: "Big talk."}},
	})

	_, err := enc.Intimidate(s.ctx, &encounter.IntimidateInput{
		Actor: alice, Target: goblin, Beaten: false, DC: 9, Total: 4, Roller: rollsLowest{},
	})
	s.Require().NoError(err)

	beats := s.reactedBeats(enc, alice)
	s.Require().Len(beats, 1)
	s.Equal("Big talk.", beats[0]["say"])
	s.Equal(false, beats[0]["beaten"])
	s.Empty(beats[0]["word"], "an entry that only speaks carries no word")
}

// No table for the outcome that happened is SILENCE: no die, no beat. Absent
// means absent, and it is distinguishable from an entry that fired and did
// nothing.
func (s *ReactionTestSuite) TestNoTableMeansNoRollAndNoBeat() {
	s.Run("nothing authored at all", func() {
		enc := s.front(nil)
		_, err := enc.Intimidate(s.ctx, &encounter.IntimidateInput{
			Actor: alice, Target: goblin, Beaten: true, DC: 9, Total: 14, Roller: refusesToRoll{fail: func(why string) { s.Fail(why) }},
		})
		s.Require().NoError(err)
		s.Empty(s.reactedBeats(enc, alice))
	})

	s.Run("a table for the other verdict only", func() {
		enc := s.front(map[string][]encounter.Reaction{
			encounter.ReactionIntimidated: {{Weight: 1, Say: "Fine."}},
		})
		_, err := enc.Intimidate(s.ctx, &encounter.IntimidateInput{
			Actor: alice, Target: goblin, Beaten: false, DC: 9, Total: 2, Roller: refusesToRoll{fail: func(why string) { s.Fail(why) }},
		})
		s.Require().NoError(err)
		s.Empty(s.reactedBeats(enc, alice), "the failed half was never authored")
	})
}

// The die is supplied, never defaulted: a verb handed no roller is refused
// before anything is written.
func (s *ReactionTestSuite) TestAVerbWithNoDieIsRefused() {
	enc := s.front(nil)

	_, err := enc.Intimidate(s.ctx, &encounter.IntimidateInput{Actor: alice, Target: goblin})
	s.ErrorIs(err, encounter.ErrNoRoller)

	_, err = enc.Persuade(s.ctx, &encounter.PersuadeInput{Actor: alice, Target: goblin})
	s.ErrorIs(err, encounter.ErrNoRoller)
}

// A table this composition could not roll is refused at the door the member
// came in through, never at the roll.
func (s *ReactionTestSuite) TestAnUnrollableTableIsRefusedAtTheDoor() {
	s.Run("an outcome key this build does not land", func() {
		_, err := encounter.NewEncounter(s.setupWith(map[string][]encounter.Reaction{
			"bribed": {{Weight: 1, Say: "ok"}},
		}))
		s.Require().ErrorIs(err, encounter.ErrBadReaction)
	})

	s.Run("an entry that can never fire", func() {
		_, err := encounter.NewEncounter(s.setupWith(map[string][]encounter.Reaction{
			encounter.ReactionPersuaded: {{Weight: 0, Say: "never"}},
		}))
		s.Require().ErrorIs(err, encounter.ErrBadReaction)
	})
}

// setupWith is the front room with one table, for the refusal scenes.
func (s *ReactionTestSuite) setupWith(table map[string][]encounter.Reaction) *encounter.SetupInput {
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
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 3, Y: 1}, Reactions: table},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	}
}
