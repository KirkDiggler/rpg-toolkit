// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package actions provides reusable monster action implementations.
// These generic action types can be composed to create monster abilities.
package actions

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

const monsterActionEntityType core.EntityType = "monster-action"

// BiteConfig holds configuration for creating a bite action with knockdown
type BiteConfig struct {
	AttackBonus int         `json:"attack_bonus"` // e.g., +4
	DamageDice  string      `json:"damage_dice"`  // e.g., "2d4+2"
	KnockdownDC int         `json:"knockdown_dc"` // DC for STR save to avoid prone
	DamageType  damage.Type `json:"damage_type"`  // typically piercing
}

// BiteAction implements a bite attack with knockdown effect.
// On hit, target must make a STR save or be knocked prone.
type BiteAction struct {
	attackBonus int
	damageDice  string
	knockdownDC int
	damageType  damage.Type
}

// Ensure BiteAction implements MonsterAction
var _ monster.MonsterAction = (*BiteAction)(nil)

// NewBiteAction creates a bite action with the given config
func NewBiteAction(config BiteConfig) *BiteAction {
	return &BiteAction{
		attackBonus: config.AttackBonus,
		damageDice:  config.DamageDice,
		knockdownDC: config.KnockdownDC,
		damageType:  config.DamageType,
	}
}

// GetID implements core.Entity
func (b *BiteAction) GetID() string {
	return "bite"
}

// GetType implements core.Entity
func (b *BiteAction) GetType() core.EntityType {
	return monsterActionEntityType
}

// Cost returns the action economy cost (uses a standard action)
func (b *BiteAction) Cost() monster.ActionCost {
	return monster.CostAction
}

// ActionType returns the type of action for target selection
func (b *BiteAction) ActionType() monster.ActionType {
	return monster.TypeMeleeAttack
}

// Score returns how desirable this action is in the current situation.
// Higher when there's an adjacent enemy due to knockdown potential.
func (b *BiteAction) Score(_ *monster.Monster, perception *monster.PerceptionData) int {
	baseScore := 50

	// Bonus if target is adjacent (melee range)
	if perception.HasAdjacentEnemy() {
		baseScore += 20
	}

	// Extra bonus for knockdown potential
	baseScore += 10

	return baseScore
}

// CanActivate checks if the action can be used
func (b *BiteAction) CanActivate(_ context.Context, _ core.Entity, input monster.MonsterActionInput) error {
	// Need a target
	if input.Target == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "no target for bite attack")
	}

	// Target must be within reach (5 feet for bite)
	if input.Perception == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "no perception data")
	}

	// Check if target is within reach (bite is always adjacent/1 hex)
	targetInReach := false
	for _, enemy := range input.Perception.Enemies {
		if enemy.Entity.GetID() == input.Target.GetID() {
			if enemy.Adjacent {
				targetInReach = true
				break
			}
		}
	}

	if !targetInReach {
		return rpgerr.New(rpgerr.CodeOutOfRange, "target not in melee range")
	}

	return nil
}

// Activate executes the bite attack
func (b *BiteAction) Activate(ctx context.Context, owner core.Entity, input monster.MonsterActionInput) error {
	// Validate we can activate
	if err := b.CanActivate(ctx, owner, input); err != nil {
		return err
	}

	// Publish attack event - the combat system handles the actual resolution
	// The knockdown effect would be applied by the combat system after a successful hit
	// by checking for the bite weapon and triggering a STR save
	attackTopic := dnd5eEvents.AttackTopic.On(input.Bus)
	err := attackTopic.Publish(ctx, dnd5eEvents.AttackEvent{
		AttackerID: owner.GetID(),
		TargetID:   input.Target.GetID(),
		WeaponRef:  "bite",
		IsMelee:    true,
	})
	if err != nil {
		return rpgerr.Wrapf(err, "failed to publish attack event")
	}

	// NOTE: The knockdown logic (STR save DC vs being knocked prone) should be
	// implemented in the combat resolution system. The bite weapon would have
	// a special property that triggers the saving throw after a successful hit.
	// For now, we just publish the attack event.

	return nil
}

// ToData converts the action to its serializable form
func (b *BiteAction) ToData() monster.ActionData {
	config := BiteConfig{
		AttackBonus: b.attackBonus,
		DamageDice:  b.damageDice,
		KnockdownDC: b.knockdownDC,
		DamageType:  b.damageType,
	}
	configJSON, _ := json.Marshal(config)

	return monster.ActionData{
		Ref:    *refs.MonsterActions.Bite(),
		Config: configJSON,
	}
}
