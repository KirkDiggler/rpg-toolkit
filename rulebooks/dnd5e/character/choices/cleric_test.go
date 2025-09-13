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

func TestClericRequirements(t *testing.T) {
	// Get Cleric requirements at level 1
	reqs := choices.GetClassRequirementsAtLevel(classes.Cleric, 1)
	require.NotNil(t, reqs)

	// 1. Check skill requirements
	require.NotNil(t, reqs.Skills, "Cleric should have skill requirements")
	assert.Equal(t, choices.ClericSkills, reqs.Skills.ID)
	assert.Equal(t, 2, reqs.Skills.Count, "Cleric should choose 2 skills")
	assert.Len(t, reqs.Skills.Options, 5, "Cleric should have 5 skill options")

	// 2. Check equipment requirements
	require.NotNil(t, reqs.Equipment, "Cleric should have equipment requirements")
	assert.Len(t, reqs.Equipment, 5, "Cleric should have 5 equipment choices")

	// Verify each equipment choice
	var hasWeapon, hasArmor, hasSecondary, hasPack, hasHolySymbol bool
	for _, eq := range reqs.Equipment {
		switch eq.ID {
		case choices.ClericWeapons:
			hasWeapon = true
			assert.Len(t, eq.Options, 2, "Should have 2 weapon options (mace or warhammer)")
		case choices.ClericArmor:
			hasArmor = true
			assert.Len(t, eq.Options, 3, "Should have 3 armor options")
		case choices.ClericSecondaryWeapon:
			hasSecondary = true
			assert.Len(t, eq.Options, 2, "Should have 2 secondary weapon options")
			// Check that option B has category choice for simple weapon
			for _, opt := range eq.Options {
				if opt.ID == "cleric-secondary-b" {
					assert.Len(t, opt.CategoryChoices, 1, "Should have category choice for simple weapon")
				}
			}
		case choices.ClericPack:
			hasPack = true
			assert.Len(t, eq.Options, 2, "Should have 2 pack options")
		case choices.ClericHolySymbol:
			hasHolySymbol = true
			assert.Len(t, eq.Options, 1, "Should have 1 holy symbol option")
		}
	}
	assert.True(t, hasWeapon, "Should have weapon choice")
	assert.True(t, hasArmor, "Should have armor choice")
	assert.True(t, hasSecondary, "Should have secondary weapon choice")
	assert.True(t, hasPack, "Should have pack choice")
	assert.True(t, hasHolySymbol, "Should have holy symbol choice")

	// 3. Check cantrip requirements
	require.NotNil(t, reqs.Cantrips, "Cleric should have cantrip requirements")
	assert.Equal(t, choices.ClericCantrips1, reqs.Cantrips.ID)
	assert.Equal(t, 3, reqs.Cantrips.Count, "Cleric should choose 3 cantrips")
	assert.Greater(t, len(reqs.Cantrips.Options), 0, "Should have cantrip options")

	// 4. Check that Cleric doesn't have spellbook (they prepare spells)
	assert.Nil(t, reqs.Spellbook, "Cleric shouldn't have spellbook requirements")

	// 5. IMPORTANT: Check that subclass IS required at level 1 (Divine Domain)
	require.NotNil(t, reqs.Subclass, "Cleric SHOULD have subclass choice at level 1")
	assert.Equal(t, choices.ChoiceID("cleric-domain"), reqs.Subclass.ID)
	assert.Equal(t, "Divine Domain", reqs.Subclass.Label)
	assert.Len(t, reqs.Subclass.Options, 7, "Cleric should have 7 domain options")

	// Verify domains include Life, War, etc.
	hasLifeDomain := false
	hasWarDomain := false
	for _, domain := range reqs.Subclass.Options {
		if domain == classes.LifeDomain {
			hasLifeDomain = true
		}
		if domain == classes.WarDomain {
			hasWarDomain = true
		}
	}
	assert.True(t, hasLifeDomain, "Should have Life Domain option")
	assert.True(t, hasWarDomain, "Should have War Domain option")
}

func TestClericNoSubclassAtLevel0(t *testing.T) {
	// This is a bit artificial, but let's test level 0 to ensure no subclass
	reqs := choices.GetClassRequirementsAtLevel(classes.Cleric, 0)
	require.NotNil(t, reqs)

	// Should NOT have subclass requirement at level 0
	assert.Nil(t, reqs.Subclass, "Cleric shouldn't have subclass choice at level 0")
}

func TestClericRequirementsJSON(t *testing.T) {
	// Get Cleric requirements at level 1
	reqs := choices.GetClassRequirementsAtLevel(classes.Cleric, 1)

	// Convert to JSON
	jsonBytes, err := json.MarshalIndent(reqs, "", "  ")
	require.NoError(t, err)

	fmt.Printf("Cleric Level 1 Requirements (includes Divine Domain):\n%s\n", string(jsonBytes))
}
