// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/conditions"
)

func TestBasicManagerCreation(t *testing.T) {
	bus := events.NewBus()
	manager := conditions.NewConditionManager(bus)
	assert.NotNil(t, manager)
}

func TestRegistrationOnly(t *testing.T) {
	conditions.RegisterConditionDefinition(&conditions.ConditionDefinition{
		Type:        conditions.ConditionType("test"),
		Name:        "Test",
		Description: "Test condition",
		Effects:     []conditions.ConditionEffect{},
	})

	def, exists := conditions.GetConditionDefinition(conditions.ConditionType("test"))
	assert.True(t, exists)
	assert.Equal(t, "Test", def.Name)
}