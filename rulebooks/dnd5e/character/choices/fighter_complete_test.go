package choices_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFighterCompleteRequirements(t *testing.T) {
	// Get Fighter requirements at level 1
	reqs := choices.GetClassRequirementsAtLevel(classes.Fighter, 1)
	require.NotNil(t, reqs)

	// 1. Check skill requirements
	require.NotNil(t, reqs.Skills, "Fighter should have skill requirements")
	assert.Equal(t, choices.FighterSkills, reqs.Skills.ID)
	assert.Equal(t, 2, reqs.Skills.Count, "Fighter should choose 2 skills")
	assert.Len(t, reqs.Skills.Options, 8, "Fighter should have 8 skill options")

	// 2. Check equipment requirements
	require.NotNil(t, reqs.Equipment, "Fighter should have equipment requirements")
	assert.Len(t, reqs.Equipment, 4, "Fighter should have 4 equipment choices")

	// Verify each equipment choice
	var hasArmor, hasPrimaryWeapon, hasSecondaryWeapon, hasPack bool
	for _, eq := range reqs.Equipment {
		switch eq.ID {
		case choices.FighterArmor:
			hasArmor = true
			assert.Len(t, eq.Options, 2, "Should have 2 armor options")
		case choices.FighterWeaponsPrimary:
			hasPrimaryWeapon = true
			assert.Len(t, eq.Options, 2, "Should have 2 primary weapon options")
			// Check that martial weapon category choices are embedded
			for _, opt := range eq.Options {
				if opt.ID == "fighter-weapon-a" {
					assert.Len(t, opt.CategoryChoices, 1, "Option A should have 1 category choice")
					assert.Equal(t, 1, opt.CategoryChoices[0].Choose, "Should choose 1 martial weapon")
				} else if opt.ID == "fighter-weapon-b" {
					assert.Len(t, opt.CategoryChoices, 1, "Option B should have 1 category choice")
					assert.Equal(t, 2, opt.CategoryChoices[0].Choose, "Should choose 2 martial weapons")
				}
			}
		case choices.FighterWeaponsSecondary:
			hasSecondaryWeapon = true
			assert.Len(t, eq.Options, 2, "Should have 2 secondary weapon options")
		case choices.FighterPack:
			hasPack = true
			assert.Len(t, eq.Options, 2, "Should have 2 pack options")
		}
	}
	assert.True(t, hasArmor, "Should have armor choice")
	assert.True(t, hasPrimaryWeapon, "Should have primary weapon choice")
	assert.True(t, hasSecondaryWeapon, "Should have secondary weapon choice")
	assert.True(t, hasPack, "Should have pack choice")

	// 3. Check fighting style requirement
	require.NotNil(t, reqs.FightingStyle, "Fighter should have fighting style requirement")
	assert.Equal(t, choices.FighterFightingStyle, reqs.FightingStyle.ID)
	assert.Len(t, reqs.FightingStyle.Options, 6, "Fighter should have 6 fighting style options")

	// 4. Check that Fighter doesn't have spell requirements
	assert.Nil(t, reqs.Cantrips, "Fighter shouldn't have cantrip requirements")
	assert.Nil(t, reqs.Spellbook, "Fighter shouldn't have spellbook requirements")

	// 5. Check that subclass is NOT required at level 1 (comes at level 3)
	assert.Nil(t, reqs.Subclass, "Fighter shouldn't have subclass choice at level 1")
}

func TestFighterSubclassAtLevel3(t *testing.T) {
	// Get Fighter requirements at level 3
	reqs := choices.GetClassRequirementsAtLevel(classes.Fighter, 3)
	require.NotNil(t, reqs)

	// Should have subclass requirement at level 3
	require.NotNil(t, reqs.Subclass, "Fighter should have subclass choice at level 3")
	assert.Equal(t, choices.ChoiceID("fighter-archetype"), reqs.Subclass.ID)
	assert.Len(t, reqs.Subclass.Options, 3, "Fighter should have 3 subclass options")
}

func TestFighterRequirementsJSON(t *testing.T) {
	// Get Fighter requirements
	reqs := choices.GetClassRequirements(classes.Fighter)

	// Convert to JSON
	jsonBytes, err := json.MarshalIndent(reqs, "", "  ")
	require.NoError(t, err)

	fmt.Printf("Fighter Complete Requirements:\n%s\n", string(jsonBytes))
}
