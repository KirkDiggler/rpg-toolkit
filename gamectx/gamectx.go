// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package gamectx provides context wrapping for game state during event processing.
// Purpose: Enables conditions and features to query loaded game state (equipment,
// characters, spatial data) without bloating event objects with all possible data.
package gamectx

// CharacterRegistry provides access to character state during event processing.
// Purpose: Allows conditions and features to query equipped items, weapons, and other
// character state without bloating event objects with all possible data.
type CharacterRegistry interface {
	// GetCharacter retrieves a character by ID
	// Returns nil if character is not found
	GetCharacter(id string) interface{}
}

// GameContext carries game state through context.Context for use during event processing.
// Purpose: Provides access to loaded game state (characters, equipment, spatial data)
// without requiring every event to carry all possible data.
//
// This enables conditions and features to make intelligent decisions based on current
// game state, such as the Dueling fighting style checking equipped weapons.
type GameContext struct {
	// characterRegistry provides access to character state
	characterRegistry CharacterRegistry
}

// GameContextConfig configures a new GameContext.
type GameContextConfig struct {
	// CharacterRegistry provides access to character state during event processing
	CharacterRegistry CharacterRegistry
}

// NewGameContext creates a new GameContext with the specified configuration.
// If no CharacterRegistry is provided, a default empty registry is used.
func NewGameContext(config GameContextConfig) *GameContext {
	registry := config.CharacterRegistry
	if registry == nil {
		registry = &emptyCharacterRegistry{}
	}

	return &GameContext{
		characterRegistry: registry,
	}
}

// Characters returns the CharacterRegistry for this GameContext.
// Purpose: Provides access to character state for conditions and features
// that need to query equipped items, weapons, or other character data.
func (g *GameContext) Characters() CharacterRegistry {
	return g.characterRegistry
}

// emptyCharacterRegistry is a default implementation that returns nil for all lookups.
type emptyCharacterRegistry struct{}

// GetCharacter always returns nil for the empty registry.
func (e *emptyCharacterRegistry) GetCharacter(_ string) interface{} {
	return nil
}
