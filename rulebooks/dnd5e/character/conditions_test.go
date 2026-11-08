// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/fightingstyles"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
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

// addRageChargesToCharacter adds rage charges resource to a character for testing.
// In production, this should be done when the barbarian class is assigned.
func addRageChargesToCharacter(char *Character, level int) {
	// Calculate rage uses based on barbarian level
	maxUses := 2 // Default for level 1-2
	switch {
	case level >= 17:
		maxUses = 6
	case level >= 12:
		maxUses = 5
	case level >= 6:
		maxUses = 4
	case level >= 3:
		maxUses = 3
	}

	rageResource := combat.NewRecoverableResource(combat.RecoverableResourceConfig{
		ID:          string(resources.RageCharges),
		Maximum:     maxUses,
		CharacterID: char.GetID(),
		ResetType:   coreResources.ResetLongRest,
	})
	char.AddResource(resources.RageCharges, rageResource)
}

func (s *CharacterConditionsTestSuite) TestCharacterReceivesRageCondition() {
	// Create a barbarian with rage feature
	draft := LoadDraftFromData(&DraftData{
		ID:       "test-barbarian",
		PlayerID: "player1",
	})

	// Setup character
	s.Require().NoError(draft.SetName(&SetNameInput{Name: "Conan"}))
	s.Require().NoError(draft.SetRace(&SetRaceInput{
		RaceID:  races.Human,
		Choices: RaceChoices{Languages: []languages.Language{languages.Elvish}},
	}))
	s.Require().NoError(draft.SetClass(&SetClassInput{
		ClassID: classes.Barbarian,
		Choices: ClassChoices{
			Skills: []skills.Skill{skills.Athletics, skills.Intimidation},
			Equipment: []EquipmentChoiceSelection{
				{ChoiceID: choices.BarbarianWeaponsPrimary, OptionID: choices.BarbarianWeaponGreataxe},
				{ChoiceID: choices.BarbarianWeaponsSecondary, OptionID: choices.BarbarianSecondaryHandaxes},
				{ChoiceID: choices.BarbarianPack, OptionID: choices.BarbarianPackExplorer},
			},
		},
	}))
	s.Require().NoError(draft.SetBackground(&SetBackgroundInput{
		BackgroundID: backgrounds.Hermit,
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

	// Add rage charges resource (in production this would be done during class assignment)
	addRageChargesToCharacter(char, 1)

	// Verify character has only Unarmored Defense condition initially (from class grants)
	initialConds := char.GetConditions()
	_, isUnarmoredDefense := conditionOfType[*conditions.UnarmoredDefenseCondition](initialConds)
	s.True(isUnarmoredDefense, "Barbarian should start with the Unarmored Defense condition")
	s.Zero(heldCount(initialConds, refs.Conditions.Raging().String()), "Barbarian should not start raging")

	// Get rage feature
	rageFeature := char.GetFeature("rage")
	s.Require().NotNil(rageFeature, "Barbarian should have rage feature")

	// Activate rage (publishes ConditionAppliedEvent)
	err = rageFeature.Activate(s.ctx, char, features.FeatureInput{Bus: s.bus})
	s.Require().NoError(err)

	// Verify character now has both Unarmored Defense and Raging conditions
	conds := char.GetConditions()
	_, keptUnarmoredDefense := conditionOfType[*conditions.UnarmoredDefenseCondition](conds)
	s.True(keptUnarmoredDefense, "Unarmored Defense stays after rage")

	// Find and verify the raging condition
	var ragingCond dnd5eEvents.ConditionBehavior
	for _, c := range conds {
		if c.Ref().String() == refs.Conditions.Raging().String() {
			ragingCond = c
			break
		}
	}
	s.Require().NotNil(ragingCond, "Should have raging condition")
	saved := composedCharacterRageData(&s.Suite, ragingCond)
	s.Equal("char-1", saved.MemberID)
	s.Equal("char-1", saved.Origin.EntityID)
	s.Equal(*refs.Features.Rage(), saved.Origin.Ref)
}

func (s *CharacterConditionsTestSuite) TestCharacterIgnoresOtherCharacterConditions() {
	// Create a character
	draft := LoadDraftFromData(&DraftData{
		ID:       "test-char",
		PlayerID: "player1",
	})

	// Minimal setup
	s.Require().NoError(draft.SetName(&SetNameInput{Name: "Test"}))
	s.Require().NoError(draft.SetRace(&SetRaceInput{
		RaceID:  races.Human,
		Choices: RaceChoices{Languages: []languages.Language{languages.Elvish}},
	}))
	s.Require().NoError(draft.SetClass(&SetClassInput{
		ClassID: classes.Fighter,
		Choices: ClassChoices{
			Skills: []skills.Skill{skills.Athletics, skills.Intimidation},
			Equipment: []EquipmentChoiceSelection{
				{ChoiceID: choices.FighterArmor, OptionID: choices.FighterArmorChainMail},
				{
					ChoiceID:           choices.FighterWeaponsPrimary,
					OptionID:           choices.FighterWeaponMartialShield,
					CategorySelections: []shared.EquipmentID{weapons.Longsword},
				},
				{ChoiceID: choices.FighterWeaponsSecondary, OptionID: choices.FighterRangedCrossbow},
				{ChoiceID: choices.FighterPack, OptionID: choices.FighterPackDungeoneer},
			},
			FightingStyle: fightingstyles.Defense,
		},
	}))
	s.Require().NoError(draft.SetBackground(&SetBackgroundInput{
		BackgroundID: backgrounds.Hermit,
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

	// Capture initial conditions - Fighter has a FightingStyleCondition from choosing Defense
	initialConditions := char.GetConditions()
	initialRefs := heldRefs(initialConditions)
	s.Contains(initialRefs, refs.Conditions.FightingStyleDefense().String(),
		"Fighter should start with the Defense fighting style condition")

	// Publish a condition for a DIFFERENT character
	differentChar := &DummyEntity{id: "char-2"}

	// Create a raging condition for the different character
	ragingCond := composedCharacterRage(&s.Suite, "char-2")

	topic := dnd5eEvents.ConditionAppliedTopic.On(s.bus)
	err = topic.Publish(s.ctx, dnd5eEvents.ConditionAppliedEvent{
		Target:    differentChar,
		Source:    "test",
		Condition: ragingCond,
	})
	s.Require().NoError(err)

	// Verify our character did NOT receive the raging condition
	// It should still only have its initial FightingStyleCondition
	s.ElementsMatch(initialRefs, heldRefs(char.GetConditions()),
		"Character should ignore conditions for other characters")

	// Verify no raging condition was added
	for _, cond := range char.GetConditions() {
		s.NotEqual(refs.Conditions.Raging().String(), cond.Ref().String(), "Character should NOT have raging condition from another character")
	}
}

func (s *CharacterConditionsTestSuite) TestCharacterRemovesExpiredCondition() {
	// Create a barbarian with rage feature
	draft := LoadDraftFromData(&DraftData{
		ID:       "test-barbarian",
		PlayerID: "player1",
	})

	// Setup character
	s.Require().NoError(draft.SetName(&SetNameInput{Name: "Conan"}))
	s.Require().NoError(draft.SetRace(&SetRaceInput{
		RaceID:  races.Human,
		Choices: RaceChoices{Languages: []languages.Language{languages.Elvish}},
	}))
	s.Require().NoError(draft.SetClass(&SetClassInput{
		ClassID: classes.Barbarian,
		Choices: ClassChoices{
			Skills: []skills.Skill{skills.Athletics, skills.Intimidation},
			Equipment: []EquipmentChoiceSelection{
				{ChoiceID: choices.BarbarianWeaponsPrimary, OptionID: choices.BarbarianWeaponGreataxe},
				{ChoiceID: choices.BarbarianWeaponsSecondary, OptionID: choices.BarbarianSecondaryHandaxes},
				{ChoiceID: choices.BarbarianPack, OptionID: choices.BarbarianPackExplorer},
			},
		},
	}))
	s.Require().NoError(draft.SetBackground(&SetBackgroundInput{
		BackgroundID: backgrounds.Hermit,
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

	// Add rage charges resource (in production this would be done during class assignment)
	addRageChargesToCharacter(char, 1)

	// Get rage feature and activate it
	rageFeature := char.GetFeature("rage")
	s.Require().NotNil(rageFeature)
	err = rageFeature.Activate(s.ctx, char, features.FeatureInput{Bus: s.bus})
	s.Require().NoError(err)

	// Verify character has both Unarmored Defense (from grants) and Raging (from activation)
	s.requireUnarmoredDefenseAndRaging(char, "Character should have Unarmored Defense + Raging conditions")

	// Simulate rage expiring by publishing turn end events without combat
	// activity. TWO of them, because the turn a rage started is not checked
	// (Kirk's ruling, rpg-project#295) -- and asserting the state in between is
	// what makes this a test of the grace rather than of the count.
	turnEndTopic := dnd5eEvents.TurnEndTopic.On(s.bus)
	err = turnEndTopic.Publish(s.ctx, dnd5eEvents.TurnEndEvent{
		SubjectID: "char-1",
		Round:     1,
	})
	s.Require().NoError(err)
	s.requireUnarmoredDefenseAndRaging(char,
		"the turn the rage started is graced -- it survives its own activation turn ending")

	err = turnEndTopic.Publish(s.ctx, dnd5eEvents.TurnEndEvent{
		SubjectID: "char-1",
		Round:     2,
	})
	s.Require().NoError(err)

	// Verify only Unarmored Defense remains after rage expires
	remainingConds := char.GetConditions()
	_, isUnarmoredDefense := conditionOfType[*conditions.UnarmoredDefenseCondition](remainingConds)
	s.True(isUnarmoredDefense, "Unarmored Defense remains after rage expires")
	s.Zero(heldCount(remainingConds, refs.Conditions.Raging().String()), "Raging is gone after rage expires")
}

func (s *CharacterConditionsTestSuite) TestCharacterIgnoresOtherCharacterRemovals() {
	// Create a barbarian
	draft := LoadDraftFromData(&DraftData{
		ID:       "test-barbarian",
		PlayerID: "player1",
	})

	// Setup character
	s.Require().NoError(draft.SetName(&SetNameInput{Name: "Conan"}))
	s.Require().NoError(draft.SetRace(&SetRaceInput{
		RaceID:  races.Human,
		Choices: RaceChoices{Languages: []languages.Language{languages.Elvish}},
	}))
	s.Require().NoError(draft.SetClass(&SetClassInput{
		ClassID: classes.Barbarian,
		Choices: ClassChoices{
			Skills: []skills.Skill{skills.Athletics, skills.Intimidation},
			Equipment: []EquipmentChoiceSelection{
				{ChoiceID: choices.BarbarianWeaponsPrimary, OptionID: choices.BarbarianWeaponGreataxe},
				{ChoiceID: choices.BarbarianWeaponsSecondary, OptionID: choices.BarbarianSecondaryHandaxes},
				{ChoiceID: choices.BarbarianPack, OptionID: choices.BarbarianPackExplorer},
			},
		},
	}))
	s.Require().NoError(draft.SetBackground(&SetBackgroundInput{
		BackgroundID: backgrounds.Hermit,
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

	// Add rage charges resource (in production this would be done during class assignment)
	addRageChargesToCharacter(char, 1)

	// Activate rage
	rageFeature := char.GetFeature("rage")
	s.Require().NotNil(rageFeature)
	err = rageFeature.Activate(s.ctx, char, features.FeatureInput{Bus: s.bus})
	s.Require().NoError(err)

	// Verify character has Unarmored Defense + Raging conditions
	s.requireUnarmoredDefenseAndRaging(char, "Character should have Unarmored Defense + Raging conditions")

	// Publish removal event for a DIFFERENT character
	removalTopic := dnd5eEvents.ConditionRemovedTopic.On(s.bus)
	err = removalTopic.Publish(s.ctx, dnd5eEvents.ConditionRemovedEvent{
		MemberID:     "char-2",
		ConditionRef: "dnd5e:conditions:raging",
		Reason:       "test",
	})
	s.Require().NoError(err)

	// Verify our character still has both conditions
	s.requireUnarmoredDefenseAndRaging(char, "Character should still have both conditions")
}

func (s *CharacterConditionsTestSuite) TestMonkReceivesMartialArtsCondition() {
	// Create a monk
	draft := LoadDraftFromData(&DraftData{
		ID:       "test-monk",
		PlayerID: "player1",
	})

	// Setup character
	s.Require().NoError(draft.SetName(&SetNameInput{Name: "Sensei"}))
	s.Require().NoError(draft.SetRace(&SetRaceInput{
		RaceID:  races.Human,
		Choices: RaceChoices{Languages: []languages.Language{languages.Elvish}},
	}))
	s.Require().NoError(draft.SetClass(&SetClassInput{
		ClassID: classes.Monk,
		Choices: ClassChoices{
			Skills: []skills.Skill{skills.Acrobatics, skills.Stealth},
			Equipment: []EquipmentChoiceSelection{
				{ChoiceID: choices.MonkWeaponsPrimary, OptionID: choices.MonkWeaponShortsword},
				{ChoiceID: choices.MonkPack, OptionID: choices.MonkPackDungeoneer},
			},
			Tools: []shared.SelectionID{"brewer-supplies"},
		},
	}))
	s.Require().NoError(draft.SetBackground(&SetBackgroundInput{
		BackgroundID: backgrounds.Hermit,
		Choices:      BackgroundChoices{},
	}))
	s.Require().NoError(draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 10,
			abilities.DEX: 16, // +3
			abilities.CON: 12, // +1
			abilities.INT: 10,
			abilities.WIS: 14, // +2
			abilities.CHA: 8,
		},
	}))

	// Convert to character with event bus
	char, err := draft.ToCharacter(s.ctx, "char-monk", s.bus)
	s.Require().NoError(err)
	s.Require().NotNil(char)

	// Verify character has both Unarmored Defense and Martial Arts conditions
	conds := char.GetConditions()

	// Find the conditions
	var hasUnarmoredDefense, hasMartialArts bool
	var martialArtsCond *conditions.MartialArtsCondition
	for _, cond := range conds {
		if _, ok := cond.(*conditions.UnarmoredDefenseCondition); ok {
			hasUnarmoredDefense = true
		}
		if ma, ok := cond.(*conditions.MartialArtsCondition); ok {
			hasMartialArts = true
			martialArtsCond = ma
		}
	}

	s.True(hasUnarmoredDefense, "Monk should have Unarmored Defense condition")
	s.True(hasMartialArts, "Monk should have Martial Arts condition")

	// Martial Arts stores no monk level: the die is read from the sheet at
	// each swing.
	s.Require().NotNil(martialArtsCond)
	raw, err := martialArtsCond.ToJSON()
	s.Require().NoError(err)
	s.NotContains(string(raw), "level")
}

func (s *CharacterConditionsTestSuite) TestCharacterConditionRoundTrip() {
	// Produce real runtime progress, then persist the current composed shape.
	ragingCond := composedCharacterRage(&s.Suite, "char-rt")
	s.Require().NoError(ragingCond.Apply(s.ctx, s.bus))
	for round := 1; round <= 3; round++ {
		s.Require().NoError(dnd5eEvents.DamageTakenTopic.On(s.bus).Publish(s.ctx, &dnd5eEvents.DamageTakenEvent{MemberID: "char-rt", Amount: 1, DamageType: damage.Slashing}))
		s.Require().NoError(dnd5eEvents.TurnEndTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.TurnEndEvent{SubjectID: "char-rt", Round: round}))
	}
	rawJSON, err := ragingCond.ToJSON()
	s.Require().NoError(err)
	s.Require().NoError(ragingCond.Remove(s.ctx, s.bus))

	// Build a minimal character Data with the serialized condition
	data := &Data{
		ID:       "char-rt",
		PlayerID: "player-rt",
		Name:     "Round Trip Barbarian",
		Level:    1,
		ClassID:  classes.Barbarian,
		RaceID:   races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14,
			abilities.CON: 14,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 8,
		},
		HitPoints:        14,
		MaxHitPoints:     14,
		ProficiencyBonus: 2,
		Conditions:       []json.RawMessage{rawJSON},
	}

	// LoadFromData should deserialize and Apply the condition
	char, err := LoadFromData(s.ctx, data, s.bus)
	s.Require().NoError(err)
	s.Require().NotNil(char)

	// Verify the character has exactly one condition and it is applied
	var loadedCond dnd5eEvents.ConditionBehavior
	for _, condition := range char.GetConditions() {
		if condition.Ref().String() == refs.Conditions.Raging().String() {
			loadedCond = condition
			break
		}
	}
	s.Require().NotNil(loadedCond, "The canonical Raging condition must survive loading")
	s.True(loadedCond.IsApplied(), "Loaded condition should be applied (subscribed to bus)")
	snapshot := composedCharacterRageData(&s.Suite, loadedCond)
	s.Equal("char-rt", snapshot.MemberID)
	s.Equal(float64(3), snapshot.Runtime.Bag["turns_active"])
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

// conditionOfType finds the first held condition of the given concrete type.
func conditionOfType[T dnd5eEvents.ConditionBehavior](held []dnd5eEvents.ConditionBehavior) (T, bool) {
	for _, condition := range held {
		if typed, ok := condition.(T); ok {
			return typed, true
		}
	}
	var none T
	return none, false
}

// requireUnarmoredDefenseAndRaging asserts both conditions are held, by
// membership: what else a sheet holds is not this suite's question.
func (s *CharacterConditionsTestSuite) requireUnarmoredDefenseAndRaging(char *Character, msg string) {
	s.T().Helper()
	_, unarmoredDefense := conditionOfType[*conditions.UnarmoredDefenseCondition](char.GetConditions())
	raging := heldCount(char.GetConditions(), refs.Conditions.Raging().String()) == 1
	s.True(unarmoredDefense && raging, msg)
}

// heldCount counts how many of the given conditions carry the ref: the claim
// "held once" is about one identity, not about how much else the sheet holds.
func heldCount(held []dnd5eEvents.ConditionBehavior, ref string) int {
	count := 0
	for _, condition := range held {
		if condition.Ref().String() == ref {
			count++
		}
	}
	return count
}
