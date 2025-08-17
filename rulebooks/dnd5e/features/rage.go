// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"context"
	"errors"
	"sync"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
)

// Rage implements a barbarian's rage feature
type Rage struct {
	id    string
	uses  int
	level int
	bus   *events.Bus

	// Protect concurrent access to state
	mu sync.RWMutex

	// Track current state (protected by mu)
	currentUses int
	active      bool
	owner       core.Entity // Who is raging

	// Track subscriptions for cleanup (protected by mu)
	subscriptions []string
}

// GetID returns the entity's unique identifier
func (r *Rage) GetID() string { return r.id }

// GetType returns the entity type (feature)
func (r *Rage) GetType() core.EntityType { return dnd5e.EntityTypeFeature }

// GetResourceType returns what resource this feature consumes
func (r *Rage) GetResourceType() ResourceType { return ResourceTypeRageUses }

// ResetsOn returns when this feature's uses reset
func (r *Rage) ResetsOn() ResetType { return ResetTypeLongRest }

// CanActivate checks if rage can be activated
func (r *Rage) CanActivate(_ context.Context, _ core.Entity, _ FeatureInput) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.currentUses <= 0 {
		return errors.New("no rage uses remaining")
	}
	if r.active {
		return errors.New("already raging")
	}
	return nil
}

// Activate enters rage mode and subscribes to combat events
func (r *Rage) Activate(ctx context.Context, owner core.Entity, input FeatureInput) error {
	if err := r.CanActivate(ctx, owner, input); err != nil {
		return err
	}

	r.mu.Lock()
	r.currentUses--
	r.active = true
	r.owner = owner // Store who is raging
	r.mu.Unlock()

	// Subscribe to attack events (for damage bonus)
	attackSub, err := r.bus.SubscribeWithFilter(
		dnd5e.EventRefAttack,
		r.onAttack,
		func(e events.Event) bool {
			if attack, ok := e.(*dnd5e.AttackEvent); ok {
				r.mu.RLock()
				defer r.mu.RUnlock()
				return attack.Attacker == r.owner
			}
			return false
		},
	)
	if err == nil {
		r.mu.Lock()
		r.subscriptions = append(r.subscriptions, attackSub)
		r.mu.Unlock()
	}

	// Subscribe to damage events (for resistance)
	damageSub, err := r.bus.SubscribeWithFilter(
		dnd5e.EventRefDamageReceived,
		r.onDamageReceived,
		func(e events.Event) bool {
			if damage, ok := e.(*dnd5e.DamageReceivedEvent); ok {
				r.mu.RLock()
				defer r.mu.RUnlock()
				return damage.Target == r.owner
			}
			return false
		},
	)
	if err == nil {
		r.mu.Lock()
		r.subscriptions = append(r.subscriptions, damageSub)
		r.mu.Unlock()
	}

	// Publish rage started event
	_ = r.bus.Publish(&dnd5e.RageStartedEvent{
		Owner:       owner,
		DamageBonus: r.getDamageBonus(),
	})

	return nil
}

// getDamageBonus returns the rage damage bonus based on barbarian level
func (r *Rage) getDamageBonus() int {
	if r.level >= 16 {
		return 4
	} else if r.level >= 9 {
		return 3
	}
	return 2
}

// onAttack handles attack events to add damage bonus
func (r *Rage) onAttack(e interface{}) error {
	attack := e.(*dnd5e.AttackEvent)

	// Only add bonus to Strength-based melee attacks
	if attack.IsMelee && attack.Ability == dnd5e.AbilityStrength {
		// Add damage bonus as a modifier
		ctx := attack.Context()
		ctx.AddModifier(events.NewSimpleModifier(
			dnd5e.ModifierSourceRage,
			dnd5e.ModifierTypeAdditive,
			dnd5e.ModifierTargetDamage,
			200, // priority (after base damage)
			r.getDamageBonus(),
		))
	}

	return nil
}

// onDamageReceived handles damage events to apply resistance
func (r *Rage) onDamageReceived(e interface{}) error {
	damage := e.(*dnd5e.DamageReceivedEvent)

	// Apply resistance to physical damage
	if damage.DamageType == dnd5e.DamageTypeBludgeoning ||
		damage.DamageType == dnd5e.DamageTypePiercing ||
		damage.DamageType == dnd5e.DamageTypeSlashing {
		// Add resistance modifier (halves damage)
		ctx := damage.Context()
		ctx.AddModifier(events.NewSimpleModifier(
			dnd5e.ModifierSourceRage,
			dnd5e.ModifierTypeResistance,
			dnd5e.ModifierTargetDamage,
			100, // priority (apply early)
			0.5, // multiplier for half damage
		))
	}

	return nil
}
