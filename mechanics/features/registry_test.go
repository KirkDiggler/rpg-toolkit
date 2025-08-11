// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/features"
)

func TestRegistryRegisterFeature(t *testing.T) {
	registry := features.NewRegistry()

	feature := features.NewBasicFeature(core.MustNewRef("test_feature", "test", "feature"), "Test Feature")
	err := registry.RegisterFeature(feature)
	require.NoError(t, err)

	// Try to register the same feature again
	err = registry.RegisterFeature(feature)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")

	// Retrieve the feature
	retrieved, exists := registry.GetFeature("test:feature:test_feature")
	assert.True(t, exists)
	assert.Equal(t, feature, retrieved)
}

func TestRegistryGetFeaturesByType(t *testing.T) {
	registry := features.NewRegistry()

	// Register features of different types
	classFeature := features.NewBasicFeature(core.MustNewRef("rage", "dnd5e", "class_feature"), "Rage").
		WithType(features.FeatureClass)
	racialFeature := features.NewBasicFeature(core.MustNewRef("darkvision", "dnd5e", "racial_feature"), "Darkvision").
		WithType(features.FeatureRacial)

	err := registry.RegisterFeature(classFeature)
	require.NoError(t, err)
	err = registry.RegisterFeature(racialFeature)
	require.NoError(t, err)

	// Get class features
	classFeatures := registry.GetFeaturesByType(features.FeatureClass)
	assert.Len(t, classFeatures, 1)
	assert.Equal(t, core.MustNewRef("rage", "dnd5e", "class_feature"), classFeatures[0].Key())

	// Get racial features
	racialFeatures := registry.GetFeaturesByType(features.FeatureRacial)
	assert.Len(t, racialFeatures, 1)
	assert.Equal(t, core.MustNewRef("darkvision", "dnd5e", "racial_feature"), racialFeatures[0].Key())
}

func TestRegistryGetFeaturesForClass(t *testing.T) {
	registry := features.NewRegistry()

	// Register features for different classes
	barbarianFeature := features.NewBasicFeature(core.MustNewRef("rage", "dnd5e", "class_feature"), "Rage").
		WithType(features.FeatureClass).
		WithLevel(1).
		WithPrerequisites("class:barbarian")

	rogueFeature := features.NewBasicFeature(core.MustNewRef("sneak_attack", "dnd5e", "class_feature"), "Sneak Attack").
		WithType(features.FeatureClass).
		WithLevel(1).
		WithPrerequisites("class:rogue")

	err := registry.RegisterFeature(barbarianFeature)
	require.NoError(t, err)
	err = registry.RegisterFeature(rogueFeature)
	require.NoError(t, err)

	// Get barbarian features
	barbFeatures := registry.GetFeaturesForClass("barbarian", 5)
	assert.Len(t, barbFeatures, 1)
	assert.Equal(t, core.MustNewRef("rage", "dnd5e", "class_feature"), barbFeatures[0].Key())

	// Get rogue features
	rogueFeatures := registry.GetFeaturesForClass("rogue", 5)
	assert.Len(t, rogueFeatures, 1)
	assert.Equal(t, core.MustNewRef("sneak_attack", "dnd5e", "class_feature"), rogueFeatures[0].Key())
}

func TestRegistryGetFeaturesForRace(t *testing.T) {
	registry := features.NewRegistry()

	// Register racial features
	halfOrcFeature := features.NewBasicFeature(core.MustNewRef("darkvision", "dnd5e", "racial_feature"), "Darkvision").
		WithType(features.FeatureRacial).
		WithPrerequisites("race:half-orc")

	drowFeature := features.NewBasicFeature(core.MustNewRef("superior_darkvision", "dnd5e", "racial_feature"), "Superior Darkvision").
		WithType(features.FeatureRacial).
		WithPrerequisites("race:drow")

	err := registry.RegisterFeature(halfOrcFeature)
	require.NoError(t, err)
	err = registry.RegisterFeature(drowFeature)
	require.NoError(t, err)

	// Get half-orc features
	halfOrcFeatures := registry.GetFeaturesForRace("half-orc")
	assert.Len(t, halfOrcFeatures, 1)
	assert.Equal(t, core.MustNewRef("darkvision", "dnd5e", "racial_feature"), halfOrcFeatures[0].Key())

	// Get drow features
	drowFeatures := registry.GetFeaturesForRace("drow")
	assert.Len(t, drowFeatures, 1)
	assert.Equal(t, core.MustNewRef("superior_darkvision", "dnd5e", "racial_feature"), drowFeatures[0].Key())
}
