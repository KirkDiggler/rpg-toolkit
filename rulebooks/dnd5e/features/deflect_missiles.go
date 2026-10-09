// Package features provides D&D 5e class features implementation
package features

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// DeflectMissilesThrow is the [FeatureInput.Action] that activates the
// catch-and-throw instead of the deflection.
const DeflectMissilesThrow = "throw"

// DeflectMissiles represents the monk's Deflect Missiles feature: a reaction
// that reduces a ranged weapon attack's damage by 1d10 + Dexterity modifier +
// monk level, and an optional catch-and-throw. It implements
// core.Action[FeatureInput]; both halves are activated by the monk.
//
// It stores no monk level and no Dexterity modifier. The deflection asks its
// owner for both at the moment it is activated.
type DeflectMissiles struct {
	id          string
	name        string
	characterID string
	bus         events.EventBus
}

// DeflectMissilesData is the JSON structure for persisting Deflect Missiles
// state. A blob saved with the old "monk_level" or "dex_modifier" keys loads
// and the copies are ignored.
type DeflectMissilesData struct {
	Ref         *core.Ref `json:"ref"`
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	CharacterID string    `json:"character_id"`
}

// Ref returns the unique ref for the Deflect Missiles feature.
func (d *DeflectMissiles) Ref() *core.Ref { return refs.Features.DeflectMissiles() }

// Status reports the feature's non-mutating status surface, without
// serializing ToJSON. Deflect Missiles owns no resource pool — the optional
// 1-Ki catch-and-throw is paid by the owner at activation, not tracked here —
// so Resource is nil.
func (d *DeflectMissiles) Status(*StatusInput) (*StatusOutput, error) {
	name := d.name
	if name == "" {
		name = "Deflect Missiles"
	}
	return &StatusOutput{Status: &Status{
		Ref:  *refs.Features.DeflectMissiles(),
		Name: name,
	}}, nil
}

// Name returns the display name for the Deflect Missiles feature.
func (d *DeflectMissiles) Name() string { return d.name }

// Description returns "" on purpose. Activation spends the reaction and
// publishes the reduction and throw events, but nothing consumes them, so no
// damage is reduced (toolkit#1992). The card shows missing information rather
// than a benefit the code does not deliver. The prose lands with that repair.
func (d *DeflectMissiles) Description() string { return "" }

// GetID implements core.Entity
func (d *DeflectMissiles) GetID() string {
	return d.id
}

// GetType implements core.Entity
func (d *DeflectMissiles) GetType() core.EntityType {
	return EntityTypeFeature
}

// IsApplied implements events.BusEffect
func (d *DeflectMissiles) IsApplied() bool {
	return d.bus != nil
}

// Apply implements events.BusEffect. Deflect Missiles subscribes to nothing:
// the reduction is the monk's reaction, taken through Activate, not a passive
// answer to every hit — a subscriber has no owner to ask for the monk level
// and Dexterity modifier the reduction is made of.
func (d *DeflectMissiles) Apply(_ context.Context, bus events.EventBus) error {
	if d.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "deflect missiles already applied")
	}
	d.bus = bus
	return nil
}

// Remove implements events.BusEffect
func (d *DeflectMissiles) Remove(_ context.Context, _ events.EventBus) error {
	d.bus = nil
	return nil
}

// CanActivate implements core.Action[FeatureInput]. The deflection is refused
// for an owner that cannot answer its monk level and ability scores, or holds
// no monk levels. The catch-and-throw is not gated here: the game server
// tracks whether the damage reached zero and spends the Ki.
func (d *DeflectMissiles) CanActivate(_ context.Context, owner core.Entity, input FeatureInput) error {
	if input.Action == DeflectMissilesThrow {
		return nil
	}
	_, err := deflectionBonus(owner)
	return err
}

// Activate implements core.Action[FeatureInput].
//
// With no action it is the deflection: it asks the owner its monk level and
// Dexterity modifier, rolls 1d10 with the input's roller, and publishes the
// reduction. With [DeflectMissilesThrow] it publishes the catch-and-throw,
// which the game server resolves (Ki, attack roll, damage).
func (d *DeflectMissiles) Activate(ctx context.Context, owner core.Entity, input FeatureInput) error {
	if err := d.CanActivate(ctx, owner, input); err != nil {
		return err
	}
	if input.Action == DeflectMissilesThrow {
		return d.publishThrow(ctx, owner, input)
	}
	bonus, err := deflectionBonus(owner)
	if err != nil {
		return err
	}
	roller := input.Roller
	if roller == nil {
		roller = dice.NewRoller()
	}
	roll, err := roller.Roll(ctx, 10)
	if err != nil {
		return rpgerr.Wrap(err, "failed to roll deflect missiles reduction")
	}
	if input.Bus != nil {
		topic := dnd5eEvents.DeflectMissilesTriggerTopic.On(input.Bus)
		err := topic.Publish(ctx, dnd5eEvents.DeflectMissilesTriggerEvent{
			CharacterID: owner.GetID(),
			Reduction:   roll + bonus,
			Source:      refs.Features.DeflectMissiles().ID,
		})
		if err != nil {
			return rpgerr.Wrap(err, "failed to publish deflect missiles trigger event")
		}
	}
	return nil
}

// publishThrow publishes the catch-and-throw for the game server to resolve.
func (d *DeflectMissiles) publishThrow(ctx context.Context, owner core.Entity, input FeatureInput) error {
	if input.Bus == nil {
		return nil
	}
	topic := dnd5eEvents.DeflectMissilesThrowTopic.On(input.Bus)
	if err := topic.Publish(ctx, dnd5eEvents.DeflectMissilesThrowEvent{
		CharacterID: owner.GetID(),
		Source:      refs.Features.DeflectMissiles().ID,
	}); err != nil {
		return rpgerr.Wrap(err, "failed to publish deflect missiles throw event")
	}
	return nil
}

// deflectionBonus is the fixed part of the reduction — Dexterity modifier
// plus monk level — asked of the owner at activation. An owner that cannot
// answer either, or holds no monk levels, is refused.
func deflectionBonus(owner core.Entity) (int, error) {
	level, err := ownerClassLevel(owner, classes.Monk, "deflect missiles")
	if err != nil {
		return 0, err
	}
	scores, ok := owner.(interface{ AbilityScores() shared.AbilityScores })
	if !ok {
		return 0, rpgerr.New(rpgerr.CodeInvalidArgument, "deflect missiles: owner cannot answer its ability scores")
	}
	return scores.AbilityScores().Modifier(abilities.DEX) + level, nil
}

// loadJSON loads Deflect Missiles state from JSON
func (d *DeflectMissiles) loadJSON(data json.RawMessage) error {
	var deflectData DeflectMissilesData
	if err := json.Unmarshal(data, &deflectData); err != nil {
		return fmt.Errorf("failed to unmarshal deflect missiles data: %w", err)
	}

	d.id = deflectData.ID
	d.name = deflectData.Name
	d.characterID = deflectData.CharacterID

	return nil
}

// ToJSON converts Deflect Missiles to JSON for persistence
func (d *DeflectMissiles) ToJSON() (json.RawMessage, error) {
	data := DeflectMissilesData{
		Ref:         refs.Features.DeflectMissiles(),
		ID:          d.id,
		Name:        d.name,
		CharacterID: d.characterID,
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal deflect missiles data: %w", err)
	}

	return bytes, nil
}

// ActionType returns the action economy cost to activate deflect missiles (reaction)
func (d *DeflectMissiles) ActionType() combat.ActionType {
	return combat.ActionReaction
}
