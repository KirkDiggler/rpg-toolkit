// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package gamectx

import (
	"context"
	"errors"
)

// ErrNoGameContext is returned when a required GameContext is not found in context.
var ErrNoGameContext = errors.New("no GameContext found in context")

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
// Returns an error if no GameContext is present in the context.
//
// Purpose: For code paths that require game context to function and need
// explicit error handling rather than silent failures.
//
// Example:
//
//	registry, err := gamectx.RequireCharacters(ctx)
//	if err != nil {
//	    return c, err
//	}
//	character := registry.GetCharacter("hero-1")
func RequireCharacters(ctx context.Context) (CharacterRegistry, error) {
	registry, ok := Characters(ctx)
	if !ok {
		return nil, ErrNoGameContext
	}
	return registry, nil
}
