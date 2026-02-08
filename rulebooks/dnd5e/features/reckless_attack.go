// Package features provides D&D 5e class features implementation
package features

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// RecklessAttack represents the Barbarian's Reckless Attack feature.
// When activated (free action), applies RecklessAttackCondition which grants
// advantage on melee STR attacks but also gives attackers advantage against you.
type RecklessAttack struct {
	id          string
	name        string
	characterID string
}

// RecklessAttackData is the JSON structure for persisting Reckless Attack state
type RecklessAttackData struct {
	Ref         *core.Ref `json:"ref"`
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	CharacterID string    `json:"character_id"`
}

// GetID implements core.Entity
func (r *RecklessAttack) GetID() string {
	return r.id
}

// GetType implements core.Entity
func (r *RecklessAttack) GetType() core.EntityType {
	return EntityTypeFeature
}

// CanActivate implements core.Action[FeatureInput].
// Reckless Attack has no resource cost — it can always be activated.
func (r *RecklessAttack) CanActivate(_ context.Context, _ core.Entity, _ FeatureInput) error {
	return nil
}

// Activate implements core.Action[FeatureInput].
// Applies the RecklessAttackCondition to the event bus, granting advantage on
// the barbarian's melee attacks and giving enemies advantage against the barbarian.
func (r *RecklessAttack) Activate(ctx context.Context, owner core.Entity, input FeatureInput) error {
	if err := r.CanActivate(ctx, owner, input); err != nil {
		return err
	}

	if input.Bus == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "event bus required for reckless attack")
	}

	// Create and apply the reckless attack condition
	condition := conditions.NewRecklessAttackCondition(owner.GetID())
	if err := condition.Apply(ctx, input.Bus); err != nil {
		return rpgerr.Wrap(err, "failed to apply reckless attack condition")
	}

	return nil
}

// loadJSON loads Reckless Attack state from JSON
func (r *RecklessAttack) loadJSON(data json.RawMessage) error {
	var raData RecklessAttackData
	if err := json.Unmarshal(data, &raData); err != nil {
		return fmt.Errorf("failed to unmarshal reckless attack data: %w", err)
	}

	r.id = raData.ID
	r.name = raData.Name
	r.characterID = raData.CharacterID

	return nil
}

// ToJSON converts Reckless Attack to JSON for persistence
func (r *RecklessAttack) ToJSON() (json.RawMessage, error) {
	data := RecklessAttackData{
		Ref:         refs.Features.RecklessAttack(),
		ID:          r.id,
		Name:        r.name,
		CharacterID: r.characterID,
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal reckless attack data: %w", err)
	}

	return bytes, nil
}

// ActionType returns the action economy cost to activate reckless attack (free action)
func (r *RecklessAttack) ActionType() combat.ActionType {
	return combat.ActionFree
}
