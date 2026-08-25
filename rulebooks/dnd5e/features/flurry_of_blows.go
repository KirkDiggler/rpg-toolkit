// Package features provides D&D 5e class features implementation
package features

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
)

// FlurryOfBlows represents the monk's Flurry of Blows feature.
// It implements core.Action[FeatureInput] for activation.
// When activated, consumes 1 Ki point and banks two flurry-strike capacity.
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

// Ref returns the unique ref for the Flurry of Blows feature.
func (f *FlurryOfBlows) Ref() *core.Ref { return refs.Features.FlurryOfBlows() }

// Status reports the monk's character-owned Ki pool through the non-mutating
// status surface, without serializing ToJSON. Flurry of Blows shares the Ki
// pool with every other monk Ki feature, so a non-nil [StatusInput.Owner] that
// carries Ki is required; the key reported is the single shared resources.Ki.
func (f *FlurryOfBlows) Status(in *StatusInput) (*StatusOutput, error) {
	return reportKiStatus(in, refs.Features.FlurryOfBlows(), f.name, "Flurry of Blows")
}

// Name returns the display name for the Flurry of Blows feature.
func (f *FlurryOfBlows) Name() string { return f.name }

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

type flurryOwner interface {
	coreResources.ResourceAccessor
	BankCapacity(combat.CapacityType, int)
}

// Activate implements core.Action[FeatureInput].
func (f *FlurryOfBlows) Activate(ctx context.Context, owner core.Entity, input FeatureInput) error {
	if err := f.CanActivate(ctx, owner, input); err != nil {
		return err
	}

	monk, ok := owner.(flurryOwner)
	if !ok {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "owner cannot bank flurry strike capacity")
	}
	if err := monk.UseResource(resources.Ki, 1); err != nil {
		return rpgerr.Wrapf(err, "failed to use ki for flurry of blows")
	}
	monk.BankCapacity(combat.CapacityFlurryStrike, 2)
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
func (f *FlurryOfBlows) ActionType() coreCombat.ActionType {
	return coreCombat.ActionBonus
}
