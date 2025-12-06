// Package features provides D&D 5e class features implementation
package features

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// DeflectMissiles represents the monk's Deflect Missiles feature.
// It implements events.BusEffect to passively reduce ranged weapon attack damage.
// It also implements core.Action[FeatureInput] for the optional catch-and-throw attack.
type DeflectMissiles struct {
	id              string
	name            string
	characterID     string
	monkLevel       int
	dexModifier     int
	subscriptionIDs []string
	bus             events.EventBus
	roller          dice.Roller
}

// DeflectMissilesData is the JSON structure for persisting Deflect Missiles state
type DeflectMissilesData struct {
	Ref         *core.Ref `json:"ref"`
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	CharacterID string    `json:"character_id"`
	MonkLevel   int       `json:"monk_level"`
	DexModifier int       `json:"dex_modifier"`
}

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

// Apply implements events.BusEffect
// Subscribes to damage received events to provide damage reduction
func (d *DeflectMissiles) Apply(ctx context.Context, bus events.EventBus) error {
	if d.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "deflect missiles already applied")
	}
	d.bus = bus

	// Subscribe to damage received events
	damageTopic := dnd5eEvents.DamageReceivedTopic.On(bus)
	subID, err := damageTopic.Subscribe(ctx, d.onDamageReceived)
	if err != nil {
		return rpgerr.Wrap(err, "failed to subscribe to damage events")
	}
	d.subscriptionIDs = append(d.subscriptionIDs, subID)

	return nil
}

// Remove implements events.BusEffect
func (d *DeflectMissiles) Remove(ctx context.Context, bus events.EventBus) error {
	if !d.IsApplied() {
		return nil // Not applied, nothing to remove
	}

	for _, subID := range d.subscriptionIDs {
		err := bus.Unsubscribe(ctx, subID)
		if err != nil {
			return rpgerr.Wrapf(err, "failed to unsubscribe from event: %s", subID)
		}
	}

	d.subscriptionIDs = nil
	d.bus = nil
	return nil
}

// CanActivate implements core.Action[FeatureInput]
// For the catch-and-throw portion, which requires damage to have been reduced to 0
func (d *DeflectMissiles) CanActivate(_ context.Context, _ core.Entity, _ FeatureInput) error {
	// This would typically check if:
	// 1. Damage was just reduced to 0 this turn
	// 2. Character has 1 Ki available
	// For now, we keep it simple - the game server manages this state
	return nil
}

// Activate implements core.Action[FeatureInput]
// Publishes event indicating the monk is throwing the missile back
func (d *DeflectMissiles) Activate(ctx context.Context, owner core.Entity, input FeatureInput) error {
	if err := d.CanActivate(ctx, owner, input); err != nil {
		return err
	}

	// Publish event for catch-and-throw
	// The game server will handle:
	// - Consuming 1 Ki point
	// - Making the attack roll
	// - Dealing damage
	if input.Bus != nil {
		topic := dnd5eEvents.DeflectMissilesThrowTopic.On(input.Bus)
		err := topic.Publish(ctx, dnd5eEvents.DeflectMissilesThrowEvent{
			CharacterID: owner.GetID(),
			Source:      refs.Features.DeflectMissiles().ID,
		})
		if err != nil {
			return rpgerr.Wrap(err, "failed to publish deflect missiles throw event")
		}
	}

	return nil
}

// onDamageReceived handles damage events to reduce ranged weapon attack damage
func (d *DeflectMissiles) onDamageReceived(ctx context.Context, event dnd5eEvents.DamageReceivedEvent) error {
	// Only process damage to this character
	if event.TargetID != d.characterID {
		return nil
	}

	// Only deflect ranged weapon attacks
	// The game server should mark ranged weapon attacks appropriately
	// For now, we assume the DamageType or SourceID indicates this
	// TODO: This needs better integration with the attack system to know if it's ranged

	// Calculate reduction: 1d10 + DEX modifier + monk level
	reduction := d.calculateReduction(ctx)

	// Publish damage reduction event
	if d.bus != nil {
		topic := dnd5eEvents.DeflectMissilesTriggerTopic.On(d.bus)
		err := topic.Publish(ctx, dnd5eEvents.DeflectMissilesTriggerEvent{
			CharacterID:      d.characterID,
			OriginalDamage:   event.Amount,
			Reduction:        reduction,
			DamageReducedTo0: event.Amount <= reduction,
			Source:           refs.Features.DeflectMissiles().ID,
		})
		if err != nil {
			return rpgerr.Wrap(err, "failed to publish deflect missiles trigger event")
		}
	}

	return nil
}

// calculateReduction calculates damage reduction: 1d10 + DEX modifier + monk level
func (d *DeflectMissiles) calculateReduction(ctx context.Context) int {
	if d.roller == nil {
		d.roller = &dice.CryptoRoller{}
	}

	// Roll 1d10
	roll, err := d.roller.Roll(ctx, 10)
	if err != nil {
		// Fallback to average on error (5.5, rounded down to 5)
		return 5 + d.dexModifier + d.monkLevel
	}

	return roll + d.dexModifier + d.monkLevel
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
	d.monkLevel = deflectData.MonkLevel
	d.dexModifier = deflectData.DexModifier

	return nil
}

// ToJSON converts Deflect Missiles to JSON for persistence
func (d *DeflectMissiles) ToJSON() (json.RawMessage, error) {
	data := DeflectMissilesData{
		Ref:         refs.Features.DeflectMissiles(),
		ID:          d.id,
		Name:        d.name,
		CharacterID: d.characterID,
		MonkLevel:   d.monkLevel,
		DexModifier: d.dexModifier,
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal deflect missiles data: %w", err)
	}

	return bytes, nil
}
