// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

const (
	commandedID = "skeleton-1"
	commanderID = "bard-1"
)

type CommandedConditionSuite struct {
	suite.Suite
	ctx      context.Context
	bus      events.EventBus
	removals []dnd5eEvents.ConditionRemovedEvent
}

func TestCommandedConditionSuite(t *testing.T) {
	suite.Run(t, new(CommandedConditionSuite))
}

func (s *CommandedConditionSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.removals = nil

	_, err := dnd5eEvents.ConditionRemovedTopic.On(s.bus).Subscribe(s.ctx,
		func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
			s.removals = append(s.removals, event)
			return nil
		})
	s.Require().NoError(err)
}

// commanded returns a live compulsion on the skeleton, for the one turn end
// the spell lasts.
func (s *CommandedConditionSuite) commanded(word string) *CommandedCondition {
	condition, err := NewCommandedCondition(commandedID, "", commanderID, word, 1)
	s.Require().NoError(err)
	s.Require().NoError(condition.Apply(s.ctx, s.bus))
	return condition
}

func (s *CommandedConditionSuite) endTurn(subjectID string) {
	s.Require().NoError(dnd5eEvents.TurnEndTopic.On(s.bus).Publish(s.ctx,
		dnd5eEvents.TurnEndEvent{SubjectID: subjectID, Round: 1}))
}

// TestTheCommandedCreaturesOwnTurnEndSpendsIt — "until the end of the target's
// next turn" with no new duration mechanism: the target's next turn end is the
// first one the condition will ever see, so one count is the whole of it.
//
// The opposite lean from Blade Ward, which is traced on the caster's own turn
// and needs two counts to survive to a swing. This one lands on somebody else,
// whose turn has not started yet.
func (s *CommandedConditionSuite) TestTheCommandedCreaturesOwnTurnEndSpendsIt() {
	condition := s.commanded("flee")

	s.endTurn(commandedID)

	s.Require().Len(s.removals, 1, "the compelled turn is the only turn it has")
	s.Equal(refs.Conditions.Commanded().String(), s.removals[0].ConditionRef)
	s.Equal("expired", s.removals[0].Reason)
	s.False(condition.IsApplied(), "and it detached rather than answering from a list it left")
}

func (s *CommandedConditionSuite) TestSomebodyElsesTurnEndDoesNotSpendIt() {
	s.commanded("approach")

	for range 5 {
		s.endTurn(commanderID)
	}

	s.Empty(s.removals, "the caster's own turns do not own this clock")
}

func (s *CommandedConditionSuite) TestCombatEndTakesIt() {
	s.commanded("grovel")

	s.Require().NoError(dnd5eEvents.CombatEndTopic.On(s.bus).Publish(s.ctx,
		dnd5eEvents.CombatEndEvent{SubjectID: commandedID}))

	s.Require().Len(s.removals, 1)
	s.Equal("combat ended", s.removals[0].Reason)
}

func (s *CommandedConditionSuite) TestApplyingItTwiceIsRefused() {
	condition := s.commanded("flee")
	s.Require().Error(condition.Apply(s.ctx, s.bus))
}

// TestItCarriesTheAnchorAndTheWord — the two things the driver reads. The
// caster is what Approach and Flee measure from, and the word is which of them
// it is.
func (s *CommandedConditionSuite) TestItCarriesTheAnchorAndTheWord() {
	condition := s.commanded("approach")
	s.Equal(commanderID, condition.CasterID())
	s.Equal("approach", condition.Word())
}

// The write side, key for key. A round trip cannot catch a rename; this can,
// and session reads two of these keys out of a stored blob without loading the
// condition at all.
func (s *CommandedConditionSuite) TestItPersistsTheExactWireShape() {
	condition, err := NewCommandedCondition(commandedID, "", commanderID, "flee", 1)
	s.Require().NoError(err)

	raw, err := condition.ToJSON()
	s.Require().NoError(err)

	s.JSONEq(`{
		"ref":"dnd5e:conditions:commanded",
		"member_id":"skeleton-1",
		"source_ref":"dnd5e:spells:command",
		"caster_id":"bard-1",
		"word":"flee",
		"turn_ends_left":1
	}`, string(raw))
}

// The read side, judged by what the loaded condition ANSWERS rather than by
// what a field reads — the half a symmetric round trip structurally cannot
// reach.
func (s *CommandedConditionSuite) TestAHandAuthoredBlobKeepsItsAnchorWordAndClock() {
	loaded, err := LoadJSON(json.RawMessage(`{
		"ref":"dnd5e:conditions:commanded",
		"member_id":"skeleton-1","source_ref":"dnd5e:spells:command",
		"caster_id":"bard-1","word":"grovel","turn_ends_left":1
	}`))
	s.Require().NoError(err)

	compulsion, ok := loaded.(*CommandedCondition)
	s.Require().True(ok)
	s.Equal("bard-1", compulsion.CasterID())
	s.Equal("grovel", compulsion.Word())

	s.Require().NoError(loaded.Apply(s.ctx, s.bus))
	s.endTurn(commandedID)
	s.Require().Len(s.removals, 1, "one left on the blob means the next turn end is its last")
}

// TestTheConstructorRefusesACompulsionNobodyCouldObey — each field is read by
// the driver, and a missing one is a turn the engine would have to invent a
// rule for: no anchor to walk at, no word to walk by, or a clock that expired
// before it began.
func (s *CommandedConditionSuite) TestTheConstructorRefusesACompulsionNobodyCouldObey() {
	s.Run("no caster to measure from", func() {
		_, err := NewCommandedCondition(commandedID, "", "", "flee", 1)
		s.Require().ErrorContains(err, "caster")
	})
	s.Run("no word to obey", func() {
		_, err := NewCommandedCondition(commandedID, "", commanderID, "", 1)
		s.Require().ErrorContains(err, "word")
	})
	s.Run("a clock that ended before it began", func() {
		for _, turnEnds := range []int{0, -1} {
			_, err := NewCommandedCondition(commandedID, "", commanderID, "flee", turnEnds)
			s.Require().ErrorContains(err, "turn end", "%d", turnEnds)
		}
	})
}

// TestTheFactoryRefusesEachMissingFieldRatherThanDefaultingIt — the factory is
// the path resolution takes, where the caster arrives by CounterpartKey and
// the word by OptionKey. A default here would put a ruling in the factory and
// hide a binding that failed to happen.
func (s *CommandedConditionSuite) TestTheFactoryRefusesEachMissingFieldRatherThanDefaultingIt() {
	for name, config := range map[string]json.RawMessage{
		"nothing at all":  nil,
		"an empty object": json.RawMessage(`{}`),
		"no caster":       json.RawMessage(`{"word":"flee","turn_ends":1}`),
		"no word":         json.RawMessage(`{"caster_id":"bard-1","turn_ends":1}`),
		"no clock":        json.RawMessage(`{"caster_id":"bard-1","word":"flee"}`),
	} {
		s.Run(name, func() {
			_, err := CreateFromRef(&CreateFromRefInput{
				Ref:      refs.Conditions.Commanded().String(),
				MemberID: commandedID,
				Config:   config,
			})
			s.Require().Error(err)
		})
	}
}

func (s *CommandedConditionSuite) TestTheFactoryBuildsAFullyBoundCompulsion() {
	out, err := CreateFromRef(&CreateFromRefInput{
		Ref:       refs.Conditions.Commanded().String(),
		MemberID:  commandedID,
		SourceRef: refs.Spells.Command().String(),
		Config:    json.RawMessage(`{"caster_id":"bard-1","word":"approach","turn_ends":1}`),
	})
	s.Require().NoError(err)

	compulsion, ok := out.Condition.(*CommandedCondition)
	s.Require().True(ok)
	s.Equal("bard-1", compulsion.CasterID())
	s.Equal("approach", compulsion.Word())
}

// HoldsRefSuite covers the two readers session has for a sheet's stored
// condition blobs: neither loads a condition or touches a bus.
type HoldsRefSuite struct {
	suite.Suite
}

func TestHoldsRefSuite(t *testing.T) {
	suite.Run(t, new(HoldsRefSuite))
}

func (s *HoldsRefSuite) TestItFindsTheRefAmongOthers() {
	stored := []json.RawMessage{
		json.RawMessage(`{"ref":"dnd5e:conditions:prone","member_id":"skeleton-1"}`),
		json.RawMessage(`{"ref":"dnd5e:conditions:commanded","member_id":"skeleton-1",` +
			`"caster_id":"bard-1","word":"flee","turn_ends_left":1}`),
	}

	held, err := HoldsRef(stored, refs.Conditions.Commanded())
	s.Require().NoError(err)
	s.True(held)
}

func (s *HoldsRefSuite) TestItSaysNoForASheetThatDoesNotHoldIt() {
	stored := []json.RawMessage{
		json.RawMessage(`{"ref":"dnd5e:conditions:prone","member_id":"skeleton-1"}`),
	}

	held, err := HoldsRef(stored, refs.Conditions.Commanded())
	s.Require().NoError(err)
	s.False(held)

	held, err = HoldsRef(nil, refs.Conditions.Commanded())
	s.Require().NoError(err)
	s.False(held, "and an empty sheet holds nothing")
}

// TestGarbageIsAnErrorRatherThanANo — a blob that cannot be read is not the
// same answer as a sheet that does not hold the condition. Reported, because
// the caller decides who is driven by it.
func (s *HoldsRefSuite) TestGarbageIsAnErrorRatherThanANo() {
	_, err := HoldsRef([]json.RawMessage{json.RawMessage(`not json`)}, refs.Conditions.Commanded())
	s.Require().Error(err)
}

func (s *HoldsRefSuite) TestItRefusesToLookForNothing() {
	_, err := HoldsRef(nil, nil)
	s.Require().Error(err, "a nil ref would match the first blob with no ref at all")
}

func (s *HoldsRefSuite) TestDecodeCommandedReturnsTheAnchorAndTheWord() {
	stored := []json.RawMessage{
		json.RawMessage(`{"ref":"dnd5e:conditions:prone","member_id":"skeleton-1"}`),
		json.RawMessage(`{"ref":"dnd5e:conditions:commanded","member_id":"skeleton-1",` +
			`"caster_id":"bard-1","word":"flee","turn_ends_left":1}`),
	}

	data, found, err := DecodeCommanded(stored)
	s.Require().NoError(err)
	s.Require().True(found)
	s.Equal("bard-1", data.CasterID)
	s.Equal("flee", data.Word)
	s.Equal("skeleton-1", data.MemberID)
}

func (s *HoldsRefSuite) TestDecodeCommandedFindsNothingOnASheetWithoutIt() {
	data, found, err := DecodeCommanded([]json.RawMessage{
		json.RawMessage(`{"ref":"dnd5e:conditions:prone","member_id":"skeleton-1"}`),
	})
	s.Require().NoError(err)
	s.False(found)
	s.Nil(data)
}

// TestDecodeCommandedReportsABlobItCannotRead — a Commanded blob with a
// malformed body is a driver that would otherwise walk a creature at nobody.
func (s *HoldsRefSuite) TestDecodeCommandedReportsABlobItCannotRead() {
	_, _, err := DecodeCommanded([]json.RawMessage{json.RawMessage(`{"ref":`)})
	s.Require().Error(err)
}
