package monstertraits

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// ConditionImmunityData is the persisted form of an innate monster trait that
// blocks standard D&D 5e conditions.
type ConditionImmunityData struct {
	Ref        *core.Ref                   `json:"ref"`
	OwnerID    string                      `json:"owner_id"`
	Conditions []dnd5eEvents.ConditionType `json:"conditions"`
}

// ConditionImmunityTrait is an innate monster trait. It may only name the
// fifteen standard D&D 5e conditions; statuses and passive effects are not
// valid targets for condition immunity.
//
// It implements ConditionBehavior because that is the toolkit's current
// technical lifecycle interface for active effects. The public rules
// vocabulary calls this a monster trait, not a condition.
type ConditionImmunityTrait struct {
	ownerID    string
	conditions []dnd5eEvents.ConditionType
	applied    bool
}

var _ dnd5eEvents.ConditionBehavior = (*ConditionImmunityTrait)(nil)

// NewConditionImmunity creates an innate trait for one or more standard
// conditions. Exhaustion is represented once and blocks all six levels.
func NewConditionImmunity(ownerID string, conditions ...dnd5eEvents.ConditionType) (*ConditionImmunityTrait, error) {
	if ownerID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "condition immunity owner is required")
	}
	if len(conditions) == 0 {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "at least one condition immunity is required")
	}

	seen := make(map[dnd5eEvents.ConditionType]struct{}, len(conditions))
	validated := make([]dnd5eEvents.ConditionType, 0, len(conditions))
	for _, conditionType := range conditions {
		if !dnd5eEvents.IsStandardCondition(conditionType) {
			return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "condition immunity must use a standard D&D condition: %s", conditionType)
		}
		if _, duplicate := seen[conditionType]; duplicate {
			return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "duplicate condition immunity: %s", conditionType)
		}
		seen[conditionType] = struct{}{}
		validated = append(validated, conditionType)
	}

	return &ConditionImmunityTrait{ownerID: ownerID, conditions: validated}, nil
}

// ConditionImmunityJSON serializes the trait for a monster factory.
func ConditionImmunityJSON(ownerID string, conditions ...dnd5eEvents.ConditionType) (json.RawMessage, error) {
	trait, err := NewConditionImmunity(ownerID, conditions...)
	if err != nil {
		return nil, err
	}
	return trait.ToJSON()
}

// MustConditionImmunityJSON serializes the trait or panics for invalid
// factory data, which is a programming error.
func MustConditionImmunityJSON(ownerID string, conditions ...dnd5eEvents.ConditionType) json.RawMessage {
	data, err := ConditionImmunityJSON(ownerID, conditions...)
	if err != nil {
		panic("monstertraits: failed to marshal condition immunity JSON: " + err.Error())
	}
	return data
}

func (c *ConditionImmunityTrait) IsApplied() bool { return c.applied }

// Apply marks this innate trait active. It has no event subscription because
// the centralized condition-application path asks the target directly.
func (c *ConditionImmunityTrait) Apply(_ context.Context, _ events.EventBus) error {
	if c.applied {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "condition immunity trait already applied")
	}
	c.applied = true
	return nil
}

// Remove marks this trait inactive.
func (c *ConditionImmunityTrait) Remove(_ context.Context, _ events.EventBus) error {
	c.applied = false
	return nil
}

// IsImmuneTo reports whether conditionType is blocked by this trait.
func (c *ConditionImmunityTrait) IsImmuneTo(conditionType dnd5eEvents.ConditionType) bool {
	for _, immuneTo := range c.conditions {
		if dnd5eEvents.MatchesConditionImmunity(immuneTo, conditionType) {
			return true
		}
	}
	return false
}

func (c *ConditionImmunityTrait) ToJSON() (json.RawMessage, error) {
	return json.Marshal(ConditionImmunityData{
		Ref:        refs.MonsterTraits.ConditionImmunity(),
		OwnerID:    c.ownerID,
		Conditions: c.conditions,
	})
}

func (c *ConditionImmunityTrait) loadJSON(data json.RawMessage) error {
	var persisted ConditionImmunityData
	if err := json.Unmarshal(data, &persisted); err != nil {
		return rpgerr.Wrap(err, "unmarshal condition immunity trait")
	}
	trait, err := NewConditionImmunity(persisted.OwnerID, persisted.Conditions...)
	if err != nil {
		return err
	}
	c.ownerID = trait.ownerID
	c.conditions = trait.conditions
	return nil
}
