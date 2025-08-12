// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package examples

import (
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/features"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/resources"
)

// RageRef is the unique identifier for the Rage feature.
var RageRef = core.MustNewRef(core.RefInput{
	Module: "dnd5e",
	Type:   "feature",
	Value:  "rage",
})

// RageFeature implements the Barbarian's Rage ability.
type RageFeature struct {
	*features.SimpleFeature
	rageResource *resources.SimpleResource
	turnsActive  int
}

// RageData represents the persistent state of Rage.
type RageData struct {
	Ref           string `json:"ref"`
	UsesRemaining int    `json:"uses_remaining"`
	IsActive      bool   `json:"is_active"`
	TurnsActive   int    `json:"turns_active"`
}

// NewRageFeature creates a new Rage feature.
func NewRageFeature(level int) (*RageFeature, error) {
	// Calculate max uses based on level
	maxUses := calculateRageUses(level)
	
	rage := &RageFeature{
		rageResource: resources.NewSimpleResource(resources.SimpleResourceConfig{
			ID:      "rage_uses",
			Type:    resources.ResourceTypeAbilityUse,
			Current: maxUses,
			Maximum: maxUses,
		}),
		turnsActive: 0,
	}
	
	// Create the SimpleFeature with our configuration
	simpleFeature, err := features.NewSimpleFeature(features.SimpleFeatureConfig{
		Ref:         RageRef,
		Name:        "Rage",
		Description: "Enter a battle fury that grants damage resistance and bonus damage",
		NeedsTarget: false, // Rage targets self
		OnActivate:  rage.activate,
		OnApply:     rage.apply,
		OnRemove:    rage.remove,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create rage feature: %w", err)
	}
	
	rage.SimpleFeature = simpleFeature
	return rage, nil
}

// activate handles the activation of Rage.
func (r *RageFeature) activate(owner core.Entity, ctx *features.ActivateContext) error {
	// Check if already active
	if r.IsActive() {
		return features.ErrAlreadyActive
	}
	
	// Check if we have uses remaining
	if r.rageResource.Current() < 1 {
		return features.ErrNoUsesRemaining
	}
	
	// Consume a use
	if err := r.rageResource.Consume(1); err != nil {
		return err
	}
	
	// Reset turn counter
	r.turnsActive = 0
	
	// Fire activation event
	// In a real implementation, this would trigger through the event bus
	// to apply damage resistance and other effects
	
	return nil
}

// apply sets up event subscriptions for Rage.
func (r *RageFeature) apply(bus events.EventBus) error {
	// Subscribe to damage events to apply resistance
	// This is simplified - real implementation would use the event system properly
	
	// Example of what this might look like with a proper event system:
	// bus.On("damage.before").
	//     ToTarget(owner.GetID()).
	//     Do(func(e events.Event) error {
	//         if r.IsActive() {
	//             // Apply damage resistance for physical damage
	//             if damageType := e.Context().Get("damage_type"); isPhysical(damageType) {
	//                 currentDamage := e.Context().Get("damage").(int)
	//                 e.Context().Set("damage", currentDamage/2)
	//             }
	//         }
	//         return nil
	//     })
	
	return nil
}

// remove cleans up event subscriptions.
func (r *RageFeature) remove(bus events.EventBus) error {
	// Clean up subscriptions
	// The SimpleFeature's embedded Core handles this through the tracker
	return nil
}

// ToJSON serializes the Rage state.
func (r *RageFeature) ToJSON() (json.RawMessage, error) {
	data := RageData{
		Ref:           RageRef.String(),
		UsesRemaining: r.rageResource.Current(),
		IsActive:      r.IsActive(),
		TurnsActive:   r.turnsActive,
	}
	
	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal rage state: %w", err)
	}
	return bytes, nil
}

// LoadRageFromJSON recreates a Rage feature from saved data.
func LoadRageFromJSON(data json.RawMessage) (features.Feature, error) {
	var rageData RageData
	if err := json.Unmarshal(data, &rageData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal rage data: %w", err)
	}
	
	// Create a new rage feature
	// In a real implementation, we'd need to know the character's level
	rage, err := NewRageFeature(1) // Default to level 1 for example
	if err != nil {
		return nil, fmt.Errorf("failed to create rage feature: %w", err)
	}
	
	// Restore state
	rage.rageResource.SetCurrent(rageData.UsesRemaining)
	rage.SetActive(rageData.IsActive)
	rage.turnsActive = rageData.TurnsActive
	
	return rage, nil
}

// calculateRageUses returns the number of rage uses based on barbarian level.
func calculateRageUses(level int) int {
	switch {
	case level < 3:
		return 2
	case level < 6:
		return 3
	case level < 12:
		return 4
	case level < 17:
		return 5
	case level < 20:
		return 6
	default:
		return -1 // Unlimited at level 20
	}
}

// GetResource returns the rage uses resource for UI display.
func (r *RageFeature) GetResource() resources.Resource {
	return r.rageResource
}