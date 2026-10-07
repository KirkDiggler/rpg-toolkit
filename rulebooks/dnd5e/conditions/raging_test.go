// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// errorOnUnsubscribeBus wraps an EventBus to return errors for specific subscription IDs.
// Used to test that Remove() continues cleaning up after individual unsubscribe failures.
type errorOnUnsubscribeBus struct {
	events.EventBus
	failIDs map[string]bool
}

// Unsubscribe returns an error for IDs in failIDs, delegates to inner bus otherwise
func (b *errorOnUnsubscribeBus) Unsubscribe(ctx context.Context, id string) error {
	if b.failIDs[id] {
		return fmt.Errorf("subscription %s not found", id)
	}
	return b.EventBus.Unsubscribe(ctx, id)
}

// ragingConditionInput provides configuration for creating a raging condition
type ragingConditionInput struct {
	CharacterID string // ID of the raging character
	DamageBonus int    // Bonus damage for rage
	Level       int    // Barbarian level
	Source      string // Ref string in "module:type:value" format (e.g., "dnd5e:features:rage")
}

// newRagingCondition creates a raging condition from input
func newRagingCondition(input ragingConditionInput) *RagingCondition {
	return &RagingCondition{
		CharacterID: input.CharacterID,
		Source:      input.Source,
	}
}

// RagingConditionTestSuite tests the RagingCondition behavior
type RagingConditionTestSuite struct {
	suite.Suite
	ctx context.Context
	bus events.EventBus
}

func (s *RagingConditionTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

func TestRagingConditionTestSuite(t *testing.T) {
	suite.Run(t, new(RagingConditionTestSuite))
}

// TestRagingConditionTracksAttackAttempts is the regression test for
// rpg-toolkit#755: DidAttackThisTurn must be set from the post-attack-roll
// chain, which fires on both a hit and a miss -- not from the damage chain,
// which only fires on a hit and was silently dropping rage on a miss.
func (s *RagingConditionTestSuite) TestRagingConditionTracksAttackAttempts() {
	// Create a raging condition
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       5,
		Source:      "dnd5e:features:rage",
	})

	// Apply it to subscribe to events
	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Verify initial state
	s.False(raging.DidAttackThisTurn)

	s.Run("hit sets DidAttackThisTurn", func() {
		raging.DidAttackThisTurn = false
		err := s.executePostAttackRoll("barbarian-1", "goblin-1", true)
		s.Require().NoError(err)
		s.True(raging.DidAttackThisTurn)
	})

	s.Run("miss also sets DidAttackThisTurn -- RAW: an attempt counts", func() {
		raging.DidAttackThisTurn = false
		err := s.executePostAttackRoll("barbarian-1", "goblin-1", false)
		s.Require().NoError(err)
		s.True(raging.DidAttackThisTurn, "a missed attack attempt should still count as combat activity")
	})

	s.Run("other character's attack does not set DidAttackThisTurn", func() {
		raging.DidAttackThisTurn = false
		err := s.executePostAttackRoll("barbarian-2", "goblin-1", true)
		s.Require().NoError(err)
		s.False(raging.DidAttackThisTurn)
	})
}

// TestRagingConditionSustainsOnMissedAttack is the end-to-end regression test
// for rpg-toolkit#755, mirroring the rage-sweep playtest log exactly: a
// barbarian rages, attacks, MISSES, and ends their turn. RAW (PHB rage):
// rage ends early only if the character hasn't "attacked a hostile creature
// since your last turn or taken damage" -- a missed attack attempt still
// counts, so rage must sustain here.
func (s *RagingConditionTestSuite) TestRagingConditionSustainsOnMissedAttack() {
	// Create a raging condition
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       5,
		Source:      "dnd5e:features:rage",
	})

	// Apply it to subscribe to events
	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Track if condition removed event is published
	var removedEvent *dnd5eEvents.ConditionRemovedEvent
	removalTopic := dnd5eEvents.ConditionRemovedTopic.On(s.bus)
	_, err = removalTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
		removedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Barbarian attacks goblin-1 and MISSES (mirrors the playtest log: "MISS
	// (6+5 vs AC 15)").
	err = s.executePostAttackRoll("barbarian-1", "goblin-1", false)
	s.Require().NoError(err)

	// End the barbarian's turn.
	turnEndTopic := dnd5eEvents.TurnEndTopic.On(s.bus)
	err = turnEndTopic.Publish(s.ctx, dnd5eEvents.TurnEndEvent{
		SubjectID: "barbarian-1",
		Round:     1,
	})
	s.Require().NoError(err)

	// Rage must still be active -- a miss is still an attack attempt.
	s.Nil(removedEvent, "rage should sustain after a missed attack attempt")
	s.True(raging.IsApplied(), "rage should still be applied after a missed attack")
}

func (s *RagingConditionTestSuite) TestRagingConditionTracksDamage() {
	// Create a raging condition
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       5,
		Source:      "dnd5e:features:rage",
	})

	// Apply it to subscribe to events
	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Verify initial state
	s.False(raging.WasHitThisTurn)

	// Publish a damage event for this character
	damageTopic := dnd5eEvents.DamageReceivedTopic.On(s.bus)
	err = damageTopic.Publish(s.ctx, dnd5eEvents.DamageReceivedEvent{
		TargetID:   "barbarian-1",
		SourceID:   "goblin-1",
		Amount:     5,
		DamageType: damage.Slashing,
	})
	s.Require().NoError(err)

	// Check that the condition tracked being hit
	s.True(raging.WasHitThisTurn)
}

// TestTheTurnARageStartedIsNotChecked is the house rule, and it is deliberately
// the test that FAILS against RAW 2014.
//
// By the letter, a barbarian who rages and then ends their turn without
// swinging drops it immediately -- they have not attacked since their last turn
// and have not been hit. Kirk ruled against that 2026-08-27: "I cannot imagine
// activating rage would end at the end of the turn activated so i think it
// lasts 1 full turn."
//
// Kept as its own named test rather than folded into the one below, so that the
// divergence from the printed rule is something a reader trips over on purpose.
func (s *RagingConditionTestSuite) TestTheTurnARageStartedIsNotChecked() {
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       5,
		Source:      "dnd5e:features:rage",
	})
	s.Require().NoError(raging.Apply(s.ctx, s.bus))

	var removedEvent *dnd5eEvents.ConditionRemovedEvent
	_, err := dnd5eEvents.ConditionRemovedTopic.On(s.bus).Subscribe(s.ctx,
		func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
			removedEvent = &event
			return nil
		})
	s.Require().NoError(err)

	// The barbarian rages and does nothing else. Their turn ends.
	s.Require().NoError(dnd5eEvents.TurnEndTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.TurnEndEvent{
		SubjectID: "barbarian-1",
		Round:     4,
	}))

	s.Nil(removedEvent, "the turn a rage started is not checked -- RAW would have ended it here")
	s.Equal(4, raging.RoundActivated,
		"and that same turn end is what anchors the duration, from the clock's own round")
}

// TestRagingConditionEndsWithoutCombatActivity is the 2014 activity check
// itself, which the grace above delays by exactly one turn rather than removes.
func (s *RagingConditionTestSuite) TestRagingConditionEndsWithoutCombatActivity() {
	// Create a raging condition
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       5,
		Source:      "dnd5e:features:rage",
	})

	// Apply it to subscribe to events
	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Track if condition removed event is published
	var removedEvent *dnd5eEvents.ConditionRemovedEvent
	removalTopic := dnd5eEvents.ConditionRemovedTopic.On(s.bus)
	_, err = removalTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
		removedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	turnEndTopic := dnd5eEvents.TurnEndTopic.On(s.bus)

	// The activation turn: graced, and the anchor.
	err = turnEndTopic.Publish(s.ctx, dnd5eEvents.TurnEndEvent{SubjectID: "barbarian-1", Round: 1})
	s.Require().NoError(err)
	s.Require().Nil(removedEvent, "precondition: the first turn end is the graced one")

	// The next turn, equally quiet -- no attack, no damage taken. This one is
	// checked, and the grace does not extend to it.
	err = turnEndTopic.Publish(s.ctx, dnd5eEvents.TurnEndEvent{SubjectID: "barbarian-1", Round: 2})
	s.Require().NoError(err)

	// Verify condition published removal event
	s.Require().NotNil(removedEvent)
	s.Equal("barbarian-1", removedEvent.MemberID)
	s.Equal("dnd5e:conditions:raging", removedEvent.ConditionRef)
	s.Equal("no_combat_activity", removedEvent.Reason)
}

func (s *RagingConditionTestSuite) TestRagingConditionContinuesWithCombatActivity() {
	// Create a raging condition
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       5,
		Source:      "dnd5e:features:rage",
	})

	// Apply it to subscribe to events
	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Track if condition removed event is published
	var removedEvent *dnd5eEvents.ConditionRemovedEvent
	removalTopic := dnd5eEvents.ConditionRemovedTopic.On(s.bus)
	_, err = removalTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
		removedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Execute a post-attack-roll chain event (simulates an attack attempt --
	// combat activity, regardless of hit or miss)
	err = s.executePostAttackRoll("barbarian-1", "goblin-1", true)
	s.Require().NoError(err)

	// Publish turn end event
	turnEndTopic := dnd5eEvents.TurnEndTopic.On(s.bus)
	err = turnEndTopic.Publish(s.ctx, dnd5eEvents.TurnEndEvent{
		SubjectID: "barbarian-1",
		Round:     1,
	})
	s.Require().NoError(err)

	// Verify condition did NOT publish removal event
	s.Nil(removedEvent, "Rage should continue when there's combat activity")

	// Verify flags were reset for next turn
	s.False(raging.DidAttackThisTurn)
	s.False(raging.WasHitThisTurn)
	s.Equal(1, raging.TurnsActive)
}

func (s *RagingConditionTestSuite) TestRagingConditionEndsAfter10Rounds() {
	// Create a raging condition
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       5,
		Source:      "dnd5e:features:rage",
	})

	// Apply it to subscribe to events
	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Track if condition removed event is published
	var removedEvent *dnd5eEvents.ConditionRemovedEvent
	removalTopic := dnd5eEvents.ConditionRemovedTopic.On(s.bus)
	_, err = removalTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
		removedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	turnEndTopic := dnd5eEvents.TurnEndTopic.On(s.bus)

	// Simulate 10 rounds of combat with attack attempts
	for round := 1; round <= 10; round++ {
		// Execute a post-attack-roll chain event each round to keep rage active
		// (simulates an attack attempt, regardless of hit or miss)
		err = s.executePostAttackRoll("barbarian-1", "goblin-1", true)
		s.Require().NoError(err)

		// End turn
		err = turnEndTopic.Publish(s.ctx, dnd5eEvents.TurnEndEvent{
			SubjectID: "barbarian-1",
			Round:     round,
		})
		s.Require().NoError(err)

		if round < 10 {
			// Before round 10, rage should continue
			s.Nil(removedEvent, "Rage should continue until round 10")
		}
	}

	// After 10 rounds, rage should end
	s.Require().NotNil(removedEvent)
	s.Equal("barbarian-1", removedEvent.MemberID)
	s.Equal("dnd5e:conditions:raging", removedEvent.ConditionRef)
	s.Equal("duration_expired", removedEvent.Reason)
}

// executePostAttackRoll publishes a PostAttackRollEvent through
// PostAttackRollChain, simulating an attack roll (hit or miss) for the
// sustain-flag tests. Matches ResolveAttackHit's real usage of the topic
// (attack_phases.go): it calls PublishWithChain and discards the returned
// chain -- it never calls Execute. Subscribers that only inspect the event
// and don't call c.Add (like onPostAttackRoll here and Shield's handler) run
// their side effects during the publish itself, so Execute is unnecessary
// and would exercise a codepath production never runs.
func (s *RagingConditionTestSuite) executePostAttackRoll(
	attackerID, targetID string,
	wouldHit bool,
) error {
	postRollEvent := &dnd5eEvents.PostAttackRollEvent{
		AttackerID: attackerID,
		TargetID:   targetID,
		WouldHit:   wouldHit,
	}

	chain := events.NewStagedChain[*dnd5eEvents.PostAttackRollEvent](combat.ModifierStages)
	postRolls := dnd5eEvents.PostAttackRollChain.On(s.bus)

	_, err := postRolls.PublishWithChain(s.ctx, postRollEvent, chain)
	return err
}

// executeDamageChain creates a damage chain event and executes it through the damage chain topic.
// Returns the final event after all chain modifications have been applied.
// This helper reduces duplication in tests that verify damage bonus modifications.
//
//nolint:unparam // Parameters kept for consistency with other test helpers in this package
func (s *RagingConditionTestSuite) executeDamageChain(
	attackerID string,
	baseDamage, damageBonus int,
) (*dnd5eEvents.DamageChainEvent, error) {
	// Create weapon component with base damage
	weaponComp := dnd5eEvents.DamageComponent{
		Source:     dnd5eEvents.DamageSourceWeapon,
		Properties: []damage.Property{damage.AddsAttackAbilityModifier},
		Roll: dnd5eEvents.RollComponent{
			Source: dnd5eEvents.RollSource{Ref: refs.Weapons.Battleaxe(), Name: "Battleaxe"},
			Dice:   testDiceTrace(8, baseDamage),
		},
		DamageType: damage.Fire,
		IsCritical: false,
	}

	// Create ability component with damage bonus (STR modifier)
	abilityComp := dnd5eEvents.DamageComponent{
		Source: dnd5eEvents.DamageSourceAbility,
		Roll: dnd5eEvents.RollComponent{
			Source:   dnd5eEvents.RollSource{Ref: refs.Abilities.Strength(), Name: "Strength"},
			Modifier: intPtr(damageBonus),
		},
		DamageType: damage.Fire,
		IsCritical: false,
	}

	damageEvent := swungDamage(&dnd5eEvents.DamageChainEvent{
		AttackerID:       attackerID,
		TargetID:         "goblin-1",
		Components:       []dnd5eEvents.DamageComponent{weaponComp, abilityComp},
		WeaponDamageType: damage.Fire,
		IsCritical:       true,
	}, swing{ClassLevels: classLevels(classes.Barbarian, 1), AbilityUsed: abilities.STR, IsMelee: true})

	chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damageTopic := dnd5eEvents.DamageChain.On(s.bus)

	modifiedChain, err := damageTopic.PublishWithChain(s.ctx, withEventFrame(damageEvent), chain)
	if err != nil {
		return nil, err
	}

	return modifiedChain.Execute(s.ctx, damageEvent)
}

// executeDamageChainWithAbility creates and executes a damage chain event with an
// explicit ability/melee combination, for testing the rage damage bonus gate.
func (s *RagingConditionTestSuite) executeDamageChainWithAbility(
	attackerID string,
	abilityUsed abilities.Ability,
	isMelee bool,
) (*dnd5eEvents.DamageChainEvent, error) {
	weaponComp := dnd5eEvents.DamageComponent{
		Source:     dnd5eEvents.DamageSourceWeapon,
		Properties: []damage.Property{damage.AddsAttackAbilityModifier},
		Roll: dnd5eEvents.RollComponent{
			Source: dnd5eEvents.RollSource{Ref: refs.Weapons.Longsword(), Name: "Longsword"},
			Dice:   testDiceTrace(8, 5),
		},
		DamageType: damage.Slashing,
	}

	abilityComp := dnd5eEvents.DamageComponent{
		Source: dnd5eEvents.DamageSourceAbility,
		Roll: dnd5eEvents.RollComponent{
			Source:   dnd5eEvents.RollSource{Ref: refs.Abilities.Strength(), Name: "Strength"},
			Modifier: intPtr(3),
		},
		DamageType: damage.Slashing,
	}

	damageEvent := swungDamage(&dnd5eEvents.DamageChainEvent{
		AttackerID:       attackerID,
		TargetID:         "goblin-1",
		Components:       []dnd5eEvents.DamageComponent{weaponComp, abilityComp},
		WeaponDamageType: damage.Slashing,
	}, swing{ClassLevels: classLevels(classes.Barbarian, 1), AbilityUsed: abilityUsed, IsMelee: isMelee})

	chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damageTopic := dnd5eEvents.DamageChain.On(s.bus)

	modifiedChain, err := damageTopic.PublishWithChain(s.ctx, withEventFrame(damageEvent), chain)
	if err != nil {
		return nil, err
	}

	return modifiedChain.Execute(s.ctx, damageEvent)
}

func (s *RagingConditionTestSuite) TestRagingConditionUsesMarkedWeaponType() {
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       3,
		Source:      "dnd5e:features:rage",
	})
	s.Require().NoError(raging.Apply(s.ctx, s.bus))

	// The marked metadata is authoritative for inherited damage type. The
	// component deliberately disagrees so an implementation reading the
	// component instead of the event envelope fails this behavior test.
	damageEvent := swungDamage(&dnd5eEvents.DamageChainEvent{
		AttackerID:       "barbarian-1",
		TargetID:         "goblin-1",
		WeaponDamageType: damage.Fire,
		Components: []dnd5eEvents.DamageComponent{{
			Source:     dnd5eEvents.DamageSourceWeapon,
			Properties: []damage.Property{damage.AddsAttackAbilityModifier},
			Roll: dnd5eEvents.RollComponent{
				Source: dnd5eEvents.RollSource{Ref: refs.Weapons.Longsword(), Name: "Longsword"},
			},
			DamageType: damage.Slashing,
		}},
	}, swing{ClassLevels: classLevels(classes.Barbarian, 1), AbilityUsed: abilities.STR, IsMelee: true})
	chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	modified, err := dnd5eEvents.DamageChain.On(s.bus).PublishWithChain(s.ctx, withEventFrame(damageEvent), chain)
	s.Require().NoError(err)
	finalEvent, err := modified.Execute(s.ctx, damageEvent)
	s.Require().NoError(err)
	s.Require().Len(finalEvent.Components, 2)
	s.Equal(damage.Fire, finalEvent.Components[1].DamageType)
}

func (s *RagingConditionTestSuite) TestRagingConditionDamageBonusRequiresSTRMelee() {
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       3,
		Source:      "dnd5e:features:rage",
	})

	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	testCases := []struct {
		name        string
		abilityUsed abilities.Ability
		isMelee     bool
	}{
		{"DEX melee attack (finesse weapon)", abilities.DEX, true},
		{"STR ranged attack (thrown weapon)", abilities.STR, false},
		{"DEX ranged attack", abilities.DEX, false},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			finalEvent, err := s.executeDamageChainWithAbility("barbarian-1", tc.abilityUsed, tc.isMelee)
			s.Require().NoError(err)
			s.Require().Len(finalEvent.Components, 2, "no rage damage bonus should be added")
		})
	}
}

func (s *RagingConditionTestSuite) TestRagingConditionDamageBonusAppliesToSTRMelee() {
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       3,
		Source:      "dnd5e:features:rage",
	})

	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	finalEvent, err := s.executeDamageChainWithAbility("barbarian-1", abilities.STR, true)
	s.Require().NoError(err)

	s.Require().Len(finalEvent.Components, 3, "rage damage bonus should be added for STR melee attacks")
	s.Equal(dnd5eEvents.DamageSourceCondition, finalEvent.Components[2].Source)
	s.Equal(2, finalEvent.Components[2].Total())
	s.Equal(damage.Slashing, finalEvent.Components[2].DamageType)
}

func (s *RagingConditionTestSuite) TestRagingConditionAddsDamageBonus() {
	// Create a level 3 raging condition (+2 damage)
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       3,
		Source:      "dnd5e:features:rage",
	})

	// Apply it to subscribe to damage chain
	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Execute damage chain for the raging barbarian
	finalEvent, err := s.executeDamageChain("barbarian-1", 5, 3)
	s.Require().NoError(err)

	// Verify rage damage component was added
	s.Require().Len(finalEvent.Components, 3, "Should have weapon, ability, and rage components")

	// Verify weapon component
	s.Equal(dnd5eEvents.DamageSourceWeapon, finalEvent.Components[0].Source)
	s.Equal(5, finalEvent.Components[0].Total())

	// Verify ability component
	s.Equal(dnd5eEvents.DamageSourceAbility, finalEvent.Components[1].Source)
	s.Equal(3, finalEvent.Components[1].Total())

	// Verify rage component was added
	s.Equal(dnd5eEvents.DamageSourceCondition, finalEvent.Components[2].Source)
	s.Equal(2, finalEvent.Components[2].Total(), "Rage should add +2 damage")
	s.Equal(2, finalEvent.Components[2].Total())
	s.Equal(damage.Fire, finalEvent.Components[2].DamageType)
	s.False(finalEvent.Components[2].IsCritical, "flat rage damage is not doubled")

	// Verify total damage
	totalDamage := 0
	for _, comp := range finalEvent.Components {
		totalDamage += comp.Total()
	}
	s.Equal(10, totalDamage, "Total should be 5 (weapon) + 3 (ability) + 2 (rage)")
}

func (s *RagingConditionTestSuite) TestRagingConditionOnlyAffectsOwnAttacks() {
	// Create a raging condition for barbarian-1
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       3,
		Source:      "dnd5e:features:rage",
	})

	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Execute damage chain for a DIFFERENT attacker
	finalEvent, err := s.executeDamageChain("barbarian-2", 5, 3)
	s.Require().NoError(err)

	// Verify NO rage component was added (only weapon and ability)
	s.Require().Len(finalEvent.Components, 2, "Should only have weapon and ability components, no rage")

	// Verify weapon component
	s.Equal(dnd5eEvents.DamageSourceWeapon, finalEvent.Components[0].Source)
	s.Equal(5, finalEvent.Components[0].Total())

	// Verify ability component
	s.Equal(dnd5eEvents.DamageSourceAbility, finalEvent.Components[1].Source)
	s.Equal(3, finalEvent.Components[1].Total())

	// Verify total damage (no rage bonus)
	totalDamage := 0
	for _, comp := range finalEvent.Components {
		totalDamage += comp.Total()
	}
	s.Equal(8, totalDamage, "Total should be 5 (weapon) + 3 (ability), no rage for other character")
}

func (s *RagingConditionTestSuite) TestRagingConditionRejectsDoubleApply() {
	// Create a raging condition
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       5,
		Source:      "dnd5e:features:rage",
	})

	// Apply it once - should succeed
	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Apply it again - should fail
	err = raging.Apply(s.ctx, s.bus)
	s.Require().Error(err)
	s.Contains(err.Error(), "already applied")
}

func (s *RagingConditionTestSuite) TestRagingConditionEndsOnRest() {
	// Create a raging condition
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       5,
		Source:      "dnd5e:features:rage",
	})

	// Apply it to subscribe to events
	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.True(raging.IsApplied(), "rage should be applied")

	// Track if condition removed event is published
	var removedEvent *dnd5eEvents.ConditionRemovedEvent
	removalTopic := dnd5eEvents.ConditionRemovedTopic.On(s.bus)
	_, err = removalTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
		removedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Publish a rest event for this character
	restTopic := dnd5eEvents.RestTopic.On(s.bus)
	err = restTopic.Publish(s.ctx, dnd5eEvents.RestEvent{
		RestType:    coreResources.ResetLongRest,
		CharacterID: "barbarian-1",
	})
	s.Require().NoError(err)

	// Verify condition published removal event
	s.Require().NotNil(removedEvent, "rage should be removed on rest")
	s.Equal("barbarian-1", removedEvent.MemberID)
	s.Equal("dnd5e:conditions:raging", removedEvent.ConditionRef)
	s.Equal("rest", removedEvent.Reason)

	// Verify condition is no longer applied
	s.False(raging.IsApplied(), "rage should no longer be applied after rest")
}

func (s *RagingConditionTestSuite) TestRagingConditionIgnoresOtherCharacterRest() {
	// Create a raging condition for barbarian-1
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       5,
		Source:      "dnd5e:features:rage",
	})

	// Apply it to subscribe to events
	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.True(raging.IsApplied(), "rage should be applied")

	// Track if condition removed event is published
	var removedEvent *dnd5eEvents.ConditionRemovedEvent
	removalTopic := dnd5eEvents.ConditionRemovedTopic.On(s.bus)
	_, err = removalTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
		removedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Publish a rest event for a DIFFERENT character
	restTopic := dnd5eEvents.RestTopic.On(s.bus)
	err = restTopic.Publish(s.ctx, dnd5eEvents.RestEvent{
		RestType:    coreResources.ResetLongRest,
		CharacterID: "barbarian-2", // Different character
	})
	s.Require().NoError(err)

	// Verify condition did NOT publish removal event
	s.Nil(removedEvent, "rage should NOT be removed when another character rests")

	// Verify condition is still applied
	s.True(raging.IsApplied(), "rage should still be applied")
}

// TestRagingConditionEndsOnCombatEnd is the regression test for
// rpg-toolkit#752: rage must end when combat ends, the same way it already
// ends on a rest, so it can never survive into a persisted character's next
// encounter.
func (s *RagingConditionTestSuite) TestRagingConditionEndsOnCombatEnd() {
	// Create a raging condition
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       5,
		Source:      "dnd5e:features:rage",
	})

	// Apply it to subscribe to events
	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.True(raging.IsApplied(), "rage should be applied")

	// Track if condition removed event is published
	var removedEvent *dnd5eEvents.ConditionRemovedEvent
	removalTopic := dnd5eEvents.ConditionRemovedTopic.On(s.bus)
	_, err = removalTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
		removedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Publish a combat-end event for this character (no attack, no hit,
	// no rest — just the encounter ending, e.g. the killing blow that ended
	// the fight).
	//
	// PUBLISHED BY HAND, and worth being honest about: until rpg-project#295
	// nothing in production published this topic at all, so this test was green
	// against a publisher that did not exist. It proves the handler, which is
	// all a unit test here can prove. What proves the CHAIN is in the session
	// package, driven through a real Dissolve and read out of the persisted
	// sheet — see rpg-project's ideas/session-combat/combat-end/design.md §6.
	// (That doc lives in the rpg-project repo, not this one.)
	combatEndTopic := dnd5eEvents.CombatEndTopic.On(s.bus)
	err = combatEndTopic.Publish(s.ctx, dnd5eEvents.CombatEndEvent{
		SubjectID: "barbarian-1",
	})
	s.Require().NoError(err)

	// Verify condition published removal event
	s.Require().NotNil(removedEvent, "rage should be removed when combat ends")
	s.Equal("barbarian-1", removedEvent.MemberID)
	s.Equal("dnd5e:conditions:raging", removedEvent.ConditionRef)
	s.Equal("combat_ended", removedEvent.Reason)

	// Verify condition is no longer applied
	s.False(raging.IsApplied(), "rage should no longer be applied after combat ends")
}

// TestRagingConditionIgnoresOtherCharacterCombatEnd mirrors
// TestRagingConditionIgnoresOtherCharacterRest: CombatEndEvent is published
// per-character (the encounter sweeps every held player), so one character's
// combat-end must not remove another's rage.
func (s *RagingConditionTestSuite) TestRagingConditionIgnoresOtherCharacterCombatEnd() {
	// Create a raging condition for barbarian-1
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       5,
		Source:      "dnd5e:features:rage",
	})

	// Apply it to subscribe to events
	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.True(raging.IsApplied(), "rage should be applied")

	// Track if condition removed event is published
	var removedEvent *dnd5eEvents.ConditionRemovedEvent
	removalTopic := dnd5eEvents.ConditionRemovedTopic.On(s.bus)
	_, err = removalTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
		removedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Publish a combat-end event for a DIFFERENT character.
	//
	// This is the test the whole shape of combat end rests on: because the
	// handler answers "is this about me?" by comparing subjects, a fight's
	// ending has to be announced ONCE PER MEMBER. A single subject-less event
	// would match nobody and expire nothing — rpg-project's
	// ideas/session-combat/combat-end/design.md §1.1.
	combatEndTopic := dnd5eEvents.CombatEndTopic.On(s.bus)
	err = combatEndTopic.Publish(s.ctx, dnd5eEvents.CombatEndEvent{
		SubjectID: "barbarian-2", // Different character
	})
	s.Require().NoError(err)

	// Verify condition did NOT publish removal event
	s.Nil(removedEvent, "rage should NOT be removed when another character's combat ends")

	// Verify condition is still applied
	s.True(raging.IsApplied(), "rage should still be applied")
}

// strikeAgainst runs both folds of one swing from attackerID at targetID, the
// way the target step does: the dealt fold first, then the incoming fold over
// what it dealt. It returns the dealt fold's event and what the target
// received.
func (s *RagingConditionTestSuite) strikeAgainst(
	attackerID, targetID string,
	baseDamage int,
	damageType damage.Type,
) (*dnd5eEvents.DamageChainEvent, *received) {
	weaponComp := dnd5eEvents.DamageComponent{
		Source:     dnd5eEvents.DamageSourceWeapon,
		Properties: []damage.Property{damage.AddsAttackAbilityModifier},
		Roll: dnd5eEvents.RollComponent{
			Source: dnd5eEvents.RollSource{Ref: refs.Weapons.Longsword(), Name: "Longsword"},
			Dice:   testDiceTrace(6, baseDamage),
		},
		DamageType: damageType,
	}

	damageEvent := withEventFrame(swungDamage(&dnd5eEvents.DamageChainEvent{
		AttackerID: attackerID,
		TargetID:   targetID,
		Components: []dnd5eEvents.DamageComponent{weaponComp},
	}, swing{ClassLevels: classLevels(classes.Barbarian, 1), AbilityUsed: abilities.STR}))

	dealt, err := foldDealt(s.ctx, s.bus, damageEvent)
	s.Require().NoError(err)
	got, err := foldIncoming(s.ctx, s.bus, damageEvent.Frame, dealt.Components)
	s.Require().NoError(err)
	return dealt, got
}

// THE SPLIT. A strike against a raging barbarian folds no resistance on the
// dealt fold; the resistance appears only on the incoming fold, as the
// target's own answer, and the settlement halves the type.
func (s *RagingConditionTestSuite) TestRagingConditionAppliesResistanceToPhysicalDamage() {
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       5,
		Source:      "dnd5e:features:rage",
	})
	s.Require().NoError(raging.Apply(s.ctx, s.bus))

	for _, dmgType := range []damage.Type{damage.Bludgeoning, damage.Piercing, damage.Slashing} {
		s.Run(string(dmgType), func() {
			dealt, got := s.strikeAgainst("goblin-1", "barbarian-1", 10, dmgType)

			for _, component := range dealt.Components {
				s.Nil(component.Multiplier, "the dealt fold carries no target answer")
				if ref := component.Roll.Source.Ref; ref != nil {
					s.NotEqual(refs.Conditions.Raging().String(), ref.String(),
						"the target's Rage adds nothing to the goblin's dealt damage")
				}
			}

			s.Equal([]dnd5eEvents.DamageMultiplier{{
				Category:   dnd5eEvents.DamageSourceCondition,
				Source:     dnd5eEvents.RollSource{Ref: refs.Conditions.Raging(), Name: "Raging"},
				DamageType: dmgType,
				Factor:     dnd5eEvents.DamageFactorResistance,
			}}, got.Folded.Multipliers, "the resistance is the target's answer on the incoming fold")
			s.Equal(5, got.taken(), "10 resisted lands as 5")
		})
	}
}

// One resistance per physical type dealt, none for the fire beside it.
func (s *RagingConditionTestSuite) TestRagingConditionResistanceUsesComponentTypes() {
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       5,
		Source:      "dnd5e:features:rage",
	})
	s.Require().NoError(raging.Apply(s.ctx, s.bus))

	got, err := foldIncoming(s.ctx, s.bus, testAttackFrame("goblin-1", "barbarian-1"), []dnd5eEvents.DamageComponent{
		{
			Source: dnd5eEvents.DamageSourceWeapon,
			Roll: dnd5eEvents.RollComponent{
				Source: dnd5eEvents.RollSource{Ref: refs.Weapons.Longsword(), Name: "Longsword"},
				Dice:   testDiceTrace(8, 8),
			},
			DamageType: damage.Slashing,
		},
		{
			Source: dnd5eEvents.DamageSourceFeature,
			Roll: dnd5eEvents.RollComponent{
				// Test-owned fire feature: the catalog has no real feature
				// to borrow for synthetic fire damage.
				Source: dnd5eEvents.RollSource{
					Ref:  &core.Ref{Module: "dnd5e", Type: "features", ID: "fire_damage"},
					Name: "Fire Damage",
				},
				Dice: testDiceTrace(7, 7),
			},
			DamageType: damage.Fire,
		},
	})
	s.Require().NoError(err)

	s.Require().Len(got.Folded.Multipliers, 1)
	s.Equal(damage.Slashing, got.Folded.Multipliers[0].DamageType)
	s.Equal(4+7, got.taken(), "slashing 8 halves to 4; fire 7 lands whole")
}

func (s *RagingConditionTestSuite) TestRagingConditionDoesNotResistNonPhysicalDamage() {
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       5,
		Source:      "dnd5e:features:rage",
	})
	s.Require().NoError(raging.Apply(s.ctx, s.bus))

	nonPhysicalTypes := []damage.Type{
		damage.Fire, damage.Cold, damage.Lightning, damage.Thunder,
		damage.Acid, damage.Poison, damage.Necrotic, damage.Radiant,
		damage.Force, damage.Psychic,
	}

	for _, dmgType := range nonPhysicalTypes {
		s.Run(string(dmgType), func() {
			_, got := s.strikeAgainst("goblin-1", "barbarian-1", 10, dmgType)
			s.Empty(got.Folded.Multipliers, "Rage resists only bludgeoning, piercing and slashing")
			s.Equal(10, got.taken())
		})
	}
}

func (s *RagingConditionTestSuite) TestRagingConditionResistanceOnlyAffectsOwnCharacter() {
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       5,
		Source:      "dnd5e:features:rage",
	})
	s.Require().NoError(raging.Apply(s.ctx, s.bus))

	_, got := s.strikeAgainst("goblin-1", "barbarian-2", 10, damage.Slashing)
	s.Empty(got.Folded.Multipliers, "Rage answers for its own holder only")
	s.Equal(10, got.taken())
}

// A raging barbarian's own Rage bonus folds on the dealt fold, and the same
// Rage answers nothing on its target's incoming fold.
func (s *RagingConditionTestSuite) TestARagingAttackersBonusFoldsOnTheDealtFoldOnly() {
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1", DamageBonus: 2, Level: 5, Source: "dnd5e:features:rage",
	})
	s.Require().NoError(raging.Apply(s.ctx, s.bus))

	event := swungDamage(&dnd5eEvents.DamageChainEvent{
		AttackerID:       "barbarian-1",
		TargetID:         "goblin-1",
		WeaponDamageType: damage.Slashing,
		Components: []dnd5eEvents.DamageComponent{{
			Source:     dnd5eEvents.DamageSourceWeapon,
			Properties: []damage.Property{damage.AddsAttackAbilityModifier},
			Roll: dnd5eEvents.RollComponent{
				Source: dnd5eEvents.RollSource{Ref: refs.Weapons.Greataxe(), Name: "Greataxe"},
				Dice:   testDiceTrace(12, 7),
			},
			DamageType: damage.Slashing,
		}},
	}, swing{
		WeaponRef: refs.Weapons.Greataxe(), IsMelee: true, AbilityUsed: abilities.STR,
		ClassLevels: classLevels(classes.Barbarian, 5),
	})
	dealt, err := foldDealt(s.ctx, s.bus, event)
	s.Require().NoError(err)

	var bonus int
	for _, component := range dealt.Components {
		if ref := component.Roll.Source.Ref; ref != nil && ref.String() == refs.Conditions.Raging().String() {
			bonus += component.Total()
		}
	}
	s.Equal(2, bonus, "Rage's +2 rides the dealt fold")

	got, err := foldIncoming(s.ctx, s.bus, event.Frame, dealt.Components)
	s.Require().NoError(err)
	s.Empty(got.Folded.Multipliers, "the goblin is not raging, and the attacker's Rage answers nothing for it")
	s.Equal(9, got.taken())
}

func (s *RagingConditionTestSuite) TestRemoveContinuesOnStaleSubscription() {
	// Apply a raging condition; it subscribes to every topic it answers on.
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       5,
		Source:      "dnd5e:features:rage",
	})

	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.Require().NotEmpty(raging.subscriptionIDs)
	total := len(raging.subscriptionIDs)

	// Wrap the bus so that the first subscription ID fails on unsubscribe
	failBus := &errorOnUnsubscribeBus{
		EventBus: s.bus,
		failIDs:  map[string]bool{raging.subscriptionIDs[0]: true},
	}

	// Remove should return an error but still clean up all other subscriptions
	err = raging.Remove(s.ctx, failBus)
	s.Require().Error(err, "Remove should report the failed unsubscribe")
	s.Contains(err.Error(), fmt.Sprintf("1/%d", total), "error should report count of failures vs total")

	// Condition should be fully cleaned up despite the error
	s.Nil(raging.subscriptionIDs, "subscriptionIDs should be nil after Remove")
	s.Nil(raging.bus, "bus should be nil after Remove")
	s.False(raging.IsApplied(), "condition should no longer be applied")
}

func (s *RagingConditionTestSuite) TestRagingConditionGrantsAdvantageOnSTRSaves() {
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       5,
		Source:      "dnd5e:features:rage",
	})
	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	s.Run("adds advantage on STR saves for this character", func() {
		saveEvent := &dnd5eEvents.SavingThrowChainEvent{
			SaverID: "barbarian-1",
			Ability: abilities.STR,
			DC:      15,
		}

		saveChain := events.NewStagedChain[*dnd5eEvents.SavingThrowChainEvent](combat.ModifierStages)
		saves := dnd5eEvents.SavingThrowChain.On(s.bus)
		modifiedChain, err := saves.PublishWithChain(s.ctx, saveEvent, saveChain)
		s.Require().NoError(err)

		finalEvent, err := modifiedChain.Execute(s.ctx, saveEvent)
		s.Require().NoError(err)
		s.Require().Len(finalEvent.AdvantageSources, 1)
		s.Equal(refs.Conditions.Raging(), finalEvent.AdvantageSources[0].SourceRef)
		s.Equal("Raging", finalEvent.AdvantageSources[0].Name)
	})

	s.Run("does not add advantage on non-STR saves", func() {
		saveEvent := &dnd5eEvents.SavingThrowChainEvent{
			SaverID: "barbarian-1",
			Ability: abilities.CON,
			DC:      15,
		}

		saveChain := events.NewStagedChain[*dnd5eEvents.SavingThrowChainEvent](combat.ModifierStages)
		saves := dnd5eEvents.SavingThrowChain.On(s.bus)
		modifiedChain, err := saves.PublishWithChain(s.ctx, saveEvent, saveChain)
		s.Require().NoError(err)

		finalEvent, err := modifiedChain.Execute(s.ctx, saveEvent)
		s.Require().NoError(err)
		s.Empty(finalEvent.AdvantageSources)
	})

	s.Run("does not add advantage for other characters", func() {
		saveEvent := &dnd5eEvents.SavingThrowChainEvent{
			SaverID: "other-character",
			Ability: abilities.STR,
			DC:      15,
		}

		saveChain := events.NewStagedChain[*dnd5eEvents.SavingThrowChainEvent](combat.ModifierStages)
		saves := dnd5eEvents.SavingThrowChain.On(s.bus)
		modifiedChain, err := saves.PublishWithChain(s.ctx, saveEvent, saveChain)
		s.Require().NoError(err)

		finalEvent, err := modifiedChain.Execute(s.ctx, saveEvent)
		s.Require().NoError(err)
		s.Empty(finalEvent.AdvantageSources)
	})
}

func (s *RagingConditionTestSuite) TestRagingConditionGrantsAdvantageOnSTRChecks() {
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       5,
		Source:      "dnd5e:features:rage",
	})
	err := raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	s.Run("adds advantage on Athletics (STR) checks for this character", func() {
		checkEvent := &dnd5eEvents.AbilityCheckChainEvent{
			CheckerID: "barbarian-1",
			Skill:     skills.Athletics,
			DC:        15,
		}

		checkChain := events.NewStagedChain[*dnd5eEvents.AbilityCheckChainEvent](combat.ModifierStages)
		checks := dnd5eEvents.AbilityCheckChain.On(s.bus)
		modifiedChain, err := checks.PublishWithChain(s.ctx, checkEvent, checkChain)
		s.Require().NoError(err)

		finalEvent, err := modifiedChain.Execute(s.ctx, checkEvent)
		s.Require().NoError(err)
		s.Require().Len(finalEvent.AdvantageSources, 1)
		s.Equal(refs.Conditions.Raging(), finalEvent.AdvantageSources[0].SourceRef)
		s.Equal("Raging", finalEvent.AdvantageSources[0].Name)
	})

	s.Run("does not add advantage on non-STR checks", func() {
		checkEvent := &dnd5eEvents.AbilityCheckChainEvent{
			CheckerID: "barbarian-1",
			Skill:     skills.Stealth,
			DC:        15,
		}

		checkChain := events.NewStagedChain[*dnd5eEvents.AbilityCheckChainEvent](combat.ModifierStages)
		checks := dnd5eEvents.AbilityCheckChain.On(s.bus)
		modifiedChain, err := checks.PublishWithChain(s.ctx, checkEvent, checkChain)
		s.Require().NoError(err)

		finalEvent, err := modifiedChain.Execute(s.ctx, checkEvent)
		s.Require().NoError(err)
		s.Empty(finalEvent.AdvantageSources)
	})

	s.Run("does not add advantage for other characters", func() {
		checkEvent := &dnd5eEvents.AbilityCheckChainEvent{
			CheckerID: "other-character",
			Skill:     skills.Athletics,
			DC:        15,
		}

		checkChain := events.NewStagedChain[*dnd5eEvents.AbilityCheckChainEvent](combat.ModifierStages)
		checks := dnd5eEvents.AbilityCheckChain.On(s.bus)
		modifiedChain, err := checks.PublishWithChain(s.ctx, checkEvent, checkChain)
		s.Require().NoError(err)

		finalEvent, err := modifiedChain.Execute(s.ctx, checkEvent)
		s.Require().NoError(err)
		s.Empty(finalEvent.AdvantageSources)
	})
}

// TestARageThatMISSESATurnEndStillExpiresOnTime is the whole reason the counter
// became an anchor, and it is the test the old implementation could not pass.
//
// TurnsActive was INCREMENTED once per turn end, so it was only ever correct if
// no turn end was missed. Turn ends went missing for months (rpg-project#294),
// and they can still go missing for one member in ordinary play -- a body
// spliced out of the order and back, a fight that re-forms around somebody.
// A counter that has lost a tick under-counts forever after, and the rage runs
// long by exactly the number it dropped.
//
// event.Round - RoundActivated is recomputed from two facts every time, so a
// gap costs nothing: the rage still ends on the round it was always going to.
//
// Here the barbarian's turn ends are heard in rounds 1..4, then rounds 8..10 --
// three turn ends never arrive. The old arithmetic would have reached
// TurnsActive == 7 and kept going; this ends on round 10, as it should.
func (s *RagingConditionTestSuite) TestARageThatMISSESATurnEndStillExpiresOnTime() {
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1", DamageBonus: 2, Level: 5, Source: "dnd5e:features:rage",
	})
	s.Require().NoError(raging.Apply(s.ctx, s.bus))

	var removedEvent *dnd5eEvents.ConditionRemovedEvent
	_, err := dnd5eEvents.ConditionRemovedTopic.On(s.bus).Subscribe(s.ctx,
		func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
			removedEvent = &event
			return nil
		})
	s.Require().NoError(err)

	turnEnds := dnd5eEvents.TurnEndTopic.On(s.bus)
	heard := []int{1, 2, 3, 4, 8, 9, 10} // rounds 5, 6 and 7 never reach this rage
	for _, round := range heard {
		s.Require().NoError(s.executePostAttackRoll("barbarian-1", "goblin-1", true))
		s.Require().NoError(turnEnds.Publish(s.ctx, dnd5eEvents.TurnEndEvent{
			SubjectID: "barbarian-1", Round: round,
		}))
		if round < 10 {
			s.Require().Nil(removedEvent, "rage must still be running at round %d", round)
		}
	}

	s.Require().NotNil(removedEvent,
		"only SEVEN turn ends were heard, but ten rounds elapsed -- the anchor knows that and a counter could not")
	s.Equal("duration_expired", removedEvent.Reason)
}

// TestTheDurationIsRelativeToWhenTheRageStarted catches the implementation that
// quietly assumes a fight begins when the rage does.
//
// A barbarian who rages in round 4 gets rounds 4 through 13, not 4 through 10.
// Anchoring at 1 -- or comparing event.Round directly against the duration --
// passes every test that happens to start raging in round 1, which is most of
// them.
func (s *RagingConditionTestSuite) TestTheDurationIsRelativeToWhenTheRageStarted() {
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1", DamageBonus: 2, Level: 5, Source: "dnd5e:features:rage",
	})
	s.Require().NoError(raging.Apply(s.ctx, s.bus))

	var removedEvent *dnd5eEvents.ConditionRemovedEvent
	_, err := dnd5eEvents.ConditionRemovedTopic.On(s.bus).Subscribe(s.ctx,
		func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
			removedEvent = &event
			return nil
		})
	s.Require().NoError(err)

	turnEnds := dnd5eEvents.TurnEndTopic.On(s.bus)
	for round := 4; round <= 12; round++ {
		s.Require().NoError(s.executePostAttackRoll("barbarian-1", "goblin-1", true))
		s.Require().NoError(turnEnds.Publish(s.ctx, dnd5eEvents.TurnEndEvent{
			SubjectID: "barbarian-1", Round: round,
		}))
		s.Require().Nil(removedEvent,
			"a rage begun in round 4 is still running in round %d", round)
	}

	// Round 13 is the tenth round of THIS rage: 13 - 4 == 9.
	s.Require().NoError(s.executePostAttackRoll("barbarian-1", "goblin-1", true))
	s.Require().NoError(turnEnds.Publish(s.ctx, dnd5eEvents.TurnEndEvent{
		SubjectID: "barbarian-1", Round: 13,
	}))
	s.Require().NotNil(removedEvent, "and ends at the end of its tenth round, round 13")
	s.Equal("duration_expired", removedEvent.Reason)
}

// TestTurnsActiveIsDerivedAndStillWhatTheClientRenders pins a cross-repo
// contract from the side that can break it.
//
// rpg-dnd5e-web's isRagingData duck-types on 'turns_active' being present
// (src/types/conditionData.ts), and ConditionBadge renders "Active for N turns"
// from it. Deleting the field -- the obvious move, since no rule reads it any
// more -- would make every rage silently stop being recognised as a rage: the
// guard returns false, no error is raised, and the tooltip quietly loses its
// damage bonus, duration and resistance line.
//
// So it stays, DERIVED from the anchor rather than incremented, and these are
// the numbers the client already shows.
func (s *RagingConditionTestSuite) TestTurnsActiveIsDerivedAndStillWhatTheClientRenders() {
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1", DamageBonus: 2, Level: 5, Source: "dnd5e:features:rage",
	})
	s.Require().NoError(raging.Apply(s.ctx, s.bus))

	turnEnds := dnd5eEvents.TurnEndTopic.On(s.bus)
	for _, tc := range []struct{ round, want int }{{4, 1}, {5, 2}, {6, 3}, {9, 6}} {
		s.Require().NoError(s.executePostAttackRoll("barbarian-1", "goblin-1", true))
		s.Require().NoError(turnEnds.Publish(s.ctx, dnd5eEvents.TurnEndEvent{
			SubjectID: "barbarian-1", Round: tc.round,
		}))
		s.Equal(tc.want, raging.TurnsActive,
			"round %d of a rage anchored at 4 is its turn %d", tc.round, tc.want)
	}

	// The last row is the one an incrementing counter gets wrong: rounds 7 and
	// 8 were never heard, so a counter would say 4 where the anchor says 6.
	b, err := raging.ToJSON()
	s.Require().NoError(err)
	s.Contains(string(b), `"turns_active":6`, "and it survives the round trip the client reads")
	s.Contains(string(b), `"round_activated":4`)
}

// ragingWithRemovalWatch builds an applied rage and a handle on whatever
// removes it, which is the shape every duration test below needs.
func (s *RagingConditionTestSuite) ragingWithRemovalWatch() (
	*RagingCondition, func() *dnd5eEvents.ConditionRemovedEvent,
) {
	s.T().Helper()
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1", DamageBonus: 2, Level: 5, Source: "dnd5e:features:rage",
	})
	s.Require().NoError(raging.Apply(s.ctx, s.bus))

	var removed *dnd5eEvents.ConditionRemovedEvent
	_, err := dnd5eEvents.ConditionRemovedTopic.On(s.bus).Subscribe(s.ctx,
		func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
			removed = &event
			return nil
		})
	s.Require().NoError(err)
	return raging, func() *dnd5eEvents.ConditionRemovedEvent { return removed }
}

// TestARoundlessTurnEndDoesNotGrantPerpetualGrace is Copilot's finding on
// rpg-toolkit#1266, and it was a real one.
//
// The grace was originally keyed on RoundActivated == 0 -- "not yet anchored"
// and "not yet checked" collapsed into one field. They only coincide while
// rounds are valid, and the publisher that proved it did not was
// combat.TurnManager, which published TurnEndEvent{SubjectID} with NO Round at
// all: Round defaulted to 0, so under the old shape the rage never anchored,
// never left the graced branch, and never checked activity OR duration again.
// Immortal rage, silently.
//
// That publisher is gone (rpg-project#319 Phase 6 deleted TurnManager), which
// changes nothing about why this test stays. A round-less TurnEndEvent is
// still constructible by anyone who publishes the topic, and the shape that
// survived the hazard is the one worth pinning -- the fix outlives the caller
// that revealed it.
//
// The fix un-collapses the two facts: SawTurnEnd is the grace, RoundActivated
// is the anchor. The activity check needs no round, so it keeps working even
// when the duration cannot be evaluated.
func (s *RagingConditionTestSuite) TestARoundlessTurnEndDoesNotGrantPerpetualGrace() {
	raging, removed := s.ragingWithRemovalWatch()
	turnEnds := dnd5eEvents.TurnEndTopic.On(s.bus)

	// A publisher that never stamps its rounds. The first is the graced turn.
	s.Require().NoError(turnEnds.Publish(s.ctx, dnd5eEvents.TurnEndEvent{SubjectID: "barbarian-1"}))
	s.Require().Nil(removed(), "the first turn end is graced whether or not it carries a round")
	s.Require().True(raging.SawTurnEnd, "and the grace is spent, which is the thing the round cannot tell us")
	s.Require().Zero(raging.RoundActivated, "with nothing to anchor to, it stays unanchored")

	// The second is checked. No attack, no damage -- so it ends, exactly as it
	// would with a well-formed round.
	s.Require().NoError(turnEnds.Publish(s.ctx, dnd5eEvents.TurnEndEvent{SubjectID: "barbarian-1"}))
	s.Require().NotNil(removed(),
		"a rage must not become immortal because its publisher forgot to say which round it is")
	s.Equal("no_combat_activity", removed().Reason)
}

// TestAnUnanchoredRageAnchorsAsSoonAsARoundArrives — recovery rather than a
// permanent handicap. A rage that never anchored has no cap, so if the clock
// starts telling the truth it takes the first round it is given.
func (s *RagingConditionTestSuite) TestAnUnanchoredRageAnchorsAsSoonAsARoundArrives() {
	raging, _ := s.ragingWithRemovalWatch()
	turnEnds := dnd5eEvents.TurnEndTopic.On(s.bus)

	s.Require().NoError(turnEnds.Publish(s.ctx, dnd5eEvents.TurnEndEvent{SubjectID: "barbarian-1"}))
	s.Require().Zero(raging.RoundActivated)

	s.Require().NoError(s.executePostAttackRoll("barbarian-1", "goblin-1", true))
	s.Require().NoError(turnEnds.Publish(s.ctx, dnd5eEvents.TurnEndEvent{
		SubjectID: "barbarian-1", Round: 6,
	}))
	s.Equal(6, raging.RoundActivated, "the first round it is actually told becomes the anchor")
}

// TestARoundGoingBACKWARDSEndsTheRage is the net under combat end.
//
// Round numbers are PER-FIGHT -- clock.Turn.SetOrder starts every bubble at 1
// and Dissolve sets it back to 0 -- so a round lower than the anchor can only
// mean this rage outlived the clock its anchor came from. Part 1 of this slice
// exists to make that unreachable by removing rage when the fight ends; if it
// is ever reached, that removal did not happen.
//
// Ending is the right rules answer (the fight it belonged to is over) AND the
// safe one: re-anchoring would silently hand out a fresh ten rounds, and doing
// nothing leaves event.Round - RoundActivated negative, so the cap never fires
// and the rage runs until someone rests.
func (s *RagingConditionTestSuite) TestARoundGoingBACKWARDSEndsTheRage() {
	_, removed := s.ragingWithRemovalWatch()
	turnEnds := dnd5eEvents.TurnEndTopic.On(s.bus)

	// Anchored deep into a long fight.
	s.Require().NoError(turnEnds.Publish(s.ctx, dnd5eEvents.TurnEndEvent{
		SubjectID: "barbarian-1", Round: 7,
	}))
	s.Require().Nil(removed())

	// A new fight's round 1, with the rage still attached: combat end did not
	// fire. Activity is present, so only the backwards check can end it.
	s.Require().NoError(s.executePostAttackRoll("barbarian-1", "goblin-1", true))
	s.Require().NoError(turnEnds.Publish(s.ctx, dnd5eEvents.TurnEndEvent{
		SubjectID: "barbarian-1", Round: 1,
	}))

	s.Require().NotNil(removed(), "a rage cannot outlive the clock its duration is measured against")
	s.Equal("clock_reset", removed().Reason)
}

// TestARoundlessTurnEndDoesNotREADAsAClockReset — the two degraded cases are
// different and must not be collapsed either.
//
// Zero is "this event does not say", not "the round went backwards". Comparing
// it as a round would satisfy `event.Round < RoundActivated` and end an
// otherwise healthy rage on a malformed event.
func (s *RagingConditionTestSuite) TestARoundlessTurnEndDoesNotREADAsAClockReset() {
	raging, removed := s.ragingWithRemovalWatch()
	turnEnds := dnd5eEvents.TurnEndTopic.On(s.bus)

	s.Require().NoError(turnEnds.Publish(s.ctx, dnd5eEvents.TurnEndEvent{
		SubjectID: "barbarian-1", Round: 3,
	}))
	s.Require().NoError(s.executePostAttackRoll("barbarian-1", "goblin-1", true))
	s.Require().NoError(turnEnds.Publish(s.ctx, dnd5eEvents.TurnEndEvent{SubjectID: "barbarian-1"}))

	s.Nil(removed(), "a turn end with no round is unevaluable, not a reset")
	s.Equal(3, raging.RoundActivated, "and it does not disturb the anchor")
}

// TestANegativeRoundNeverBecomesTheAnchor pins the `event.Round > 0` guard on
// the initial anchor, which mutation testing showed was otherwise asserting
// nothing.
//
// Zero cannot distinguish it: assigning 0 to a field that is already 0 is a
// no-op, so removing the guard passes every test above. Only a NEGATIVE round
// tells the two apart -- and it matters, because a negative anchor poisons the
// arithmetic rather than merely disabling it. Anchored at -5, a later round 1
// computes 1 - (-5) == 6 and the rage silently runs six rounds' worth of
// duration it never earned, while the backwards-check (1 < -5) never fires.
//
// play/clock cannot produce one. TurnEndEvent.Round is a plain int with no
// validation, so nothing between a publisher and here says it may not.
func (s *RagingConditionTestSuite) TestANegativeRoundNeverBecomesTheAnchor() {
	raging, removed := s.ragingWithRemovalWatch()
	turnEnds := dnd5eEvents.TurnEndTopic.On(s.bus)

	s.Require().NoError(turnEnds.Publish(s.ctx, dnd5eEvents.TurnEndEvent{
		SubjectID: "barbarian-1", Round: -5,
	}))
	s.Require().Zero(raging.RoundActivated, "a negative round is not a round to measure from")
	s.Require().True(raging.SawTurnEnd, "though it does spend the grace, like any other turn end")

	// And the rage recovers: the next real round anchors it honestly.
	s.Require().NoError(s.executePostAttackRoll("barbarian-1", "goblin-1", true))
	s.Require().NoError(turnEnds.Publish(s.ctx, dnd5eEvents.TurnEndEvent{
		SubjectID: "barbarian-1", Round: 2,
	}))
	s.Require().Nil(removed())
	s.Equal(2, raging.RoundActivated)
	s.Equal(1, raging.TurnsActive, "and its first counted turn is that one, not a number derived from -5")
}

// TestRageEndsWhenTheBarbarianDropsToZero is rpg-toolkit#1965 tier 1 #3.
//
// RAW: rage ends early if you are knocked unconscious. The rage used to listen
// for a ConditionApplied fact typed Unconscious, which nothing has published
// since life state moved onto the sheet (DeathSaveState, combat.LifeState) —
// so a barbarian at 0 hit points kept raging. The fact that does say "this
// member went down" is DamageTaken with DroppedToZero, published by every
// machine that applies damage.
func (s *RagingConditionTestSuite) TestRageEndsWhenTheBarbarianDropsToZero() {
	raging, removed := s.ragingWithRemovalWatch()

	s.Require().NoError(dnd5eEvents.DamageTakenTopic.On(s.bus).Publish(s.ctx, &dnd5eEvents.DamageTakenEvent{
		MemberID: "barbarian-1", Amount: 14, DamageType: damage.Slashing, DroppedToZero: true,
	}))

	s.Require().NotNil(removed(), "a barbarian at 0 hit points is not raging")
	s.Equal("barbarian-1", removed().MemberID)
	s.Equal(refs.Conditions.Raging().String(), removed().ConditionRef)
	s.Equal("unconscious", removed().Reason)
	s.False(raging.IsApplied())
}

// TestRageSurvivesDamageThatDoesNotDropTheBarbarian pins the other side: being
// hurt is not going down, and somebody else going down is not this rage's
// business.
func (s *RagingConditionTestSuite) TestRageSurvivesDamageThatDoesNotDropTheBarbarian() {
	raging, removed := s.ragingWithRemovalWatch()
	damageTaken := dnd5eEvents.DamageTakenTopic.On(s.bus)

	s.Require().NoError(damageTaken.Publish(s.ctx, &dnd5eEvents.DamageTakenEvent{
		MemberID: "barbarian-1", Amount: 5, DamageType: damage.Slashing,
	}))
	s.Require().NoError(damageTaken.Publish(s.ctx, &dnd5eEvents.DamageTakenEvent{
		MemberID: "goblin-1", Amount: 7, DamageType: damage.Slashing, DroppedToZero: true,
	}))

	s.Nil(removed())
	s.True(raging.IsApplied())
}

// TestASavingThrowFrameFoldsOnTheDamageChain answers, for rpg-toolkit#1965
// tier 1 #1, whether DamageChainEvent forces attack-only facts on a spell or
// save damage source: it does not.
//
// A contest's damage framed as a SAVING THROW — the caster as actor, the saver
// as target, weapon pool known false, no weapon dice or type — folds without
// error. The target's Rage still resists the bludgeoning it takes, on the
// incoming fold, and the
// caster's Sneak Attack, an attacker-side rule asked by the same fold, answers
// DoesNotApply rather than Depends because the weapon pool is known. Leave the
// weapon pool unknown and Sneak Attack answers Depends and fails the fold, so a
// contest's frame must state it.
func (s *RagingConditionTestSuite) TestASavingThrowFrameFoldsOnTheDamageChain() {
	raging := newRagingCondition(ragingConditionInput{
		CharacterID: "barbarian-1", DamageBonus: 2, Level: 5, Source: "dnd5e:features:rage",
	})
	s.Require().NoError(raging.Apply(s.ctx, s.bus))
	sneak := NewSneakAttackCondition(SneakAttackInput{MemberID: "caster-1"})
	s.Require().NoError(sneak.Apply(s.ctx, s.bus))

	event := dnd5eEvents.NewDamageChainEvent(dnd5eEvents.DamageChainInput{
		AttackerID: "caster-1",
		TargetID:   "barbarian-1",
		Components: []dnd5eEvents.DamageComponent{{
			Source:     dnd5eEvents.DamageSourceSpell,
			Roll:       dnd5eEvents.RollComponent{Dice: testDiceTrace(8, 10)},
			DamageType: damage.Bludgeoning,
		}},
		Frame: contributions.Frame{
			Actor:            "caster-1",
			ActorClassLevels: classLevels(classes.Rogue, 1),
			Target:           contributions.Known("barbarian-1"),
			Action: contributions.ActionFacts{
				Roll:       contributions.Known(contributions.RollKindSavingThrow),
				WeaponPool: contributions.Known(false),
			},
		},
	})

	final, err := foldDealt(s.ctx, s.bus, event)
	s.Require().NoError(err)

	for _, component := range final.Components {
		if ref := component.Roll.Source.Ref; ref != nil {
			s.NotEqual(refs.Conditions.SneakAttack().String(), ref.String(),
				"Sneak Attack adds to weapon attacks, not to a spell's save damage")
		}
	}

	got, err := foldIncoming(s.ctx, s.bus, event.Frame, final.Components)
	s.Require().NoError(err)
	s.Require().Len(got.Folded.Multipliers, 1)
	s.Equal(refs.Conditions.Raging(), got.Folded.Multipliers[0].Source.Ref,
		"the raging target resists bludgeoning whatever dealt it")
	s.Equal(5, got.taken())
}
