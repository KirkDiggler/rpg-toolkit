// Package affinity provides reusable D&D 5e damage resistance, vulnerability,
// and immunity conditions for creatures, equipment, and temporary effects.
package affinity

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// Kind identifies whether an affinity halves, doubles, or negates damage.
type Kind string

const (
	Resistance    Kind = "resistance"
	Vulnerability Kind = "vulnerability"
	Immunity      Kind = "immunity"
)

// Data is the persisted form of an affinity. SourceID identifies the thing
// granting it, such as an equipped item's instance ID, so that effect can be
// removed independently later.
type Data struct {
	Ref        *core.Ref   `json:"ref"`
	OwnerID    string      `json:"owner_id"`
	DamageType damage.Type `json:"damage_type"`
	SourceID   string      `json:"source_id,omitempty"`
}

// Condition applies one damage affinity to a single creature or character.
type Condition struct {
	kind       Kind
	ownerID    string
	damageType damage.Type
	sourceID   string
	bus        events.EventBus
	subID      string
}

var _ dnd5eEvents.ConditionBehavior = (*Condition)(nil)

// New creates an affinity condition. SourceID may be empty for an innate trait.
func New(kind Kind, ownerID string, damageType damage.Type, sourceID string) (*Condition, error) {
	if refFor(kind) == nil {
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "unknown damage affinity kind: %s", kind)
	}
	if ownerID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "damage affinity owner is required")
	}
	if _, err := damage.GetByID(string(damageType)); err != nil || damageType == damage.None {
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "invalid damage affinity type: %s", damageType)
	}
	return &Condition{kind: kind, ownerID: ownerID, damageType: damageType, sourceID: sourceID}, nil
}

func refFor(kind Kind) *core.Ref {
	switch kind {
	case Resistance:
		return refs.DamageAffinities.Resistance()
	case Vulnerability:
		return refs.DamageAffinities.Vulnerability()
	case Immunity:
		return refs.DamageAffinities.Immunity()
	default:
		return nil
	}
}

// LoadJSON restores an affinity condition from persisted JSON.
func LoadJSON(raw json.RawMessage) (*Condition, error) {
	var data Data
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, rpgerr.Wrap(err, "unmarshal damage affinity")
	}
	if data.Ref == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "damage affinity ref is required")
	}
	return New(Kind(data.Ref.ID), data.OwnerID, data.DamageType, data.SourceID)
}

func (c *Condition) IsApplied() bool { return c.bus != nil }

// Apply begins modifying matching incoming damage for the owner.
func (c *Condition) Apply(ctx context.Context, bus events.EventBus) error {
	if c.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "damage affinity already applied")
	}
	c.bus = bus
	subID, err := dnd5eEvents.DamageChain.On(bus).SubscribeWithChain(ctx, c.onDamageChain)
	if err != nil {
		c.bus = nil
		return err
	}
	c.subID = subID
	return nil
}

// Remove stops the affinity. Equipment can call this when it is unequipped.
func (c *Condition) Remove(ctx context.Context, bus events.EventBus) error {
	if !c.IsApplied() {
		return nil
	}
	if err := bus.Unsubscribe(ctx, c.subID); err != nil {
		return err
	}
	c.subID = ""
	c.bus = nil
	return nil
}

func (c *Condition) ToJSON() (json.RawMessage, error) {
	return json.Marshal(Data{Ref: refFor(c.kind), OwnerID: c.ownerID, DamageType: c.damageType, SourceID: c.sourceID})
}

func (c *Condition) onDamageChain(_ context.Context, event *dnd5eEvents.DamageChainEvent, chain chain.Chain[*dnd5eEvents.DamageChainEvent]) (chain.Chain[*dnd5eEvents.DamageChainEvent], error) {
	if event.TargetID != c.ownerID {
		return chain, nil
	}
	for _, component := range event.Components {
		if component.DamageType == c.damageType {
			apply := func(_ context.Context, e *dnd5eEvents.DamageChainEvent) (*dnd5eEvents.DamageChainEvent, error) {
				e.Components = append(e.Components, dnd5eEvents.DamageComponent{Source: dnd5eEvents.DamageSourceCondition, SourceRef: refFor(c.kind), DamageType: c.damageType, Multiplier: c.multiplier()})
				return e, nil
			}
			id := fmt.Sprintf("damage-affinity:%s:%s:%s:%s", c.kind, c.ownerID, c.damageType, c.sourceID)
			if err := chain.Add(combat.StageFinal, id, apply); err != nil {
				return chain, rpgerr.Wrap(err, "apply damage affinity")
			}
			break
		}
	}
	return chain, nil
}

func (c *Condition) multiplier() float64 {
	switch c.kind {
	case Resistance:
		return 0.5
	case Vulnerability:
		return 2
	default:
		return 0
	}
}
