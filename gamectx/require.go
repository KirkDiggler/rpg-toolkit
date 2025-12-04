// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package gamectx

import "context"

// gameContextKey is the key type for storing GameContext in context.Context.
type gameContextKey struct{}

// WithGameContext wraps a context.Context with the provided GameContext.
// Purpose: Enables passing game state through the context chain during event processing.
//
// Example:
//
//	gameCtx := gamectx.NewGameContext(gamectx.GameContextConfig{
//	    CharacterRegistry: myRegistry,
//	})
//	ctx = gamectx.WithGameContext(ctx, gameCtx)
func WithGameContext(ctx context.Context, gameCtx *GameContext) context.Context {
	return context.WithValue(ctx, gameContextKey{}, gameCtx)
}

// Characters retrieves the CharacterRegistry from the context.
// Returns the registry and true if found, nil and false otherwise.
//
// Purpose: Allows conditions and features to query character state when available,
// gracefully handling cases where no GameContext is present.
//
// Example:
//
//	if registry, ok := gamectx.Characters(ctx); ok {
//	    character := registry.GetCharacter("hero-1")
//	    // ... use character data
//	}
func Characters(ctx context.Context) (CharacterRegistry, bool) {
	if gameCtx, ok := ctx.Value(gameContextKey{}).(*GameContext); ok && gameCtx != nil {
		return gameCtx.Characters(), true
	}
	return nil, false
}

// RequireCharacters retrieves the CharacterRegistry from the context.
// Panics if no GameContext is present in the context.
//
// Purpose: For code paths that absolutely require game context to function.
// Use Characters() instead if missing context is a valid scenario.
//
// Example:
//
//	registry := gamectx.RequireCharacters(ctx)
//	character := registry.GetCharacter("hero-1")
func RequireCharacters(ctx context.Context) CharacterRegistry {
	registry, ok := Characters(ctx)
	if !ok {
		panic("RequireCharacters: no GameContext found in context")
	}
	return registry
}
