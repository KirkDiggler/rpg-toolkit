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

func TestWizardSpellRequirements(t *testing.T) {
	// Get Wizard requirements
	reqs := choices.GetClassRequirements(classes.Wizard)
	require.NotNil(t, reqs)

	// Test cantrip requirements
	require.NotNil(t, reqs.Cantrips, "Wizard should have cantrip requirements")
	assert.Equal(t, choices.WizardCantrips1, reqs.Cantrips.ID)
	assert.Equal(t, 3, reqs.Cantrips.Count, "Wizard should choose 3 cantrips")
	assert.Greater(t, len(reqs.Cantrips.Options), 0, "Should have cantrip options")
	assert.Contains(t, reqs.Cantrips.Label, "3 cantrips")

	// Test spellbook requirements
	require.NotNil(t, reqs.Spellbook, "Wizard should have spellbook requirements")
	assert.Equal(t, choices.WizardSpells1, reqs.Spellbook.ID)
	assert.Equal(t, 6, reqs.Spellbook.Count, "Wizard should choose 6 spells")
	assert.Equal(t, 1, reqs.Spellbook.SpellLevel, "Should be 1st level spells")
	assert.Greater(t, len(reqs.Spellbook.Options), 0, "Should have spell options")
	assert.Contains(t, reqs.Spellbook.Label, "6 1st-level spells")
}

func TestWizardRequirementsJSON(t *testing.T) {
	// Get Wizard requirements
	reqs := choices.GetClassRequirements(classes.Wizard)

	// Convert to JSON to see the structure
	jsonBytes, err := json.MarshalIndent(reqs, "", "  ")
	require.NoError(t, err)

	// Print for visual inspection (partial - just spell choices)
	var data map[string]interface{}
	err = json.Unmarshal(jsonBytes, &data)
	require.NoError(t, err)

	// Extract just the spell-related parts
	spellChoices := map[string]interface{}{
		"cantrips":  data["cantrips"],
		"spellbook": data["spellbook"],
	}

	spellJSON, err := json.MarshalIndent(spellChoices, "", "  ")
	require.NoError(t, err)

	fmt.Printf("Wizard Spell Choice Structure:\n%s\n", string(spellJSON))
}
