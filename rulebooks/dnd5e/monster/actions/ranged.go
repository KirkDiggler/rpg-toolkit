// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

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

// RangedConfig holds configuration for creating a ranged action
type RangedConfig struct {
	Name        string          `json:"name"`         // e.g., "shortbow", "light crossbow"
	AttackBonus int             `json:"attack_bonus"` // e.g., +4
	Damage      []damage.Damage `json:"damage"`
	RangeNormal int             `json:"range_normal"` // in hexes, typically 16 (80ft / 5)
	RangeLong   int             `json:"range_long"`   // in hexes, typically 64 (320ft / 5)
}

// RangedAction implements a generic ranged weapon attack.
type RangedAction struct {
	name        string
	attackBonus int
	damage      []damage.Damage
	rangeNormal int
	rangeLong   int
}

// Ensure RangedAction implements MonsterAction
var _ monster.MonsterAction = (*RangedAction)(nil)

// NewRangedAction creates a ranged action with the given config
func NewRangedAction(config RangedConfig) (*RangedAction, error) {
	if err := damage.Validate(config.Damage); err != nil {
		return nil, rpgerr.Wrap(err, "invalid ranged action damage")
	}

	return &RangedAction{
		name:        config.Name,
		attackBonus: config.AttackBonus,
		damage:      copyDamage(config.Damage),
		rangeNormal: config.RangeNormal,
		rangeLong:   config.RangeLong,
	}, nil
}

// GetID implements core.Entity
func (r *RangedAction) GetID() string {
	return r.name
}

// GetType implements core.Entity
func (r *RangedAction) GetType() core.EntityType {
	return monsterActionEntityType
}

// Cost returns the action economy cost (uses a standard action)
func (r *RangedAction) Cost() monster.ActionCost {
	return monster.CostAction
}

// ActionType returns the type of action for target selection
func (r *RangedAction) ActionType() monster.ActionType {
	return monster.TypeRangedAttack
}

// Score returns how desirable this action is in the current situation.
// Higher when there's a distant enemy (no disadvantage from adjacent enemies).
func (r *RangedAction) Score(_ *monster.Monster, perception *monster.PerceptionData) int {
	baseScore := 50

	// Bonus if no adjacent enemy (avoids disadvantage)
	if !perception.HasAdjacentEnemy() {
		baseScore += 20
	}

	return baseScore
}

// CanActivate checks if the action can be used
func (r *RangedAction) CanActivate(_ context.Context, _ core.Entity, input monster.MonsterActionInput) error {
	// Need a target
	if input.Target == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "no target for ranged attack")
	}

	// Target must be within long range
	if input.Perception == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "no perception data")
	}

	// Check if target is within range
	targetInRange := false
	for _, enemy := range input.Perception.Enemies {
		if enemy.Entity.GetID() == input.Target.GetID() {
			if enemy.Distance <= r.rangeLong {
				targetInRange = true
				break
			}
		}
	}

	if !targetInRange {
		return rpgerr.New(rpgerr.CodeOutOfRange, "target out of range")
	}

	return nil
}

// Activate executes the ranged attack
func (r *RangedAction) Activate(ctx context.Context, owner core.Entity, input monster.MonsterActionInput) error {
	// Validate we can activate
	if err := r.CanActivate(ctx, owner, input); err != nil {
		return err
	}

	// Publish attack event - the combat system handles the actual resolution
	attackTopic := dnd5eEvents.AttackTopic.On(input.Bus)
	err := attackTopic.Publish(ctx, dnd5eEvents.AttackEvent{
		AttackerID: owner.GetID(),
		TargetID:   input.Target.GetID(),
		WeaponRef:  r.name,
		IsMelee:    false,
	})
	if err != nil {
		return rpgerr.Wrapf(err, "failed to publish attack event")
	}

	return nil
}

// ToData converts the action to its serializable form
func (r *RangedAction) ToData() monster.ActionData {
	config := RangedConfig{
		Name:        r.name,
		AttackBonus: r.attackBonus,
		Damage:      copyDamage(r.damage),
		RangeNormal: r.rangeNormal,
		RangeLong:   r.rangeLong,
	}
	configJSON, _ := json.Marshal(config)

	return monster.ActionData{
		Ref:    *refs.MonsterActions.Ranged(),
		Config: configJSON,
	}
}
