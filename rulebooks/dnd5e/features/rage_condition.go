// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"sync"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/damage"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
)

// RagingCondition represents active rage with duration and combat tracking
type RagingCondition struct {
	owner  string
	level  int
	bus    *events.Bus

	// Thread-safe state
	mu                sync.RWMutex
	ticksRemaining    int
	attackedThisRound bool
	wasHitThisRound   bool

	// Event subscriptions for cleanup
	subscriptions []string
}

// NewRagingCondition creates a rage condition from event data
func NewRagingCondition(data map[string]any, bus *events.Bus) (*RagingCondition, error) {
	rc := &RagingCondition{
		bus:            bus,
		ticksRemaining: 10, // Default duration
	}

	// Extract level from data
	if level, ok := data["level"].(int); ok {
		rc.level = level
	}

	// Extract duration if provided
	if duration, ok := data["duration"].(int); ok {
		rc.ticksRemaining = duration
	}

	// Extract initial combat state if provided
	if attacked, ok := data["attacked_this_round"].(bool); ok {
		rc.attackedThisRound = attacked
	}
	if wasHit, ok := data["was_hit_this_round"].(bool); ok {
		rc.wasHitThisRound = wasHit
	}

	return rc, nil
}

// Apply subscribes to relevant events and sets up the condition
func (rc *RagingCondition) Apply(bus *events.Bus, owner core.Entity) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	rc.bus = bus
	rc.owner = owner.GetID()

	// Subscribe to combat events
	// We'll filter to only our events in the handlers
	attackSub, _ := bus.Subscribe(dnd5e.EventRefAttack, rc.onAttack)
	damageSub, _ := bus.Subscribe(dnd5e.EventRefDamageReceived, rc.onDamageReceived)
	roundSub, _ := bus.Subscribe(dnd5e.EventRefRoundEnd, rc.onRoundEnd)
	
	rc.subscriptions = []string{attackSub, damageSub, roundSub}

	return nil
}

// onAttack tracks attacks and adds damage bonus
func (rc *RagingCondition) onAttack(e any) error {
	attack, ok := e.(*dnd5e.AttackEvent)
	if !ok {
		return nil
	}

	// Only care about our attacks
	if attack.Attacker.GetID() != rc.owner {
		return nil
	}
	
	// Check if we're still active
	rc.mu.RLock()
	if rc.subscriptions == nil {
		rc.mu.RUnlock()
		return nil
	}
	rc.mu.RUnlock()

	rc.mu.Lock()
	rc.attackedThisRound = true
	rc.mu.Unlock()

	// Add damage bonus to STR melee attacks
	if attack.IsMelee && attack.Ability == dnd5e.AbilityStrength {
		ctx := attack.Context()
		damageBonus := rc.calculateDamageBonus()
		ctx.AddModifier(events.NewSimpleModifier(
			dnd5e.ModifierSourceRage,
			dnd5e.ModifierTypeAdditive,
			dnd5e.ModifierTargetDamage,
			200, // High priority
			float64(damageBonus),
		))
	}

	return nil
}

// onDamageReceived tracks being hit and applies resistance
func (rc *RagingCondition) onDamageReceived(e any) error {
	dmg, ok := e.(*dnd5e.DamageReceivedEvent)
	if !ok {
		return nil
	}

	// Only care about damage to us
	if dmg.Target.GetID() != rc.owner {
		return nil
	}
	
	// Check if we're still active
	rc.mu.RLock()
	if rc.subscriptions == nil {
		rc.mu.RUnlock()
		return nil
	}
	rc.mu.RUnlock()

	rc.mu.Lock()
	rc.wasHitThisRound = true
	rc.mu.Unlock()

	// Apply resistance to physical damage
	if isPhysicalDamage(dmg.DamageType) {
		ctx := dmg.Context()
		ctx.AddModifier(events.NewSimpleModifier(
			dnd5e.ModifierSourceRage,
			dnd5e.ModifierTypeResistance,
			dnd5e.ModifierTargetDamage,
			100, // Resistance priority
			0.5, // Half damage
		))
	}

	return nil
}

// onRoundEnd checks if rage continues
func (rc *RagingCondition) onRoundEnd(e any) error {
	_, ok := e.(*dnd5e.RoundEndEvent)
	if !ok {
		return nil
	}
	
	// Check if we're still active
	rc.mu.RLock()
	if rc.subscriptions == nil {
		rc.mu.RUnlock()
		return nil
	}
	rc.mu.RUnlock()

	rc.mu.Lock()
	defer rc.mu.Unlock()

	// Check if rage ends due to no combat
	if !rc.attackedThisRound && !rc.wasHitThisRound {
		return rc.removeUnsafe("You didn't attack or take damage")
	}

	// Check duration
	rc.ticksRemaining--
	if rc.ticksRemaining <= 0 {
		return rc.removeUnsafe("Rage duration expired")
	}

	// Reset for next round
	rc.attackedThisRound = false
	rc.wasHitThisRound = false

	return nil
}

// removeUnsafe removes the condition (must be called with lock held)
func (rc *RagingCondition) removeUnsafe(reason string) error {
	// Clear subscriptions list to prevent further processing
	subs := rc.subscriptions
	rc.subscriptions = nil
	
	// Schedule cleanup after we release the lock
	go func() {
		// Unsubscribe from all events
		for _, sub := range subs {
			rc.bus.Unsubscribe(sub)
		}
		
		// Publish removal event
		rc.bus.Publish(dnd5e.NewConditionRemovedEvent(
			rc.owner,
			dnd5e.ConditionRefRaging.String(),
			reason,
		))
	}()
	
	return nil
}

// Remove cleans up the condition
func (rc *RagingCondition) Remove(reason string) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return rc.removeUnsafe(reason)
}

// calculateDamageBonus returns the rage damage bonus based on barbarian level
func (rc *RagingCondition) calculateDamageBonus() int {
	switch {
	case rc.level >= 16:
		return 4
	case rc.level >= 9:
		return 3
	default:
		return 2
	}
}

// isPhysicalDamage checks if damage type is physical (resisted by rage)
func isPhysicalDamage(dmgType damage.Type) bool {
	switch dmgType {
	case dnd5e.DamageTypeSlashing, dnd5e.DamageTypePiercing, dnd5e.DamageTypeBludgeoning:
		return true
	default:
		return false
	}
}

// GetTicksRemaining returns how many rounds are left
func (rc *RagingCondition) GetTicksRemaining() int {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.ticksRemaining
}

// IsActive returns true if the condition is still active
func (rc *RagingCondition) IsActive() bool {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.ticksRemaining > 0
}