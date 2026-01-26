package classes

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type GrantTestSuite struct {
	suite.Suite
}

func TestGrantSuite(t *testing.T) {
	suite.Run(t, new(GrantTestSuite))
}

func (s *GrantTestSuite) TestGetGrants_Rogue_ReturnsGrants() {
	// RED: This test should fail because getRogueGrants() doesn't exist
	// and Rogue isn't in the switch statement

	grants := GetGrants(Rogue)

	s.Require().NotNil(grants, "GetGrants(Rogue) should not return nil")
	s.Require().Len(grants, 1, "Rogue should have 1 grant at level 1")

	level1 := grants[0]
	s.Equal(1, level1.Level, "First grant should be at level 1")
}

func (s *GrantTestSuite) TestGetGrants_Rogue_Level1Proficiencies() {
	grants := GetGrants(Rogue)
	s.Require().NotNil(grants)
	s.Require().Len(grants, 1)

	level1 := grants[0]

	// Armor: Light armor only
	s.Contains(level1.ArmorProficiencies, proficiencies.ArmorLight,
		"Rogue should have light armor proficiency")
	s.Len(level1.ArmorProficiencies, 1,
		"Rogue should only have light armor proficiency")

	// Weapons: Simple + specific martial weapons
	s.Contains(level1.WeaponProficiencies, proficiencies.WeaponSimple,
		"Rogue should have simple weapon proficiency")
	s.Contains(level1.WeaponProficiencies, proficiencies.WeaponHandCrossbow,
		"Rogue should have hand crossbow proficiency")
	s.Contains(level1.WeaponProficiencies, proficiencies.WeaponLongsword,
		"Rogue should have longsword proficiency")
	s.Contains(level1.WeaponProficiencies, proficiencies.WeaponRapier,
		"Rogue should have rapier proficiency")
	s.Contains(level1.WeaponProficiencies, proficiencies.WeaponShortsword,
		"Rogue should have shortsword proficiency")

	// Tools: Thieves' tools
	s.Contains(level1.ToolProficiencies, proficiencies.ToolThieves,
		"Rogue should have thieves' tools proficiency")
}

func (s *GrantTestSuite) TestGetGrants_Rogue_Level1SneakAttack() {
	grants := GetGrants(Rogue)
	s.Require().NotNil(grants)
	s.Require().Len(grants, 1)

	level1 := grants[0]

	// Sneak Attack condition
	s.Require().Len(level1.Conditions, 1, "Rogue should have 1 condition at level 1")

	sneakAttack := level1.Conditions[0]
	s.Equal(refs.Conditions.SneakAttack().String(), sneakAttack.Ref,
		"Rogue should have sneak attack condition")
}

func (s *GrantTestSuite) TestGetGrants_Rogue_Level1ThievesCant() {
	grants := GetGrants(Rogue)
	s.Require().NotNil(grants)
	s.Require().Len(grants, 1)

	level1 := grants[0]

	// Thieves' Cant language
	s.Require().Len(level1.Languages, 1, "Rogue should have 1 language at level 1")
	s.Equal(languages.ThievesCant, level1.Languages[0],
		"Rogue should have Thieves' Cant language")
}

func (s *GrantTestSuite) TestGetGrants_Rogue_EndToEndConditionCreation() {
	// End-to-end test: verify that we can get grants and create conditions from them
	// This simulates what the game server would do when compiling a character

	grants := GetGrants(Rogue)
	s.Require().NotNil(grants)
	s.Require().Len(grants, 1)

	level1 := grants[0]
	s.Require().Len(level1.Conditions, 1)

	// Verify the condition ref can be parsed
	condRef := level1.Conditions[0]
	s.NotEmpty(condRef.Ref, "Condition ref should not be empty")
	s.NotEmpty(condRef.Config, "Condition config should not be empty")

	// The full flow would call conditions.CreateFromRef with these values
	// That's tested in conditions/factory_test.go - this test verifies the data is correct
	s.Equal("dnd5e:conditions:sneak_attack", condRef.Ref)
}

// =============================================================================
// Fighter Tests
// =============================================================================

func (s *GrantTestSuite) TestGetGrants_Fighter_ReturnsGrants() {
	grants := GetGrants(Fighter)

	s.Require().NotNil(grants, "GetGrants(Fighter) should not return nil")
	s.Require().Len(grants, 1, "Fighter should have 1 grant at level 1")

	level1 := grants[0]
	s.Equal(1, level1.Level, "First grant should be at level 1")
}

func (s *GrantTestSuite) TestGetGrants_Fighter_Level1Proficiencies() {
	grants := GetGrants(Fighter)
	s.Require().NotNil(grants)
	s.Require().Len(grants, 1)

	level1 := grants[0]

	// Armor: All armor and shields (PHB p.71)
	s.Contains(level1.ArmorProficiencies, proficiencies.ArmorLight,
		"Fighter should have light armor proficiency")
	s.Contains(level1.ArmorProficiencies, proficiencies.ArmorMedium,
		"Fighter should have medium armor proficiency")
	s.Contains(level1.ArmorProficiencies, proficiencies.ArmorHeavy,
		"Fighter should have heavy armor proficiency")
	s.Contains(level1.ArmorProficiencies, proficiencies.ArmorShields,
		"Fighter should have shield proficiency")
	s.Len(level1.ArmorProficiencies, 4,
		"Fighter should have exactly 4 armor proficiencies")

	// Weapons: Simple and martial weapons (PHB p.71)
	s.Contains(level1.WeaponProficiencies, proficiencies.WeaponSimple,
		"Fighter should have simple weapon proficiency")
	s.Contains(level1.WeaponProficiencies, proficiencies.WeaponMartial,
		"Fighter should have martial weapon proficiency")
	s.Len(level1.WeaponProficiencies, 2,
		"Fighter should have exactly 2 weapon proficiencies")

	// No tool proficiencies for Fighter
	s.Empty(level1.ToolProficiencies,
		"Fighter should have no tool proficiencies")
}

func (s *GrantTestSuite) TestGetGrants_Fighter_Level1SecondWind() {
	grants := GetGrants(Fighter)
	s.Require().NotNil(grants)
	s.Require().Len(grants, 1)

	level1 := grants[0]

	// Second Wind feature (PHB p.72)
	s.Require().Len(level1.Features, 1, "Fighter should have 1 feature at level 1")

	secondWind := level1.Features[0]
	s.Equal(refs.Features.SecondWind().String(), secondWind.Ref,
		"Fighter should have Second Wind feature")
	s.NotEmpty(secondWind.Config, "Second Wind should have config")
}

func (s *GrantTestSuite) TestGetGrants_Fighter_NoConditionsAtLevel1() {
	grants := GetGrants(Fighter)
	s.Require().NotNil(grants)
	s.Require().Len(grants, 1)

	level1 := grants[0]

	// Fighter has no conditions at level 1 (Fighting Style is a choice, not a grant)
	s.Empty(level1.Conditions,
		"Fighter should have no conditions at level 1 (Fighting Style is a choice)")
}

// =============================================================================
// Barbarian Tests
// =============================================================================

func (s *GrantTestSuite) TestGetGrants_Barbarian_ReturnsGrants() {
	grants := GetGrants(Barbarian)

	s.Require().NotNil(grants, "GetGrants(Barbarian) should not return nil")
	s.Require().Len(grants, 1, "Barbarian should have 1 grant at level 1")

	level1 := grants[0]
	s.Equal(1, level1.Level, "First grant should be at level 1")
}

func (s *GrantTestSuite) TestGetGrants_Barbarian_Level1Proficiencies() {
	grants := GetGrants(Barbarian)
	s.Require().NotNil(grants)
	s.Require().Len(grants, 1)

	level1 := grants[0]

	// Armor: Light, medium, and shields - NO heavy armor (PHB p.47)
	s.Contains(level1.ArmorProficiencies, proficiencies.ArmorLight,
		"Barbarian should have light armor proficiency")
	s.Contains(level1.ArmorProficiencies, proficiencies.ArmorMedium,
		"Barbarian should have medium armor proficiency")
	s.Contains(level1.ArmorProficiencies, proficiencies.ArmorShields,
		"Barbarian should have shield proficiency")
	s.NotContains(level1.ArmorProficiencies, proficiencies.ArmorHeavy,
		"Barbarian should NOT have heavy armor proficiency")
	s.Len(level1.ArmorProficiencies, 3,
		"Barbarian should have exactly 3 armor proficiencies")

	// Weapons: Simple and martial weapons (PHB p.47)
	s.Contains(level1.WeaponProficiencies, proficiencies.WeaponSimple,
		"Barbarian should have simple weapon proficiency")
	s.Contains(level1.WeaponProficiencies, proficiencies.WeaponMartial,
		"Barbarian should have martial weapon proficiency")
	s.Len(level1.WeaponProficiencies, 2,
		"Barbarian should have exactly 2 weapon proficiencies")

	// No tool proficiencies for Barbarian
	s.Empty(level1.ToolProficiencies,
		"Barbarian should have no tool proficiencies")
}

func (s *GrantTestSuite) TestGetGrants_Barbarian_Level1Rage() {
	grants := GetGrants(Barbarian)
	s.Require().NotNil(grants)
	s.Require().Len(grants, 1)

	level1 := grants[0]

	// Rage feature (PHB p.48)
	s.Require().Len(level1.Features, 1, "Barbarian should have 1 feature at level 1")

	rage := level1.Features[0]
	s.Equal(refs.Features.Rage().String(), rage.Ref,
		"Barbarian should have Rage feature")
	s.NotEmpty(rage.Config, "Rage should have config (uses, damage_bonus)")
}

func (s *GrantTestSuite) TestGetGrants_Barbarian_Level1UnarmoredDefense() {
	grants := GetGrants(Barbarian)
	s.Require().NotNil(grants)
	s.Require().Len(grants, 1)

	level1 := grants[0]

	// Unarmored Defense condition (PHB p.48)
	s.Require().Len(level1.Conditions, 1, "Barbarian should have 1 condition at level 1")

	unarmoredDefense := level1.Conditions[0]
	s.Equal(refs.Conditions.UnarmoredDefense().String(), unarmoredDefense.Ref,
		"Barbarian should have Unarmored Defense condition")
	s.Contains(string(unarmoredDefense.Config), "barbarian",
		"Unarmored Defense should be configured for barbarian variant (CON-based)")
}

// =============================================================================
// Monk Tests
// =============================================================================

func (s *GrantTestSuite) TestGetGrants_Monk_ReturnsGrants() {
	grants := GetGrants(Monk)

	s.Require().NotNil(grants, "GetGrants(Monk) should not return nil")
	s.Require().Len(grants, 1, "Monk should have 1 grant at level 1")

	level1 := grants[0]
	s.Equal(1, level1.Level, "First grant should be at level 1")
}

func (s *GrantTestSuite) TestGetGrants_Monk_Level1Proficiencies() {
	grants := GetGrants(Monk)
	s.Require().NotNil(grants)
	s.Require().Len(grants, 1)

	level1 := grants[0]

	// Armor: NO armor proficiencies (PHB p.77) - Monks don't wear armor
	s.Empty(level1.ArmorProficiencies,
		"Monk should have NO armor proficiencies")

	// Weapons: Simple weapons and shortswords (PHB p.77)
	s.Contains(level1.WeaponProficiencies, proficiencies.WeaponSimple,
		"Monk should have simple weapon proficiency")
	s.Contains(level1.WeaponProficiencies, proficiencies.WeaponShortsword,
		"Monk should have shortsword proficiency")
	s.Len(level1.WeaponProficiencies, 2,
		"Monk should have exactly 2 weapon proficiencies")

	// Tool proficiency is a CHOICE (artisan's tool or musical instrument)
	// So it shouldn't be in grants
	s.Empty(level1.ToolProficiencies,
		"Monk tool proficiency is a choice, not a grant")
}

func (s *GrantTestSuite) TestGetGrants_Monk_Level1UnarmoredDefense() {
	grants := GetGrants(Monk)
	s.Require().NotNil(grants)
	s.Require().Len(grants, 1)

	level1 := grants[0]

	// Unarmored Defense condition - monk variant uses WIS (PHB p.78)
	s.Require().GreaterOrEqual(len(level1.Conditions), 1,
		"Monk should have at least Unarmored Defense condition")

	// Find Unarmored Defense in conditions
	var unarmoredDefense *ConditionRef
	for i := range level1.Conditions {
		if level1.Conditions[i].Ref == refs.Conditions.UnarmoredDefense().String() {
			unarmoredDefense = &level1.Conditions[i]
			break
		}
	}
	s.Require().NotNil(unarmoredDefense,
		"Monk should have Unarmored Defense condition")
	s.Contains(string(unarmoredDefense.Config), "monk",
		"Unarmored Defense should be configured for monk variant (WIS-based)")
}

func (s *GrantTestSuite) TestGetGrants_Monk_Level1MartialArts() {
	grants := GetGrants(Monk)
	s.Require().NotNil(grants)
	s.Require().Len(grants, 1)

	level1 := grants[0]

	// Martial Arts condition (PHB p.78)
	// Find Martial Arts in conditions
	var martialArts *ConditionRef
	for i := range level1.Conditions {
		if level1.Conditions[i].Ref == refs.Conditions.MartialArts().String() {
			martialArts = &level1.Conditions[i]
			break
		}
	}
	s.Require().NotNil(martialArts,
		"Monk should have Martial Arts condition")
	s.NotEmpty(martialArts.Config,
		"Martial Arts should have config (monk_level)")
}

func (s *GrantTestSuite) TestGetGrants_Monk_NoFeaturesAtLevel1() {
	grants := GetGrants(Monk)
	s.Require().NotNil(grants)
	s.Require().Len(grants, 1)

	level1 := grants[0]

	// Monk gets Ki and Flurry of Blows at level 2, not level 1
	s.Empty(level1.Features,
		"Monk should have no features at level 1 (Ki comes at level 2)")
}
