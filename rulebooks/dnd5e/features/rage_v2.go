// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"context"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
)

// RageV2Input extends the standard FeatureInput with rage-specific options
type RageV2Input struct {
	FeatureInput
	ConditionManager conditions.Manager // Required: manages conditions
}

// RageV2 implements a barbarian's rage feature using conditions
type RageV2 struct {
	id    string
	uses  int
	level int
	bus   *events.Bus

	// Track current uses (no longer tracking active state)
	currentUses int
}

// NewRageV2 creates a new rage feature
func NewRageV2(id string, uses int, level int, bus *events.Bus) *RageV2 {
	return &RageV2{
		id:          id,
		uses:        uses,
		level:       level,
		bus:         bus,
		currentUses: uses,
	}
}

// GetID returns the entity's unique identifier
func (r *RageV2) GetID() string { return r.id }

// GetType returns the entity type (feature)
func (r *RageV2) GetType() core.EntityType { return dnd5e.EntityTypeFeature }

// GetResourceType returns what resource this feature consumes
func (r *RageV2) GetResourceType() ResourceType { return ResourceTypeRageUses }

// ResetsOn returns when this feature's uses reset
func (r *RageV2) ResetsOn() ResetType { return ResetTypeLongRest }

// CanActivate checks if rage can be activated
func (r *RageV2) CanActivate(ctx context.Context, owner core.Entity, input FeatureInput) error {
	// Validate we have the extended input
	rageInput, ok := input.(RageV2Input)
	if !ok || rageInput.ConditionManager == nil {
		return errors.New("rage requires a condition manager")
	}

	// Check uses
	if r.currentUses <= 0 {
		return errors.New("no rage uses remaining")
	}

	// Check if already raging via condition manager
	hasRage, err := rageInput.ConditionManager.HasCondition(ctx, owner, conditions.Raging)
	if err != nil {
		return fmt.Errorf("failed to check raging condition: %w", err)
	}
	if hasRage {
		return errors.New("already raging")
	}

	return nil
}

// Activate enters rage mode by applying the raging condition
func (r *RageV2) Activate(ctx context.Context, owner core.Entity, input FeatureInput) error {
	// Validate input
	rageInput, ok := input.(RageV2Input)
	if !ok || rageInput.ConditionManager == nil {
		return errors.New("rage requires a condition manager")
	}

	// Check if can activate
	if err := r.CanActivate(ctx, owner, input); err != nil {
		return err
	}

	// Consume a use
	r.currentUses--

	// Create the raging condition with metadata
	ragingCondition := conditions.Condition{
		Type:         conditions.Raging,
		Source:       "rage_feature",
		SourceEntity: r.id,
		DurationType: conditions.DurationRounds,
		Remaining:    10, // Rage lasts 10 rounds (1 minute)
		Metadata: map[string]interface{}{
			"barbarian_level": r.level,
			"feature_id":      r.id,
		},
	}

	// Apply the condition
	applyInput := &conditions.ApplyConditionInput{
		Target:    owner,
		Condition: ragingCondition,
		EventBus:  r.bus,
	}

	output, err := rageInput.ConditionManager.ApplyCondition(ctx, applyInput)
	if err != nil {
		// Restore the use if we failed to apply the condition
		r.currentUses++
		return fmt.Errorf("failed to apply raging condition: %w", err)
	}

	if !output.Applied {
		// Restore the use if condition wasn't applied
		r.currentUses++
		return errors.New("raging condition was not applied")
	}

	return nil
}

// Deactivate ends rage by removing the condition
func (r *RageV2) Deactivate(ctx context.Context, owner core.Entity, manager conditions.Manager) error {
	removeInput := &conditions.RemoveConditionInput{
		Target:   owner,
		Type:     conditions.Raging,
		Source:   "rage_feature",
		EventBus: r.bus,
	}

	output, err := manager.RemoveCondition(ctx, removeInput)
	if err != nil {
		return fmt.Errorf("failed to remove raging condition: %w", err)
	}

	if !output.Removed {
		return errors.New("no raging condition to remove")
	}

	return nil
}

// GetCurrentUses returns the remaining rage uses
func (r *RageV2) GetCurrentUses() int {
	return r.currentUses
}

// Reset restores all rage uses (called on long rest)
func (r *RageV2) Reset() {
	r.currentUses = r.uses
}

// SetLevel updates the barbarian level (affects damage bonus)
func (r *RageV2) SetLevel(level int) {
	r.level = level
}