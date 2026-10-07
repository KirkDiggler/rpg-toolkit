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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
)

// Rage represents the barbarian rage feature.
// It implements core.Action[FeatureInput] for activation.
// Rage uses the owner's resources via ResourceAccessor - the character owns the
// rage charges resource, and this feature consumes from it.
//
// It stores no barbarian level. Activation asks its owner whether rages are
// unlimited (barbarian 20); the Raging condition it applies reads its damage
// bonus from each attack's frame.
type Rage struct {
	id   string
	name string
}

// RageData is the JSON structure for persisting rage state.
// Note: Resource state (uses/max) is owned by the Character, not the feature.
// A blob saved with the old "level" key loads and the copy is ignored.
type RageData struct {
	Ref  *core.Ref `json:"ref"`
	ID   string    `json:"id"`
	Name string    `json:"name"`
}

// unlimitedRageLevel is the barbarian level from which rages are unlimited.
const unlimitedRageLevel = 20

// Ref returns the unique ref for the Rage feature.
func (r *Rage) Ref() *core.Ref { return refs.Features.Rage() }

// Status reports the barbarian's character-owned RageCharges pool through the
// non-mutating status surface, without serializing ToJSON. Rage does not own
// its resource — the Character does — so a non-nil [StatusInput.Owner] that
// carries RageCharges is required.
func (r *Rage) Status(in *StatusInput) (*StatusOutput, error) {
	if in == nil || in.Owner == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "rage status requires an owner resource reader")
	}
	current, maximum, ok := in.Owner.ResourceStatus(resources.RageCharges)
	if !ok {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "owner does not carry rage_charges")
	}
	name := r.name
	if name == "" {
		name = "Rage"
	}
	return &StatusOutput{Status: &Status{
		Ref:  *refs.Features.Rage(),
		Name: name,
		Resource: &ResourceStatus{
			Key:     resources.RageCharges,
			Name:    "Rage",
			Current: current,
			Maximum: maximum,
		},
	}}, nil
}

// Name returns the display name for the Rage feature.
func (r *Rage) Name() string { return r.name }

// GetID implements core.Entity
func (r *Rage) GetID() string {
	return r.id
}

// GetType implements core.Entity
func (r *Rage) GetType() core.EntityType {
	return EntityTypeFeature
}

// CanActivate implements core.Action[FeatureInput]. It refuses a barbarian
// who is already raging — a second Rage would put a second Raging condition on
// the sheet and spend a charge on nothing — and one with no charges left. It
// runs on every Afford with an empty FeatureInput, so its refusal is also the
// reason the Rage row reads unavailable. An owner whose conditions cannot be
// read is refused: not knowing is not "not raging". So is an owner that cannot
// answer its barbarian level, or holds none: whether rages are unlimited is
// asked of the owner, never assumed.
func (r *Rage) CanActivate(_ context.Context, owner core.Entity, _ FeatureInput) error {
	if err := refuseWhileHolding(&refuseWhileHoldingInput{
		Owner: owner, Ref: refs.Conditions.Raging(), Reason: "already raging",
	}); err != nil {
		return err
	}

	level, err := ownerClassLevel(owner, classes.Barbarian, "rage")
	if err != nil {
		return err
	}
	// At level 20, barbarians have unlimited rages
	if level >= unlimitedRageLevel {
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

	level, err := ownerClassLevel(owner, classes.Barbarian, "rage")
	if err != nil {
		return err
	}
	// Consume a use (unless level 20)
	if level < unlimitedRageLevel {
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

	return nil
}

// ToJSON converts rage to JSON for persistence
func (r *Rage) ToJSON() (json.RawMessage, error) {
	data := RageData{
		Ref:  refs.Features.Rage(),
		ID:   r.id,
		Name: r.name,
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
