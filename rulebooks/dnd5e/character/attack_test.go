// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character_test

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/class"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/constants"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/race"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/stretchr/testify/suite"
)

// AttackTestSuite tests character attack functionality with features
type AttackTestSuite struct {
	suite.Suite
	ctx context.Context
	bus events.EventBus
}

func (s *AttackTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

func TestAttackTestSuite(t *testing.T) {
	suite.Run(t, new(AttackTestSuite))
}

func (s *AttackTestSuite) TestCharacterAttackWithRageBonus() {
	// Create a level 5 barbarian
	charData := character.Data{
		ID:       "test-barbarian",
		PlayerID: "player1",
		Name:     "Ragnar",
		Level:    5,
		ClassID:  "barbarian",
		RaceID:   "human",
		AbilityScores: shared.AbilityScores{
			constants.STR: 16, // +3 modifier
			constants.DEX: 14,
			constants.CON: 16,
			constants.INT: 10,
			constants.WIS: 12,
			constants.CHA: 8,
		},
		HitPoints:    45,
		MaxHitPoints: 45,
	}

	// Create minimal race/class data
	raceData := &race.Data{
		ID:   "human",
		Name: "Human",
	}

	classData := &class.Data{
		ID:      "barbarian",
		Name:    "Barbarian",
		HitDice: 12,
	}

	backgroundData := &shared.Background{
		ID:   "soldier",
		Name: "Soldier",
	}

	// Load the character
	char, err := character.LoadCharacterFromData(charData, raceData, classData, backgroundData)
	s.Require().NoError(err)

	// Subscribe character to events
	err = char.ApplyToEventBus(s.ctx, s.bus)
	s.Require().NoError(err)

	// Track attack events
	var attackEvent *character.AttackEvent
	attackTopic := character.AttackTopic.On(s.bus)
	_, err = attackTopic.Subscribe(s.ctx, func(_ context.Context, event character.AttackEvent) error {
		attackEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Create a simple weapon (greatsword: 2d6 damage)
	weapon := &testWeapon{
		ref:        "greatsword",
		damage:     "2d6",
		isMelee:    true,
		properties: []string{"heavy", "two-handed"},
	}

	// Attack without rage - should have base damage
	result1 := char.Attack(s.ctx, weapon)
	s.Require().NotNil(result1)

	// Level 5 = +3 proficiency, STR 16 = +3 modifier
	s.Equal(3, result1.ProficiencyBonus, "Level 5 should have +3 proficiency")
	s.Equal(3, result1.AbilityModifier, "STR 16 should give +3 modifier")
	s.Equal(6, result1.AttackBonus, "Attack bonus should be proficiency + ability modifier")
	s.Equal(3, result1.DamageBonus, "Damage bonus should be ability modifier without rage")

	// Verify attack event was published
	s.Require().NotNil(attackEvent)
	s.Equal("test-barbarian", attackEvent.AttackerID)
	s.Equal("greatsword", attackEvent.WeaponRef)
	s.True(attackEvent.IsMelee)

	// Now add rage feature
	rage := features.NewRage(features.RageConfig{
		ID:    "rage-feature-1",
		Level: 5,
		Uses:  3,
		Bus:   s.bus,
	})

	// Add feature to character
	char.AddFeature(rage)

	// Activate rage
	err = rage.Activate(s.ctx, char, features.FeatureInput{})
	s.Require().NoError(err)

	// Character should now have raging condition
	s.True(char.HasCondition("dnd5e:conditions:raging"))

	// Attack with rage - should have bonus damage
	result2 := char.Attack(s.ctx, weapon)
	s.Require().NotNil(result2)

	// Attack bonus stays the same
	s.Equal(6, result2.AttackBonus, "Attack bonus should be unchanged by rage")

	// Damage bonus should increase by rage bonus (+2 at level 5)
	s.Equal(5, result2.DamageBonus, "Damage bonus should be ability modifier + rage bonus")
}

func (s *AttackTestSuite) TestCharacterAttackWithoutProficiency() {
	// Create a wizard trying to use a martial weapon
	charData := character.Data{
		ID:       "test-wizard",
		PlayerID: "player1",
		Name:     "Merlin",
		Level:    5,
		ClassID:  "wizard",
		RaceID:   "elf",
		AbilityScores: shared.AbilityScores{
			constants.STR: 10, // +0 modifier
			constants.DEX: 14,
			constants.CON: 12,
			constants.INT: 16,
			constants.WIS: 13,
			constants.CHA: 11,
		},
		HitPoints:    22,
		MaxHitPoints: 22,
	}

	// Create minimal race/class data
	raceData := &race.Data{
		ID:   "elf",
		Name: "Elf",
	}

	classData := &class.Data{
		ID:      "wizard",
		Name:    "Wizard",
		HitDice: 6,
	}

	backgroundData := &shared.Background{
		ID:   "sage",
		Name: "Sage",
	}

	// Load the character
	char, err := character.LoadCharacterFromData(charData, raceData, classData, backgroundData)
	s.Require().NoError(err)

	// Create a martial weapon the wizard isn't proficient with
	weapon := &testWeapon{
		ref:        "greatsword",
		damage:     "2d6",
		isMelee:    true,
		properties: []string{"heavy", "two-handed", "martial"},
	}

	// Attack without proficiency
	result := char.Attack(s.ctx, weapon)
	s.Require().NotNil(result)

	// Should NOT add proficiency bonus when not proficient
	s.Equal(0, result.AttackBonus, "Attack bonus should be 0 without proficiency")
	s.Equal(0, result.DamageBonus, "STR 10 gives +0 damage modifier")
}

// testWeapon is a simple weapon implementation for testing
type testWeapon struct {
	ref        string
	damage     string
	isMelee    bool
	properties []string
}

func (w *testWeapon) GetRef() string          { return w.ref }
func (w *testWeapon) GetDamage() string       { return w.damage }
func (w *testWeapon) IsMelee() bool           { return w.isMelee }
func (w *testWeapon) GetProperties() []string { return w.properties }
