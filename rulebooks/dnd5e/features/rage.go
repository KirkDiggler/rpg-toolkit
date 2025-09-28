// Package features provides D&D 5e class features implementation
package features

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
)

// RageEventData contains rage-specific data for the ConditionAppliedEvent
type RageEventData struct {
	DamageBonus int `json:"damage_bonus"`
	Level       int `json:"level"`
}

// Rage represents the barbarian rage feature.
// It implements core.Action[FeatureInput] for activation.
type Rage struct {
	id       string
	name     string
	level    int                 // Barbarian level for determining damage bonus
	resource *resources.Resource // Tracks rage uses
}

// RageData is the JSON structure for persisting rage state
type RageData struct {
	Ref     core.Ref `json:"ref"`
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Level   int      `json:"level"`
	Uses    int      `json:"uses"`
	MaxUses int      `json:"max_uses"`
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
	return "feature"
}

// CanActivate implements core.Action[FeatureInput]
func (r *Rage) CanActivate(_ context.Context, _ core.Entity, _ FeatureInput) error {
	// At level 20, barbarians have unlimited rages
	if r.level >= 20 {
		return nil
	}

	// Check if we have uses remaining
	if !r.resource.IsAvailable() {
		return fmt.Errorf("no rage uses remaining")
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
		if err := r.resource.Use(1); err != nil {
			return fmt.Errorf("failed to use rage: %w", err)
		}
	}

	// Publish condition applied event
	if input.Bus != nil {
		topic := dnd5e.ConditionAppliedTopic.On(input.Bus)
		err := topic.Publish(ctx, dnd5e.ConditionAppliedEvent{
			Target: owner,
			Type:   dnd5e.ConditionRaging,
			Source: r.id,
			Data: RageEventData{
				DamageBonus: calculateRageDamage(r.level),
				Level:       r.level,
			},
		})
		if err != nil {
			return fmt.Errorf("failed to publish rage condition: %w", err)
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

	// Set up resource with current and max uses
	r.resource = resources.NewResource("rage", rageData.MaxUses)
	r.resource.SetCurrent(rageData.Uses)

	return nil
}

// ToJSON converts rage to JSON for persistence
func (r *Rage) ToJSON() (json.RawMessage, error) {
	data := RageData{
		Ref: core.Ref{
			Module: "dnd5e",
			Type:   "features",
			Value:  "rage",
		},
		ID:      r.id,
		Name:    r.name,
		Level:   r.level,
		Uses:    r.resource.Current,
		MaxUses: r.resource.Maximum,
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal rage data: %w", err)
	}

	return bytes, nil
}

// RestoreOnLongRest restores all rage uses on a long rest
func (r *Rage) RestoreOnLongRest() {
	r.resource.RestoreToFull()
}
