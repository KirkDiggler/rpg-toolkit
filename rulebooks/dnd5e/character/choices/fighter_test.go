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

func TestFighterEquipmentChoices(t *testing.T) {
	// Get Fighter requirements
	reqs := choices.GetClassRequirements(classes.Fighter)
	require.NotNil(t, reqs)
	require.NotNil(t, reqs.Equipment)

	// Find the primary weapon choice
	var primaryWeaponChoice *choices.EquipmentRequirement
	for _, eq := range reqs.Equipment {
		if eq.ID == choices.FighterWeaponsPrimary {
			primaryWeaponChoice = eq
			break
		}
	}
	require.NotNil(t, primaryWeaponChoice, "Should have primary weapon choice")

	// Should have 2 options: shield+weapon or 2 weapons
	assert.Len(t, primaryWeaponChoice.Options, 2)

	// Option A: Shield + 1 martial weapon
	optionA := primaryWeaponChoice.Options[0]
	assert.Equal(t, choices.FighterWeaponMartialShield, optionA.ID)
	assert.Contains(t, optionA.Label, "martial weapon and a shield")

	// Should have 1 concrete item (shield)
	assert.Len(t, optionA.Items, 1)
	assert.Equal(t, "shield", string(optionA.Items[0].ID))

	// Should have 1 category choice for 1 martial weapon
	require.Len(t, optionA.CategoryChoices, 1)
	assert.Equal(t, 1, optionA.CategoryChoices[0].Choose)
	assert.Contains(t, optionA.CategoryChoices[0].Label, "martial weapon")

	// Option B: 2 martial weapons
	optionB := primaryWeaponChoice.Options[1]
	assert.Equal(t, choices.FighterWeaponTwoMartial, optionB.ID)
	assert.Contains(t, optionB.Label, "Two martial weapons")

	// Should have no concrete items
	assert.Len(t, optionB.Items, 0)

	// Should have 1 category choice for 2 martial weapons
	require.Len(t, optionB.CategoryChoices, 1)
	assert.Equal(t, 2, optionB.CategoryChoices[0].Choose)
	assert.Contains(t, optionB.CategoryChoices[0].Label, "two martial weapons")
}

func TestFighterEquipmentJSON(t *testing.T) {
	// Get Fighter requirements
	reqs := choices.GetClassRequirements(classes.Fighter)

	// Find the primary weapon choice
	var primaryWeaponChoice *choices.EquipmentRequirement
	for _, eq := range reqs.Equipment {
		if eq.ID == choices.FighterWeaponsPrimary {
			primaryWeaponChoice = eq
			break
		}
	}

	// Convert to JSON to see the structure
	jsonBytes, err := json.MarshalIndent(primaryWeaponChoice, "", "  ")
	require.NoError(t, err)

	// Print for visual inspection
	fmt.Printf("Fighter Primary Weapon Choice Structure:\n%s\n", string(jsonBytes))

	// Verify it round-trips correctly
	var decoded choices.EquipmentRequirement
	err = json.Unmarshal(jsonBytes, &decoded)
	require.NoError(t, err)
	assert.Equal(t, primaryWeaponChoice.ID, decoded.ID)
}
