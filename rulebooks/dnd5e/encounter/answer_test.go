// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// answer_test.go is THE AUTHOR'S TABLE ON A REAL BOARD (rpg-project#458,
// ideas/shenanigans/front-room-goblin.md, and since rpg-project#569 the rows
// live on the option that was spoken): the world's die picks one entry, a
// `fact` reaches every witness and turns a disposition, a `flee` walks the
// creature away without a fight existing, and a failed check reads its own
// half of the option.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
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

// threat is the one line these scenes speak: an Intimidation check whose
// success and failure rows are the scene's own.
func threat(success, failure []encounter.Answer) encounter.Option {
	return encounter.Option{
		ID: "threat", Say: "Stand aside.",
		Check:   []encounter.CheckApproach{{Ability: "intimidation", DC: 9}},
		Success: success, Failure: failure,
	}
}

// front is the goblin's own room: alice at one end, the goblin at the other,
// no wall between them and — deliberately — NO FIGHT. Nothing here is
// hostile, nobody rolled initiative, and the goblin still gets to act.
func (s *AnswerTestSuite) front(table encounter.Table, options ...encounter.Option) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
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
				Faction: campFaction, Table: table, Options: options},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sight:         everyoneSeesTheWholeMap{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        sheetFacts{goblin: {SpeedFeet: 30}, alice: {}},
			Participation: everyoneStanding{},
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

// allBeats is one member's whole story, decoded, in the order a client reads
// it — what an ordering claim has to be made against.
func (s *AnswerTestSuite) allBeats(enc *encounter.Encounter, who core.EntityID) []map[string]any {
	story, err := enc.Story(&encounter.StoryInput{Audience: who})
	s.Require().NoError(err)
	out := make([]map[string]any, 0, len(story))
	for _, entry := range story {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		out = append(out, beat)
	}

	return out
}

// whereIsBeat is the position of the first beat of a kind whose `key` names
// `who`, or -1. Beats name their member under different keys — an arrival
// says `id`, a deal says `member` — so the key is the caller's.
func (s *AnswerTestSuite) whereIsBeat(beats []map[string]any, kind, key, who string) int {
	for i, beat := range beats {
		if beat["beat"] == kind && beat[key] == who {
			return i
		}
	}

	return -1
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
	line := threat([]encounter.Answer{
		{Weight: 70, Say: "Fine, fine!", Fact: campFact},
		{Weight: 30, Say: "BOSS!"},
	}, nil)

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
			enc := s.front(nil, line)
			var of int
			_, err := enc.Speak(s.ctx, &encounter.SpeakInput{
				OptionID: "threat", Skill: "intimidation", Actor: alice, Target: goblin, Beaten: true, DC: 9, Total: 14,
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
	enc := s.front(nil, threat([]encounter.Answer{{Weight: 1, Say: "Fine.", Fact: campFact}}, nil))

	before, err := enc.Stance(campFaction, encounter.FactionParty)
	s.Require().NoError(err)
	s.Require().Equal(encounter.StanceHostile, before, "precondition: nobody knows yet")

	_, err = enc.Speak(s.ctx, &encounter.SpeakInput{
		OptionID: "threat", Skill: "intimidation", Actor: alice, Target: goblin, Beaten: true, DC: 9, Total: 14, Roller: rollsLowest{},
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
	s.Equal("speak", beats[0]["verb"])
	s.Equal("threat", beats[0]["option"], "the line it answered")
	s.Equal(true, beats[0]["beaten"])
}

// neutralFront is THE FRONT ROOM AS THE DESIGN DESCRIBES IT: a declared
// faction the party is NOT hostile to, so no bubble forms, nobody rolls
// initiative, and the goblin is on the world clock — the room the whole slice
// exists for.
func (s *AnswerTestSuite) neutralFront(table encounter.Table, options ...encounter.Option) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
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
				Faction: "goblins", Table: table, Options: options},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sight:         everyoneSeesTheWholeMap{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        sheetFacts{goblin: {SpeedFeet: 30}, "latecomer": {SpeedFeet: 30}, alice: {}},
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        tableDriver(),
			// The WORLD's die, which only a deal reads here: nobody in this room
			// has a mix until somebody joins carrying one.
			Roller: rollsLowest{},
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
		encounter.AnswerTime: {
			{Weight: 1, When: []encounter.When{{Deed: "fled", Within: 3}},
				Away: &encounter.Selector{Word: encounter.SelectorActor}},
			{Weight: 1, Hold: true},
		},
	}, threat([]encounter.Answer{{Weight: 1, Say: "BOSS!", Flee: true}}, nil))

	clock, err := enc.ClockOf(&encounter.ClockOfInput{Member: goblin})
	s.Require().NoError(err)
	s.Require().Equal(encounter.ClockWorld, clock.Kind, "precondition: nothing here is fighting")

	start := s.cellOf(enc, goblin)
	s.Require().Equal(float64(3), start.X)

	// The threat, which pays the world a round on its way out — so the goblin
	// gets that round and spends it running.
	_, err = enc.Speak(s.ctx, &encounter.SpeakInput{
		OptionID: "threat", Skill: "intimidation", Actor: alice, Target: goblin, Beaten: true, DC: 9, Total: 14, Roller: rollsLowest{},
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
		Field: encounter.FieldInput{
			Canvas:  openAir(),
			Regions: []encounter.RegionInput{rectRegion("cell", 0, 0, 2, 1)},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 0}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 1, Y: 0},
				Options: []encounter.Option{threat([]encounter.Answer{{Weight: 1, Flee: true}}, nil)}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sight:         everyoneSeesTheWholeMap{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        sheetFacts{goblin: {SpeedFeet: 30}, alice: {}},
			Participation: everyoneStanding{},
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

	_, err = enc.Speak(s.ctx, &encounter.SpeakInput{
		OptionID: "threat", Skill: "intimidation", Actor: alice, Target: goblin, Beaten: true, DC: 9, Total: 14, Roller: rollsLowest{},
	})
	s.Require().NoError(err)

	s.Equal(float64(1), s.cellOf(enc, goblin).X, "there was nowhere farther to stand")
	s.Require().Len(s.answeredBeats(enc, alice), 1, "the entry fired even though nothing moved")
}

// A failed check reads the FAILED half of the table, which is what makes
// attempting worse than not attempting.
func (s *AnswerTestSuite) TestAFailedCheckReadsItsOwnTable() {
	enc := s.front(nil, threat(
		[]encounter.Answer{{Weight: 1, Say: "Fine."}},
		[]encounter.Answer{{Weight: 1, Say: "Big talk."}},
	))

	_, err := enc.Speak(s.ctx, &encounter.SpeakInput{
		OptionID: "threat", Skill: "intimidation", Actor: alice, Target: goblin, Beaten: false, DC: 9, Total: 4, Roller: rollsLowest{},
	})
	s.Require().NoError(err)

	beats := s.answeredBeats(enc, alice)
	s.Require().Len(beats, 1)
	s.Equal("Big talk.", beats[0]["say"])
	s.Equal(false, beats[0]["beaten"])
	s.Empty(beats[0]["word"], "an entry that only speaks carries no word")
}

// No rows for the outcome that happened is SILENCE: no die, no beat. Absent
// means absent, and it is distinguishable from an entry that fired and did
// nothing.
func (s *AnswerTestSuite) TestNoTableMeansNoRollAndNoBeat() {
	s.Run("a line with no answer at all", func() {
		enc := s.front(nil, threat(nil, nil))
		_, err := enc.Speak(s.ctx, &encounter.SpeakInput{
			OptionID: "threat", Skill: "intimidation", Actor: alice, Target: goblin, Beaten: true, DC: 9, Total: 14, Roller: refusesToRoll{fail: func(why string) { s.Fail(why) }},
		})
		s.Require().NoError(err)
		s.Empty(s.answeredBeats(enc, alice))
	})

	s.Run("rows for the other verdict only", func() {
		enc := s.front(nil, threat([]encounter.Answer{{Weight: 1, Say: "Fine."}}, nil))
		_, err := enc.Speak(s.ctx, &encounter.SpeakInput{
			OptionID: "threat", Skill: "intimidation", Actor: alice, Target: goblin, Beaten: false, DC: 9, Total: 2, Roller: refusesToRoll{fail: func(why string) { s.Fail(why) }},
		})
		s.Require().NoError(err)
		s.Empty(s.answeredBeats(enc, alice), "the failed half was never authored")
	})
}

// The die is supplied, never defaulted: a verb handed no roller is refused
// before anything is written.
func (s *AnswerTestSuite) TestAVerbWithNoDieIsRefused() {
	enc := s.front(nil, threat(nil, nil))

	_, err := enc.Speak(s.ctx, &encounter.SpeakInput{Actor: alice, Target: goblin, OptionID: "threat"})
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
			encounter.AnswerTime: {{Weight: 0, Say: "never"}},
		}))
		s.Require().ErrorIs(err, encounter.ErrBadAnswer)
	})

	// A RETIRED KEY IS NOT A TABLE KEY (rpg-project#569 R2): what a creature
	// answers when spoken to lives on the option that was spoken, and a
	// persisted blob still carrying the old key is refused by name.
	s.Run("a retired social key", func() {
		_, err := encounter.NewEncounter(s.setupWith(encounter.Table{
			"intimidated": {{Weight: 1, Say: "Fine."}},
		}))
		s.Require().ErrorIs(err, encounter.ErrBadAnswer)
	})
}

// setupWith is the front room with one table, for the refusal scenes.
func (s *AnswerTestSuite) setupWith(table encounter.Table) *encounter.SetupInput {
	return &encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:  openAir(),
			Regions: []encounter.RegionInput{rectRegion("front", 0, 0, 6, 6)},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 1}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 3, Y: 1}, Table: table},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sight:         everyoneSeesTheWholeMap{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        zeroSheets{},
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        passDriver{},
			Actors: encounter.Actors{
				Striker:   passStriker{},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	}
}

// --- the straggler's nerve, across a save ------------------------------------

// banditMix is a faction's authored spread and what each of its words means —
// the `temper: { coward: 1, soldier: 2, aggressive: 1 }` an author writes,
// with the rulebook's numbers behind it.
var (
	banditMix      = map[string]int{"coward": 1, "soldier": 2, "aggressive": 1}
	banditProfiles = map[string]encounter.TemperProfile{
		"coward":     {Attack: 50, Toward: 50, Away: 300, Flee: 300, Hold: 100},
		"soldier":    {Attack: 100, Toward: 100, Away: 100, Flee: 100, Hold: 100},
		"aggressive": {Attack: 300, Toward: 200, Away: 50, Flee: 50, Hold: 50},
	}
)

// straggler is the front room with somebody still to come: the goblin's table
// teaches a fact when it is threatened, and a bandit waits on that fact with
// its temperament UNDEALT — the faction's mix, because a straggler is dealt at
// the door it arrives through and not when it was written down.
func (s *AnswerTestSuite) straggler() *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
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
				Faction: campFaction,
				Options: []encounter.Option{threat([]encounter.Answer{{Weight: 1, Say: "Fine.", Fact: campFact}}, nil)}},
			{ID: "straggler", Kind: encounter.KindMonster, Position: spatial.Position{X: 9, Y: 3},
				Faction: campFaction,
				Arrives: encounter.TriggerFact{Fact: campFact},
				Temper:  encounter.Temper{Mix: banditMix, Profiles: banditProfiles}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sight:         everyoneSeesTheWholeMap{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        sheetFacts{goblin: {SpeedFeet: 30}, "straggler": {SpeedFeet: 30}, alice: {}},
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        tableDriver(),
			Roller:        rollsLowest{},
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

// reload round-trips an encounter through its own blob, the way the host does
// between two RPCs — which on a live stack is between EVERY two.
func (s *AnswerTestSuite) reload(enc *encounter.Encounter) *encounter.Encounter {
	out, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: enc.ToData(),
		Capabilities: encounter.Capabilities{
			Sight:         everyoneSeesTheWholeMap{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        sheetFacts{goblin: {SpeedFeet: 30}, "straggler": {SpeedFeet: 30}, alice: {}},
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        tableDriver(),
			Roller:        rollsLowest{},
			Actors: encounter.Actors{
				Striker:   passStriker{},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	s.Require().NoError(err)

	return out
}

// A STRAGGLER'S UNDEALT MIX SURVIVES THE SAVE, so the creature that walks in is
// the creature the author described (rpg-project#465, the walk's first
// finding).
//
// A member on the board stores the temperament it was DEALT and discards the
// mix on purpose: re-dealing on every load would hand a streamer a different
// goblin each time the run was reopened. A member still in the reserve has not
// been dealt yet, and the blob was rendering it through the dealt shape — so
// the spread went in the bin the first time the encounter was saved, and the
// arrival was handed an empty temperament with no `tempered` beat at all.
// rpg-api saves after every RPC, which made that every straggler on the real
// stack: the walk's bandits arrived as soldiers nobody had dealt.
func (s *AnswerTestSuite) TestAReservedMemberIsDealtFromTheMixItWasSavedWith() {
	enc := s.straggler()

	data := enc.ToData()
	s.Require().Len(data.Reserve, 1, "precondition: it is waiting, not standing")
	s.Equal(banditMix, data.Reserve[0].Temper.Mix, "the spread it has not been dealt from yet")
	s.Len(data.Reserve[0].Temper.Profiles, len(banditProfiles), "and what every word in it means")
	s.Empty(data.Reserve[0].Temper.Word, "nothing has been dealt, so there is no word to store")

	// The save, then the threat that plants the fact the straggler waits on.
	back := s.reload(enc)
	_, err := back.Speak(s.ctx, &encounter.SpeakInput{
		OptionID: "threat", Skill: "intimidation", Actor: alice, Target: goblin, Beaten: true, DC: 9, Total: 14, Roller: rollsLowest{},
	})
	s.Require().NoError(err)

	arrived := s.beatsOf(back, alice, "arrived")
	s.Require().Len(arrived, 1, "the fact brought it in")
	s.Equal("straggler", arrived[0]["id"])

	dealt := s.beatsOf(back, alice, encounter.BeatTempered)
	s.Require().Len(dealt, 1, "and its nerve was dealt at the door it came in through")

	// THE ARRIVAL IS ANNOUNCED FIRST. A client learns who is on the board from
	// the arrival beat, so a deal in front of it puts a raw id on the one line
	// that exists to introduce the creature. The constructor keeps the same
	// rule when it deals after first light.
	beats := s.allBeats(back, alice)
	arrivedAt := s.whereIsBeat(beats, "arrived", "id", "straggler")
	temperedAt := s.whereIsBeat(beats, encounter.BeatTempered, "member", "straggler")
	s.Require().NotEqual(-1, arrivedAt)
	s.Require().NotEqual(-1, temperedAt)
	s.Less(arrivedAt, temperedAt, "it is on the board before the story says what kind of creature it is")
	s.Equal("straggler", dealt[0]["member"])
	s.Equal(string(campFaction), dealt[0]["faction"], "the side whose spread it was")
	s.EqualValues(4, dealt[0]["of"], "one plus two plus one: the shares the author wrote")
	word, _ := dealt[0]["temper"].(string)
	s.Contains(banditMix, word, "a word out of the mix, not an empty temperament")

	// AND THE CREATURE STORES WHAT THE DEAL PRODUCED: the word, that word's
	// numbers, and no mix — the member shape, now that it is a member.
	after := back.ToData()
	s.Empty(after.Reserve, "it is on the board")
	var stored encounter.TemperData
	for _, m := range after.Members {
		if m.ID == "straggler" {
			stored = m.Temper
		}
	}
	s.Equal(word, stored.Word, "the same word the beat announced")
	profile := banditProfiles[word]
	s.Equal(profile.Away, stored.Profile.Away, "with the numbers the author gave that word")
	s.Equal(profile.Attack, stored.Profile.Attack)
}

// --- one memory per actor AND kind -------------------------------------------

// neutralFrontPair is [AnswerTestSuite.neutralFront] with a SECOND goblin for
// alice to shout at: the same room, the same camp, nothing hostile, and a
// creature standing out of the first one's way whose only job is to be
// somebody else.
func (s *AnswerTestSuite) neutralFrontPair(table encounter.Table, options ...encounter.Option) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
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
				Faction: "goblins", Table: table, Options: options},
			{ID: "cousin", Kind: encounter.KindMonster, Position: spatial.Position{X: 1, Y: 5},
				Faction: "goblins", Options: []encounter.Option{threat(nil, nil)}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sight:         everyoneSeesTheWholeMap{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        sheetFacts{"cousin": {SpeedFeet: 30}, goblin: {SpeedFeet: 30}, alice: {}},
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        tableDriver(),
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

// A CREATURE DOES NOT FORGET WHAT WAS DONE TO IT BECAUSE THE SAME PERSON THEN
// DID SOMETHING TO SOMEBODY ELSE (rpg-project#465, the walk's second finding).
//
// The goblin fled alice and its table says to keep running for three rounds.
// Alice spends those rounds threatening the OTHER goblin, in full view of the
// first. Every one of those deeds is hers, and while a witness held one deed
// per ACTOR the second one landed on the handle the flight was filed under:
// the goblin stopped running a round in, because somebody else got shouted at.
// A deed is held per actor and KIND now, so the flight ages out on the
// schedule the author wrote and on nothing else.
func (s *AnswerTestSuite) TestAFlightSurvivesWhatTheSameActorDoesToSomebodyElse() {
	enc := s.neutralFrontPair(encounter.Table{
		encounter.AnswerTime: {
			{Weight: 1, When: []encounter.When{{Deed: "fled", Within: 3}},
				Away: &encounter.Selector{Word: encounter.SelectorActor}},
			{Weight: 1, Hold: true},
		},
	}, threat([]encounter.Answer{{Weight: 1, Say: "BOSS!", Flee: true}}, nil))

	start := s.cellOf(enc, goblin)
	_, err := enc.Speak(s.ctx, &encounter.SpeakInput{
		OptionID: "threat", Skill: "intimidation", Actor: alice, Target: goblin, Beaten: true, DC: 9, Total: 14, Roller: rollsLowest{},
	})
	s.Require().NoError(err)

	previous := s.cellOf(enc, goblin)
	s.Greater(enc.Distance(s.cellOf(enc, alice), previous), enc.Distance(s.cellOf(enc, alice), start),
		"it ran on the round the threat paid for")

	// ROUNDS TWO AND THREE, spent on the cousin. Each threat pays the world a
	// round, so the fleeing goblin gets its consult — and what it holds about
	// alice now includes something that happened to somebody else.
	for round := 2; round <= 3; round++ {
		_, ierr := enc.Speak(s.ctx, &encounter.SpeakInput{
			OptionID: "threat", Skill: "intimidation", Actor: alice, Target: "cousin", Beaten: true, DC: 9, Total: 14, Roller: rollsLowest{},
		})
		s.Require().NoError(ierr)

		now := s.cellOf(enc, goblin)
		s.Greater(enc.Distance(s.cellOf(enc, alice), now), enc.Distance(s.cellOf(enc, alice), previous),
			"round %d: the flight is its own memory and the shouting is not", round)
		previous = now
	}

	// AND IT STILL STOPS WHEN THE AUTHOR SAID IT WOULD. Surviving the eviction
	// is not outliving the span: the deed is four rounds old and `within: 3`
	// no longer holds.
	_, err = enc.Speak(s.ctx, &encounter.SpeakInput{
		OptionID: "threat", Skill: "intimidation", Actor: alice, Target: "cousin", Beaten: true, DC: 9, Total: 14, Roller: rollsLowest{},
	})
	s.Require().NoError(err)
	s.Equal(previous, s.cellOf(enc, goblin), "three rounds is what was written, and three is what it ran")
}

// A MONSTER THAT JOINS MID-RUN IS ANNOUNCED BEFORE ITS NERVE IS DEALT, which
// is the arrival's rule at the other door (rpg-project#465).
//
// Join is how a host spawns a creature into a running scene, and it deals the
// faction's mix exactly as Setup and an arrival do. The `joined` beat is where
// a client learns the member exists, so the deal follows it — and it still
// happens before the sight refresh below it, because forming a bubble can
// consult a table and a creature consulted before its deal would roll its
// orders as the soldier an empty temperament looks like.
func (s *AnswerTestSuite) TestAJoinerIsAnnouncedBeforeItsNerveIsDealt() {
	enc := s.neutralFront(encounter.Table{
		encounter.AnswerTime: {{Weight: 1, Hold: true}},
	})

	_, err := enc.Join(&encounter.JoinInput{
		Member: "latecomer", Kind: encounter.KindMonster, Cell: spatial.Position{X: 9, Y: 3},
		Faction: "goblins",
		Temper:  encounter.Temper{Mix: banditMix, Profiles: banditProfiles},
	})
	s.Require().NoError(err)

	beats := s.allBeats(enc, alice)
	joinedAt := s.whereIsBeat(beats, "joined", "member", "latecomer")
	temperedAt := s.whereIsBeat(beats, encounter.BeatTempered, "member", "latecomer")
	s.Require().NotEqual(-1, joinedAt, "the join is in the story")
	s.Require().NotEqual(-1, temperedAt, "and so is the deal")
	s.Less(joinedAt, temperedAt, "introduced, then described")

	word, _ := beats[temperedAt]["temper"].(string)
	s.Contains(banditMix, word, "a word out of the mix the joiner carried in")
	s.Equal("goblins", beats[temperedAt]["faction"], "dealt as the side it joined")
}

// --- hearing on a creature's own time (rpg-project#569) ----------------------
//
// These scenes stand in the three-room hall noise_test.go builds: a | b | c,
// an open door in each seam on row 5. A line said is a voice, DC 10 where it
// is said and 10 a room over; a guard with passive 10 hears one room away
// and not two.

const (
	hCrier  encounter.MemberID = "crier"
	hWalker encounter.MemberID = "walker"
	hOther  encounter.MemberID = "other"
	hDeaf   encounter.MemberID = "deaf"
	hAlarm                     = "alarm-raised"
)

// tabled is a monster at an authored [col,row] with its own table.
func tabled(id encounter.MemberID, col, row int, table encounter.Table) encounter.MemberInput {
	m := at(id, encounter.KindMonster, col, row)
	m.Table = table

	return m
}

// cries is the crier's standing order: shout the alarm with every round it has.
func cries() encounter.Table {
	return encounter.Table{encounter.AnswerTime: {{Weight: 1, Say: "To arms!", Fact: hAlarm}}}
}

// knows reports whether a member's own journal fold holds a fact — read off
// the saved journal, which is what a reload would hand back.
func knows(t *testing.T, enc *encounter.Encounter, member encounter.MemberID, id string) bool {
	t.Helper()
	data := enc.ToData()
	if data.Journal == nil {
		return false
	}
	for _, f := range data.Journal.Facts {
		if f.Kind == "known:fact:"+id && f.Subject == string(member) {
			return true
		}
	}

	return false
}

// TestATimeRowsFactTeachesItsHearersAndNotTheFarRoom is R6: `fact` on a
// `time` row is an alarm. Its audience is whoever SEES the crier (alice,
// across room a) and whoever HEARS it (the guard one room over), and nobody
// two rooms away with the same ears.
func TestATimeRowsFactTeachesItsHearersAndNotTheFarRoom(t *testing.T) {
	enc := tableHall(t,
		sheetFacts{
			noiseAlice: {SpeedFeet: 30}, hCrier: {SpeedFeet: 30},
			noiseKeen: {SpeedFeet: 30, PassivePerception: 10}, noiseFar: {SpeedFeet: 30, PassivePerception: 10},
		},
		at(noiseAlice, encounter.KindPlayer, 2, 8),
		tabled(hCrier, 1, 1, cries()),
		at(noiseKeen, encounter.KindMonster, 18, 1),
		at(noiseFar, encounter.KindMonster, 28, 1),
	)
	require.False(t, knows(t, enc, noiseKeen, hAlarm), "precondition: nobody has said anything")

	_, err := aRound(enc)
	require.NoError(t, err)

	require.True(t, knows(t, enc, noiseAlice, hAlarm), "alice saw it shouted — a witness, as every fact has")
	require.True(t, knows(t, enc, noiseKeen, hAlarm), "the guard next door heard it: 10 beats 5 + 5")
	require.False(t, knows(t, enc, noiseFar, hAlarm), "two rooms is 15, and 10 does not carry that far")
	require.Contains(t, soundsHeldBy(t, enc, noiseKeen), encounter.SoundVoice, "and what it heard was a voice")
	require.Empty(t, soundsHeldBy(t, enc, noiseFar))
}

// TestAWhenFactRowIsEligibleOnlyForAHolder is R6's other half: the same row
// on two guards side by side, and only the one who heard the alarm acts on it.
func TestAWhenFactRowIsEligibleOnlyForAHolder(t *testing.T) {
	post := cellAt(12, 4)
	onAlarm := encounter.Table{encounter.AnswerTime: {
		{Weight: 1, When: []encounter.When{{Fact: hAlarm}}, Toward: &encounter.Selector{At: &post}},
		{Weight: 1, Hold: true},
	}}
	enc := tableHall(t,
		sheetFacts{
			noiseAlice: {SpeedFeet: 30}, hCrier: {SpeedFeet: 30},
			noiseKeen: {SpeedFeet: 30, PassivePerception: 15}, hDeaf: {SpeedFeet: 30},
		},
		at(noiseAlice, encounter.KindPlayer, 2, 8),
		tabled(hCrier, 1, 1, cries()),
		tabled(noiseKeen, 18, 1, onAlarm),
		tabled(hDeaf, 18, 0, onAlarm),
	)
	keenStart, deafStart := whereIs(t, enc, noiseKeen), whereIs(t, enc, hDeaf)

	for range 2 {
		_, err := aRound(enc)
		require.NoError(t, err)
	}

	require.False(t, knows(t, enc, hDeaf, hAlarm), "precondition: the deaf guard neither heard nor saw it")
	require.NotEqual(t, keenStart, whereIs(t, enc, noiseKeen), "the guard who holds the fact went to its post")
	require.Equal(t, deafStart, whereIs(t, enc, hDeaf), "the one who does not hold it had only `hold`")
}

// TestNotEnemySeenHoldsWithNoEnemyInSight: `not` inverts what it wraps, read
// through the real pick — with nobody opposed anywhere, `not: { enemy: seen }`
// holds and `enemy: seen` does not.
func TestNotEnemySeenHoldsWithNoEnemyInSight(t *testing.T) {
	post := cellAt(12, 4)
	walkWhen := func(when encounter.When) encounter.Table {
		return encounter.Table{encounter.AnswerTime: {
			{Weight: 1, When: []encounter.When{when}, Toward: &encounter.Selector{At: &post}},
			{Weight: 1, Hold: true},
		}}
	}
	enc := tableHall(t,
		sheetFacts{noiseAlice: {SpeedFeet: 30}, hWalker: {SpeedFeet: 30}, hOther: {SpeedFeet: 30}},
		at(noiseAlice, encounter.KindPlayer, 2, 8),
		tabled(hWalker, 18, 1, walkWhen(encounter.When{Not: &encounter.When{Enemy: encounter.EnemySeen}})),
		tabled(hOther, 18, 8, walkWhen(encounter.When{Enemy: encounter.EnemySeen})),
	)
	walkerStart, otherStart := whereIs(t, enc, hWalker), whereIs(t, enc, hOther)

	_, err := aRound(enc)
	require.NoError(t, err)

	require.NotEqual(t, walkerStart, whereIs(t, enc, hWalker), "no enemy seen, so `not: { enemy: seen }` held")
	require.Equal(t, otherStart, whereIs(t, enc, hOther), "and `enemy: seen` did not")
}

// TestAListedWhenNeedsEveryEntryThroughTheRealPick is R8: a row's `when` is a
// list and every entry must hold. Two guards both heard the alarm; one row
// also needs nobody in sight (true), the other also needs a fact nobody
// taught (false), and only the first walks.
func TestAListedWhenNeedsEveryEntryThroughTheRealPick(t *testing.T) {
	post := cellAt(12, 4)
	walkWhenAll := func(when ...encounter.When) encounter.Table {
		return encounter.Table{encounter.AnswerTime: {
			{Weight: 1, When: when, Toward: &encounter.Selector{At: &post}},
			{Weight: 1, Hold: true},
		}}
	}
	enc := tableHall(t,
		sheetFacts{
			noiseAlice: {SpeedFeet: 30}, hCrier: {SpeedFeet: 30},
			hWalker: {SpeedFeet: 30, PassivePerception: 15}, hOther: {SpeedFeet: 30, PassivePerception: 15},
		},
		at(noiseAlice, encounter.KindPlayer, 2, 8),
		tabled(hCrier, 1, 1, cries()),
		tabled(hWalker, 18, 1, walkWhenAll(
			encounter.When{Fact: hAlarm}, encounter.When{Not: &encounter.When{Enemy: encounter.EnemySeen}})),
		tabled(hOther, 18, 0, walkWhenAll(
			encounter.When{Fact: hAlarm}, encounter.When{Fact: "guards-turned"})),
	)
	walkerStart, otherStart := whereIs(t, enc, hWalker), whereIs(t, enc, hOther)

	for range 2 {
		_, err := aRound(enc)
		require.NoError(t, err)
	}

	require.True(t, knows(t, enc, hOther, hAlarm), "precondition: both heard the alarm")
	require.NotEqual(t, walkerStart, whereIs(t, enc, hWalker), "both entries held")
	require.Equal(t, otherStart, whereIs(t, enc, hOther), "one of two is not enough")
}

// TestAPathWalksItsLoopAndResumesAtTheNearest is R7 as ruled on the review
// of rpg-toolkit#2044: a patrol walks the hex line of each leg, waypoint to
// waypoint and round again, at its full movement — six cells a round here,
// past a waypoint when it has movement left. Knocked off the route, it walks
// back to the nearest waypoint and carries on from there. No cursor: every
// step is read off where it stands.
//
// THE LOOP'S CELLS ARE WRITTEN OUT, not computed, so this pins the walk the
// author's four waypoints produce rather than agreeing with itself: the
// square's sixteen cells, from the first waypoint, in walking order.
func TestAPathWalksItsLoopAndResumesAtTheNearest(t *testing.T) {
	route := []spatial.Position{cellAt(12, 2), cellAt(16, 2), cellAt(16, 6), cellAt(12, 6)}
	loop := []spatial.Position{
		{X: 12, Y: 2}, {X: 13, Y: 2}, {X: 14, Y: 2}, {X: 15, Y: 2}, // leg 1, along row 2
		{X: 15, Y: 3}, {X: 14, Y: 4}, {X: 14, Y: 5}, {X: 13, Y: 6}, // leg 2
		{X: 12, Y: 6}, {X: 11, Y: 6}, {X: 10, Y: 6}, {X: 9, Y: 6}, // leg 3, along row 6
		{X: 10, Y: 5}, {X: 10, Y: 4}, {X: 11, Y: 3}, {X: 11, Y: 2}, // leg 4, home
	}
	enc := tableHall(t,
		sheetFacts{noiseAlice: {SpeedFeet: 30}, hWalker: {SpeedFeet: 30}},
		at(noiseAlice, encounter.KindPlayer, 2, 8),
		tabled(hWalker, 12, 2, encounter.Table{encounter.AnswerTime: {
			{Weight: 1, Toward: &encounter.Selector{Path: route}},
		}}),
	)
	require.Equal(t, route[0], whereIs(t, enc, hWalker), "precondition: it starts on the first waypoint")
	require.Equal(t, route[0], loop[len(loop)-1], "precondition: the loop ends where it starts")

	for round, steps := range []int{6, 12, 18} {
		_, err := aRound(enc)
		require.NoError(t, err)
		require.Equal(t, loop[(steps-1)%len(loop)], whereIs(t, enc, hWalker),
			"round %d is six more cells round the loop", round+1)
	}

	// SHOVED OFF THE ROUTE — a commanded step the table did not choose, onto a
	// cell on no leg. The first waypoint is two cells away and the second
	// three, so the first is where it goes back to.
	off := spatial.Position{X: 13, Y: 1}
	_, err := enc.Direct(context.Background(), encounter.DirectInput{
		Mover: hWalker, Route: []spatial.Position{off},
		Cause: core.Ref{Module: "test", Type: "spell", ID: "command"},
	})
	require.NoError(t, err)
	require.Equal(t, off, whereIs(t, enc, hWalker), "precondition: it stands on no leg")

	_, err = aRound(enc)
	require.NoError(t, err)
	require.Equal(t, route[0], whereIs(t, enc, hWalker), "off the route, it goes back to the nearest waypoint")

	_, err = aRound(enc)
	require.NoError(t, err)
	require.Equal(t, loop[5], whereIs(t, enc, hWalker), "and walks the loop on from there")
}

// TestTowardSoundWalksToTheClang is R10: a guard who heard steel walks to
// where it rang, through the door, and finds out who by looking. The clang's
// cell is alice's, so the guard ends beside her.
func TestTowardSoundWalksToTheClang(t *testing.T) {
	enc := tableHall(t,
		sheetFacts{
			noiseAlice: {SpeedFeet: 30}, noiseThug: {SpeedFeet: 30},
			noiseKeen: {SpeedFeet: 30, PassivePerception: 15},
		},
		at(noiseAlice, encounter.KindPlayer, 2, 5), at(noiseThug, encounter.KindMonster, 3, 5),
		tabled(noiseKeen, 18, 1, encounter.Table{encounter.AnswerTime: {
			{Weight: 1, When: []encounter.When{{Heard: &encounter.HeardWhen{Kind: encounter.SoundClang, Within: 6}}},
				Toward: &encounter.Selector{Word: encounter.SelectorSound}},
			{Weight: 1, Hold: true},
		}}),
	)
	clangAt := cellAt(2, 5)
	start := whereIs(t, enc, noiseKeen)

	aliceStrikesTheThug(t, enc)
	for range 4 {
		_, err := aRound(enc)
		require.NoError(t, err)
	}

	end := whereIs(t, enc, noiseKeen)
	require.Less(t, enc.Distance(end, clangAt), enc.Distance(start, clangAt))
	require.LessOrEqual(t, enc.Distance(end, clangAt), 1.0, "it walked all the way to where the clang rang")
}

// TestAwaySoundWalksFromIt: the coward's half of R10.
func TestAwaySoundWalksFromIt(t *testing.T) {
	enc := tableHall(t,
		sheetFacts{
			noiseAlice: {SpeedFeet: 30}, noiseThug: {SpeedFeet: 30},
			hWalker: {SpeedFeet: 30, PassivePerception: 15},
		},
		at(noiseAlice, encounter.KindPlayer, 2, 5), at(noiseThug, encounter.KindMonster, 3, 5),
		tabled(hWalker, 6, 5, encounter.Table{encounter.AnswerTime: {
			{Weight: 1, When: []encounter.When{{Heard: &encounter.HeardWhen{Kind: encounter.SoundClang, Within: 2}}},
				Away: &encounter.Selector{Word: encounter.SelectorSound}},
			{Weight: 1, Hold: true},
		}}),
	)
	clangAt := cellAt(2, 5)
	start := whereIs(t, enc, hWalker)

	aliceStrikesTheThug(t, enc)
	_, err := aRound(enc)
	require.NoError(t, err)

	require.Greater(t, enc.Distance(whereIs(t, enc, hWalker), clangAt), enc.Distance(start, clangAt))
}

// TestSoundWalksToTheKindItsRowListensFor is R10 as the behavior review
// refined it: `sound` is where the freshest sound OF THE KIND THE ROW HEARD
// rang, not the freshest sound of all. The guard heard a clang, then went on
// hearing a talker's voice every round after it; its row listens for the
// clang, so it walks to the clang.
func TestSoundWalksToTheKindItsRowListensFor(t *testing.T) {
	enc := tableHall(t,
		sheetFacts{
			noiseAlice: {SpeedFeet: 30}, noiseThug: {SpeedFeet: 30}, hCrier: {SpeedFeet: 30},
			noiseKeen: {SpeedFeet: 30, PassivePerception: 15},
		},
		at(noiseAlice, encounter.KindPlayer, 2, 5), at(noiseThug, encounter.KindMonster, 3, 5),
		tabled(hCrier, 7, 0, encounter.Table{encounter.AnswerTime: {{Weight: 1, Say: "Who's there?"}}}),
		tabled(noiseKeen, 18, 1, encounter.Table{encounter.AnswerTime: {
			{Weight: 1, When: []encounter.When{{Heard: &encounter.HeardWhen{Kind: encounter.SoundClang, Within: 6}}},
				Toward: &encounter.Selector{Word: encounter.SelectorSound}},
			{Weight: 1, Hold: true},
		}}),
	)
	clangAt, voiceAt := cellAt(2, 5), cellAt(7, 0)

	aliceStrikesTheThug(t, enc)
	for range 4 {
		_, err := aRound(enc)
		require.NoError(t, err)
	}

	heard := soundsHeldBy(t, enc, noiseKeen)
	require.Contains(t, heard, encounter.SoundClang)
	require.Contains(t, heard, encounter.SoundVoice)
	require.Greater(t, heard[encounter.SoundVoice].at, heard[encounter.SoundClang].at,
		"precondition: the voice is the fresher sound")

	end := whereIs(t, enc, noiseKeen)
	require.LessOrEqual(t, enc.Distance(end, clangAt), 1.0, "it walked to the clang it was listening for")
	require.Greater(t, enc.Distance(end, voiceAt), 1.0, "not to the fresher voice")
}

// TestASoundRowWithNoHeardConditionIsRefusedAtTheDoor: `sound` resolves
// against what the row heard, so a row that listens for nothing cannot name
// it — mind/behavior's refusal, met at this composition's door.
func TestASoundRowWithNoHeardConditionIsRefusedAtTheDoor(t *testing.T) {
	_, err := encounter.NewEncounter(noiseHallSetup(encounter.DoorIsOpen(), encounter.DoorIsOpen(),
		sheetFacts{noiseAlice: {SpeedFeet: 30}, hWalker: {SpeedFeet: 30}},
		tableDriver(),
		at(noiseAlice, encounter.KindPlayer, 2, 8),
		tabled(hWalker, 18, 1, encounter.Table{encounter.AnswerTime: {
			{Weight: 1, Toward: &encounter.Selector{Word: encounter.SelectorSound}},
		}}),
	))
	require.ErrorIs(t, err, encounter.ErrBadAnswer)
	require.Contains(t, err.Error(), "`sound` needs a `heard` condition")
}

// TestAnAnswersLineIsAVoice: every `say` on any answer row is a voice — here
// a creature answering a spoken option, heard by the guard next door.
func TestAnAnswersLineIsAVoice(t *testing.T) {
	thug := at(noiseThug, encounter.KindMonster, 3, 5)
	thug.Options = []encounter.Option{threat([]encounter.Answer{{Weight: 1, Say: "Fine! FINE."}}, nil)}
	enc := noiseHall(t, encounter.DoorIsOpen(), encounter.DoorIsOpen(),
		sheetFacts{
			noiseAlice: {SpeedFeet: 30}, noiseThug: {SpeedFeet: 30},
			noiseKeen: {SpeedFeet: 30, PassivePerception: 15},
		},
		at(noiseAlice, encounter.KindPlayer, 2, 5), thug,
		at(noiseKeen, encounter.KindMonster, 15, 1),
	)

	_, err := enc.Speak(context.Background(), &encounter.SpeakInput{
		OptionID: "threat", Skill: "intimidation", Actor: noiseAlice, Target: noiseThug,
		Beaten: true, DC: 9, Total: 14, Roller: rollsLowest{},
	})
	require.NoError(t, err)

	voice, ok := soundsHeldBy(t, enc, noiseKeen)[encounter.SoundVoice]
	require.True(t, ok, "the guard next door heard the thug answer")
	where := voice.payload["where"].(map[string]any)
	require.Equal(t, cellAt(3, 5).X, where["x"], "at the one who answered, not the one who asked")
}

// TestSoundOnAttackIsRefusedAtTheDoor: a table that swings at a sound never
// reaches the board (R10).
func TestSoundOnAttackIsRefusedAtTheDoor(t *testing.T) {
	_, err := encounter.NewEncounter(noiseHallSetup(encounter.DoorIsOpen(), encounter.DoorIsOpen(),
		sheetFacts{noiseAlice: {SpeedFeet: 30}, hWalker: {SpeedFeet: 30}},
		tableDriver(),
		at(noiseAlice, encounter.KindPlayer, 2, 8),
		tabled(hWalker, 18, 1, encounter.Table{encounter.AnswerTime: {
			{Weight: 1, Attack: &encounter.Selector{Word: encounter.SelectorSound}},
		}}),
	))
	require.ErrorIs(t, err, encounter.ErrBadAnswer)
	require.Contains(t, err.Error(), "a sound cannot be struck")
}

// TestALongLegIsWalkedToItsEndNotBackToItsStart is the patrol rule as ruled
// on the review (rpg-project#569 R7): a creature partway along a leg walks on
// to the leg's far end, however near the waypoint it left still is. A walker
// that moves one cell a round on a triangle whose first leg is seven cells
// long is nearer the waypoint behind it for three rounds — "nearest" alone
// would turn it round on the first.
//
// AND IT WALKS ALL THREE LEGS FORWARD: one lap visits the second waypoint,
// then the third, then comes home, and never stands on a cell twice before
// it does.
func TestALongLegIsWalkedToItsEndNotBackToItsStart(t *testing.T) {
	route := []spatial.Position{cellAt(11, 1), cellAt(18, 1), cellAt(18, 8)}
	enc := tableHall(t,
		sheetFacts{noiseAlice: {SpeedFeet: 30}, hWalker: {SpeedFeet: 5}},
		at(noiseAlice, encounter.KindPlayer, 2, 8),
		tabled(hWalker, 11, 1, encounter.Table{encounter.AnswerTime: {
			{Weight: 1, Toward: &encounter.Selector{Path: route}},
		}}),
	)

	for i := 1; i <= 3; i++ {
		_, err := aRound(enc)
		require.NoError(t, err)
		require.Equal(t, cellAt(11+i, 1), whereIs(t, enc, hWalker), "round %d is one more cell along the long leg", i)
	}

	seen := map[spatial.Position]bool{cellAt(12, 1): true, cellAt(13, 1): true, cellAt(14, 1): true}
	var arrivals []spatial.Position
	previous := cellAt(14, 1)
	for lap := 0; lap < 40; lap++ {
		_, err := aRound(enc)
		require.NoError(t, err)
		now := whereIs(t, enc, hWalker)
		require.Equal(t, 1.0, enc.Distance(previous, now), "one cell a round, every round")
		previous = now
		for _, w := range route {
			if now == w {
				arrivals = append(arrivals, w)
			}
		}
		if now == route[0] {
			break
		}
		require.False(t, seen[now], "never back on a cell it already walked this lap: %v", now)
		seen[now] = true
	}
	require.Equal(t, []spatial.Position{route[1], route[2], route[0]}, arrivals,
		"the second waypoint, then the third, then home")
}

// TestARouteThatRetracesItselfIsRefusedAtTheDoor: A to B and back is the same
// cells twice, so a creature standing on one would be on both legs — refused
// at the member door, the reviewer's probe (a walker four cells along A→B on
// [(0,0),(10,0)]) unreachable by construction. The corridor pace is the named
// next use case for a direction derivation, not a thing read by guessing.
func TestARouteThatRetracesItselfIsRefusedAtTheDoor(t *testing.T) {
	_, err := encounter.NewEncounter(noiseHallSetup(encounter.DoorIsOpen(), encounter.DoorIsOpen(),
		sheetFacts{noiseAlice: {SpeedFeet: 30}, hWalker: {SpeedFeet: 30}},
		tableDriver(),
		at(noiseAlice, encounter.KindPlayer, 2, 8),
		tabled(hWalker, 12, 2, encounter.Table{encounter.AnswerTime: {
			{Weight: 1, Toward: &encounter.Selector{Path: []spatial.Position{cellAt(12, 2), cellAt(18, 2)}}},
		}}),
	))
	require.ErrorIs(t, err, encounter.ErrBadMember)
	require.ErrorIs(t, err, encounter.ErrBadAnswer)
	require.Contains(t, err.Error(), "this route retraces itself at")
	require.Contains(t, err.Error(), "add a waypoint so no leg crosses another")
}

// TestAFactOnlyTimeRowIsAVoice: an alarm with no line written for it is still
// said aloud — the hearers who learn it heard something, so they hold a voice.
func TestAFactOnlyTimeRowIsAVoice(t *testing.T) {
	enc := tableHall(t,
		sheetFacts{
			noiseAlice: {SpeedFeet: 30}, hCrier: {SpeedFeet: 30},
			noiseKeen: {SpeedFeet: 30, PassivePerception: 15},
		},
		at(noiseAlice, encounter.KindPlayer, 2, 8),
		tabled(hCrier, 1, 1, encounter.Table{encounter.AnswerTime: {{Weight: 1, Fact: hAlarm}}}),
		at(noiseKeen, encounter.KindMonster, 18, 1),
	)

	_, err := aRound(enc)
	require.NoError(t, err)

	require.True(t, knows(t, enc, noiseKeen, hAlarm), "the guard next door learned it by hearing it")
	require.Contains(t, soundsHeldBy(t, enc, noiseKeen), encounter.SoundVoice, "and what it heard was a voice")
}
