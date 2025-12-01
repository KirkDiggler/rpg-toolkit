// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

type CharacterConditionsTestSuite struct {
	suite.Suite
	ctx context.Context
	bus events.EventBus
}

func (s *CharacterConditionsTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

func TestCharacterConditionsTestSuite(t *testing.T) {
	suite.Run(t, new(CharacterConditionsTestSuite))
}

func (s *CharacterConditionsTestSuite) TestCharacterReceivesRageCondition() {
	// Create a barbarian with rage feature
	draft := LoadDraftFromData(&DraftData{
		ID:       "test-barbarian",
		PlayerID: "player1",
	})

	// Setup character
	s.Require().NoError(draft.SetName(&SetNameInput{Name: "Conan"}))
	s.Require().NoError(draft.SetRace(&SetRaceInput{RaceID: races.Human}))
	s.Require().NoError(draft.SetClass(&SetClassInput{
		ClassID: classes.Barbarian,
		Choices: ClassChoices{
			Skills: []skills.Skill{skills.Athletics, skills.Intimidation},
		},
	}))
	s.Require().NoError(draft.SetBackground(&SetBackgroundInput{
		BackgroundID: backgrounds.Soldier,
		Choices:      BackgroundChoices{},
	}))
	s.Require().NoError(draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 16, // +3
			abilities.DEX: 14, // +2
			abilities.CON: 14, // +2
			abilities.INT: 10, // +0
			abilities.WIS: 12, // +1
			abilities.CHA: 8,  // -1
		},
	}))

	// Convert to character with event bus
	char, err := draft.ToCharacter(s.ctx, "char-1", s.bus)
	s.Require().NoError(err)
	s.Require().NotNil(char)

	// Verify character starts with Unarmored Defense condition from grants
	initialConds := char.GetConditions()
	s.Len(initialConds, 1, "Barbarian should start with Unarmored Defense condition from grants")
	_, isUnarmoredDefense := initialConds[0].(*conditions.UnarmoredDefenseCondition)
	s.True(isUnarmoredDefense, "Initial condition should be Unarmored Defense")

	// Get rage feature
	rageFeature := char.GetFeature("rage")
	s.Require().NotNil(rageFeature, "Barbarian should have rage feature")

	// Activate rage (publishes ConditionAppliedEvent)
	err = rageFeature.Activate(s.ctx, char, features.FeatureInput{Bus: s.bus})
	s.Require().NoError(err)

	// Verify character now has both conditions (Unarmored Defense + Raging)
	conds := char.GetConditions()
	s.Len(conds, 2, "Character should have two conditions after rage (Unarmored Defense + Raging)")

	// Find and verify the raging condition
	var ragingCond *conditions.RagingCondition
	for _, cond := range conds {
		if rc, ok := cond.(*conditions.RagingCondition); ok {
			ragingCond = rc
			break
		}
	}
	s.NotNil(ragingCond, "Should have a RagingCondition")
	s.Equal("char-1", ragingCond.CharacterID)
	s.Equal(2, ragingCond.DamageBonus) // Level 1 barbarian = +2 rage damage
	s.Equal("rage", ragingCond.Source)
}

func (s *CharacterConditionsTestSuite) TestCharacterIgnoresOtherCharacterConditions() {
	// Create a character
	draft := LoadDraftFromData(&DraftData{
		ID:       "test-char",
		PlayerID: "player1",
	})

	// Minimal setup
	s.Require().NoError(draft.SetName(&SetNameInput{Name: "Test"}))
	s.Require().NoError(draft.SetRace(&SetRaceInput{RaceID: races.Human}))
	s.Require().NoError(draft.SetClass(&SetClassInput{
		ClassID: classes.Fighter,
		Choices: ClassChoices{
			Skills: []skills.Skill{skills.Athletics, skills.Intimidation},
		},
	}))
	s.Require().NoError(draft.SetBackground(&SetBackgroundInput{
		BackgroundID: backgrounds.Soldier,
		Choices:      BackgroundChoices{},
	}))
	s.Require().NoError(draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 16, abilities.DEX: 14, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 8,
		},
	}))

	char, err := draft.ToCharacter(s.ctx, "char-1", s.bus)
	s.Require().NoError(err)

	// Publish a condition for a DIFFERENT character
	differentChar := &DummyEntity{id: "char-2"}

	// Create a raging condition for the different character
	ragingCond := &conditions.RagingCondition{
		CharacterID: "char-2",
		DamageBonus: 2,
		Level:       5,
		Source:      "test",
	}

	topic := dnd5eEvents.ConditionAppliedTopic.On(s.bus)
	err = topic.Publish(s.ctx, dnd5eEvents.ConditionAppliedEvent{
		Target:    differentChar,
		Type:      dnd5eEvents.ConditionRaging,
		Source:    "test",
		Condition: ragingCond,
	})
	s.Require().NoError(err)

	// Verify our character did NOT receive the condition
	s.Empty(char.GetConditions(), "Character should ignore conditions for other characters")
}

func (s *CharacterConditionsTestSuite) TestCharacterRemovesExpiredCondition() {
	// Create a barbarian with rage feature
	draft := LoadDraftFromData(&DraftData{
		ID:       "test-barbarian",
		PlayerID: "player1",
	})

	// Setup character
	s.Require().NoError(draft.SetName(&SetNameInput{Name: "Conan"}))
	s.Require().NoError(draft.SetRace(&SetRaceInput{RaceID: races.Human}))
	s.Require().NoError(draft.SetClass(&SetClassInput{
		ClassID: classes.Barbarian,
		Choices: ClassChoices{
			Skills: []skills.Skill{skills.Athletics, skills.Intimidation},
		},
	}))
	s.Require().NoError(draft.SetBackground(&SetBackgroundInput{
		BackgroundID: backgrounds.Soldier,
		Choices:      BackgroundChoices{},
	}))
	s.Require().NoError(draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 16, abilities.DEX: 14, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 8,
		},
	}))

	// Convert to character with event bus
	char, err := draft.ToCharacter(s.ctx, "char-1", s.bus)
	s.Require().NoError(err)

	// Barbarian starts with 1 condition (Unarmored Defense from grants)
	s.Len(char.GetConditions(), 1, "Barbarian should start with Unarmored Defense condition")

	// Get rage feature and activate it
	rageFeature := char.GetFeature("rage")
	s.Require().NotNil(rageFeature)
	err = rageFeature.Activate(s.ctx, char, features.FeatureInput{Bus: s.bus})
	s.Require().NoError(err)

	// Verify character has 2 conditions (Unarmored Defense + Raging)
	s.Len(char.GetConditions(), 2, "Character should have two conditions (Unarmored Defense + Raging)")

	// Simulate rage expiring by publishing turn end event without combat activity
	turnEndTopic := dnd5eEvents.TurnEndTopic.On(s.bus)
	err = turnEndTopic.Publish(s.ctx, dnd5eEvents.TurnEndEvent{
		CharacterID: "char-1",
		Round:       1,
	})
	s.Require().NoError(err)

	// Verify raging condition was removed, but Unarmored Defense remains
	s.Len(char.GetConditions(), 1, "Character should have only Unarmored Defense after rage expires")
}

func (s *CharacterConditionsTestSuite) TestCharacterIgnoresOtherCharacterRemovals() {
	// Create a barbarian
	draft := LoadDraftFromData(&DraftData{
		ID:       "test-barbarian",
		PlayerID: "player1",
	})

	// Setup character
	s.Require().NoError(draft.SetName(&SetNameInput{Name: "Conan"}))
	s.Require().NoError(draft.SetRace(&SetRaceInput{RaceID: races.Human}))
	s.Require().NoError(draft.SetClass(&SetClassInput{
		ClassID: classes.Barbarian,
		Choices: ClassChoices{
			Skills: []skills.Skill{skills.Athletics, skills.Intimidation},
		},
	}))
	s.Require().NoError(draft.SetBackground(&SetBackgroundInput{
		BackgroundID: backgrounds.Soldier,
		Choices:      BackgroundChoices{},
	}))
	s.Require().NoError(draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 16, abilities.DEX: 14, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 8,
		},
	}))

	// Convert to character with event bus
	char, err := draft.ToCharacter(s.ctx, "char-1", s.bus)
	s.Require().NoError(err)

	// Barbarian starts with 1 condition (Unarmored Defense from grants)
	s.Len(char.GetConditions(), 1, "Barbarian should start with Unarmored Defense condition")

	// Activate rage
	rageFeature := char.GetFeature("rage")
	s.Require().NotNil(rageFeature)
	err = rageFeature.Activate(s.ctx, char, features.FeatureInput{Bus: s.bus})
	s.Require().NoError(err)

	// Verify character has 2 conditions (Unarmored Defense + Raging)
	s.Len(char.GetConditions(), 2, "Character should have two conditions (Unarmored Defense + Raging)")

	// Publish removal event for a DIFFERENT character
	removalTopic := dnd5eEvents.ConditionRemovedTopic.On(s.bus)
	err = removalTopic.Publish(s.ctx, dnd5eEvents.ConditionRemovedEvent{
		CharacterID:  "char-2",
		ConditionRef: "dnd5e:conditions:raging",
		Reason:       "test",
	})
	s.Require().NoError(err)

	// Verify our character still has both conditions (removal was for different character)
	s.Len(char.GetConditions(), 2, "Character should still have both conditions")
}

// DummyEntity implements core.Entity for testing
type DummyEntity struct {
	id string
}

func (d *DummyEntity) GetID() string {
	return d.id
}

func (d *DummyEntity) GetType() core.EntityType {
	return "test"
}
