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

func (s *UnarmoredDefenseTestSuite) TestUnarmoredDefenseSecondaryAbility() {
	barbarianUD := NewUnarmoredDefenseCondition(UnarmoredDefenseInput{
		MemberID: "barbarian-1",
		Type:     UnarmoredDefenseBarbarian,
		Source:   "dnd5e:classes:barbarian",
	})
	s.Equal(abilities.CON, barbarianUD.SecondaryAbility(), "Barbarian should use CON")

	monkUD := NewUnarmoredDefenseCondition(UnarmoredDefenseInput{
		MemberID: "monk-1",
		Type:     UnarmoredDefenseMonk,
		Source:   "dnd5e:classes:monk",
	})
	s.Equal(abilities.WIS, monkUD.SecondaryAbility(), "Monk should use WIS")
}

func (s *UnarmoredDefenseTestSuite) TestUnarmoredDefenseApplyRemove() {
	ud := NewUnarmoredDefenseCondition(UnarmoredDefenseInput{
		MemberID: "barbarian-1",
		Type:     UnarmoredDefenseBarbarian,
		Source:   "dnd5e:classes:barbarian",
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
		MemberID: characterID,
		Type:     UnarmoredDefenseMonk,
		Source:   "dnd5e:classes:monk",
	})

	// Apply the condition
	err := ud.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Own sheet, read off the cast the way the one door installs it.
	// DEX 16 (+3), WIS 14 (+2) -> Unarmored Defense adds +2 (WIS mod)
	ctx := castOf(s.ctx, &fakeConditionOwner{
		id: characterID,
		scores: shared.AbilityScores{
			abilities.STR: 10, // +0
			abilities.DEX: 16, // +3
			abilities.CON: 12, // +1
			abilities.INT: 10, // +0
			abilities.WIS: 14, // +2
			abilities.CHA: 10, // +0
		},
	})

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
		MemberID: characterID,
		Type:     UnarmoredDefenseMonk,
		Source:   "dnd5e:classes:monk",
	})

	err := ud.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	ctx := castOf(s.ctx, &fakeConditionOwner{
		id: characterID,
		scores: shared.AbilityScores{
			abilities.DEX: 16,
			abilities.WIS: 14,
		},
	})

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
		MemberID: "barbarian-1",
		Type:     UnarmoredDefenseBarbarian,
		Source:   "dnd5e:classes:barbarian",
	})

	jsonData, err := ud.ToJSON()
	s.Require().NoError(err)

	// Verify JSON contains expected fields
	s.Contains(string(jsonData), `"member_id":"barbarian-1"`)
	s.Contains(string(jsonData), `"type":"barbarian"`)
	s.Contains(string(jsonData), `"source":"dnd5e:classes:barbarian"`)
	s.Contains(string(jsonData), `"ref":"dnd5e:conditions:unarmored_defense"`)
}
