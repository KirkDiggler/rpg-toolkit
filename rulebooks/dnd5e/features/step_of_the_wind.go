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

// StepOfTheWind represents the monk's Step of the Wind feature.
// It implements core.Action[FeatureInput] for activation.
// When activated, consumes 1 Ki point and allows the monk to take the Disengage or Dash action as a bonus action.
type StepOfTheWind struct {
	id          string
	name        string
	characterID string // Character this feature belongs to
}

// StepOfTheWindData is the JSON structure for persisting Step of the Wind state
type StepOfTheWindData struct {
	Ref         *core.Ref `json:"ref"`
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	CharacterID string    `json:"character_id"`
}

// GetID implements core.Entity
func (s *StepOfTheWind) GetID() string {
	return s.id
}

// GetType implements core.Entity
func (s *StepOfTheWind) GetType() core.EntityType {
	return EntityTypeFeature
}

// CanActivate implements core.Action[FeatureInput]
func (s *StepOfTheWind) CanActivate(_ context.Context, owner core.Entity, _ FeatureInput) error {
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
func (s *StepOfTheWind) Activate(ctx context.Context, owner core.Entity, input FeatureInput) error {
	// Check if we can activate
	if err := s.CanActivate(ctx, owner, input); err != nil {
		return err
	}

	// Cast owner to ResourceAccessor to consume Ki
	accessor, ok := owner.(coreResources.ResourceAccessor)
	if !ok {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "owner must implement ResourceAccessor")
	}

	// Consume 1 Ki point
	if err := accessor.UseResource(resources.Ki, 1); err != nil {
		return rpgerr.Wrapf(err, "failed to use ki for step of the wind")
	}

	// Determine which action to take (defaults to "disengage" if not specified)
	action := input.Action
	if action == "" {
		action = "disengage"
	}

	// Validate action choice
	if action != "disengage" && action != "dash" {
		return rpgerr.Newf(rpgerr.CodeInvalidArgument, "invalid action: %s (must be 'disengage' or 'dash')", action)
	}

	// Publish event granting Disengage or Dash action
	// The game server is responsible for applying the appropriate effect
	if input.Bus != nil {
		topic := dnd5eEvents.StepOfTheWindActivatedTopic.On(input.Bus)
		err := topic.Publish(ctx, dnd5eEvents.StepOfTheWindActivatedEvent{
			CharacterID: owner.GetID(),
			Action:      action,
			Source:      refs.Features.StepOfTheWind().ID,
		})
		if err != nil {
			return rpgerr.Wrapf(err, "failed to publish step of the wind event")
		}
	}

	return nil
}

// loadJSON loads Step of the Wind state from JSON
func (s *StepOfTheWind) loadJSON(data json.RawMessage) error {
	var stepData StepOfTheWindData
	if err := json.Unmarshal(data, &stepData); err != nil {
		return fmt.Errorf("failed to unmarshal step of the wind data: %w", err)
	}

	s.id = stepData.ID
	s.name = stepData.Name
	s.characterID = stepData.CharacterID

	return nil
}

// ToJSON converts Step of the Wind to JSON for persistence
func (s *StepOfTheWind) ToJSON() (json.RawMessage, error) {
	data := StepOfTheWindData{
		Ref:         refs.Features.StepOfTheWind(),
		ID:          s.id,
		Name:        s.name,
		CharacterID: s.characterID,
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal step of the wind data: %w", err)
	}

	return bytes, nil
}

// ActionType returns the action economy cost to activate step of the wind (bonus action)
func (s *StepOfTheWind) ActionType() combat.ActionType {
	return combat.ActionBonus
}
