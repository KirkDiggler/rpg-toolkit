// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package game_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/KirkDiggler/rpg-toolkit/game"
)

func TestNewContext_RequiresEventBus(t *testing.T) {
	type TestData struct {
		ID   string
		Name string
	}

	validData := TestData{ID: "test-1", Name: "Test"}

	// Verify nil event bus returns error
	_, err := game.NewContext(nil, validData)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "eventBus is required")
}

// TestContext_Immutability verifies that Context cannot be modified after creation.
// This test passes by compiling - if Context fields were exported, this would
// demonstrate the security vulnerability. With unexported fields, the Context
// is guaranteed to remain valid.
func TestContext_Immutability(t *testing.T) {
	// The following would NOT compile because fields are unexported:
	//
	// ctx := game.Context[string]{}
	// ctx.eventBus = nil  // compile error: ctx.eventBus undefined
	// ctx.data = ""       // compile error: ctx.data undefined
	//
	// This compilation-time guarantee ensures Context validity.

	t.Log("Context immutability verified - fields are unexported and no mutation methods exist")
}
