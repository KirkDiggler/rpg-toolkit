// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package gamectx

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

// combatantKey is the key type for storing combatants in context.
type combatantKey struct{}

// CombatantRegistry stores combatants for lookup during combat resolution.
type CombatantRegistry struct {
	combatants map[string]combat.Combatant
}

// NewCombatantRegistry creates a new empty combatant registry.
func NewCombatantRegistry() *CombatantRegistry {
	return &CombatantRegistry{
		combatants: make(map[string]combat.Combatant),
	}
}

// Add registers a combatant by ID.
// If a combatant with the same ID exists, it is replaced.
func (r *CombatantRegistry) Add(combatant combat.Combatant) {
	r.combatants[combatant.GetID()] = combatant
}

// Get retrieves a combatant by ID.
// Returns the combatant and nil error if found.
// Returns nil and an error if not found.
func (r *CombatantRegistry) Get(id string) (combat.Combatant, error) {
	if c, ok := r.combatants[id]; ok {
		return c, nil
	}
	return nil, rpgerr.Newf(rpgerr.CodeNotFound, "combatant %s not found in registry", id)
}

// All returns all combatants in the registry.
func (r *CombatantRegistry) All() []combat.Combatant {
	result := make([]combat.Combatant, 0, len(r.combatants))
	for _, c := range r.combatants {
		result = append(result, c)
	}
	return result
}

// WithCombatants adds a CombatantRegistry to the context.
// Purpose: Enables combat functions to look up combatants for damage application.
func WithCombatants(ctx context.Context, registry *CombatantRegistry) context.Context {
	return context.WithValue(ctx, combatantKey{}, registry)
}

// GetCombatant retrieves a combatant from context by ID.
// Returns the combatant and nil error if found.
// Returns nil and an error if no registry in context or combatant not found.
func GetCombatant(ctx context.Context, id string) (combat.Combatant, error) {
	registry, ok := ctx.Value(combatantKey{}).(*CombatantRegistry)
	if !ok || registry == nil {
		return nil, rpgerr.New(rpgerr.CodeNotFound, "no combatant registry in context")
	}
	return registry.Get(id)
}

// GetAllCombatants retrieves all combatants from context.
// Returns nil if no registry in context.
func GetAllCombatants(ctx context.Context) []combat.Combatant {
	registry, ok := ctx.Value(combatantKey{}).(*CombatantRegistry)
	if !ok || registry == nil {
		return nil
	}
	return registry.All()
}
