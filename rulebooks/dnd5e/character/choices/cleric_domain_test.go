package choices_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClericWarDomain(t *testing.T) {
	// Get Cleric requirements with War Domain
	reqs := choices.GetClassRequirementsWithSubclass(classes.Cleric, 1, classes.WarDomain)
	require.NotNil(t, reqs)

	// War Domain should add martial weapon option to weapon choices
	var weaponReq *choices.EquipmentRequirement
	for _, eq := range reqs.Equipment {
		if eq.ID == choices.ClericWeapons {
			weaponReq = eq
			break
		}
	}
	require.NotNil(t, weaponReq, "Should have weapon requirement")

	// Should have 3 options now: mace, warhammer, and martial weapon
	assert.Len(t, weaponReq.Options, 3, "War Domain should add martial weapon option")

	// Find the War Domain martial weapon option
	var hasWarOption bool
	for _, opt := range weaponReq.Options {
		if opt.ID == "cleric-weapon-war" {
			hasWarOption = true
			assert.Equal(t, "martial weapon (War Domain)", opt.Label)
			require.Len(t, opt.CategoryChoices, 1)
			assert.Contains(t, opt.CategoryChoices[0].Categories, weapons.CategoryMartialMelee)
			assert.Contains(t, opt.CategoryChoices[0].Categories, weapons.CategoryMartialRanged)
			assert.Equal(t, shared.EquipmentTypeWeapon, opt.CategoryChoices[0].Type)
			assert.Equal(t, 1, opt.CategoryChoices[0].Choose)
			break
		}
	}
	assert.True(t, hasWarOption, "Should have War Domain martial weapon option")
}

func TestClericLifeDomain(t *testing.T) {
	// Get Cleric requirements with Life Domain
	reqs := choices.GetClassRequirementsWithSubclass(classes.Cleric, 1, classes.LifeDomain)
	require.NotNil(t, reqs)

	// Life Domain should add heavy armor option to armor choices
	var armorReq *choices.EquipmentRequirement
	for _, eq := range reqs.Equipment {
		if eq.ID == choices.ClericArmor {
			armorReq = eq
			break
		}
	}
	require.NotNil(t, armorReq, "Should have armor requirement")

	// Should have 4 options now: leather, scale mail, chain shirt (if proficient), and chain mail (Life Domain)
	assert.Len(t, armorReq.Options, 4, "Life Domain should add heavy armor option")

	// Find the Life Domain chain mail option
	var hasLifeOption bool
	for _, opt := range armorReq.Options {
		if opt.ID == "cleric-armor-life" {
			hasLifeOption = true
			assert.Equal(t, "chain mail (Life Domain)", opt.Label)
			require.Len(t, opt.Items, 1)
			// Note: We'd need to verify the item is chain mail, but that requires armor.ChainMail constant
			break
		}
	}
	assert.True(t, hasLifeOption, "Should have Life Domain chain mail option")
}

func TestClericTempestDomain(t *testing.T) {
	// Get Cleric requirements with Tempest Domain
	reqs := choices.GetClassRequirementsWithSubclass(classes.Cleric, 1, classes.TempestDomain)
	require.NotNil(t, reqs)

	// Tempest Domain should add both martial weapon and heavy armor options

	// Check weapon requirement
	var weaponReq *choices.EquipmentRequirement
	for _, eq := range reqs.Equipment {
		if eq.ID == choices.ClericWeapons {
			weaponReq = eq
			break
		}
	}
	require.NotNil(t, weaponReq, "Should have weapon requirement")
	assert.Len(t, weaponReq.Options, 3, "Tempest Domain should add martial weapon option")

	var hasTempestWeapon bool
	for _, opt := range weaponReq.Options {
		if opt.ID == "cleric-weapon-tempest" {
			hasTempestWeapon = true
			assert.Equal(t, "martial weapon (Tempest Domain)", opt.Label)
			break
		}
	}
	assert.True(t, hasTempestWeapon, "Should have Tempest Domain martial weapon option")

	// Check armor requirement
	var armorReq *choices.EquipmentRequirement
	for _, eq := range reqs.Equipment {
		if eq.ID == choices.ClericArmor {
			armorReq = eq
			break
		}
	}
	require.NotNil(t, armorReq, "Should have armor requirement")
	assert.Len(t, armorReq.Options, 4, "Tempest Domain should add heavy armor option")

	var hasTempestArmor bool
	for _, opt := range armorReq.Options {
		if opt.ID == "cleric-armor-tempest" {
			hasTempestArmor = true
			assert.Equal(t, "chain mail (Tempest Domain)", opt.Label)
			break
		}
	}
	assert.True(t, hasTempestArmor, "Should have Tempest Domain chain mail option")
}

func TestClericLightDomain(t *testing.T) {
	// Get Cleric requirements with Light Domain
	reqs := choices.GetClassRequirementsWithSubclass(classes.Cleric, 1, classes.LightDomain)
	require.NotNil(t, reqs)

	// Light Domain doesn't modify equipment, just grants the light cantrip
	// Equipment should be the same as base Cleric

	// Check weapon requirement - should have only base 2 options
	var weaponReq *choices.EquipmentRequirement
	for _, eq := range reqs.Equipment {
		if eq.ID == choices.ClericWeapons {
			weaponReq = eq
			break
		}
	}
	require.NotNil(t, weaponReq, "Should have weapon requirement")
	assert.Len(t, weaponReq.Options, 2, "Light Domain shouldn't add weapon options")

	// Check armor requirement - should have only base 3 options
	var armorReq *choices.EquipmentRequirement
	for _, eq := range reqs.Equipment {
		if eq.ID == choices.ClericArmor {
			armorReq = eq
			break
		}
	}
	require.NotNil(t, armorReq, "Should have armor requirement")
	assert.Len(t, armorReq.Options, 3, "Light Domain shouldn't add armor options")
}
