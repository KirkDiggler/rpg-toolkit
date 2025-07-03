// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/conditions"
)

// Basic smoke test to ensure package compiles
func TestPackageCompiles(t *testing.T) {
	// This just ensures our package can be imported and basic types work
	bus := events.NewBus()
	_ = conditions.NewRelationshipManager(bus)
	
	// SimpleCondition can be created
	_ = conditions.NewSimpleCondition(conditions.SimpleConditionConfig{
		ID:     "test",
		Type:   "test",
		Target: nil,
		Source: "test",
	})
}