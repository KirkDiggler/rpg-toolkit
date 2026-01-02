// Package features provides D&D 5e class features implementation
package features

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
)

// FlurryOfBlows represents the monk's Flurry of Blows feature.
// It implements core.Action[FeatureInput] for activation.
// When activated, consumes 1 Ki point and grants two unarmed strikes as a bonus action.
type FlurryOfBlows struct {
	id          string
	name        string
	characterID string // Character this feature belongs to
}

// FlurryOfBlowsData is the JSON structure for persisting Flurry of Blows state
type FlurryOfBlowsData struct {
	Ref         *core.Ref `json:"ref"`
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	CharacterID string    `json:"character_id"`
}

// GetID implements core.Entity
func (f *FlurryOfBlows) GetID() string {
	return f.id
}

// GetType implements core.Entity
func (f *FlurryOfBlows) GetType() core.EntityType {
	return EntityTypeFeature
}

// CanActivate implements core.Action[FeatureInput]
func (f *FlurryOfBlows) CanActivate(_ context.Context, owner core.Entity, _ FeatureInput) error {
	// Cast owner to ResourceAccessor to check Ki
	accessor, ok := owner.(coreResources.ResourceAccessor)
	if !ok {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "owner does not implement ResourceAccessor")
	}

	// Check if Ki is available
	if !accessor.IsResourceAvailable(resources.Ki) {
		return rpgerr.New(rpgerr.CodeResourceExhausted, "no ki points remaining")
	}

	return nil
}

// Activate implements core.Action[FeatureInput]
func (f *FlurryOfBlows) Activate(ctx context.Context, owner core.Entity, input FeatureInput) error {
	// Check if we can activate
	if err := f.CanActivate(ctx, owner, input); err != nil {
		return err
	}

	// Cast owner to ResourceAccessor to consume Ki
	accessor, ok := owner.(coreResources.ResourceAccessor)
	if !ok {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "owner does not implement ResourceAccessor")
	}

	// Cast owner to ActionHolder to grant FlurryStrike actions
	holder, ok := owner.(actions.ActionHolder)
	if !ok {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "owner does not implement ActionHolder")
	}

	// Consume 1 Ki point
	if err := accessor.UseResource(resources.Ki, 1); err != nil {
		return rpgerr.Wrapf(err, "failed to use ki for flurry of blows")
	}

	// Grant two FlurryStrike actions
	ownerID := owner.GetID()
	strike1 := actions.NewFlurryStrike(actions.FlurryStrikeConfig{
		ID:      fmt.Sprintf("%s-flurry-strike-1", ownerID),
		OwnerID: ownerID,
	})
	strike2 := actions.NewFlurryStrike(actions.FlurryStrikeConfig{
		ID:      fmt.Sprintf("%s-flurry-strike-2", ownerID),
		OwnerID: ownerID,
	})

	// Apply actions to event bus (subscribe to turn end for cleanup)
	if input.Bus != nil {
		if err := strike1.Apply(ctx, input.Bus); err != nil {
			return rpgerr.Wrapf(err, "failed to apply flurry strike 1")
		}
		if err := strike2.Apply(ctx, input.Bus); err != nil {
			// Rollback strike1
			_ = strike1.Remove(ctx, input.Bus)
			return rpgerr.Wrapf(err, "failed to apply flurry strike 2")
		}
	}

	// Add actions to the character
	if err := holder.AddAction(strike1); err != nil {
		// Rollback subscriptions
		if input.Bus != nil {
			_ = strike1.Remove(ctx, input.Bus)
			_ = strike2.Remove(ctx, input.Bus)
		}
		return rpgerr.Wrapf(err, "failed to add flurry strike 1 to character")
	}
	if err := holder.AddAction(strike2); err != nil {
		// Rollback strike1 from holder and subscriptions
		_ = holder.RemoveAction(strike1.GetID())
		if input.Bus != nil {
			_ = strike1.Remove(ctx, input.Bus)
			_ = strike2.Remove(ctx, input.Bus)
		}
		return rpgerr.Wrapf(err, "failed to add flurry strike 2 to character")
	}

	return nil
}

// loadJSON loads Flurry of Blows state from JSON
func (f *FlurryOfBlows) loadJSON(data json.RawMessage) error {
	var flurryData FlurryOfBlowsData
	if err := json.Unmarshal(data, &flurryData); err != nil {
		return fmt.Errorf("failed to unmarshal flurry of blows data: %w", err)
	}

	f.id = flurryData.ID
	f.name = flurryData.Name
	f.characterID = flurryData.CharacterID

	return nil
}

// ToJSON converts Flurry of Blows to JSON for persistence
func (f *FlurryOfBlows) ToJSON() (json.RawMessage, error) {
	data := FlurryOfBlowsData{
		Ref:         refs.Features.FlurryOfBlows(),
		ID:          f.id,
		Name:        f.name,
		CharacterID: f.characterID,
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal flurry of blows data: %w", err)
	}

	return bytes, nil
}

// ActionType returns the action economy cost to activate flurry of blows (bonus action)
func (f *FlurryOfBlows) ActionType() combat.ActionType {
	return combat.ActionBonus
}
