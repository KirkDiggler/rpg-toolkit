// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package game provides runtime infrastructure for loading and managing game entities.
// It bridges static data structures with active game objects that participate in the event system.
//
// This package is rule-agnostic and focuses solely on infrastructure concerns like
// entity lifecycle, event bus integration, and state management patterns.
//
// Example:
//
//	// Create context with game infrastructure
//	ctx := context.Background()
//	gameCtx, err := game.NewContext(eventBus, characterData)
//	if err != nil {
//	    // Handle validation error
//	}
//
//	// Access data through getter methods
//	data := gameCtx.Data()
//	bus := gameCtx.EventBus()
//
//	// Load entity (implementation in rulebook/entity package)
//	character, err := LoadCharacterFromContext(ctx, gameCtx)
package game
