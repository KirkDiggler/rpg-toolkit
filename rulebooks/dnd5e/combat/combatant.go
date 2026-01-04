// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
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

	// AC returns the combatant's armor class
	AC() int

	// ApplyDamage reduces HP by the damage amount(s).
	// HP cannot go below 0.
	ApplyDamage(ctx context.Context, input *ApplyDamageInput) *ApplyDamageResult

	// IsDirty returns true if the combatant has been modified since last save
	IsDirty() bool

	// MarkClean marks the combatant as saved (not dirty)
	MarkClean()

	// GetAbilityScores returns all ability scores for attack/damage calculations
	GetAbilityScores() shared.AbilityScores

	// GetProficiencyBonus returns the proficiency bonus for attack calculations
	GetProficiencyBonus() int
}

// EffectiveACCalculator is implemented by combatants that support dynamic AC calculation.
// Characters implement this to support spells like Shield that modify AC through the event chain.
// Combatants that don't implement this will use their base AC() value.
type EffectiveACCalculator interface {
	// EffectiveAC calculates AC through the modifier chain, allowing conditions/spells to adjust it.
	// Returns an ACBreakdown with the final AC and all contributing components.
	EffectiveAC(ctx context.Context) *ACBreakdown
}

// GetEffectiveAC returns the effective AC for a combatant.
// If the combatant implements EffectiveACCalculator (like Character), uses the chain-based calculation.
// Otherwise, returns the base AC() value.
func GetEffectiveAC(ctx context.Context, c Combatant) int {
	if calc, ok := c.(EffectiveACCalculator); ok {
		breakdown := calc.EffectiveAC(ctx)
		return breakdown.Total
	}
	return c.AC()
}

// CombatantLookup provides combatant lookup from context.
// This interface is satisfied by gamectx.CombatantRegistry.
type CombatantLookup interface {
	// Get retrieves a combatant by ID
	Get(id string) (Combatant, error)
}

// combatantLookupKey is the context key for CombatantLookup
type combatantLookupKey struct{}

// WithCombatantLookup adds a CombatantLookup to the context.
// Use this to enable ResolveAttack to look up combatants by ID.
func WithCombatantLookup(ctx context.Context, lookup CombatantLookup) context.Context {
	return context.WithValue(ctx, combatantLookupKey{}, lookup)
}

// GetCombatantFromContext retrieves a combatant by ID from context.
// Returns error if no lookup is in context or combatant not found.
func GetCombatantFromContext(ctx context.Context, id string) (Combatant, error) {
	lookup, ok := ctx.Value(combatantLookupKey{}).(CombatantLookup)
	if !ok || lookup == nil {
		return nil, rpgerr.New(rpgerr.CodeNotFound, "no combatant lookup in context")
	}
	return lookup.Get(id)
}
