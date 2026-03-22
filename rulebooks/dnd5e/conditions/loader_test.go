// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// LoaderTestSuite tests the condition loader functionality
type LoaderTestSuite struct {
	suite.Suite
	ctx context.Context
	bus events.EventBus
}

func TestLoaderTestSuite(t *testing.T) {
	suite.Run(t, new(LoaderTestSuite))
}

func (s *LoaderTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

// executeDamageChain creates and executes a damage chain for the given attacker.
// Returns the final event after all chain modifications have been applied.
func (s *LoaderTestSuite) executeDamageChain(
	attackerID string,
	baseDamage, abilityBonus int,
) (*dnd5eEvents.DamageChainEvent, error) {
	weaponComp := dnd5eEvents.DamageComponent{
		Source:            dnd5eEvents.DamageSourceWeapon,
		OriginalDiceRolls: []int{baseDamage},
		FinalDiceRolls:    []int{baseDamage},
		DamageType:        damage.Slashing,
	}

	abilityComp := dnd5eEvents.DamageComponent{
		Source:    dnd5eEvents.DamageSourceAbility,
		FlatBonus: abilityBonus,
	}

	damageEvent := &dnd5eEvents.DamageChainEvent{
		AttackerID:   attackerID,
		TargetID:     "goblin-1",
		Components:   []dnd5eEvents.DamageComponent{weaponComp, abilityComp},
		DamageType:   damage.Slashing,
		WeaponDamage: "1d8",
		AbilityUsed:  abilities.STR,
	}

	ch := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damageTopic := dnd5eEvents.DamageChain.On(s.bus)

	modifiedChain, err := damageTopic.PublishWithChain(s.ctx, damageEvent, ch)
	if err != nil {
		return nil, err
	}

	return modifiedChain.Execute(s.ctx, damageEvent)
}

func (s *LoaderTestSuite) TestLoadRagingCondition() {
	// Create a raging condition
	original := &RagingCondition{
		CharacterID:       "barbarian-1",
		DamageBonus:       2,
		Level:             5,
		Source:            "dnd5e:features:rage",
		TurnsActive:       3,
		WasHitThisTurn:    true,
		DidAttackThisTurn: true,
	}

	// Serialize to JSON
	jsonData, err := original.ToJSON()
	s.Require().NoError(err)

	// Load from JSON
	loaded, err := LoadJSON(jsonData)
	s.Require().NoError(err)

	// Verify it's a RagingCondition
	raging, ok := loaded.(*RagingCondition)
	s.Require().True(ok, "Expected *RagingCondition")

	// Verify all fields match
	s.Equal(original.CharacterID, raging.CharacterID)
	s.Equal(original.DamageBonus, raging.DamageBonus)
	s.Equal(original.Level, raging.Level)
	s.Equal(original.Source, raging.Source)
	s.Equal(original.TurnsActive, raging.TurnsActive)
	s.Equal(original.WasHitThisTurn, raging.WasHitThisTurn)
	s.Equal(original.DidAttackThisTurn, raging.DidAttackThisTurn)
}

func (s *LoaderTestSuite) TestLoadBrutalCriticalCondition() {
	// Create a brutal critical condition
	original := NewBrutalCriticalCondition(BrutalCriticalInput{
		CharacterID: "barbarian-1",
		Level:       13,
	})

	// Serialize to JSON
	jsonData, err := original.ToJSON()
	s.Require().NoError(err)

	// Load from JSON
	loaded, err := LoadJSON(jsonData)
	s.Require().NoError(err)

	// Verify it's a BrutalCriticalCondition
	brutal, ok := loaded.(*BrutalCriticalCondition)
	s.Require().True(ok, "Expected *BrutalCriticalCondition")

	// Verify all fields match
	s.Equal(original.CharacterID, brutal.CharacterID)
	s.Equal(original.Level, brutal.Level)
	s.Equal(original.ExtraDice, brutal.ExtraDice)
}

func (s *LoaderTestSuite) TestLoadUnarmoredDefenseCondition() {
	// Create an unarmored defense condition
	original := NewUnarmoredDefenseCondition(UnarmoredDefenseInput{
		CharacterID: "barbarian-1",
		Type:        UnarmoredDefenseBarbarian,
		Source:      "dnd5e:classes:barbarian",
	})

	// Serialize to JSON
	jsonData, err := original.ToJSON()
	s.Require().NoError(err)

	// Load from JSON
	loaded, err := LoadJSON(jsonData)
	s.Require().NoError(err)

	// Verify it's an UnarmoredDefenseCondition
	unarmored, ok := loaded.(*UnarmoredDefenseCondition)
	s.Require().True(ok, "Expected *UnarmoredDefenseCondition")

	// Verify all fields match
	s.Equal(original.CharacterID, unarmored.CharacterID)
	s.Equal(original.Type, unarmored.Type)
	s.Equal(original.Source, unarmored.Source)
}

func (s *LoaderTestSuite) TestLoadMonkUnarmoredDefense() {
	// Verify monk variant loads correctly
	original := NewUnarmoredDefenseCondition(UnarmoredDefenseInput{
		CharacterID: "monk-1",
		Type:        UnarmoredDefenseMonk,
		Source:      "dnd5e:classes:monk",
	})

	jsonData, err := original.ToJSON()
	s.Require().NoError(err)

	loaded, err := LoadJSON(jsonData)
	s.Require().NoError(err)

	unarmored, ok := loaded.(*UnarmoredDefenseCondition)
	s.Require().True(ok)
	s.Equal(UnarmoredDefenseMonk, unarmored.Type)
}

func (s *LoaderTestSuite) TestLoadUnknownCondition() {
	// Test loading unknown condition ref
	jsonData := []byte(`{"ref":{"module":"dnd5e","type":"conditions","id":"unknown"}}`)

	_, err := LoadJSON(jsonData)
	s.Error(err)
	s.Contains(err.Error(), "unknown condition ref")
}

func (s *LoaderTestSuite) TestLoadInvalidJSON() {
	// Test loading invalid JSON
	jsonData := []byte(`{invalid json}`)

	_, err := LoadJSON(jsonData)
	s.Error(err)
	s.Contains(err.Error(), "failed to peek at condition ref")
}

func (s *LoaderTestSuite) TestRagingConditionRoundTripWithSubscriptions() {
	// Create a raging condition with known state
	original := &RagingCondition{
		CharacterID:       "barbarian-1",
		DamageBonus:       2,
		Level:             5,
		Source:            "dnd5e:features:rage",
		TurnsActive:       3,
		WasHitThisTurn:    true,
		DidAttackThisTurn: true,
	}

	// Apply the original to the bus
	err := original.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.True(original.IsApplied(), "original should be applied")

	// Serialize to JSON
	jsonData, err := original.ToJSON()
	s.Require().NoError(err)

	// Deserialize via LoadJSON — this gives a new condition instance
	loaded, err := LoadJSON(jsonData)
	s.Require().NoError(err)

	raging, ok := loaded.(*RagingCondition)
	s.Require().True(ok, "Expected *RagingCondition")

	// The loader does NOT call Apply — so IsApplied should be false
	s.False(raging.IsApplied(), "deserialized condition should not be applied yet")

	// Remove the original so it doesn't interfere with the loaded copy's subscriptions
	err = original.Remove(s.ctx, s.bus)
	s.Require().NoError(err)

	// Apply the deserialized condition to the bus
	err = raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.True(raging.IsApplied(), "deserialized condition should be applied after Apply()")

	// Publish a DamageChainEvent and verify the rage damage bonus is added
	finalEvent, err := s.executeDamageChain("barbarian-1", 5, 3)
	s.Require().NoError(err)

	// Should have 3 components: weapon, ability, and rage bonus
	s.Require().Len(finalEvent.Components, 3, "Should have weapon, ability, and rage components")
	s.Equal(dnd5eEvents.DamageSourceCondition, finalEvent.Components[2].Source)
	s.Equal(2, finalEvent.Components[2].FlatBonus, "Rage should add +2 damage from deserialized condition")
}

func (s *LoaderTestSuite) TestRagingConditionRoundTripCleanup() {
	// Create condition, Apply, serialize, deserialize, Apply the loaded copy, then Remove
	original := &RagingCondition{
		CharacterID: "barbarian-1",
		DamageBonus: 2,
		Level:       5,
		Source:      "dnd5e:features:rage",
	}

	err := original.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Serialize and deserialize
	jsonData, err := original.ToJSON()
	s.Require().NoError(err)

	loaded, err := LoadJSON(jsonData)
	s.Require().NoError(err)

	raging, ok := loaded.(*RagingCondition)
	s.Require().True(ok)

	// Remove the original first
	err = original.Remove(s.ctx, s.bus)
	s.Require().NoError(err)
	s.False(original.IsApplied())

	// Apply the loaded copy
	err = raging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.True(raging.IsApplied())

	// Remove the loaded copy — verify cleanup works on a re-applied condition
	err = raging.Remove(s.ctx, s.bus)
	s.Require().NoError(err)
	s.False(raging.IsApplied(), "deserialized condition should be unapplied after Remove()")

	// Verify no rage bonus is applied after removal
	finalEvent, err := s.executeDamageChain("barbarian-1", 5, 3)
	s.Require().NoError(err)

	// Should only have 2 components: weapon and ability (no rage)
	s.Require().Len(finalEvent.Components, 2, "Should only have weapon and ability after Remove()")
}
