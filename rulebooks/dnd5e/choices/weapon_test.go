package choices_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWeaponCategoryValidation(t *testing.T) {
	t.Run("validates weapon category selection", func(t *testing.T) {
		// Valid: longsword is a martial melee weapon
		err := choices.ValidateWeaponSelection(weapons.CategoryMartialMelee, "longsword")
		assert.NoError(t, err)

		// Invalid: longbow is not a melee weapon
		err = choices.ValidateWeaponSelection(weapons.CategoryMartialMelee, "longbow")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "weapon not in category")

		// Invalid: weapon doesn't exist
		err = choices.ValidateWeaponSelection(weapons.CategoryMartialMelee, "lightsaber")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "weapon not found")
	})
}

func TestWeaponCategoryOptions(t *testing.T) {
	t.Run("weapon category expands to actual weapons", func(t *testing.T) {
		choice := choices.Choice{
			ID:       choices.ChoiceID("test-weapon"),
			Category: choices.CategoryEquipment,
			Choose:   1,
			Options: []choices.Option{
				choices.WeaponCategoryOption{
					Category: weapons.CategoryMartialMelee,
				},
			},
		}

		// Get available options should return actual weapons
		options, err := choices.GetAvailableOptions(choice)
		require.NoError(t, err)
		require.NotEmpty(t, options)

		// Should include our martial melee weapons
		optMap := make(map[string]bool)
		for _, opt := range options {
			optMap[opt] = true
		}

		assert.True(t, optMap["longsword"])
		assert.True(t, optMap["greatsword"])
		assert.True(t, optMap["rapier"])
		assert.True(t, optMap["shortsword"])

		// Should NOT include ranged weapons
		assert.False(t, optMap["longbow"])
		assert.False(t, optMap["shortbow"])
	})

	t.Run("validates selection against weapon category", func(t *testing.T) {
		choice := choices.Choice{
			ID:       choices.ChoiceID("test-weapon"),
			Category: choices.CategoryEquipment,
			Choose:   1,
			Source:   choices.SourceClass,
			Options: []choices.Option{
				choices.WeaponCategoryOption{
					Category: weapons.CategoryMartialMelee,
				},
			},
		}

		// Valid selection
		err := choices.ValidateSelection(choice, []string{"longsword"})
		assert.NoError(t, err)

		// Invalid - wrong category
		err = choices.ValidateSelection(choice, []string{"longbow"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "selection not found")

		// Invalid - doesn't exist
		err = choices.ValidateSelection(choice, []string{"lightsaber"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "selection not found")
	})

	t.Run("fighter martial weapon choice", func(t *testing.T) {
		// This simulates a fighter choosing a martial weapon
		choice := choices.Choice{
			ID:          choices.ChoiceID("fighter-martial-weapon"),
			Category:    choices.CategoryEquipment,
			Description: "Choose a martial weapon",
			Choose:      1,
			Source:      choices.SourceClass,
			Options: []choices.Option{
				choices.WeaponCategoryOption{
					Category: weapons.CategoryMartialMelee,
				},
				choices.WeaponCategoryOption{
					Category: weapons.CategoryMartialRanged,
				},
			},
		}

		// Get all available options
		options, err := choices.GetAvailableOptions(choice)
		require.NoError(t, err)

		// Should have both melee and ranged martial weapons
		optMap := make(map[string]bool)
		for _, opt := range options {
			optMap[opt] = true
		}

		// Melee
		assert.True(t, optMap["longsword"])
		assert.True(t, optMap["greatsword"])

		// Ranged
		assert.True(t, optMap["longbow"])
		assert.True(t, optMap["heavy-crossbow"])

		// But not simple weapons
		assert.False(t, optMap["club"])
		assert.False(t, optMap["shortbow"])

		// Valid selections
		assert.NoError(t, choices.ValidateSelection(choice, []string{"longsword"}))
		assert.NoError(t, choices.ValidateSelection(choice, []string{"longbow"}))

		// Invalid selection (simple weapon)
		assert.Error(t, choices.ValidateSelection(choice, []string{"club"}))
	})
}
