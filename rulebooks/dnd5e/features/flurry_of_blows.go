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
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
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
		return rpgerr.New(rpgerr.CodeInvalidArgument, "owner must implement ResourceAccessor")
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
		return rpgerr.New(rpgerr.CodeInvalidArgument, "owner must implement ResourceAccessor")
	}

	// Consume 1 Ki point
	if err := accessor.UseResource(resources.Ki, 1); err != nil {
		return rpgerr.Wrapf(err, "failed to use ki for flurry of blows")
	}

	// Publish event granting two unarmed strikes
	// The game server is responsible for tracking and resolving these strikes
	if input.Bus != nil {
		topic := dnd5eEvents.FlurryOfBlowsActivatedTopic.On(input.Bus)
		err := topic.Publish(ctx, dnd5eEvents.FlurryOfBlowsActivatedEvent{
			CharacterID:    owner.GetID(),
			UnarmedStrikes: 2,
			Source:         refs.Features.FlurryOfBlows().ID,
		})
		if err != nil {
			return rpgerr.Wrapf(err, "failed to publish flurry of blows event")
		}
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
