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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
)

// ActionSurge represents the fighter's Action Surge feature.
// It implements core.Action[FeatureInput] for the sheet's list of abilities.
// Its uses are counted in the owner's shared resources.ActionSurge pool, as
// Second Wind's are; the feature keeps no counter of its own, and it is used
// from its description, not from its own code.
type ActionSurge struct {
	id          string
	name        string
	characterID string // Character this feature belongs to
}

// ActionSurgeData is the JSON structure for persisting Action Surge state. The
// uses live in the owner's shared pool, not here. A blob saved with the old
// "uses" or "max_uses" keys loads and the copies are ignored.
type ActionSurgeData struct {
	Ref         *core.Ref `json:"ref"`
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	CharacterID string    `json:"character_id"`
}

// Ref returns the unique ref for the Action Surge feature.
func (a *ActionSurge) Ref() *core.Ref { return refs.Features.ActionSurge() }

// Status reports the fighter's character-owned ActionSurge pool through the
// non-mutating status surface, without serializing ToJSON. Action Surge does
// not own its resource — the Character does — so a non-nil
// [StatusInput.Owner] that carries resources.ActionSurge is required.
func (a *ActionSurge) Status(in *StatusInput) (*StatusOutput, error) {
	if in == nil || in.Owner == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "action surge status requires an owner resource reader")
	}
	current, maximum, ok := in.Owner.ResourceStatus(resources.ActionSurge)
	if !ok {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "owner does not carry action_surge")
	}
	name := a.name
	if name == "" {
		name = "Action Surge"
	}
	return &StatusOutput{Status: &Status{
		Ref:  *refs.Features.ActionSurge(),
		Name: name,
		Resource: &ResourceStatus{
			Key:     resources.ActionSurge,
			Name:    "Action Surge",
			Current: current,
			Maximum: maximum,
		},
	}}, nil
}

// Name returns the display name for the Action Surge feature.
func (a *ActionSurge) Name() string { return a.name }

// Description returns what activating this feature does, in the player's voice.
func (a *ActionSurge) Description() string {
	return "Spend a use to take one additional action this turn."
}

// Definition describes Action Surge as data: no profile, only its price, one
// point of the shared pool, which gives one more action. This is the one way
// to use it: the gate pays the cost and [ActionSurge.Activate] refuses.
func (a *ActionSurge) Definition(owner core.Entity) (*actions.Definition, error) {
	definition, err := DescribeFromRef(&DescribeFromRefInput{
		Ref: *refs.Features.ActionSurge(),
		Giver: dnd5eEvents.RollSource{
			Ref: refs.Classes.Fighter(), Name: "Fighter", Label: "Fighter", SourceID: owner.GetID(),
		},
		Pools: map[coreResources.ResourceKey]int{resources.ActionSurge: 1},
	})
	if err != nil {
		return nil, err
	}
	definition.Description = a.Description()
	return definition, nil
}

// GetID implements core.Entity
func (a *ActionSurge) GetID() string {
	return a.id
}

// GetType implements core.Entity
func (a *ActionSurge) GetType() core.EntityType {
	return EntityTypeFeature
}

// CanActivate implements core.Action[FeatureInput]: the owner's shared pool
// has a use remaining.
func (a *ActionSurge) CanActivate(_ context.Context, owner core.Entity, _ FeatureInput) error {
	accessor, ok := owner.(coreResources.ResourceAccessor)
	if !ok {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "owner does not implement ResourceAccessor")
	}
	if !accessor.IsResourceAvailable(resources.ActionSurge) {
		return rpgerr.New(rpgerr.CodeResourceExhausted, "no action surge uses remaining")
	}
	return nil
}

// Activate implements core.Action[FeatureInput] by refusing: Action Surge is
// used from its description. Build the action with [ActionSurge.Definition]
// and pay its cost at the gate; nothing here spends or grants.
func (a *ActionSurge) Activate(_ context.Context, _ core.Entity, _ FeatureInput) error {
	return rpgerr.Wrapf(ErrUsedFromItsDescription, "action surge")
}

// loadJSON loads Action Surge state from JSON. Old "uses" and "max_uses" keys
// are not read: the count is the owner's pool.
func (a *ActionSurge) loadJSON(data json.RawMessage) error {
	var actionSurgeData ActionSurgeData
	if err := json.Unmarshal(data, &actionSurgeData); err != nil {
		return fmt.Errorf("failed to unmarshal action surge data: %w", err)
	}

	a.id = actionSurgeData.ID
	a.name = actionSurgeData.Name
	a.characterID = actionSurgeData.CharacterID

	return nil
}

// ToJSON converts Action Surge to JSON for persistence
func (a *ActionSurge) ToJSON() (json.RawMessage, error) {
	data := ActionSurgeData{
		Ref:         refs.Features.ActionSurge(),
		ID:          a.id,
		Name:        a.name,
		CharacterID: a.characterID,
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal action surge data: %w", err)
	}

	return bytes, nil
}

// ActionType returns the action economy cost to activate action surge (free - it grants an extra action)
func (a *ActionSurge) ActionType() combat.ActionType {
	return combat.ActionFree
}
