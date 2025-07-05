// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/features"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/resources"
)

// mockEntity implements core.Entity for testing
type mockEntity struct {
	id         string
	entityType string
}

func (m *mockEntity) GetID() string   { return m.id }
func (m *mockEntity) GetType() string { return m.entityType }

func TestBasicFeature(t *testing.T) {
	feature := features.NewBasicFeature("test_feature", "Test Feature").
		WithDescription("A test feature").
		WithType(features.FeatureClass).
		WithLevel(1).
		WithSource("Test Class").
		WithTiming(features.TimingPassive)

	assert.Equal(t, "test_feature", feature.Key())
	assert.Equal(t, "Test Feature", feature.Name())
	assert.Equal(t, "A test feature", feature.Description())
	assert.Equal(t, features.FeatureClass, feature.Type())
	assert.Equal(t, 1, feature.Level())
	assert.Equal(t, "Test Class", feature.Source())
	assert.True(t, feature.IsPassive())
	assert.Equal(t, features.TimingPassive, feature.GetTiming())
}

func TestFeatureWithModifiers(t *testing.T) {
	modifier := events.NewModifier(
		"test_mod",
		events.ModifierAttackBonus,
		events.NewRawValue(2, "test"),
		100,
	)

	feature := features.NewBasicFeature("mod_feature", "Modifier Feature").
		WithModifiers(modifier)

	mods := feature.GetModifiers()
	require.Len(t, mods, 1)
	assert.Equal(t, "test_mod", mods[0].Source())
}

func TestFeatureWithProficiencies(t *testing.T) {
	feature := features.NewBasicFeature("prof_feature", "Proficiency Feature").
		WithProficiencies("longsword", "shortsword", "shields")

	profs := feature.GetProficiencies()
	assert.Len(t, profs, 3)
	assert.Contains(t, profs, "longsword")
	assert.Contains(t, profs, "shortsword")
	assert.Contains(t, profs, "shields")
}

func TestFeatureWithResources(t *testing.T) {
	resource := resources.NewSimpleResource(resources.SimpleResourceConfig{
		ID:      "test_uses_1",
		Type:    resources.ResourceTypeAbilityUse,
		Key:     "test_uses",
		Current: 3,
		Maximum: 3,
	})

	feature := features.NewBasicFeature("resource_feature", "Resource Feature").
		WithResources(resource)

	res := feature.GetResources()
	require.Len(t, res, 1)
	assert.Equal(t, "test_uses", res[0].Key())
}

func TestFeatureWithPrerequisites(t *testing.T) {
	feature := features.NewBasicFeature("prereq_feature", "Prerequisite Feature").
		WithPrerequisites("class:fighter", "level:5", "feat:weapon_master")

	assert.True(t, feature.HasPrerequisites())
	prereqs := feature.GetPrerequisites()
	assert.Len(t, prereqs, 3)
	assert.Contains(t, prereqs, "class:fighter")

	// Test that without a prerequisite checker, MeetsPrerequisites returns false
	mockEntity := &mockEntity{id: "test", entityType: "character"}
	assert.False(t, feature.MeetsPrerequisites(mockEntity))
}

func TestFeaturePrerequisiteChecker(t *testing.T) {
	// Create a feature with prerequisites
	feature := features.NewBasicFeature("prereq_feature", "Prerequisite Feature").
		WithPrerequisites("class:fighter", "level:5")

	// Create a mock entity
	mockEntity := &mockEntity{id: "test", entityType: "character"}

	// Without checker, should return false
	assert.False(t, feature.MeetsPrerequisites(mockEntity))

	// Add a custom prerequisite checker
	checker := func(_ core.Entity, prereq string) bool {
		switch prereq {
		case "class:fighter":
			// In a real game, you'd check the entity's class
			return true
		case "level:5":
			// In a real game, you'd check the entity's level
			return true
		default:
			return false
		}
	}

	feature.WithPrerequisiteChecker(checker)

	// Now should return true
	assert.True(t, feature.MeetsPrerequisites(mockEntity))

	// Test with failing prerequisites
	failingChecker := func(_ core.Entity, prereq string) bool {
		if prereq == "level:5" {
			return false // Character is not level 5
		}
		return true
	}

	feature.WithPrerequisiteChecker(failingChecker)
	assert.False(t, feature.MeetsPrerequisites(mockEntity))
}

func TestTriggeredFeature(t *testing.T) {
	// Create a mock event listener
	listener := &mockEventListener{
		eventTypes: []string{events.EventOnAttackRoll},
		priority:   100,
		called:     false,
	}

	feature := features.NewBasicFeature("triggered_feature", "Triggered Feature").
		WithTiming(features.TimingTriggered).
		WithEventListeners(listener)

	// Create a mock event
	mockEvent := events.NewGameEvent(events.EventOnAttackRoll, nil, nil)

	// Feature should be able to trigger on this event
	assert.True(t, feature.CanTrigger(mockEvent))

	// Trigger the feature
	entity := &mockEntity{id: "test-1", entityType: "character"}
	err := feature.TriggerFeature(entity, mockEvent)
	require.NoError(t, err)
	assert.True(t, listener.called)
}

func TestPassiveFeatureIsAlwaysActive(t *testing.T) {
	feature := features.NewBasicFeature("passive_feature", "Passive Feature").
		WithTiming(features.TimingPassive)

	entity := &mockEntity{id: "test-1", entityType: "character"}
	assert.True(t, feature.IsActive(entity))
}

// mockEventListener for testing
type mockEventListener struct {
	eventTypes []string
	priority   int
	called     bool
}

func (m *mockEventListener) EventTypes() []string {
	return m.eventTypes
}

func (m *mockEventListener) Priority() int {
	return m.priority
}

func (m *mockEventListener) HandleEvent(_ features.Feature, _ core.Entity, _ events.Event) error {
	m.called = true
	return nil
}
