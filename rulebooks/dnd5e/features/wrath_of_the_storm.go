// Package features provides D&D 5e class features implementation
package features

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	dnd5eCombat "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
)

// WrathOfTheStorm represents the fighter's Wrath of the Storm feature.
// It implements core.Action[FeatureInput] for activation and events.BusEffect for resource management.
type WrathOfTheStorm struct {
	id          string
	name        string
	characterID string                           // Character this feature belongs to
	resource    *dnd5eCombat.RecoverableResource // Tracks wrath of the storm uses (1 per short/long rest)
}

// WrathOfTheStormData is the JSON structure for persisting Wrath of the Storm state
type WrathOfTheStormData struct {
	Ref         *core.Ref `json:"ref"`
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	CharacterID string    `json:"character_id"`
	Uses        int       `json:"uses"`
	MaxUses     int       `json:"max_uses"`
}

// Ref returns the unique ref for the Wrath of the Storm feature.
func (a *WrathOfTheStorm) Ref() *core.Ref { return refs.Features.WrathOfTheStorm() }

// Status reports the feature's privately-owned Wrath of the Storm resource through
// the non-mutating status surface, without serializing ToJSON. The owner is
// not required: Wrath of the Storm owns its own [RecoverableResource].
func (a *WrathOfTheStorm) Status(in *StatusInput) (*StatusOutput, error) {
	if in == nil || in.Owner == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "wrath of the storm status requires an owner resource reader")
	}
	current, maximum, ok := in.Owner.ResourceStatus(resources.WrathOfTheStorm)
	if !ok {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "owner does not carry wrath of the storm")
	}
	return &StatusOutput{Status: &Status{Ref: *refs.Features.WrathOfTheStorm(), Name: "Wrath of the Storm", Resource: &ResourceStatus{Key: resources.WrathOfTheStorm, Name: "Wrath of the Storm", Current: current, Maximum: maximum}}}, nil
}

// Name returns the display name for the Wrath of the Storm feature.
func (a *WrathOfTheStorm) Name() string { return a.name }

// GetID implements core.Entity
func (a *WrathOfTheStorm) GetID() string {
	return a.id
}

// GetType implements core.Entity
func (a *WrathOfTheStorm) GetType() core.EntityType {
	return EntityTypeFeature
}

// CanActivate implements core.Action[FeatureInput]
func (a *WrathOfTheStorm) CanActivate(context.Context, core.Entity, FeatureInput) error {
	return rpgerr.New(rpgerr.CodeInvalidArgument, "Wrath of the Storm is offered only after a hit")
}

// Apply subscribes the recoverable resource to the event bus for automatic rest recovery.
// This should be called when the feature is granted to a character.
func (a *WrathOfTheStorm) Apply(ctx context.Context, bus events.EventBus) error {
	return a.resource.Apply(ctx, bus)
}

// Remove unsubscribes the recoverable resource from the event bus.
// This should be called when the feature is removed from a character.
func (a *WrathOfTheStorm) Remove(ctx context.Context, bus events.EventBus) error {
	return a.resource.Remove(ctx, bus)
}

// Activate implements core.Action[FeatureInput]
func (a *WrathOfTheStorm) Activate(ctx context.Context, owner core.Entity, input FeatureInput) error {
	// Check if we can activate
	if err := a.CanActivate(ctx, owner, input); err != nil {
		return err
	}

	// Consume a use
	if err := a.resource.Use(1); err != nil {
		return rpgerr.Wrapf(err, "failed to use wrath of the storm")
	}

	return nil
}

// loadJSON loads Wrath of the Storm state from JSON, rejecting negative or
// over-maximum persisted uses before constructing its private resource.
func (a *WrathOfTheStorm) loadJSON(data json.RawMessage) error {
	var wrathOfTheStormData WrathOfTheStormData
	if err := json.Unmarshal(data, &wrathOfTheStormData); err != nil {
		return fmt.Errorf("failed to unmarshal wrath of the storm data: %w", err)
	}

	if err := validateResourceBounds("wrath of the storm", wrathOfTheStormData.Uses, wrathOfTheStormData.MaxUses); err != nil {
		return err
	}

	a.id = wrathOfTheStormData.ID
	a.name = wrathOfTheStormData.Name
	a.characterID = wrathOfTheStormData.CharacterID

	// Set up recoverable resource with current and max uses
	a.resource = dnd5eCombat.NewRecoverableResource(dnd5eCombat.RecoverableResourceConfig{
		ID:          string(resources.WrathOfTheStorm),
		Maximum:     wrathOfTheStormData.MaxUses,
		CharacterID: wrathOfTheStormData.CharacterID,
		ResetType:   coreResources.ResetLongRest,
	})
	// Restore to the saved state
	if wrathOfTheStormData.Uses < wrathOfTheStormData.MaxUses {
		if err := a.resource.Use(wrathOfTheStormData.MaxUses - wrathOfTheStormData.Uses); err != nil {
			return fmt.Errorf("failed to set resource uses: %w", err)
		}
	}

	return nil
}

// ToJSON converts Wrath of the Storm to JSON for persistence
func (a *WrathOfTheStorm) ToJSON() (json.RawMessage, error) {
	data := WrathOfTheStormData{
		Ref:         refs.Features.WrathOfTheStorm(),
		ID:          a.id,
		Name:        a.name,
		CharacterID: a.characterID,
		Uses:        a.resource.Current(),
		MaxUses:     a.resource.Maximum(),
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal wrath of the storm data: %w", err)
	}

	return bytes, nil
}

// ActionType returns the action economy cost to activate wrath of the storm (free - it grants an extra action)
func (a *WrathOfTheStorm) ActionType() combat.ActionType {
	return combat.ActionReaction
}
