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
