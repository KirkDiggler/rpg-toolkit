// Package features provides D&D 5e class features implementation
package features

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// ActionSurge represents the fighter's Action Surge feature.
// It implements core.Action[FeatureInput] for activation and events.BusEffect for resource management.
type ActionSurge struct {
	id          string
	name        string
	characterID string                      // Character this feature belongs to
	resource    *combat.RecoverableResource // Tracks action surge uses (1 per short/long rest)
}

// ActionSurgeData is the JSON structure for persisting Action Surge state
type ActionSurgeData struct {
	Ref         *core.Ref `json:"ref"`
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	CharacterID string    `json:"character_id"`
	Uses        int       `json:"uses"`
	MaxUses     int       `json:"max_uses"`
}

// GetID implements core.Entity
func (a *ActionSurge) GetID() string {
	return a.id
}

// GetType implements core.Entity
func (a *ActionSurge) GetType() core.EntityType {
	return EntityTypeFeature
}

// CanActivate implements core.Action[FeatureInput]
func (a *ActionSurge) CanActivate(_ context.Context, _ core.Entity, input FeatureInput) error {
	// Check if we have uses remaining
	if !a.resource.IsAvailable() {
		return rpgerr.New(rpgerr.CodeResourceExhausted, "no action surge uses remaining")
	}

	// Check if ActionEconomy is provided
	if input.ActionEconomy == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "action economy is required for action surge")
	}

	return nil
}

// Apply subscribes the recoverable resource to the event bus for automatic rest recovery.
// This should be called when the feature is granted to a character.
func (a *ActionSurge) Apply(ctx context.Context, bus events.EventBus) error {
	return a.resource.Apply(ctx, bus)
}

// Remove unsubscribes the recoverable resource from the event bus.
// This should be called when the feature is removed from a character.
func (a *ActionSurge) Remove(ctx context.Context, bus events.EventBus) error {
	return a.resource.Remove(ctx, bus)
}

// Activate implements core.Action[FeatureInput]
func (a *ActionSurge) Activate(ctx context.Context, owner core.Entity, input FeatureInput) error {
	// Check if we can activate
	if err := a.CanActivate(ctx, owner, input); err != nil {
		return err
	}

	// Consume a use
	if err := a.resource.Use(1); err != nil {
		return rpgerr.Wrapf(err, "failed to use action surge")
	}

	// Grant an extra action via ActionEconomy
	input.ActionEconomy.GrantExtraAction()

	return nil
}

// loadJSON loads Action Surge state from JSON
func (a *ActionSurge) loadJSON(data json.RawMessage) error {
	var actionSurgeData ActionSurgeData
	if err := json.Unmarshal(data, &actionSurgeData); err != nil {
		return fmt.Errorf("failed to unmarshal action surge data: %w", err)
	}

	a.id = actionSurgeData.ID
	a.name = actionSurgeData.Name
	a.characterID = actionSurgeData.CharacterID

	// Set up recoverable resource with current and max uses
	a.resource = combat.NewRecoverableResource(combat.RecoverableResourceConfig{
		ID:          refs.Features.ActionSurge().ID,
		Maximum:     actionSurgeData.MaxUses,
		CharacterID: actionSurgeData.CharacterID,
		ResetType:   coreResources.ResetShortRest,
	})
	// Restore to the saved state
	if actionSurgeData.Uses < actionSurgeData.MaxUses {
		if err := a.resource.Use(actionSurgeData.MaxUses - actionSurgeData.Uses); err != nil {
			return fmt.Errorf("failed to set resource uses: %w", err)
		}
	}

	return nil
}

// ToJSON converts Action Surge to JSON for persistence
func (a *ActionSurge) ToJSON() (json.RawMessage, error) {
	data := ActionSurgeData{
		Ref:         refs.Features.ActionSurge(),
		ID:          a.id,
		Name:        a.name,
		CharacterID: a.characterID,
		Uses:        a.resource.Current(),
		MaxUses:     a.resource.Maximum(),
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal action surge data: %w", err)
	}

	return bytes, nil
}
