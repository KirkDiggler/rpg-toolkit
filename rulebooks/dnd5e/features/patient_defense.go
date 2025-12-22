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

// PatientDefense represents the monk's Patient Defense feature.
// It implements core.Action[FeatureInput] for activation.
// When activated, consumes 1 Ki point and allows the monk to take the Dodge action as a bonus action.
type PatientDefense struct {
	id          string
	name        string
	characterID string // Character this feature belongs to
}

// PatientDefenseData is the JSON structure for persisting Patient Defense state
type PatientDefenseData struct {
	Ref         *core.Ref `json:"ref"`
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	CharacterID string    `json:"character_id"`
}

// GetID implements core.Entity
func (p *PatientDefense) GetID() string {
	return p.id
}

// GetType implements core.Entity
func (p *PatientDefense) GetType() core.EntityType {
	return EntityTypeFeature
}

// CanActivate implements core.Action[FeatureInput]
func (p *PatientDefense) CanActivate(_ context.Context, owner core.Entity, _ FeatureInput) error {
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
func (p *PatientDefense) Activate(ctx context.Context, owner core.Entity, input FeatureInput) error {
	// Check if we can activate
	if err := p.CanActivate(ctx, owner, input); err != nil {
		return err
	}

	// Cast owner to ResourceAccessor to consume Ki
	accessor, ok := owner.(coreResources.ResourceAccessor)
	if !ok {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "owner must implement ResourceAccessor")
	}

	// Consume 1 Ki point
	if err := accessor.UseResource(resources.Ki, 1); err != nil {
		return rpgerr.Wrapf(err, "failed to use ki for patient defense")
	}

	// Publish event granting Dodge effect (attackers have disadvantage)
	// The game server is responsible for applying and tracking the Dodge condition
	if input.Bus != nil {
		topic := dnd5eEvents.PatientDefenseActivatedTopic.On(input.Bus)
		err := topic.Publish(ctx, dnd5eEvents.PatientDefenseActivatedEvent{
			CharacterID: owner.GetID(),
			Source:      refs.Features.PatientDefense().ID,
		})
		if err != nil {
			return rpgerr.Wrapf(err, "failed to publish patient defense event")
		}
	}

	return nil
}

// loadJSON loads Patient Defense state from JSON
func (p *PatientDefense) loadJSON(data json.RawMessage) error {
	var patientDefenseData PatientDefenseData
	if err := json.Unmarshal(data, &patientDefenseData); err != nil {
		return fmt.Errorf("failed to unmarshal patient defense data: %w", err)
	}

	p.id = patientDefenseData.ID
	p.name = patientDefenseData.Name
	p.characterID = patientDefenseData.CharacterID

	return nil
}

// ToJSON converts Patient Defense to JSON for persistence
func (p *PatientDefense) ToJSON() (json.RawMessage, error) {
	data := PatientDefenseData{
		Ref:         refs.Features.PatientDefense(),
		ID:          p.id,
		Name:        p.name,
		CharacterID: p.characterID,
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal patient defense data: %w", err)
	}

	return bytes, nil
}

// ActionType returns the action economy cost to activate patient defense (bonus action)
func (p *PatientDefense) ActionType() combat.ActionType {
	return combat.ActionBonus
}
