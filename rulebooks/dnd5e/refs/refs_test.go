package refs_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/stretchr/testify/assert"
)

func TestFeaturesNamespace(t *testing.T) {
	t.Run("Rage returns correct ref", func(t *testing.T) {
		ref := refs.Features.Rage()
		assert.Equal(t, core.Module("dnd5e"), ref.Module)
		assert.Equal(t, core.Type("features"), ref.Type)
		assert.Equal(t, core.ID("rage"), ref.ID)
	})

	t.Run("SecondWind returns correct ref", func(t *testing.T) {
		ref := refs.Features.SecondWind()
		assert.Equal(t, core.Module("dnd5e"), ref.Module)
		assert.Equal(t, core.Type("features"), ref.Type)
		assert.Equal(t, core.ID("second_wind"), ref.ID)
	})
}

func TestConditionsNamespace(t *testing.T) {
	t.Run("Raging returns correct ref", func(t *testing.T) {
		ref := refs.Conditions.Raging()
		assert.Equal(t, core.Module("dnd5e"), ref.Module)
		assert.Equal(t, core.Type("conditions"), ref.Type)
		assert.Equal(t, core.ID("raging"), ref.ID)
	})

	t.Run("BrutalCritical returns correct ref", func(t *testing.T) {
		ref := refs.Conditions.BrutalCritical()
		assert.Equal(t, core.Module("dnd5e"), ref.Module)
		assert.Equal(t, core.Type("conditions"), ref.Type)
		assert.Equal(t, core.ID("brutal_critical"), ref.ID)
	})

	t.Run("UnarmoredDefense returns correct ref", func(t *testing.T) {
		ref := refs.Conditions.UnarmoredDefense()
		assert.Equal(t, core.Module("dnd5e"), ref.Module)
		assert.Equal(t, core.Type("conditions"), ref.Type)
		assert.Equal(t, core.ID("unarmored_defense"), ref.ID)
	})

	t.Run("FightingStyle returns correct ref", func(t *testing.T) {
		ref := refs.Conditions.FightingStyle()
		assert.Equal(t, core.Module("dnd5e"), ref.Module)
		assert.Equal(t, core.Type("conditions"), ref.Type)
		assert.Equal(t, core.ID("fighting_style"), ref.ID)
	})
}

func TestClassesNamespace(t *testing.T) {
	tests := []struct {
		name     string
		refFunc  func() *core.Ref
		expected core.ID
	}{
		{"Barbarian", refs.Classes.Barbarian, "barbarian"},
		{"Bard", refs.Classes.Bard, "bard"},
		{"Cleric", refs.Classes.Cleric, "cleric"},
		{"Druid", refs.Classes.Druid, "druid"},
		{"Fighter", refs.Classes.Fighter, "fighter"},
		{"Monk", refs.Classes.Monk, "monk"},
		{"Paladin", refs.Classes.Paladin, "paladin"},
		{"Ranger", refs.Classes.Ranger, "ranger"},
		{"Rogue", refs.Classes.Rogue, "rogue"},
		{"Sorcerer", refs.Classes.Sorcerer, "sorcerer"},
		{"Warlock", refs.Classes.Warlock, "warlock"},
		{"Wizard", refs.Classes.Wizard, "wizard"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ref := tc.refFunc()
			assert.Equal(t, core.Module("dnd5e"), ref.Module)
			assert.Equal(t, core.Type("classes"), ref.Type)
			assert.Equal(t, tc.expected, ref.ID)
		})
	}
}

func TestModuleConstants(t *testing.T) {
	assert.Equal(t, core.Module("dnd5e"), refs.Module)
	assert.Equal(t, core.Type("features"), refs.TypeFeatures)
	assert.Equal(t, core.Type("conditions"), refs.TypeConditions)
	assert.Equal(t, core.Type("classes"), refs.TypeClasses)
}
