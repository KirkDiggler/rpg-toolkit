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
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
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

// Ref returns the unique ref for the Reckless Attack feature.
func (r *RecklessAttack) Ref() *core.Ref { return refs.Features.RecklessAttack() }

// Status reports the feature's non-mutating status surface, without
// serializing ToJSON. Reckless Attack owns no resource, so Resource is nil.
func (r *RecklessAttack) Status(*StatusInput) (*StatusOutput, error) {
	name := r.name
	if name == "" {
		name = "Reckless Attack"
	}
	return &StatusOutput{Status: &Status{
		Ref:  *refs.Features.RecklessAttack(),
		Name: name,
	}}, nil
}

// Name returns the display name for the Reckless Attack feature.
func (r *RecklessAttack) Name() string { return r.name }

// GetID implements core.Entity
func (r *RecklessAttack) GetID() string {
	return r.id
}

// GetType implements core.Entity
func (r *RecklessAttack) GetType() core.EntityType {
	return EntityTypeFeature
}

// CanActivate implements core.Action[FeatureInput]. Reckless Attack costs
// nothing, so the only refusal is a barbarian already attacking recklessly: a
// second activation would put a second condition on the sheet. That refusal
// is also the reason the row reads unavailable on Afford. An owner whose
// conditions cannot be read is refused.
func (r *RecklessAttack) CanActivate(_ context.Context, owner core.Entity, _ FeatureInput) error {
	return refuseWhileHolding(&refuseWhileHoldingInput{
		Owner: owner, Ref: refs.Conditions.RecklessAttack(), Reason: "already attacking recklessly",
	})
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

	// Publish via ConditionAppliedTopic so the character's condition manager
	// applies and stores it (same pattern as Rage). A second one would replace
	// the first on the sheet; it never gets that far, because CanActivate
	// refuses an already-active Reckless Attack (operator ruling on #1944).
	condition := conditions.NewRecklessAttackCondition(owner.GetID())
	topic := dnd5eEvents.ConditionAppliedTopic.On(input.Bus)
	if err := topic.Publish(ctx, dnd5eEvents.ConditionAppliedEvent{
		Target:    owner,
		Type:      dnd5eEvents.ConditionRecklessAttack,
		Source:    dnd5eEvents.ConditionSourceFeature,
		Condition: condition,
	}); err != nil {
		return rpgerr.Wrapf(err, "failed to publish reckless attack condition")
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
