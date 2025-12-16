// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monstertraits

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// PackTacticsData is the JSON structure for persisting pack tactics trait state
type PackTacticsData struct {
	Ref     *core.Ref `json:"ref"`
	OwnerID string    `json:"owner_id"`
}

// packTacticsCondition represents a creature's Pack Tactics ability.
// Pack Tactics grants advantage on attack rolls against a creature if at least
// one of the attacker's allies is within 5 feet of the target and not incapacitated.
//
// Note: This is a simplified implementation. The full logic of determining if an ally
// is adjacent to the target would be handled by the game server using spatial/perception
// data. This trait simply provides the advantage bonus when applicable.
type packTacticsCondition struct {
	ownerID string
	bus     events.EventBus
	subID   string
}

// Ensure packTacticsCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*packTacticsCondition)(nil)

// PackTactics creates a new pack tactics trait
func PackTactics(ownerID string) dnd5eEvents.ConditionBehavior {
	return &packTacticsCondition{
		ownerID: ownerID,
	}
}

// IsApplied returns true if this condition is currently applied
func (p *packTacticsCondition) IsApplied() bool {
	return p.bus != nil
}

// Apply subscribes this condition to relevant combat events
func (p *packTacticsCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if p.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "pack tactics condition already applied")
	}
	p.bus = bus

	// Subscribe to attack chain to grant advantage when ally is adjacent to target
	attackChain := dnd5eEvents.AttackChain.On(bus)
	subID, err := attackChain.SubscribeWithChain(ctx, p.onAttackChain)
	if err != nil {
		return err
	}
	p.subID = subID

	return nil
}

// Remove unsubscribes this condition from events
func (p *packTacticsCondition) Remove(ctx context.Context, bus events.EventBus) error {
	if p.bus == nil {
		return nil // Not applied, nothing to remove
	}

	if p.subID != "" {
		err := bus.Unsubscribe(ctx, p.subID)
		if err != nil {
			return err
		}
	}

	p.subID = ""
	p.bus = nil
	return nil
}

// ToJSON converts the condition to JSON for persistence
func (p *packTacticsCondition) ToJSON() (json.RawMessage, error) {
	data := PackTacticsData{
		Ref:     refs.MonsterTraits.PackTactics(),
		OwnerID: p.ownerID,
	}
	return json.Marshal(data)
}

// loadJSON loads pack tactics condition state from JSON
func (p *packTacticsCondition) loadJSON(data json.RawMessage) error {
	var tacticsData PackTacticsData
	if err := json.Unmarshal(data, &tacticsData); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal pack tactics data")
	}

	p.ownerID = tacticsData.OwnerID

	return nil
}

// onAttackChain grants advantage on attacks when ally is adjacent to target.
//
// Note: This is a placeholder implementation. The full Pack Tactics logic requires:
// 1. Access to spatial/perception data to know where allies are
// 2. Checking if any ally (same faction, not incapacitated) is within 5 feet of target
//
// For now, this demonstrates the chain pattern. The actual ally-checking logic
// would be implemented by the game server before publishing attack events, or this
// function would need access to a perception/spatial service.
func (p *packTacticsCondition) onAttackChain(
	_ context.Context,
	event dnd5eEvents.AttackChainEvent,
	c chain.Chain[dnd5eEvents.AttackChainEvent],
) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
	// Only process if we're the attacker
	if event.AttackerID != p.ownerID {
		return c, nil
	}

	// TODO: In full implementation, check if ally is adjacent to target
	// For now, this is a no-op that preserves the chain pattern
	// The game server would determine if Pack Tactics applies before creating the attack

	// Example of how advantage would be granted:
	// modifyAttack := func(_ context.Context, e dnd5eEvents.AttackChainEvent) (dnd5eEvents.AttackChainEvent, error) {
	// 	e.AdvantageSources = append(e.AdvantageSources, dnd5eEvents.AttackModifierSource{
	// 		SourceRef: refs.MonsterTraits.PackTactics(),
	// 		SourceID:  p.ownerID,
	// 		Reason:    "Pack Tactics - ally adjacent to target",
	// 	})
	// 	return e, nil
	// }
	//
	// err := c.Add(combat.StageFeatures, "pack_tactics", modifyAttack)
	// if err != nil {
	// 	return c, rpgerr.Wrapf(err, "error applying pack tactics for owner %s", p.ownerID)
	// }

	return c, nil
}
