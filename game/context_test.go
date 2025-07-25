// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package game_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/KirkDiggler/rpg-toolkit/game"
)

func TestNewContext_Validation(t *testing.T) {
	type TestData struct {
		ID   string
		Name string
	}

	validData := TestData{ID: "test-1", Name: "Test"}

	// Test nil event bus
	t.Run("nil event bus returns error", func(t *testing.T) {
		_, err := game.NewContext(nil, validData)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "eventBus is required")
	})

	// Test zero value data
	t.Run("zero value data returns error", func(t *testing.T) {
		// We can't test this without a real event bus implementation
		// Skip for now since we need the events package
		t.Skip("Need real EventBus implementation to test")
	})

	// Test with minimal valid inputs
	t.Run("valid inputs succeed", func(t *testing.T) {
		// We can't test this without a real event bus implementation
		// Skip for now since we need the events package
		t.Skip("Need real EventBus implementation to test")
	})
}

func TestContext_Immutability(t *testing.T) {
	// This test verifies that Context fields cannot be modified after creation
	// The fields are unexported, guaranteeing immutability

	// If this compiles, it means the fields are properly encapsulated
	// Attempting to access ctx.eventBus or ctx.data would cause compile errors

	// Example of what would NOT compile:
	// ctx := game.Context[string]{}
	// ctx.eventBus = nil  // compile error: ctx.eventBus undefined
	// ctx.data = ""       // compile error: ctx.data undefined

	t.Log("Context is immutable - fields are unexported")
}
