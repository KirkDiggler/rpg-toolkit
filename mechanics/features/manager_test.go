// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mechanics/features"
)

func TestManagerRegisterFeature(t *testing.T) {
	manager := features.NewManager()

	feature := features.NewBasicFeature("test_feature", "Test Feature")
	err := manager.RegisterFeature(feature)
	require.NoError(t, err)

	// Try to register the same feature again
	err = manager.RegisterFeature(feature)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")

	// Retrieve the feature
	retrieved, exists := manager.GetFeature("test_feature")
	assert.True(t, exists)
	assert.Equal(t, feature, retrieved)
}

func TestManagerAddFeature(t *testing.T) {
	manager := features.NewManager()
	entity := &mockEntity{id: "test-1", entityType: "character"}

	feature := features.NewBasicFeature("rage", "Rage").
		WithType(features.FeatureClass).
		WithTiming(features.TimingActivated)

	// Add feature to entity
	err := manager.AddFeature(entity, feature)
	require.NoError(t, err)

	// Try to add the same feature again
	err = manager.AddFeature(entity, feature)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already has feature")

	// Get features for the entity
	classFeatures := manager.GetFeatures(entity, features.FeatureClass)
	assert.Len(t, classFeatures, 1)
	assert.Equal(t, "rage", classFeatures[0].Key())
}

func TestManagerPassiveFeatures(t *testing.T) {
	manager := features.NewManager()
	entity := &mockEntity{id: "test-1", entityType: "character"}

	// Add a passive feature
	passiveFeature := features.NewBasicFeature("darkvision", "Darkvision").
		WithType(features.FeatureRacial).
		WithTiming(features.TimingPassive)

	err := manager.AddFeature(entity, passiveFeature)
	require.NoError(t, err)

	// Passive features should be automatically active
	activeFeatures := manager.GetActiveFeatures(entity)
	assert.Len(t, activeFeatures, 1)
	assert.Equal(t, "darkvision", activeFeatures[0].Key())
}

func TestManagerActivatedFeatures(t *testing.T) {
	manager := features.NewManager()
	entity := &mockEntity{id: "test-1", entityType: "character"}

	// Add an activated feature
	activatedFeature := features.NewBasicFeature("rage", "Rage").
		WithType(features.FeatureClass).
		WithTiming(features.TimingActivated)

	err := manager.AddFeature(entity, activatedFeature)
	require.NoError(t, err)

	// Initially, activated features should not be active
	activeFeatures := manager.GetActiveFeatures(entity)
	assert.Len(t, activeFeatures, 0)

	// Activate the feature
	err = manager.ActivateFeature(entity, "rage")
	require.NoError(t, err)

	// Now it should be active
	activeFeatures = manager.GetActiveFeatures(entity)
	assert.Len(t, activeFeatures, 1)
	assert.Equal(t, "rage", activeFeatures[0].Key())

	// Deactivate the feature
	err = manager.DeactivateFeature(entity, "rage")
	require.NoError(t, err)

	// Should no longer be active
	activeFeatures = manager.GetActiveFeatures(entity)
	assert.Len(t, activeFeatures, 0)
}

func TestManagerRemoveFeature(t *testing.T) {
	manager := features.NewManager()
	entity := &mockEntity{id: "test-1", entityType: "character"}

	feature := features.NewBasicFeature("test_feature", "Test Feature")

	// Add the feature
	err := manager.AddFeature(entity, feature)
	require.NoError(t, err)

	// Remove the feature
	err = manager.RemoveFeature(entity, "test_feature")
	require.NoError(t, err)

	// Try to remove a non-existent feature
	err = manager.RemoveFeature(entity, "non_existent")
	assert.Error(t, err)

	// Verify the feature is gone
	features := manager.GetFeatures(entity, features.FeatureClass)
	assert.Len(t, features, 0)
}

func TestManagerGetAvailableFeatures(t *testing.T) {
	manager := features.NewManager()
	entity := &mockEntity{id: "test-1", entityType: "character"}

	// Register some features
	level1Feature := features.NewBasicFeature("feature1", "Level 1 Feature").
		WithLevel(1)
	level5Feature := features.NewBasicFeature("feature5", "Level 5 Feature").
		WithLevel(5)

	err := manager.RegisterFeature(level1Feature)
	require.NoError(t, err)
	err = manager.RegisterFeature(level5Feature)
	require.NoError(t, err)

	// At level 3, only level 1 feature should be available
	available := manager.GetAvailableFeatures(entity, 3)
	assert.Len(t, available, 1)
	assert.Equal(t, "feature1", available[0].Key())

	// At level 5, both features should be available
	available = manager.GetAvailableFeatures(entity, 5)
	assert.Len(t, available, 2)

	// Add level 1 feature to entity
	err = manager.AddFeature(entity, level1Feature)
	require.NoError(t, err)

	// Now only level 5 feature should be available at level 5
	available = manager.GetAvailableFeatures(entity, 5)
	assert.Len(t, available, 1)
	assert.Equal(t, "feature5", available[0].Key())
}

func TestManagerActivateNonActivatedFeature(t *testing.T) {
	manager := features.NewManager()
	entity := &mockEntity{id: "test-1", entityType: "character"}

	// Add a passive feature
	passiveFeature := features.NewBasicFeature("passive", "Passive").
		WithTiming(features.TimingPassive)

	err := manager.AddFeature(entity, passiveFeature)
	require.NoError(t, err)

	// Try to activate a passive feature
	err = manager.ActivateFeature(entity, "passive")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not an activated feature")
}
