// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package gamectx

import "context"

// CombatState provides combat context during event processing.
// Purpose: Enables conditions and features to query current combat state
// (e.g., current round for Rage turn tracking, active entity for reaction timing).
type CombatState struct {
	// EncounterID is the unique identifier for the current encounter
	EncounterID string

	// Round is the current combat round number (1-indexed)
	Round int

	// ActiveEntityID is the ID of the entity whose turn it currently is
	ActiveEntityID string

	// ActiveEntityType is the type of the active entity ("character" or "monster")
	ActiveEntityType string
}

// combatStateKey is the key type for storing CombatState in context.Context.
type combatStateKey struct{}

// WithCombatState wraps a context.Context with the provided CombatState.
// Purpose: Enables features and conditions to query combat state during event processing.
//
// Example:
//
//	state := &gamectx.CombatState{
//	    EncounterID: "enc-123",
//	    Round: 3,
//	    ActiveEntityID: "hero-1",
//	    ActiveEntityType: "character",
//	}
//	ctx = gamectx.WithCombatState(ctx, state)
func WithCombatState(ctx context.Context, state *CombatState) context.Context {
	return context.WithValue(ctx, combatStateKey{}, state)
}

// CombatStateFromContext retrieves the CombatState from the context.
// Returns the state and true if found, nil and false otherwise.
//
// Purpose: Allows conditions and features to query combat state when available,
// gracefully handling cases where no CombatState is present.
//
// Example:
//
//	if state, ok := gamectx.CombatStateFromContext(ctx); ok {
//	    if state.Round > 1 {
//	        // Rage has been active for multiple rounds
//	    }
//	}
func CombatStateFromContext(ctx context.Context) (*CombatState, bool) {
	if state, ok := ctx.Value(combatStateKey{}).(*CombatState); ok && state != nil {
		return state, true
	}
	return nil, false
}

// RequireCombatState retrieves the CombatState from the context.
// Panics if no CombatState is present in the context.
//
// Purpose: For code paths that absolutely require combat state to function.
// Use CombatStateFromContext() instead if missing state is a valid scenario.
//
// Example:
//
//	state := gamectx.RequireCombatState(ctx)
//	currentRound := state.Round
func RequireCombatState(ctx context.Context) *CombatState {
	state, ok := CombatStateFromContext(ctx)
	if !ok {
		panic("RequireCombatState: no CombatState found in context")
	}
	return state
}
