// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// RecklessAttackData is the JSON structure for persisting reckless attack condition state
type RecklessAttackData struct {
	Ref         *core.Ref `json:"ref"`
	CharacterID string    `json:"character_id"`
}

// RecklessAttackCondition represents the Barbarian's Reckless Attack.
// When active:
//   - The barbarian has advantage on melee STR-based attack rolls
//   - Attack rolls against the barbarian have advantage
//
// The condition lasts until the start of the barbarian's next turn.
type RecklessAttackCondition struct {
	CharacterID     string
	subscriptionIDs []string
	bus             events.EventBus
}

// Ensure RecklessAttackCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*RecklessAttackCondition)(nil)

// NewRecklessAttackCondition creates a new reckless attack condition
func NewRecklessAttackCondition(characterID string) *RecklessAttackCondition {
	return &RecklessAttackCondition{
		CharacterID: characterID,
	}
}

// IsApplied returns true if this condition is currently applied
func (r *RecklessAttackCondition) IsApplied() bool {
	return r.bus != nil
}

// Apply subscribes this condition to AttackChain and TurnStart events.
// AttackChain: grants advantage on own melee attacks AND to enemies targeting this character.
// TurnStart: removes the condition at the start of the character's next turn.
func (r *RecklessAttackCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if r.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "reckless attack already applied")
	}
	r.bus = bus

	// Subscribe to AttackChain to grant advantage on own attacks and to enemies
	attackChain := dnd5eEvents.AttackChain.On(bus)
	subID1, err := attackChain.SubscribeWithChain(ctx, r.onAttackChain)
	if err != nil {
		r.bus = nil
		return rpgerr.Wrap(err, "failed to subscribe to attack chain")
	}
	r.subscriptionIDs = append(r.subscriptionIDs, subID1)

	// Subscribe to TurnStart to remove condition at the start of the character's next turn
	turnStartTopic := dnd5eEvents.TurnStartTopic.On(bus)
	subID2, err := turnStartTopic.Subscribe(ctx, r.onTurnStart)
	if err != nil {
		_ = r.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe to turn start")
	}
	r.subscriptionIDs = append(r.subscriptionIDs, subID2)

	return nil
}

// Remove unsubscribes this condition from events.
func (r *RecklessAttackCondition) Remove(ctx context.Context, bus events.EventBus) error {
	if r.bus == nil {
		return nil
	}

	for _, subID := range r.subscriptionIDs {
		if err := bus.Unsubscribe(ctx, subID); err != nil {
			return rpgerr.Wrap(err, "failed to unsubscribe from event")
		}
	}

	r.subscriptionIDs = nil
	r.bus = nil
	return nil
}

// ToJSON converts the condition to JSON for persistence
func (r *RecklessAttackCondition) ToJSON() (json.RawMessage, error) {
	data := RecklessAttackData{
		Ref:         refs.Conditions.RecklessAttack(),
		CharacterID: r.CharacterID,
	}
	return json.Marshal(data)
}

// loadJSON loads reckless attack condition state from JSON
//
//nolint:unused // Used by loader.go
func (r *RecklessAttackCondition) loadJSON(data json.RawMessage) error {
	var raData RecklessAttackData
	if err := json.Unmarshal(data, &raData); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal reckless attack data")
	}

	r.CharacterID = raData.CharacterID
	return nil
}

// onAttackChain handles attack events to:
// 1. Grant advantage when the barbarian makes melee attacks (attacker == self)
// 2. Grant advantage to enemies attacking the barbarian (target == self)
func (r *RecklessAttackCondition) onAttackChain(
	_ context.Context,
	event dnd5eEvents.AttackChainEvent,
	c chain.Chain[dnd5eEvents.AttackChainEvent],
) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
	isAttacker := event.AttackerID == r.CharacterID
	isTarget := event.TargetID == r.CharacterID

	if !isAttacker && !isTarget {
		return c, nil
	}

	// When the barbarian is the attacker: advantage on melee attacks made during their turn.
	// RAW: "When you make your first attack on your turn" — does not apply to opportunity attacks.
	if isAttacker && event.IsMelee && event.AttackType != dnd5eEvents.AttackTypeOpportunity {
		modifyAttack := func(_ context.Context, e dnd5eEvents.AttackChainEvent) (dnd5eEvents.AttackChainEvent, error) {
			e.AdvantageSources = append(e.AdvantageSources, dnd5eEvents.AttackModifierSource{
				SourceRef: refs.Conditions.RecklessAttack(),
				SourceID:  r.CharacterID,
				Reason:    "Reckless Attack",
			})
			return e, nil
		}

		if err := c.Add(combat.StageFeatures, "reckless_attack_advantage", modifyAttack); err != nil {
			return c, rpgerr.Wrapf(err, "failed to add reckless attack advantage for character %s", r.CharacterID)
		}
	}

	// When the barbarian is the target: enemies get advantage
	if isTarget {
		modifyAttack := func(_ context.Context, e dnd5eEvents.AttackChainEvent) (dnd5eEvents.AttackChainEvent, error) {
			e.AdvantageSources = append(e.AdvantageSources, dnd5eEvents.AttackModifierSource{
				SourceRef: refs.Conditions.RecklessAttack(),
				SourceID:  r.CharacterID,
				Reason:    "Target is reckless",
			})
			return e, nil
		}

		if err := c.Add(combat.StageConditions, "reckless_attack_vulnerability", modifyAttack); err != nil {
			return c, rpgerr.Wrapf(err, "failed to add reckless vulnerability for character %s", r.CharacterID)
		}
	}

	return c, nil
}

// onTurnStart removes the condition when the barbarian's turn starts.
func (r *RecklessAttackCondition) onTurnStart(ctx context.Context, event dnd5eEvents.TurnStartEvent) error {
	if event.CharacterID != r.CharacterID {
		return nil
	}

	if r.bus == nil {
		return nil
	}

	// Publish condition removed event before unsubscribing so the character's
	// condition tracker knows to drop this condition from persistence.
	removals := dnd5eEvents.ConditionRemovedTopic.On(r.bus)
	if err := removals.Publish(ctx, dnd5eEvents.ConditionRemovedEvent{
		CharacterID:  r.CharacterID,
		ConditionRef: refs.Conditions.RecklessAttack().String(),
		Reason:       "turn_start",
	}); err != nil {
		return rpgerr.Wrapf(err, "failed to publish reckless attack removal for character %s", r.CharacterID)
	}

	return r.Remove(ctx, r.bus)
}
