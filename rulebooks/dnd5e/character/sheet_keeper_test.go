// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// keptCondition is a condition that does nothing but exist and serialize —
// enough to be applied to a sheet and found on it again.
type keptCondition struct {
	applied bool
}

func (c *keptCondition) Ref() *core.Ref { return refs.Conditions.Dodging() }

func (c *keptCondition) IsApplied() bool { return c.applied }

func (c *keptCondition) Apply(_ context.Context, _ events.EventBus) error {
	c.applied = true
	return nil
}

func (c *keptCondition) Remove(_ context.Context, _ events.EventBus) error {
	c.applied = false
	return nil
}

func (c *keptCondition) ToJSON() (json.RawMessage, error) {
	return json.Marshal(map[string]any{"ref": refs.Conditions.Dodging()})
}

// SheetKeeperTestSuite drives each of the three things a sheet does about the
// world through a real bus. Every test here fails outright for a keeper whose
// Apply subscribes nothing, which is the mutation this file exists to catch:
// the wiring it replaced was invisible, so nothing would have noticed its
// absence except behaviour that quietly stopped happening.
type SheetKeeperTestSuite struct {
	suite.Suite

	ctx  context.Context
	bus  events.EventBus
	char *Character
}

// HealingAppliedTestSuite isolates the applied-fact contract so the brief's
// focused RED/GREEN command selects these tests directly.
type HealingAppliedTestSuite struct {
	suite.Suite

	ctx  context.Context
	bus  events.EventBus
	char *Character
}

func (s *SheetKeeperTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()

	char, err := Load(s.ctx, s.sheet())
	s.Require().NoError(err)
	s.Require().NoError(Attach(s.ctx, char, s.bus))

	s.char = char
}

func (s *SheetKeeperTestSuite) sheet() *Data {
	return keeperSheet()
}

func (s *HealingAppliedTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()

	char, err := Load(s.ctx, keeperSheet())
	s.Require().NoError(err)
	s.Require().NoError(Attach(s.ctx, char, s.bus))

	s.char = char
}

func keeperSheet() *Data {
	return &Data{
		ID:               "keeper-char",
		PlayerID:         "keeper-player",
		Name:             "Kept",
		Level:            3,
		ProficiencyBonus: 2,
		RaceID:           races.Human,
		ClassID:          classes.Barbarian,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14,
			abilities.CON: 15,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 8,
		},
		HitPoints:    10,
		MaxHitPoints: 30,
		ArmorClass:   14,
		Resources: map[coreResources.ResourceKey]RecoverableResourceData{
			resources.RageCharges: {Current: 1, Maximum: 3, ResetType: coreResources.ResetLongRest},
		},
	}
}

// A condition applied to this character lands on its sheet — and the sheet goes
// dirty, because ToData serializes conditions and only dirty sheets get written
// back.
func (s *SheetKeeperTestSuite) TestConditionAppliedLandsOnTheSheet() {
	condition := &keptCondition{}

	err := dnd5eEvents.ConditionAppliedTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.ConditionAppliedEvent{
		Target:    s.char,
		Type:      dnd5eEvents.ConditionDodging,
		Condition: condition,
	})

	s.Require().NoError(err)
	s.Require().Len(s.char.GetConditions(), 1)
	s.Require().True(condition.applied, "the keeper applies the condition it was handed")
	s.Require().True(s.char.IsDirty(), "a sheet that gained a condition needs saving")
}

func (s *SheetKeeperTestSuite) TestConditionAppliedToSomeoneElseIsIgnored() {
	other, err := Load(s.ctx, s.sheet())
	s.Require().NoError(err)
	other.id = "someone-else"

	condition := &keptCondition{}
	err = dnd5eEvents.ConditionAppliedTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.ConditionAppliedEvent{
		Target:    other,
		Condition: condition,
	})

	s.Require().NoError(err)
	s.Require().Empty(s.char.GetConditions())
	s.Require().False(s.char.IsDirty())
}

func (s *SheetKeeperTestSuite) TestConditionRemovedLeavesTheSheet() {
	s.Require().NoError(dnd5eEvents.ConditionAppliedTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.ConditionAppliedEvent{
		Target:    s.char,
		Condition: &keptCondition{},
	}))
	markSaved(s.char)

	err := dnd5eEvents.ConditionRemovedTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.ConditionRemovedEvent{
		MemberID:     s.char.GetID(),
		ConditionRef: refs.Conditions.Dodging().String(),
		Reason:       "test",
	})

	s.Require().NoError(err)
	s.Require().Empty(s.char.GetConditions())
	s.Require().True(s.char.IsDirty(), "a sheet that lost a condition needs saving")
}

func (s *SheetKeeperTestSuite) TestHealingReceivedMovesHitPoints() {
	err := dnd5eEvents.HealingReceivedTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.HealingReceivedEvent{
		TargetID: s.char.GetID(),
		Amount:   5,
		Source:   "test",
	})

	s.Require().NoError(err)
	s.Require().Equal(15, s.char.GetHitPoints())
	s.Require().True(s.char.IsDirty())
}

func (s *HealingAppliedTestSuite) TestHealingAppliedReportsPostClampFacts() {
	s.char.hitPoints = 8
	s.char.maxHitPoints = 10
	markSaved(s.char)

	source := *refs.Features.SecondWind()
	var got *dnd5eEvents.HealingAppliedEvent
	_, err := dnd5eEvents.HealingAppliedTopic.On(s.bus).Subscribe(
		s.ctx, func(_ context.Context, event dnd5eEvents.HealingAppliedEvent) error {
			got = &event
			return nil
		})
	s.Require().NoError(err)

	err = dnd5eEvents.HealingReceivedTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.HealingReceivedEvent{
		TargetID:   s.char.GetID(),
		Amount:     7,
		Roll:       6,
		Modifier:   1,
		SourceRef:  &source,
		SourceName: "Second Wind",
	})

	s.Require().NoError(err)
	s.Require().NotNil(got)
	s.Require().Equal(s.char.GetID(), got.TargetID)
	s.Require().Equal(7, got.Requested)
	s.Require().Equal(2, got.Applied)
	s.Require().Equal(8, got.HPBefore)
	s.Require().Equal(10, got.HPAfter)
	s.Require().Equal(6, got.Roll)
	s.Require().Equal(1, got.Modifier)
	s.Require().True(got.SourceRef.Equals(refs.Features.SecondWind()))
	s.Require().Equal("Second Wind", got.SourceName)
	s.Require().NotSame(&source, got.SourceRef, "the applied fact owns its source identity")

	source.ID = "mutated_by_caller"
	s.Require().True(got.SourceRef.Equals(refs.Features.SecondWind()),
		"mutating the request after publication cannot rewrite the applied fact")
	s.Require().True(s.char.IsDirty())
}

func (s *HealingAppliedTestSuite) TestHealingAppliedAtMaximumReportsZeroApplied() {
	s.char.hitPoints = 30
	markSaved(s.char)

	var got *dnd5eEvents.HealingAppliedEvent
	_, err := dnd5eEvents.HealingAppliedTopic.On(s.bus).Subscribe(
		s.ctx, func(_ context.Context, event dnd5eEvents.HealingAppliedEvent) error {
			got = &event
			return nil
		})
	s.Require().NoError(err)

	err = dnd5eEvents.HealingReceivedTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.HealingReceivedEvent{
		TargetID: s.char.GetID(),
		Amount:   7,
		Roll:     6,
		Modifier: 1,
	})

	s.Require().NoError(err)
	s.Require().NotNil(got)
	s.Require().Equal(7, got.Requested)
	s.Require().Zero(got.Applied)
	s.Require().Equal(30, got.HPBefore)
	s.Require().Equal(30, got.HPAfter)
	s.Require().Equal(6, got.Roll)
	s.Require().Equal(1, got.Modifier)
	s.Require().True(s.char.IsDirty())
}

func (s *HealingAppliedTestSuite) TestHealingAppliedForSomeoneElseIsIgnored() {
	markSaved(s.char)

	published := 0
	_, err := dnd5eEvents.HealingAppliedTopic.On(s.bus).Subscribe(
		s.ctx, func(_ context.Context, _ dnd5eEvents.HealingAppliedEvent) error {
			published++
			return nil
		})
	s.Require().NoError(err)

	err = dnd5eEvents.HealingReceivedTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.HealingReceivedEvent{
		TargetID: "someone-else",
		Amount:   7,
	})

	s.Require().NoError(err)
	s.Require().Zero(published)
	s.Require().Equal(10, s.char.GetHitPoints())
	s.Require().False(s.char.IsDirty())
}

func (s *HealingAppliedTestSuite) TestHealingAppliedSubscriberErrorPropagatesAfterMutation() {
	s.char.hitPoints = 8
	s.char.maxHitPoints = 10
	markSaved(s.char)

	sentinel := errors.New("healing applied subscriber failed")
	_, err := dnd5eEvents.HealingAppliedTopic.On(s.bus).Subscribe(
		s.ctx, func(_ context.Context, _ dnd5eEvents.HealingAppliedEvent) error {
			return sentinel
		})
	s.Require().NoError(err)

	err = dnd5eEvents.HealingReceivedTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.HealingReceivedEvent{
		TargetID: s.char.GetID(),
		Amount:   7,
	})

	s.Require().ErrorIs(err, sentinel)
	s.Require().Equal(10, s.char.GetHitPoints(), "the mutation precedes applied-fact publication")
	s.Require().True(s.char.IsDirty())
}

func (s *SheetKeeperTestSuite) TestHealingIsCappedAtMaximum() {
	err := dnd5eEvents.HealingReceivedTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.HealingReceivedEvent{
		TargetID: s.char.GetID(),
		Amount:   500,
	})

	s.Require().NoError(err)
	s.Require().Equal(30, s.char.GetHitPoints())
}

// Character-owned recoverable resources belong to the Character rest verbs,
// not RestTopic. A naked event cannot recover the same pool a second time;
// LongRest remains the positive owner proof.
func (s *SheetKeeperTestSuite) TestCharacterResourcesRecoverOnlyThroughCharacterRest() {
	s.Require().Equal(1, s.char.GetResource(resources.RageCharges).Current())

	err := dnd5eEvents.RestTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.RestEvent{
		RestType:    coreResources.ResetLongRest,
		CharacterID: s.char.GetID(),
	})
	s.Require().NoError(err)
	s.Require().Equal(1, s.char.GetResource(resources.RageCharges).Current(),
		"a naked RestEvent does not own character pools")

	s.Require().NoError(s.char.LongRest(s.ctx))
	s.Require().Equal(3, s.char.GetResource(resources.RageCharges).Current(),
		"the Character rest verb owns pool recovery")
}

// Remove gives the bus back: nothing the keeper subscribed is still listening,
// and the sheet is not left claiming subscriptions that no longer exist.
func (s *SheetKeeperTestSuite) TestRemoveStopsTheSheetListening() {
	s.Require().NoError(s.char.SheetKeeper().Remove(s.ctx, s.bus))

	err := dnd5eEvents.HealingReceivedTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.HealingReceivedEvent{
		TargetID: s.char.GetID(),
		Amount:   5,
	})

	s.Require().NoError(err)
	s.Require().Equal(10, s.char.GetHitPoints(), "the sheet is no longer listening")
	s.Require().Empty(s.char.subscriptionIDs, "and no longer claims to be")
}

// A keeper is the character's own, not a fresh one per call: two callers asking
// cannot subscribe the same sheet twice between them.
func (s *SheetKeeperTestSuite) TestKeeperIsTheCharactersOwn() {
	s.Require().Same(s.char.SheetKeeper(), s.char.SheetKeeper())
}

// A condition reporting a change to its OWN persisted state marks the sheet.
// The condition stores its turn-scoped memory in its own fields, those fields
// serialize as part of this character, and nothing else can see them move — so
// without this row the update is written perfectly and then discarded, because
// resolution keeps only the sheets reporting IsDirty.
func (s *SheetKeeperTestSuite) TestConditionStateChangedMarksTheSheet() {
	markSaved(s.char)

	err := dnd5eEvents.ConditionStateChangedTopic.On(s.bus).Publish(
		s.ctx, dnd5eEvents.ConditionStateChangedEvent{
			MemberID:     s.char.GetID(),
			ConditionRef: refs.Conditions.Raging(),
		})

	s.Require().NoError(err)
	s.Require().True(s.char.IsDirty(), "a condition's own state is this sheet's state")
}

func (s *SheetKeeperTestSuite) TestConditionStateChangedForSomeoneElseIsIgnored() {
	markSaved(s.char)

	err := dnd5eEvents.ConditionStateChangedTopic.On(s.bus).Publish(
		s.ctx, dnd5eEvents.ConditionStateChangedEvent{
			MemberID:     "someone-else",
			ConditionRef: refs.Conditions.Raging(),
		})

	s.Require().NoError(err)
	s.Require().False(s.char.IsDirty(), "every sheet hears it; only one of them is it")
}

// A spend request debits this character's economy, and does it AT THE PUBLISH:
// the bus is synchronous, so an effect that asks and then reads back sees the
// slot already gone, exactly as a direct SpendSlots call left it. That is what
// lets a rule publish where it used to write without any ordering analysis.
func (s *SheetKeeperTestSuite) TestSpendRequestedDebitsTheEconomy() {
	_, err := s.char.StartTurn(s.ctx, &StartTurnInput{TurnNumber: 1, Speed: 30})
	s.Require().NoError(err)
	s.Require().Equal(1, s.char.SlotsLeft(coreCombat.ActionReaction))
	markSaved(s.char)

	pubErr := dnd5eEvents.SpendRequestedTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.SpendRequestedEvent{
		MemberID:   s.char.GetID(),
		ActionType: coreCombat.ActionReaction,
		Amount:     1,
		SourceRef:  refs.Conditions.OpportunityAttack(),
	})

	s.Require().NoError(pubErr)
	s.Require().Equal(0, s.char.SlotsLeft(coreCombat.ActionReaction), "the slot is spent by the time Publish returns")
	s.Require().False(s.char.CanReact(), "and the reader the reacting rules gate on agrees")
	s.Require().True(s.char.IsDirty(), "SpendSlots marks the sheet itself; the debit IS the change")
}

func (s *SheetKeeperTestSuite) TestSpendRequestedForSomeoneElseIsIgnored() {
	_, err := s.char.StartTurn(s.ctx, &StartTurnInput{TurnNumber: 1, Speed: 30})
	s.Require().NoError(err)
	markSaved(s.char)

	pubErr := dnd5eEvents.SpendRequestedTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.SpendRequestedEvent{
		MemberID:   "someone-else",
		ActionType: coreCombat.ActionReaction,
		Amount:     1,
	})

	s.Require().NoError(pubErr)
	s.Require().Equal(1, s.char.SlotsLeft(coreCombat.ActionReaction), "somebody else's bill")
	s.Require().False(s.char.IsDirty())
}

// CanReact answers from this sheet's own economy, which is what makes it the
// question a rule can ask without holding a ledger. A sheet with no fight
// around it says no, and says it because there is no economy at all rather
// than because one refused.
func (s *SheetKeeperTestSuite) TestCanReactFollowsTheReactionSlot() {
	s.Require().False(s.char.InCombat(), "the fixture is a sheet outside a fight")
	s.Require().False(s.char.CanReact(), "no economy, no reaction")

	_, err := s.char.StartTurn(s.ctx, &StartTurnInput{TurnNumber: 1, Speed: 30})
	s.Require().NoError(err)
	s.Require().True(s.char.CanReact(), "a fresh turn grants one")

	s.char.SpendSlots(coreCombat.ActionReaction, 1)
	s.Require().False(s.char.CanReact(), "and spending it is what takes it away")
}

func TestSheetKeeperSuite(t *testing.T) {
	suite.Run(t, new(SheetKeeperTestSuite))
}

func TestHealingAppliedSuite(t *testing.T) {
	suite.Run(t, new(HealingAppliedTestSuite))
}

// namelessCondition breaks [dnd5eEvents.ConditionBehavior]'s contract: its
// Ref() is nil, so it can name itself in JSON but not when asked.
type namelessCondition struct{}

func (c *namelessCondition) Ref() *core.Ref { return nil }

func (c *namelessCondition) IsApplied() bool { return true }

func (c *namelessCondition) Apply(_ context.Context, _ events.EventBus) error { return nil }

func (c *namelessCondition) Remove(_ context.Context, _ events.EventBus) error { return nil }

func (c *namelessCondition) ToJSON() (json.RawMessage, error) {
	return json.Marshal(map[string]any{"ref": refs.Conditions.Dodging()})
}

// TestARefLessConditionIsRefusedAtTheDoor pins the admission check that lets
// every later reader stop asking.
//
// Removals are matched by ref, so a condition whose Ref() is nil could sit on
// the sheet unremovable while every removal aimed at it reported success. The
// keeper refuses it on the way IN instead — Kirk's ruling, "if we protect the
// construction, we don't need to worry about the nil" — which is why
// onConditionRemoved can call Ref().String() bare.
//
// What is pinned is the BUS path specifically: ConditionAppliedEvent.Condition
// is whatever a publisher put in it, so this is where an arbitrary
// implementation reaches a sheet. namelessCondition's ToJSON returns a valid
// ref, which no longer matters to either keeper — both stopped reading it in
// rpg-project#319 Phase 6 — and is kept only so the fake is well-formed in
// every respect except the one under test.
func (s *SheetKeeperTestSuite) TestARefLessConditionIsRefusedAtTheDoor() {
	err := dnd5eEvents.ConditionAppliedTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.ConditionAppliedEvent{
		Target:    s.char,
		Condition: &namelessCondition{},
	})

	s.Require().Error(err, "a condition that cannot name itself must not reach the sheet")
	s.Require().Empty(s.char.GetConditions(), "and nothing was admitted")
	s.Require().False(s.char.IsDirty(), "a refused application changes nothing to save")
}
