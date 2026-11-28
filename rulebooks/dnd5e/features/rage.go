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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
)

// Rage represents the barbarian rage feature.
// It implements core.Action[FeatureInput] for the sheet's list of abilities,
// and it is used from its description ([Rage.Definition]), not from its own
// code. The character owns the rage charges resource; the description's cost
// names it and the gate spends it.
//
// It stores no barbarian level. The description asks its owner whether rages
// are unlimited (barbarian 20); the Raging condition it names reads its damage
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

// Description returns what activating this feature does, in the player's voice.
func (r *Rage) Description() string {
	return "Spend a rage charge to enter a rage. While raging you add bonus damage to Strength melee weapon attacks, have advantage on Strength checks and saving throws, and resist bludgeoning, piercing and slashing damage."
}

// GetID implements core.Entity
func (r *Rage) GetID() string {
	return r.id
}

// GetType implements core.Entity
func (r *Rage) GetType() core.EntityType {
	return EntityTypeFeature
}

// refuseOwner refuses what Rage asks about WHO the owner is and WHAT she
// holds, and returns her barbarian level: a barbarian already raging is refused
// (a second Rage would put a second Raging condition on the sheet), and so is
// an owner whose conditions cannot be read, since not knowing is not "not
// raging". So is an owner that cannot answer its barbarian level, or holds
// none: whether rages are unlimited is asked of the owner, never assumed. Both
// [Rage.CanActivate] and [Rage.Definition] ask it, so the two never differ. It
// does not ask for a charge: that is the gate's, and the row's CanActivate.
func (r *Rage) refuseOwner(owner core.Entity) (int, error) {
	if err := refuseWhileHolding(&refuseWhileHoldingInput{
		Owner: owner, Ref: refs.Conditions.Raging(), Reason: "already raging",
	}); err != nil {
		return 0, err
	}
	return ownerClassLevel(owner, classes.Barbarian, "rage")
}

// Definition describes Rage as data, with this owner's numbers: a bonus action
// and one rage charge (none at barbarian 20, whose rages are unlimited), to put
// the raging status on its user. The status's damage bonus is read from her
// barbarian level when she strikes, so no number is written here. This is the
// one way to use it: the resolver builds the status, pays the cost at the gate
// and publishes the status, and [Rage.Activate] refuses. A barbarian already
// raging, and an owner with no barbarian level, are refused as
// [Rage.CanActivate] refuses them; an empty pool is not refused here, the gate
// refuses that payment.
func (r *Rage) Definition(owner core.Entity) (*actions.Definition, error) {
	level, err := r.refuseOwner(owner)
	if err != nil {
		return nil, err
	}
	var pools map[coreResources.ResourceKey]int
	if level < unlimitedRageLevel {
		pools = map[coreResources.ResourceKey]int{resources.RageCharges: 1}
	}
	definition, err := DescribeFromRef(&DescribeFromRefInput{
		Ref:        *refs.Features.Rage(),
		Parameters: json.RawMessage(`{"damage_bonus": "barbarian level"}`),
		Giver: dnd5eEvents.RollSource{
			Ref: refs.Classes.Barbarian(), Name: "Barbarian", Label: "Barbarian", SourceID: owner.GetID(),
		},
		Pools: pools,
	})
	if err != nil {
		return nil, err
	}
	definition.Description = r.Description()
	if r.name != "" {
		definition.Name = r.name
	}
	if err := definition.Validate(); err != nil {
		return nil, rpgerr.Wrapf(err, "rage description is invalid")
	}
	return definition, nil
}

// CanActivate implements core.Action[FeatureInput]. It refuses what
// [Rage.Definition] refuses (a barbarian already raging, an owner with no
// barbarian level) and, for a barbarian below level 20, one with no charges
// left. It runs on every Afford with an empty FeatureInput, so its refusal is
// also the reason the Rage row reads unavailable.
func (r *Rage) CanActivate(_ context.Context, owner core.Entity, _ FeatureInput) error {
	level, err := r.refuseOwner(owner)
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

// Activate implements core.Action[FeatureInput] by refusing: Rage is used from
// its description. Build the action with [Rage.Definition]; the resolver builds
// the status and pays its cost at the gate. Nothing here builds, spends or
// publishes.
func (r *Rage) Activate(_ context.Context, _ core.Entity, _ FeatureInput) error {
	return rpgerr.Wrapf(ErrUsedFromItsDescription, "rage")
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
