// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// ConcentrationKeeperSuite is rpg-project#407's hop 2: what a keeper owes a
// removal fact it did not publish, and what the caster answers about the spell
// it is holding.
type ConcentrationKeeperSuite struct {
	suite.Suite
	ctx context.Context
	bus events.EventBus
}

func TestConcentrationKeeperSuite(t *testing.T) {
	suite.Run(t, new(ConcentrationKeeperSuite))
}

func (s *ConcentrationKeeperSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

// bardData is a level-1 bard carrying the two conditions a True Strike cast
// leaves behind: the hold, and the advantage it owns.
func (s *ConcentrationKeeperSuite) bardData(carry ...dnd5eEvents.ConditionBehavior) *Data {
	blobs := make([]json.RawMessage, 0, len(carry))
	for _, condition := range carry {
		raw, err := condition.ToJSON()
		s.Require().NoError(err)
		blobs = append(blobs, raw)
	}

	return &Data{
		ID:               "bard-1",
		PlayerID:         "player-1",
		Name:             "Hop Two",
		Level:            1,
		ProficiencyBonus: 2,
		RaceID:           races.Human,
		ClassID:          classes.Bard,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 10,
			abilities.DEX: 14,
			abilities.CON: 12,
			abilities.INT: 10,
			abilities.WIS: 10,
			abilities.CHA: 16,
		},
		HitPoints:      9,
		MaxHitPoints:   9,
		ArmorClass:     12,
		EquipmentSlots: EquipmentSlots{},
		Conditions:     blobs,
	}
}

// THE HOP-2 REGRESSION, and it fails on the code this test shipped with.
//
// Until now every removal on the bus was published by a condition ending
// ITSELF, which calls its own Remove immediately after — so the keeper dropping
// the behavior from its slice without unsubscribing it was invisible.
// Concentration is the first thing a removal arrives for from OUTSIDE (the
// recast drop, R3), and a hold dropped from the list but left on the bus keeps
// answering damage with a check for a spell nobody is holding any more.
//
// The condition here is deliberately one that does NOT consume itself: a
// self-consuming condition would have ended before the fact arrived, and the
// assertion would pass against the very defect it is written to catch.
func (s *ConcentrationKeeperSuite) TestAPrunedConditionIsUnsubscribed() {
	hold := conditions.NewConcentratingCondition(
		"bard-1", refs.Spells.TrueStrike().String(), conditions.TrueStrikeName, 2)
	loaded, err := Load(s.ctx, s.bardData(hold))
	s.Require().NoError(err)
	s.Require().NoError(Attach(s.ctx, loaded, s.bus))

	s.Require().Len(s.damage("bard-1", 12).FollowUps, 1,
		"the fixture has to answer damage before removal, or this proves nothing")

	// Somebody ELSE publishes the fact. The hold did not end itself, so nothing
	// has called its Remove.
	s.Require().NoError(dnd5eEvents.ConditionRemovedTopic.On(s.bus).Publish(s.ctx,
		dnd5eEvents.ConditionRemovedEvent{
			MemberID:     "bard-1",
			ConditionRef: refs.Conditions.Concentrating().String(),
			Reason:       conditions.ConcentrationEndedRecast,
		}))

	s.Empty(loaded.GetConditions(), "dropped from the sheet")
	s.Empty(s.damage("bard-1", 12).FollowUps,
		"AND off the bus — a condition that keeps answering events was never really removed")
}

// The self-ending path is unchanged: a condition that already detached reports
// IsApplied false, so the keeper's Remove is a no-op rather than a second one.
func (s *ConcentrationKeeperSuite) TestASelfEndingConditionIsStillDroppedCleanly() {
	trueStrike := conditions.NewTrueStrikeCondition("bard-1", "goblin-1", "")
	loaded, err := Load(s.ctx, s.bardData(trueStrike))
	s.Require().NoError(err)
	s.Require().NoError(Attach(s.ctx, loaded, s.bus))

	// The attack consumes it, which is the condition publishing its own removal
	// and calling its own Remove before the keeper hears anything.
	s.Require().Len(s.attack("bard-1", "goblin-1").AdvantageSources, 1)

	s.Empty(loaded.GetConditions())
	s.Empty(s.attack("bard-1", "goblin-1").AdvantageSources)
}

func (s *ConcentrationKeeperSuite) TestACharacterHoldingNothingSaysSo() {
	loaded, err := Load(s.ctx, s.bardData())
	s.Require().NoError(err)

	_, holding := loaded.Concentration()

	s.False(holding)
}

func (s *ConcentrationKeeperSuite) TestACasterAnswersWhichSpellItIsHolding() {
	hold := conditions.NewConcentratingCondition(
		"bard-1", refs.Spells.TrueStrike().String(), conditions.TrueStrikeName, 2)
	s.Require().NoError(hold.AddChild(s.ctx, dnd5eEvents.ChildRef{
		MemberID:     "bard-1",
		ConditionRef: refs.Conditions.TrueStrike().String(),
	}))
	loaded, err := Load(s.ctx, s.bardData(hold))
	s.Require().NoError(err)

	view, holding := loaded.Concentration()

	s.Require().True(holding)
	s.Equal(refs.Spells.TrueStrike().String(), view.SpellRef)
	s.Equal(conditions.TrueStrikeName, view.SpellName)
	s.Equal(1, view.ChildCount)
}

// attack publishes one attack down the chain and returns the folded event.
func (s *ConcentrationKeeperSuite) attack(attackerID, targetID string) dnd5eEvents.AttackChainEvent {
	event := dnd5eEvents.AttackChainEvent{AttackerID: attackerID, TargetID: targetID}
	staged := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)

	modified, err := dnd5eEvents.AttackChain.On(s.bus).PublishWithChain(s.ctx, event, staged)
	s.Require().NoError(err)

	folded, err := modified.Execute(s.ctx, event)
	s.Require().NoError(err)
	return folded
}

// damage publishes one damage-taken fact and hands back what subscribers wrote.
func (s *ConcentrationKeeperSuite) damage(memberID string, amount int) *dnd5eEvents.DamageTakenEvent {
	event := &dnd5eEvents.DamageTakenEvent{MemberID: memberID, Amount: amount}
	s.Require().NoError(dnd5eEvents.DamageTakenTopic.On(s.bus).Publish(s.ctx, event))
	return event
}

// THE REAL KEEPER, and the invariant it kept breaking: a condition hears a
// fact addressed to it and acts on it BEFORE its keeper detaches it.
//
// Measured inside a real Resolve before it was fixed. Two order defects, both
// invisible on a bare bus with the condition alone, which is why the
// conditions-package tests passed:
//
//  1. The keeper subscribes to removals when the sheet attaches, and every
//     condition subscribes later — at attach for a persisted one, and at
//     ConditionAppliedTopic for one delivered mid-fight. So the keeper's
//     handler always runs first and detached the hold before the hold could
//     answer.
//  2. Detaching publishes. That re-enters this same keeper handler, and a
//     handler that assigned its pruned list AFTER detaching would overwrite
//     the nested assignment and put the child back on the sheet.
//
// Neither is about concentration. Concentration is just the first thing that
// answers a removal addressed to itself.
func (s *ConcentrationKeeperSuite) TestAHoldEndedByAFactStripsItsChildren() {
	for _, reason := range []string{
		conditions.ConcentrationEndedRecast,
		conditions.ConcentrationEndedDamage,
	} {
		s.Run(reason, func() {
			s.SetupTest()

			var ended []dnd5eEvents.ConcentrationEndedEvent
			_, err := dnd5eEvents.ConcentrationEndedTopic.On(s.bus).Subscribe(s.ctx,
				func(_ context.Context, event dnd5eEvents.ConcentrationEndedEvent) error {
					ended = append(ended, event)
					return nil
				})
			s.Require().NoError(err)

			child := dnd5eEvents.ChildRef{
				MemberID:     "bard-1",
				ConditionRef: refs.Conditions.TrueStrike().String(),
			}
			hold := conditions.NewConcentratingCondition(
				"bard-1", refs.Spells.TrueStrike().String(), conditions.TrueStrikeName, 2)
			trueStrike := conditions.NewTrueStrikeCondition("bard-1", "goblin-1", "")
			loaded, loadErr := Load(s.ctx, s.bardData(hold, trueStrike))
			s.Require().NoError(loadErr)
			s.Require().NoError(Attach(s.ctx, loaded, s.bus))

			var live *conditions.ConcentratingCondition
			for _, cond := range loaded.GetConditions() {
				if hold, isHold := cond.(*conditions.ConcentratingCondition); isHold {
					live = hold
				}
			}
			s.Require().NotNil(live)
			s.Require().NoError(live.AddChild(s.ctx, child))
			// Both are on the sheet and both are live. NOT asserted with an
			// attack: True Strike consumes itself on one, which would end the
			// hold as "last child left" and test the wrong path entirely.
			// TestASelfEndingConditionIsStillDroppedCleanly is where the
			// advantage itself is pinned.
			s.Require().Len(loaded.GetConditions(), 2)
			s.Require().True(live.IsApplied())

			// Exactly what resolution publishes: ONE removal, for the owner.
			s.Require().NoError(dnd5eEvents.ConditionRemovedTopic.On(s.bus).Publish(s.ctx,
				dnd5eEvents.ConditionRemovedEvent{
					MemberID:     "bard-1",
					ConditionRef: refs.Conditions.Concentrating().String(),
					Reason:       reason,
				}))

			s.Require().Len(ended, 1, "exactly one ended fact, carrying the reason")
			s.Equal(reason, ended[0].Reason)
			s.Equal([]dnd5eEvents.ChildRef{child}, ended[0].Removed)

			s.Empty(loaded.GetConditions(),
				"the hold AND its child are off the sheet — a nested publish must not resurrect the child")
			s.Empty(s.attack("bard-1", "goblin-1").AdvantageSources,
				"and the child is off the bus too")
			s.False(live.IsApplied(), "the hold itself is unsubscribed")
		})
	}
}
