// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package gamectx

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
)

// ApplyDamageInput contains parameters for applying damage to a combatant.
// Supports multiple damage instances for attacks with multiple damage types
// (e.g., flametongue sword deals slashing + fire).
type ApplyDamageInput struct {
	// Instances contains individual damage amounts per type.
	// Each instance is applied separately so resistance can work per-type.
	Instances []DamageInstance

	// IsCritical indicates if this damage came from a critical hit.
	// Some features react differently to crits (e.g., Uncanny Dodge can't reduce crit damage from certain sources).
	IsCritical bool
}

// DamageInstance represents a single damage amount with its type.
// Multiple instances allow mixed-type damage (fire + slashing).
type DamageInstance struct {
	// Amount is the damage to apply (after modifiers, before resistance)
	Amount int

	// Type is the damage type (slashing, fire, etc.)
	// This is a string to avoid circular imports with the damage package.
	Type string
}

// ApplyDamageResult contains the outcome of applying damage.
type ApplyDamageResult struct {
	// TotalDamage is the sum of all damage applied
	TotalDamage int

	// CurrentHP is the combatant's HP after damage
	CurrentHP int

	// DroppedToZero is true if this damage reduced HP to 0
	DroppedToZero bool

	// PreviousHP is the HP before damage was applied
	PreviousHP int
}

// Combatant represents an entity that can take damage in combat.
// Both Character and Monster implement this interface.
type Combatant interface {
	// GetID returns the combatant's unique identifier
	GetID() string

	// GetHitPoints returns current HP
	GetHitPoints() int

	// GetMaxHitPoints returns maximum HP
	GetMaxHitPoints() int

	// ApplyDamage reduces HP by the damage amount(s).
	// HP cannot go below 0.
	ApplyDamage(ctx context.Context, input *ApplyDamageInput) *ApplyDamageResult
}

// combatantKey is the key type for storing combatants in context.
type combatantKey struct{}

// CombatantRegistry stores combatants for lookup during combat resolution.
type CombatantRegistry struct {
	combatants map[string]Combatant
}

// NewCombatantRegistry creates a new empty combatant registry.
func NewCombatantRegistry() *CombatantRegistry {
	return &CombatantRegistry{
		combatants: make(map[string]Combatant),
	}
}

// Add registers a combatant by ID.
// If a combatant with the same ID exists, it is replaced.
func (r *CombatantRegistry) Add(combatant Combatant) {
	r.combatants[combatant.GetID()] = combatant
}

// Get retrieves a combatant by ID.
// Returns the combatant and nil error if found.
// Returns nil and an error if not found.
func (r *CombatantRegistry) Get(id string) (Combatant, error) {
	if c, ok := r.combatants[id]; ok {
		return c, nil
	}
	return nil, rpgerr.Newf(rpgerr.CodeNotFound, "combatant %s not found in registry", id)
}

// All returns all combatants in the registry.
func (r *CombatantRegistry) All() []Combatant {
	result := make([]Combatant, 0, len(r.combatants))
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
func GetCombatant(ctx context.Context, id string) (Combatant, error) {
	registry, ok := ctx.Value(combatantKey{}).(*CombatantRegistry)
	if !ok || registry == nil {
		return nil, rpgerr.New(rpgerr.CodeNotFound, "no combatant registry in context")
	}
	return registry.Get(id)
}

// GetAllCombatants retrieves all combatants from context.
// Returns nil if no registry in context.
func GetAllCombatants(ctx context.Context) []Combatant {
	registry, ok := ctx.Value(combatantKey{}).(*CombatantRegistry)
	if !ok || registry == nil {
		return nil
	}
	return registry.All()
}
