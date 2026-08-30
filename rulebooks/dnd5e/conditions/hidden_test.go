// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type HiddenConditionTestSuite struct {
	suite.Suite
	ctx         context.Context
	bus         events.EventBus
	condition   *HiddenCondition
	characterID string
}

func TestHiddenConditionSuite(t *testing.T) {
	suite.Run(t, new(HiddenConditionTestSuite))
}

func (s *HiddenConditionTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.characterID = "char-hidden"
	s.condition = NewHiddenCondition(s.characterID)
}

func (s *HiddenConditionTestSuite) TestNewHiddenCondition() {
	s.Equal(s.characterID, s.condition.MemberID)
	s.False(s.condition.IsApplied())
}

func (s *HiddenConditionTestSuite) TestApply() {
	s.Run("applies successfully", func() {
		err := s.condition.Apply(s.ctx, s.bus)
		s.Require().NoError(err)
		s.True(s.condition.IsApplied())
		s.Len(s.condition.subscriptionIDs, 1)
	})

	s.Run("returns error if already applied", func() {
		condition := NewHiddenCondition(s.characterID)
		err := condition.Apply(s.ctx, s.bus)
		s.Require().NoError(err)

		err = condition.Apply(s.ctx, s.bus)
		s.Require().Error(err)
		s.Contains(err.Error(), "already applied")
	})
}

func (s *HiddenConditionTestSuite) TestRemove() {
	s.Run("removes successfully after apply", func() {
		condition := NewHiddenCondition(s.characterID)
		err := condition.Apply(s.ctx, s.bus)
		s.Require().NoError(err)

		err = condition.Remove(s.ctx, s.bus)
		s.Require().NoError(err)
		s.False(condition.IsApplied())
		s.Nil(condition.subscriptionIDs)
	})

	s.Run("no-op if not applied", func() {
		condition := NewHiddenCondition(s.characterID)
		err := condition.Remove(s.ctx, s.bus)
		s.Require().NoError(err)
	})
}

func (s *HiddenConditionTestSuite) runAttackChain(event dnd5eEvents.AttackChainEvent) dnd5eEvents.AttackChainEvent {
	attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	attacks := dnd5eEvents.AttackChain.On(s.bus)
	modifiedChain, err := attacks.PublishWithChain(s.ctx, event, attackChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, event)
	s.Require().NoError(err)
	return finalEvent
}

func (s *HiddenConditionTestSuite) TestAttackChain_HiddenCharacterAttacks() {
	condition := NewHiddenCondition(s.characterID)
	err := condition.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	var removedEvent *dnd5eEvents.ConditionRemovedEvent
	_, err = dnd5eEvents.ConditionRemovedTopic.On(s.bus).Subscribe(s.ctx,
		func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
			removedEvent = &event
			return nil
		})
	s.Require().NoError(err)

	finalEvent := s.runAttackChain(dnd5eEvents.AttackChainEvent{
		AttackerID: s.characterID,
		TargetID:   "goblin-1",
	})

	s.Require().Len(finalEvent.AdvantageSources, 1, "hidden attacker gets advantage on their attack")
	s.Equal(refs.Conditions.Hidden(), finalEvent.AdvantageSources[0].SourceRef)

	s.False(condition.IsApplied(), "Hidden ends when the hider attacks")
	s.Require().NotNil(removedEvent)
	s.Equal(s.characterID, removedEvent.MemberID)
	s.Equal(refs.Conditions.Hidden().String(), removedEvent.ConditionRef)
	s.Equal("attacked", removedEvent.Reason)
}

func (s *HiddenConditionTestSuite) TestAttackChain_HiddenCharacterIsTargeted() {
	condition := NewHiddenCondition(s.characterID)
	err := condition.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	finalEvent := s.runAttackChain(dnd5eEvents.AttackChainEvent{
		AttackerID: "goblin-1",
		TargetID:   s.characterID,
	})

	s.Require().Len(finalEvent.DisadvantageSources, 1, "attacks against the hidden character have disadvantage")
	s.Equal(refs.Conditions.Hidden(), finalEvent.DisadvantageSources[0].SourceRef)

	s.True(condition.IsApplied(), "being attacked does not end Hidden this wave")
}

func (s *HiddenConditionTestSuite) TestAttackChain_UnrelatedAttackIsUntouched() {
	condition := NewHiddenCondition(s.characterID)
	err := condition.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	finalEvent := s.runAttackChain(dnd5eEvents.AttackChainEvent{
		AttackerID: "goblin-1",
		TargetID:   "goblin-2",
	})

	s.Empty(finalEvent.AdvantageSources)
	s.Empty(finalEvent.DisadvantageSources)
	s.True(condition.IsApplied())
}

func (s *HiddenConditionTestSuite) TestToJSON() {
	condition := NewHiddenCondition(s.characterID)
	data, err := condition.ToJSON()
	s.Require().NoError(err)

	loaded := &HiddenCondition{}
	err = loaded.loadJSON(data)
	s.Require().NoError(err)
	s.Equal(s.characterID, loaded.MemberID)
}

// TestLoaderRoundTrip proves HiddenCondition is registered in loader.go — an
// unregistered condition would fail here with "unknown condition ref"
// instead of round-tripping. This is the RPC-boundary regression test named
// by R3: Hide's Stealth→attack flow crosses a turn boundary, so a condition
// that doesn't survive ToJSON→LoadJSON would silently evaporate before the
// attack it's supposed to affect.
func (s *HiddenConditionTestSuite) TestLoaderRoundTrip() {
	original := NewHiddenCondition(s.characterID)
	data, err := original.ToJSON()
	s.Require().NoError(err)

	loaded, err := LoadJSON(data)
	s.Require().NoError(err)

	hidden, ok := loaded.(*HiddenCondition)
	s.Require().True(ok, "loaded condition should be a *HiddenCondition")
	s.Equal(s.characterID, hidden.MemberID)
	s.False(hidden.IsApplied(), "a freshly loaded condition is not yet applied to a bus")

	// Reconstitute it onto a bus and confirm it still subscribes and functions —
	// this is the "still functioning after reload" half of R3's round trip.
	err = hidden.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.True(hidden.IsApplied())

	finalEvent := s.runAttackChain(dnd5eEvents.AttackChainEvent{
		AttackerID: "goblin-1",
		TargetID:   s.characterID,
	})
	s.Require().Len(finalEvent.DisadvantageSources, 1, "reloaded condition still grants disadvantage to attackers")
}

// TestFactoryRoundTrip proves HiddenCondition is registered in factory.go.
func (s *HiddenConditionTestSuite) TestFactoryRoundTrip() {
	output, err := CreateFromRef(&CreateFromRefInput{
		Ref:      refs.Conditions.Hidden().String(),
		MemberID: s.characterID,
	})
	s.Require().NoError(err)
	s.Require().NotNil(output)

	hidden, ok := output.Condition.(*HiddenCondition)
	s.Require().True(ok)
	s.Equal(s.characterID, hidden.MemberID)
}
