// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/features"
)

func TestSimpleFeatureHolder(t *testing.T) {
	holder := features.NewSimpleFeatureHolder()

	// Add a feature
	feature := features.NewBasicFeature("test_feature", "Test Feature")
	err := holder.AddFeature(feature)
	require.NoError(t, err)

	// Try to add the same feature again
	err = holder.AddFeature(feature)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	// Get the feature
	retrieved, exists := holder.GetFeature("test_feature")
	assert.True(t, exists)
	assert.Equal(t, feature, retrieved)

	// Get all features
	allFeatures := holder.GetFeatures()
	assert.Len(t, allFeatures, 1)

	// Remove the feature
	err = holder.RemoveFeature("test_feature")
	require.NoError(t, err)

	// Try to get removed feature
	_, exists = holder.GetFeature("test_feature")
	assert.False(t, exists)
}

func TestFeatureHolderActivation(t *testing.T) {
	holder := features.NewSimpleFeatureHolder()
	bus := events.NewBus()

	// Add an activated feature
	activatedFeature := features.NewBasicFeature("rage", "Rage").
		WithType(features.FeatureClass).
		WithTiming(features.TimingActivated)

	err := holder.AddFeature(activatedFeature)
	require.NoError(t, err)

	// Initially, activated features should not be active
	activeFeatures := holder.GetActiveFeatures()
	assert.Len(t, activeFeatures, 0)

	// Activate the feature
	err = holder.ActivateFeature("rage", bus)
	require.NoError(t, err)

	// Now it should be active
	activeFeatures = holder.GetActiveFeatures()
	assert.Len(t, activeFeatures, 1)

	// Deactivate the feature
	err = holder.DeactivateFeature("rage", bus)
	require.NoError(t, err)

	// Should no longer be active
	activeFeatures = holder.GetActiveFeatures()
	assert.Len(t, activeFeatures, 0)
}

func TestFeatureHolderPassiveFeatures(t *testing.T) {
	holder := features.NewSimpleFeatureHolder()

	// Add a passive feature
	passiveFeature := features.NewBasicFeature("darkvision", "Darkvision").
		WithType(features.FeatureRacial).
		WithTiming(features.TimingPassive)

	err := holder.AddFeature(passiveFeature)
	require.NoError(t, err)

	// Passive features should be automatically active
	activeFeatures := holder.GetActiveFeatures()
	assert.Len(t, activeFeatures, 1)
	assert.Equal(t, "darkvision", activeFeatures[0].Key())
}
