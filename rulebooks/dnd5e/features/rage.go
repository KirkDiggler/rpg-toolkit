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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
)

// Rage represents the barbarian rage feature.
// It implements core.Action[FeatureInput] for activation.
// Rage uses the owner's resources via ResourceAccessor - the character owns the
// rage charges resource, and this feature consumes from it.
type Rage struct {
	id    string
	name  string
	level int // Barbarian level for determining damage bonus
}

// RageData is the JSON structure for persisting rage state.
// Note: Resource state (uses/max) is owned by the Character, not the feature.
type RageData struct {
	Ref   *core.Ref `json:"ref"`
	ID    string    `json:"id"`
	Name  string    `json:"name"`
	Level int       `json:"level"`
}

// calculateRageUses determines max rage uses based on barbarian level
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

// calculateRageDamage determines rage damage bonus based on barbarian level
func calculateRageDamage(level int) int {
	switch {
	case level < 9:
		return 2
	case level < 16:
		return 3
	default:
		return 4
	}
}

// GetID implements core.Entity
func (r *Rage) GetID() string {
	return r.id
}

// GetType implements core.Entity
func (r *Rage) GetType() core.EntityType {
	return EntityTypeFeature
}

// CanActivate implements core.Action[FeatureInput]
func (r *Rage) CanActivate(_ context.Context, owner core.Entity, _ FeatureInput) error {
	// At level 20, barbarians have unlimited rages
	if r.level >= 20 {
		return nil
	}

	// Get resource accessor from owner
	accessor, ok := owner.(coreResources.ResourceAccessor)
	if !ok {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "owner does not implement ResourceAccessor")
	}

	// Check if we have uses remaining
	if !accessor.IsResourceAvailable(resources.RageCharges) {
		return rpgerr.New(rpgerr.CodeResourceExhausted, "no rage uses remaining")
	}

	return nil
}

// Activate implements core.Action[FeatureInput]
func (r *Rage) Activate(ctx context.Context, owner core.Entity, input FeatureInput) error {
	// Check if we can activate
	if err := r.CanActivate(ctx, owner, input); err != nil {
		return err
	}

	// Consume a use (unless level 20)
	if r.level < 20 {
		accessor, ok := owner.(coreResources.ResourceAccessor)
		if !ok {
			return rpgerr.New(rpgerr.CodeInvalidArgument, "owner does not implement ResourceAccessor")
		}
		if err := accessor.UseResource(resources.RageCharges, 1); err != nil {
			return rpgerr.Wrapf(err, "failed to use rage")
		}
	}

	// Create the raging condition
	ragingCondition := &conditions.RagingCondition{
		CharacterID: owner.GetID(),
		DamageBonus: calculateRageDamage(r.level),
		Level:       r.level,
		Source:      r.id,
	}

	// Publish condition applied event with the actual condition
	if input.Bus != nil {
		topic := dnd5eEvents.ConditionAppliedTopic.On(input.Bus)
		err := topic.Publish(ctx, dnd5eEvents.ConditionAppliedEvent{
			Target:    owner,
			Type:      dnd5eEvents.ConditionRaging,
			Source:    dnd5eEvents.ConditionSourceFeature,
			Condition: ragingCondition,
		})
		if err != nil {
			return rpgerr.Wrapf(err, "failed to publish rage condition")
		}
	}

	return nil
}

// loadJSON loads rage state from JSON
func (r *Rage) loadJSON(data json.RawMessage) error {
	var rageData RageData
	if err := json.Unmarshal(data, &rageData); err != nil {
		return fmt.Errorf("failed to unmarshal rage data: %w", err)
	}

	r.id = rageData.ID
	r.name = rageData.Name
	r.level = rageData.Level

	return nil
}

// ToJSON converts rage to JSON for persistence
func (r *Rage) ToJSON() (json.RawMessage, error) {
	data := RageData{
		Ref:   refs.Features.Rage(),
		ID:    r.id,
		Name:  r.name,
		Level: r.level,
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal rage data: %w", err)
	}

	return bytes, nil
}

// ActionType returns the action economy cost to activate rage (bonus action)
func (r *Rage) ActionType() combat.ActionType {
	return combat.ActionBonus
}
