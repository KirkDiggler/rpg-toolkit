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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// UnarmoredDefenseTestSuite tests the UnarmoredDefenseCondition behavior
type UnarmoredDefenseTestSuite struct {
	suite.Suite
	ctx context.Context
	bus events.EventBus
}

func (s *UnarmoredDefenseTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

func TestUnarmoredDefenseTestSuite(t *testing.T) {
	suite.Run(t, new(UnarmoredDefenseTestSuite))
}

func (s *UnarmoredDefenseTestSuite) TestBarbarianUnarmoredDefenseAC() {
	// Barbarian: AC = 10 + DEX + CON
	ud := NewUnarmoredDefenseCondition(UnarmoredDefenseInput{
		CharacterID: "barbarian-1",
		Type:        UnarmoredDefenseBarbarian,
		Source:      "dnd5e:classes:barbarian",
	})

	// DEX 14 (+2), CON 16 (+3) = 10 + 2 + 3 = 15
	scores := shared.AbilityScores{
		abilities.STR: 16,
		abilities.DEX: 14,
		abilities.CON: 16,
		abilities.INT: 8,
		abilities.WIS: 12,
		abilities.CHA: 10,
	}

	ac := ud.CalculateAC(scores)
	s.Equal(15, ac, "AC should be 10 + 2 (DEX) + 3 (CON) = 15")
}

func (s *UnarmoredDefenseTestSuite) TestMonkUnarmoredDefenseAC() {
	// Monk: AC = 10 + DEX + WIS
	ud := NewUnarmoredDefenseCondition(UnarmoredDefenseInput{
		CharacterID: "monk-1",
		Type:        UnarmoredDefenseMonk,
		Source:      "dnd5e:classes:monk",
	})

	// DEX 16 (+3), WIS 14 (+2) = 10 + 3 + 2 = 15
	scores := shared.AbilityScores{
		abilities.STR: 10,
		abilities.DEX: 16,
		abilities.CON: 12,
		abilities.INT: 10,
		abilities.WIS: 14,
		abilities.CHA: 10,
	}

	ac := ud.CalculateAC(scores)
	s.Equal(15, ac, "AC should be 10 + 3 (DEX) + 2 (WIS) = 15")
}

func (s *UnarmoredDefenseTestSuite) TestUnarmoredDefenseWithNegativeModifiers() {
	// Test with negative modifiers
	ud := NewUnarmoredDefenseCondition(UnarmoredDefenseInput{
		CharacterID: "barbarian-1",
		Type:        UnarmoredDefenseBarbarian,
		Source:      "dnd5e:classes:barbarian",
	})

	// DEX 8 (-1), CON 8 (-1) = 10 + (-1) + (-1) = 8
	scores := shared.AbilityScores{
		abilities.STR: 16,
		abilities.DEX: 8,
		abilities.CON: 8,
		abilities.INT: 10,
		abilities.WIS: 10,
		abilities.CHA: 10,
	}

	ac := ud.CalculateAC(scores)
	s.Equal(8, ac, "AC should be 10 + (-1) (DEX) + (-1) (CON) = 8")
}

func (s *UnarmoredDefenseTestSuite) TestUnarmoredDefenseMaxStats() {
	// Test with maximum ability scores (20)
	ud := NewUnarmoredDefenseCondition(UnarmoredDefenseInput{
		CharacterID: "barbarian-1",
		Type:        UnarmoredDefenseBarbarian,
		Source:      "dnd5e:classes:barbarian",
	})

	// DEX 20 (+5), CON 20 (+5) = 10 + 5 + 5 = 20
	scores := shared.AbilityScores{
		abilities.STR: 20,
		abilities.DEX: 20,
		abilities.CON: 20,
		abilities.INT: 10,
		abilities.WIS: 10,
		abilities.CHA: 10,
	}

	ac := ud.CalculateAC(scores)
	s.Equal(20, ac, "AC should be 10 + 5 (DEX) + 5 (CON) = 20")
}

func (s *UnarmoredDefenseTestSuite) TestUnarmoredDefenseSecondaryAbility() {
	barbarianUD := NewUnarmoredDefenseCondition(UnarmoredDefenseInput{
		CharacterID: "barbarian-1",
		Type:        UnarmoredDefenseBarbarian,
		Source:      "dnd5e:classes:barbarian",
	})
	s.Equal(abilities.CON, barbarianUD.SecondaryAbility(), "Barbarian should use CON")

	monkUD := NewUnarmoredDefenseCondition(UnarmoredDefenseInput{
		CharacterID: "monk-1",
		Type:        UnarmoredDefenseMonk,
		Source:      "dnd5e:classes:monk",
	})
	s.Equal(abilities.WIS, monkUD.SecondaryAbility(), "Monk should use WIS")
}

func (s *UnarmoredDefenseTestSuite) TestUnarmoredDefenseApplyRemove() {
	ud := NewUnarmoredDefenseCondition(UnarmoredDefenseInput{
		CharacterID: "barbarian-1",
		Type:        UnarmoredDefenseBarbarian,
		Source:      "dnd5e:classes:barbarian",
	})

	// Apply should succeed
	err := ud.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.True(ud.IsApplied(), "condition should be applied")

	// Apply again should fail
	err = ud.Apply(s.ctx, s.bus)
	s.Error(err, "should not be able to apply twice")

	// Remove should succeed
	err = ud.Remove(s.ctx, s.bus)
	s.Require().NoError(err)
	s.False(ud.IsApplied(), "condition should not be applied after remove")
}

func (s *UnarmoredDefenseTestSuite) TestUnarmoredDefenseACChainIntegration() {
	// Test that the condition modifies AC through the ACChain
	characterID := "monk-1"

	// Create Monk Unarmored Defense: AC = 10 + DEX + WIS
	ud := NewUnarmoredDefenseCondition(UnarmoredDefenseInput{
		CharacterID: characterID,
		Type:        UnarmoredDefenseMonk,
		Source:      "dnd5e:classes:monk",
	})

	// Apply the condition
	err := ud.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Set up game context with ability scores
	// DEX 16 (+3), WIS 14 (+2) -> Unarmored Defense adds +2 (WIS mod)
	registry := gamectx.NewBasicCharacterRegistry()
	registry.AddAbilityScores(characterID, &gamectx.AbilityScores{
		Strength:     10, // +0
		Dexterity:    16, // +3
		Constitution: 12, // +1
		Intelligence: 10, // +0
		Wisdom:       14, // +2
		Charisma:     10, // +0
	})
	gameCtx := gamectx.NewGameContext(gamectx.GameContextConfig{
		CharacterRegistry: registry,
	})
	ctx := gamectx.WithGameContext(s.ctx, gameCtx)

	// Create AC event for unarmored character
	// Base would be 10 + 3 (DEX) = 13, Unarmored Defense should add +2 (WIS) = 15
	breakdown := &combat.ACBreakdown{
		Total:      13, // Base 10 + DEX 3
		Components: []combat.ACComponent{},
	}
	acEvent := &combat.ACChainEvent{
		CharacterID: characterID,
		Breakdown:   breakdown,
		HasArmor:    false, // Unarmored!
		HasShield:   false,
	}

	// Execute through AC chain
	acChain := events.NewStagedChain[*combat.ACChainEvent](combat.ModifierStages)
	acTopic := combat.ACChain.On(s.bus)

	modifiedChain, err := acTopic.PublishWithChain(ctx, acEvent, acChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(ctx, acEvent)
	s.Require().NoError(err)

	// Verify the WIS modifier (+2) was added
	s.Equal(15, finalEvent.Breakdown.Total, "AC should be 13 + 2 (WIS from Unarmored Defense) = 15")

	// Verify the component was added
	s.Len(finalEvent.Breakdown.Components, 1)
	s.Equal(combat.ACSourceFeature, finalEvent.Breakdown.Components[0].Type)
	s.Equal(2, finalEvent.Breakdown.Components[0].Value)

	// Clean up
	err = ud.Remove(ctx, s.bus)
	s.Require().NoError(err)
}

func (s *UnarmoredDefenseTestSuite) TestUnarmoredDefenseIgnoredWhenWearingArmor() {
	// Test that Unarmored Defense does NOT apply when wearing armor
	characterID := "monk-1"

	ud := NewUnarmoredDefenseCondition(UnarmoredDefenseInput{
		CharacterID: characterID,
		Type:        UnarmoredDefenseMonk,
		Source:      "dnd5e:classes:monk",
	})

	err := ud.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Set up game context
	registry := gamectx.NewBasicCharacterRegistry()
	registry.AddAbilityScores(characterID, &gamectx.AbilityScores{
		Dexterity: 16,
		Wisdom:    14,
	})
	gameCtx := gamectx.NewGameContext(gamectx.GameContextConfig{
		CharacterRegistry: registry,
	})
	ctx := gamectx.WithGameContext(s.ctx, gameCtx)

	// Create AC event for character WEARING ARMOR
	breakdown := &combat.ACBreakdown{
		Total:      16, // Armor provides this
		Components: []combat.ACComponent{},
	}
	acEvent := &combat.ACChainEvent{
		CharacterID: characterID,
		Breakdown:   breakdown,
		HasArmor:    true, // Wearing armor!
		HasShield:   false,
	}

	// Execute through AC chain
	acChain := events.NewStagedChain[*combat.ACChainEvent](combat.ModifierStages)
	acTopic := combat.ACChain.On(s.bus)

	modifiedChain, err := acTopic.PublishWithChain(ctx, acEvent, acChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(ctx, acEvent)
	s.Require().NoError(err)

	// Verify NO modification was made (still 16, no components added)
	s.Equal(16, finalEvent.Breakdown.Total, "AC should remain unchanged when wearing armor")
	s.Empty(finalEvent.Breakdown.Components, "No components should be added when wearing armor")

	err = ud.Remove(ctx, s.bus)
	s.Require().NoError(err)
}

func (s *UnarmoredDefenseTestSuite) TestUnarmoredDefenseToJSON() {
	ud := NewUnarmoredDefenseCondition(UnarmoredDefenseInput{
		CharacterID: "barbarian-1",
		Type:        UnarmoredDefenseBarbarian,
		Source:      "dnd5e:classes:barbarian",
	})

	jsonData, err := ud.ToJSON()
	s.Require().NoError(err)

	// Verify JSON contains expected fields
	s.Contains(string(jsonData), `"character_id":"barbarian-1"`)
	s.Contains(string(jsonData), `"type":"barbarian"`)
	s.Contains(string(jsonData), `"source":"dnd5e:classes:barbarian"`)
	s.Contains(string(jsonData), `"ref":"dnd5e:conditions:unarmored_defense"`)
}

func (s *UnarmoredDefenseTestSuite) TestDifferentScoreCombinations() {
	// Test various score combinations to ensure calculation is correct
	testCases := []struct {
		name       string
		udType     UnarmoredDefenseType
		dex        int
		secondary  int // CON for barbarian, WIS for monk
		expectedAC int
	}{
		{"Barbarian average stats", UnarmoredDefenseBarbarian, 10, 10, 10},     // +0 DEX, +0 CON
		{"Barbarian high DEX", UnarmoredDefenseBarbarian, 18, 10, 14},          // +4 DEX, +0 CON
		{"Barbarian high CON", UnarmoredDefenseBarbarian, 10, 18, 14},          // +0 DEX, +4 CON
		{"Barbarian both high", UnarmoredDefenseBarbarian, 16, 16, 16},         // +3 DEX, +3 CON
		{"Monk average stats", UnarmoredDefenseMonk, 10, 10, 10},               // +0 DEX, +0 WIS
		{"Monk high DEX", UnarmoredDefenseMonk, 18, 10, 14},                    // +4 DEX, +0 WIS
		{"Monk high WIS", UnarmoredDefenseMonk, 10, 18, 14},                    // +0 DEX, +4 WIS
		{"Monk both high", UnarmoredDefenseMonk, 16, 16, 16},                   // +3 DEX, +3 WIS
		{"Barbarian point buy typical", UnarmoredDefenseBarbarian, 14, 15, 14}, // +2 DEX, +2 CON
		{"Monk point buy typical", UnarmoredDefenseMonk, 15, 14, 14},           // +2 DEX, +2 WIS
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			ud := NewUnarmoredDefenseCondition(UnarmoredDefenseInput{
				CharacterID: "test-char",
				Type:        tc.udType,
				Source:      "test",
			})

			scores := shared.AbilityScores{
				abilities.STR: 10,
				abilities.DEX: tc.dex,
				abilities.CON: 10,
				abilities.INT: 10,
				abilities.WIS: 10,
				abilities.CHA: 10,
			}

			// Set the secondary ability based on type
			if tc.udType == UnarmoredDefenseBarbarian {
				scores[abilities.CON] = tc.secondary
			} else {
				scores[abilities.WIS] = tc.secondary
			}

			ac := ud.CalculateAC(scores)
			s.Equal(tc.expectedAC, ac)
		})
	}
}
