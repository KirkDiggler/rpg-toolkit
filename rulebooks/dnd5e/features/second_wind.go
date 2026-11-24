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
	dnd5eCombat "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
)

// SecondWind represents the fighter's Second Wind feature.
// It implements core.Action[FeatureInput] for activation. Its uses are counted
// in the owner's shared resources.SecondWind pool, as Rage's are; the feature
// spends from that pool and keeps no counter of its own.
// It stores no fighter level: activation asks its owner, and heals 1d10 plus
// the level the owner answers.
type SecondWind struct {
	id          string
	name        string
	characterID string // Character this feature belongs to
}

// SecondWindData is the JSON structure for persisting Second Wind state. The
// uses live in the owner's shared pool, not here. A blob saved with the old
// "level", "uses" or "max_uses" keys loads and the copies are ignored.
type SecondWindData struct {
	Ref         *core.Ref `json:"ref"`
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	CharacterID string    `json:"character_id"`
}

// Ref returns the unique ref for the Second Wind feature.
func (s *SecondWind) Ref() *core.Ref { return refs.Features.SecondWind() }

// Status reports the fighter's character-owned SecondWind pool through the
// non-mutating status surface, without serializing ToJSON. Second Wind does not
// own its resource — the Character does — so a non-nil [StatusInput.Owner]
// that carries resources.SecondWind is required.
func (s *SecondWind) Status(in *StatusInput) (*StatusOutput, error) {
	if in == nil || in.Owner == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "second wind status requires an owner resource reader")
	}
	current, maximum, ok := in.Owner.ResourceStatus(resources.SecondWind)
	if !ok {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "owner does not carry second_wind")
	}
	name := s.name
	if name == "" {
		name = "Second Wind"
	}
	return &StatusOutput{Status: &Status{
		Ref:  *refs.Features.SecondWind(),
		Name: name,
		Resource: &ResourceStatus{
			Key:     resources.SecondWind,
			Name:    "Second Wind",
			Current: current,
			Maximum: maximum,
		},
	}}, nil
}

// Name returns the display name for the Second Wind feature.
func (s *SecondWind) Name() string { return s.name }

// Description returns what activating this feature does, in the player's voice.
func (s *SecondWind) Description() string {
	return "Spend a use to regain 1d10 hit points plus your fighter level."
}

// Definition describes Second Wind as data, with this owner's numbers: a bonus
// action and one point of the shared pool, to heal the user 1d10 plus the
// owner's fighter level. This is the one way to use it: the resolver pays the
// cost at the gate and resolves the healing, and [SecondWind.Activate] refuses.
// An owner with no fighter level is refused, as [SecondWind.CanActivate] refuses
// it.
func (s *SecondWind) Definition(owner core.Entity) (*actions.Definition, error) {
	level, err := ownerClassLevel(owner, classes.Fighter, "second wind")
	if err != nil {
		return nil, err
	}
	parameters := fmt.Sprintf(`{"level": %d}`, level)
	definition, err := DescribeFromRef(&DescribeFromRefInput{
		Ref:        *refs.Features.SecondWind(),
		Parameters: json.RawMessage(parameters),
		Giver: dnd5eEvents.RollSource{
			Ref: refs.Classes.Fighter(), Name: "Fighter", Label: "Fighter level", SourceID: owner.GetID(),
		},
		Pools: map[coreResources.ResourceKey]int{resources.SecondWind: 1},
	})
	if err != nil {
		return nil, err
	}
	definition.Description = s.Description()
	return definition, nil
}

// GetID implements core.Entity
func (s *SecondWind) GetID() string {
	return s.id
}

// GetType implements core.Entity
func (s *SecondWind) GetType() core.EntityType {
	return EntityTypeFeature
}

type lifeStateProvider interface {
	LifeState() dnd5eCombat.LifeState
}

// CanActivate implements core.Action[FeatureInput]
func (s *SecondWind) CanActivate(_ context.Context, owner core.Entity, _ FeatureInput) error {
	if provider, ok := owner.(lifeStateProvider); ok &&
		provider.LifeState() == dnd5eCombat.LifeStateDead {
		return rpgerr.New(rpgerr.CodeInvalidState, "dead characters cannot use second wind")
	}

	// Check the owner's shared pool has a use remaining
	accessor, ok := owner.(coreResources.ResourceAccessor)
	if !ok {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "owner does not implement ResourceAccessor")
	}
	if !accessor.IsResourceAvailable(resources.SecondWind) {
		return rpgerr.New(rpgerr.CodeResourceExhausted, "no second wind uses remaining")
	}

	// The healing is 1d10 + fighter level, so an owner that cannot answer its
	// fighter level, or holds none, cannot use it — refused here so the Afford
	// row never reads available for a use Activate would refuse.
	if _, err := ownerClassLevel(owner, classes.Fighter, "second wind"); err != nil {
		return err
	}

	return nil
}

// Activate implements core.Action[FeatureInput] by refusing: Second Wind is
// used from its description. Build the action with [SecondWind.Definition] and
// pay its cost at the gate; nothing here spends, rolls or announces.
func (s *SecondWind) Activate(_ context.Context, _ core.Entity, _ FeatureInput) error {
	return rpgerr.Wrapf(ErrUsedFromItsDescription, "second wind")
}

// loadJSON loads Second Wind state from JSON. Old "uses" and "max_uses" keys
// are not read: the count is the owner's pool.
func (s *SecondWind) loadJSON(data json.RawMessage) error {
	var secondWindData SecondWindData
	if err := json.Unmarshal(data, &secondWindData); err != nil {
		return fmt.Errorf("failed to unmarshal second wind data: %w", err)
	}

	s.id = secondWindData.ID
	s.name = secondWindData.Name
	s.characterID = secondWindData.CharacterID

	return nil
}

// ToJSON converts Second Wind to JSON for persistence
func (s *SecondWind) ToJSON() (json.RawMessage, error) {
	data := SecondWindData{
		Ref:         refs.Features.SecondWind(),
		ID:          s.id,
		Name:        s.name,
		CharacterID: s.characterID,
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal second wind data: %w", err)
	}

	return bytes, nil
}

// ActionType returns the action economy cost to activate second wind (bonus action)
func (s *SecondWind) ActionType() combat.ActionType {
	return combat.ActionBonus
}
