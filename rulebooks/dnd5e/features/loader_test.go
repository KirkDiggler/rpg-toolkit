// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features_test

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadJSON_Rage(t *testing.T) {
	// Simple JSON for a rage feature
	featureJSON := `{
		"ref": "dnd5e:features:rage",
		"id": "barbarian-1-rage",
		"data": {
			"uses": 3,
			"level": 5
		}
	}`

	// Load the feature
	action, err := features.LoadJSON([]byte(featureJSON))
	require.NoError(t, err)
	require.NotNil(t, action)

	// Verify it's an Entity
	entity, ok := action.(core.Entity)
	require.True(t, ok, "rage should implement Entity")
	assert.Equal(t, "barbarian-1-rage", entity.GetID())
	assert.Equal(t, core.EntityType("feature"), entity.GetType())

	// Create a mock owner
	owner := &mockEntity{id: "conan", entityType: core.EntityType("character")}

	// Test CanActivate
	input := features.FeatureInput{}
	err = action.CanActivate(context.Background(), owner, input)
	assert.NoError(t, err, "should be able to activate rage with uses remaining")

	// Test Activate
	err = action.Activate(context.Background(), owner, input)
	assert.NoError(t, err, "should activate successfully")

	// Can't activate again while raging
	err = action.CanActivate(context.Background(), owner, input)
	assert.EqualError(t, err, "already raging")
}

func TestLoadJSON_UnknownFeature(t *testing.T) {
	featureJSON := `{
		"ref": "dnd5e:features:unknown",
		"id": "test",
		"data": {}
	}`

	action, err := features.LoadJSON([]byte(featureJSON))
	assert.Nil(t, action)
	assert.EqualError(t, err, "unknown feature: unknown")
}

func TestLoadJSON_InvalidJSON(t *testing.T) {
	action, err := features.LoadJSON([]byte("not json"))
	assert.Nil(t, action)
	assert.Error(t, err)
}

// mockEntity for testing
type mockEntity struct {
	id         string
	entityType core.EntityType
}

func (m *mockEntity) GetID() string            { return m.id }
func (m *mockEntity) GetType() core.EntityType { return m.entityType }