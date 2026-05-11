// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package gamectx

import "context"

// reactionReadinessKey is the context key for the reaction readiness map.
type reactionReadinessKey struct{}

// ReactionReadinessMap is a read-only view of per-entity reaction readiness.
// Keys are entity IDs; values are maps from reaction ref strings
// (e.g. "dnd5e:conditions:opportunity_attack") to ready booleans.
//
// Returned by IsReactionReady when the context carries this value.
// Populated by the encounter SDK (rpg-api side) before invoking attack chains.
type ReactionReadinessMap map[string]map[string]bool

// WithReactionReadiness wraps a context.Context with the provided readiness map.
// Purpose: Enables condition handlers to call IsReactionReady without coupling
// to rpg-api or the encounter SDK.
//
// The caller (rpg-api's CombatResolver) is responsible for passing the
// encounter's reaction readiness map into the context before invoking the
// rulebook attack chain.
//
// Example:
//
//	ctx = gamectx.WithReactionReadiness(ctx, gamectx.ReactionReadinessMap{
//	    "char-alice": {"dnd5e:conditions:opportunity_attack": true},
//	})
func WithReactionReadiness(ctx context.Context, m ReactionReadinessMap) context.Context {
	return context.WithValue(ctx, reactionReadinessKey{}, m)
}

// IsReactionReady reports whether the named reaction is currently ready for
// the given entity. Returns false when:
//   - no ReactionReadinessMap is present in the context, OR
//   - the entity is not in the map, OR
//   - the reaction ref is not in the entity's inner map.
//
// Not-ready is the safe default — reactions that haven't been explicitly
// readied must never fire prompts, preventing accidental spell-slot burns.
//
// Condition handlers call this before emitting a ReactionTrigger event:
//
//	if !gamectx.IsReactionReady(ctx, s.CharacterID, refs.Conditions.Shield().String()) {
//	    return c, nil // reaction not readied; run single-phase end-to-end
//	}
func IsReactionReady(ctx context.Context, charID, reactionRef string) bool {
	m, ok := ctx.Value(reactionReadinessKey{}).(ReactionReadinessMap)
	if !ok || m == nil {
		return false
	}
	inner, ok := m[charID]
	if !ok {
		return false
	}
	return inner[reactionRef]
}
