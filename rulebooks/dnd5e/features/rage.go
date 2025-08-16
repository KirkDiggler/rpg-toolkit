// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"context"
	"errors"

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

	// Track current state
	currentUses int
	active      bool
	owner       core.Entity // Who is raging
	
	// Track subscriptions for cleanup
	subscriptions []string
}

// Entity interface
func (r *Rage) GetID() string            { return r.id }
func (r *Rage) GetType() core.EntityType { return dnd5e.EntityTypeFeature }

// Feature interface methods
func (r *Rage) GetResourceType() ResourceType { return ResourceTypeRageUses }
func (r *Rage) ResetsOn() ResetType           { return ResetTypeLongRest }

// Action interface
func (r *Rage) CanActivate(ctx context.Context, owner core.Entity, input FeatureInput) error {
	if r.currentUses <= 0 {
		return errors.New("no rage uses remaining")
	}
	if r.active {
		return errors.New("already raging")
	}
	return nil
}

func (r *Rage) Activate(ctx context.Context, owner core.Entity, input FeatureInput) error {
	if err := r.CanActivate(ctx, owner, input); err != nil {
		return err
	}

	r.currentUses--
	r.active = true
	r.owner = owner // Store who is raging

	// Subscribe to attack events (for damage bonus)
	attackSub, err := r.bus.SubscribeWithFilter(
		dnd5e.EventRefAttack,
		r.onAttack,
		func(e events.Event) bool {
			if attack, ok := e.(*dnd5e.AttackEvent); ok {
				return attack.Attacker == r.owner
			}
			return false
		},
	)
	if err == nil {
		r.subscriptions = append(r.subscriptions, attackSub)
	}

	// Subscribe to damage events (for resistance)
	damageSub, err := r.bus.SubscribeWithFilter(
		dnd5e.EventRefDamageReceived,
		r.onDamageReceived,
		func(e events.Event) bool {
			if damage, ok := e.(*dnd5e.DamageReceivedEvent); ok {
				return damage.Target == r.owner
			}
			return false
		},
	)
	if err == nil {
		r.subscriptions = append(r.subscriptions, damageSub)
	}

	// Publish rage started event
	r.bus.Publish(&dnd5e.RageStartedEvent{
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

// endRage handles cleanup when rage ends
func (r *Rage) endRage() {
	r.active = false
	
	// Unsubscribe from all events
	for _, subID := range r.subscriptions {
		r.bus.Unsubscribe(subID)
	}
	r.subscriptions = nil
	
	// Publish rage ended event
	r.bus.Publish(&dnd5e.RageEndedEvent{
		Owner: r.owner,
	})
	
	r.owner = nil
}