// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// OutcomeTestSuite covers Record: the one way a rule resolved somewhere else
// reaches this encounter's story.
type OutcomeTestSuite struct {
	suite.Suite
}

func TestOutcomeSuite(t *testing.T) {
	suite.Run(t, new(OutcomeTestSuite))
}

const outcomeRoom = "yard"

// scene opens a room with alice and a goblin standing apart, out of each
// other's sight, so nothing forms a fight before a test asks for one.
func (s *OutcomeTestSuite) scene() *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		Field: encounter.FieldInput{Rooms: []encounter.RoomInput{{
			ID: outcomeRoom, Width: 12, Height: 12, Occluders: wallRow(6, 4, 8),
		}}},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Room: outcomeRoom, Position: spatial.Position{X: 6, Y: 2}},
			{ID: goblin, Kind: encounter.KindMonster, Room: outcomeRoom, Position: spatial.Position{X: 6, Y: 10}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	return enc
}

// TestARuleResolvedElsewhereReachesTheStory is why Record exists.
//
// A strike resolves in another module entirely: it returns a value and writes
// no beat, and this package's appendBeat is unexported. Before Record there
// was no way for that to become something a player could read — the SDK builds
// every client event by reading each member's story, so an attack was
// invisible in both places at once (rpg-toolkit#966).
func (s *OutcomeTestSuite) TestARuleResolvedElsewhereReachesTheStory() {
	enc := s.scene()

	out, err := enc.Record(&encounter.RecordInput{
		Kind:    encounter.OutcomeStruck,
		Actor:   alice,
		Targets: []encounter.MemberID{goblin},
		Values: map[encounter.OutcomeValue]int{
			encounter.ValueRoll:    17,
			encounter.ValueTotal:   22,
			encounter.ValueAgainst: 15,
			encounter.ValueAmount:  9,
		},
	})
	s.Require().NoError(err)
	s.NotZero(out.Seq)

	story, err := enc.Story(&encounter.StoryInput{Audience: goblin})
	s.Require().NoError(err)
	s.Require().NotEmpty(story)
	last := story[len(story)-1]
	s.Equal(out.Seq, last.Seq, "RecordOutput.Seq references the beat it wrote")
	s.Equal("outcome", last.Tags["tag"])

	var beat map[string]any
	s.Require().NoError(json.Unmarshal(last.Payload, &beat))
	s.Equal("struck", beat["beat"])
	s.Equal("alice", beat["actor"])
	s.Equal([]any{"goblin"}, beat["targets"])
	s.Equal(float64(17), beat["roll"])
	s.Equal(float64(22), beat["total"])
	s.Equal(float64(15), beat["against"])
	s.Equal(float64(9), beat["amount"])
}

// TestTheTargetHearsItToo pins the audience rule.
//
// An outcome is not secret. A fight is localized, but it is not private: a
// client that learned about a strike only from the striker's own response
// could not render the scene the rest of the party is standing in.
func (s *OutcomeTestSuite) TestTheTargetHearsItToo() {
	enc := s.scene()
	_, err := enc.Record(&encounter.RecordInput{
		Kind: encounter.OutcomeMissed, Actor: alice, Targets: []encounter.MemberID{goblin},
	})
	s.Require().NoError(err)

	for _, who := range []encounter.MemberID{alice, goblin} {
		story, serr := enc.Story(&encounter.StoryInput{Audience: who})
		s.Require().NoError(serr)
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(story[len(story)-1].Payload, &beat))
		s.Equal("missed", beat["beat"], "%s hears it", who)
	}
}

// TestAnOutcomeCarriesNoProse is the shape's whole claim, asserted rather than
// promised.
//
// The reason the composition owns its record is that a reader can trust what
// is in it. An append taking free bytes would have kept the transcript in one
// place and given that up — any caller could narrate at other players through
// it. So RecordInput has NO string field: the kind is a closed enum, members
// are IDs checked against the roster, values are integers under closed keys.
//
// This test reads the type rather than exercising it, because the guarantee is
// structural: prose is not filtered here, it is INEXPRESSIBLE, and the day
// someone adds a Description field this fails and says why.
func (s *OutcomeTestSuite) TestAnOutcomeCarriesNoProse() {
	s.Equal([]string{"Kind", "Actor", "Targets", "Values"},
		structFieldNames(encounter.RecordInput{}),
		"a new field on RecordInput needs an argument: free text here is prose "+
			"in a transcript other players read")
}

// TestRefusalsAreCheckedAgainstTheRoster pins that the composition validates
// what it stamps, which is the other half of owning the record.
func (s *OutcomeTestSuite) TestRefusalsAreCheckedAgainstTheRoster() {
	s.Run("nil input", func() {
		_, err := s.scene().Record(nil)
		s.ErrorIs(err, encounter.ErrNilInput)
	})

	s.Run("a kind this composition does not know", func() {
		_, err := s.scene().Record(&encounter.RecordInput{
			Kind: encounter.OutcomeKind("disintegrated"), Actor: alice,
		})
		s.ErrorIs(err, encounter.ErrInvalidData)
	})

	s.Run("a value name this composition does not know", func() {
		_, err := s.scene().Record(&encounter.RecordInput{
			Kind: encounter.OutcomeStruck, Actor: alice,
			Values: map[encounter.OutcomeValue]int{encounter.OutcomeValue("temp_hp"): 3},
		})
		s.ErrorIs(err, encounter.ErrInvalidData)
	})

	s.Run("an actor who is not a member", func() {
		_, err := s.scene().Record(&encounter.RecordInput{
			Kind: encounter.OutcomeStruck, Actor: "nobody",
		})
		s.ErrorIs(err, encounter.ErrNoMember)
	})

	s.Run("an empty actor", func() {
		_, err := s.scene().Record(&encounter.RecordInput{Kind: encounter.OutcomeStruck})
		s.ErrorIs(err, encounter.ErrNoMember)
	})

	s.Run("a target who is not a member", func() {
		_, err := s.scene().Record(&encounter.RecordInput{
			Kind: encounter.OutcomeStruck, Actor: alice,
			Targets: []encounter.MemberID{"ghost"},
		})
		s.ErrorIs(err, encounter.ErrNoMember)
	})

	s.Run("a closed encounter", func() {
		enc := s.scene()
		_, err := enc.End(&encounter.EndInput{Ending: "withdrawn"})
		s.Require().NoError(err)
		_, err = enc.Record(&encounter.RecordInput{
			Kind: encounter.OutcomeStruck, Actor: alice,
		})
		s.ErrorIs(err, encounter.ErrClosed)
	})
}

// TestTheOutcomeLandsAfterTheVerbThatCausedIt is Record's half of the
// cause-before-effect law (see refreshSight).
//
// An outcome is a CONSEQUENCE — of a walk, an arrival, whatever produced it —
// so it lands after that verb's own beat. Record holds that by appending its own
// beat FIRST and refreshing no sight, detecting no trigger and moving no clock,
// so there is nothing it could append ahead of the outcome and nothing it could
// reorder.
//
// Nobody is down in this scene, so the standing consult Record now runs
// (rpg-toolkit#1083) finds nothing to say and the beat list is the same list it
// always was. That is the point of leaving this pin exactly as it was written:
// the news a consult has none of must cost the story nothing. killingblow_test.go
// owns the scene where there IS news, and pins that it lands after the outcome
// rather than before it.
func (s *OutcomeTestSuite) TestTheOutcomeLandsAfterTheVerbThatCausedIt() {
	enc := s.scene()

	moved, err := enc.Step(&encounter.StepInput{Member: alice, To: spatial.Position{X: 5, Y: 2}})
	s.Require().NoError(err)
	s.Require().Nil(moved.Formed, "the wall keeps this walk quiet")

	recorded, err := enc.Record(&encounter.RecordInput{
		Kind: encounter.OutcomeMissed, Actor: alice, Targets: []encounter.MemberID{goblin},
	})
	s.Require().NoError(err)
	s.Greater(recorded.Seq, moved.Seq, "the walk happened, THEN what came of it")

	story, err := enc.Story(&encounter.StoryInput{Audience: alice})
	s.Require().NoError(err)
	kinds := make([]string, 0, len(story))
	for _, entry := range story {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		kinds = append(kinds, beat["beat"].(string))
	}
	s.Equal([]string{"scene-opened", "moved", "missed"}, kinds,
		"recording appends exactly one beat and nothing before it")
}
