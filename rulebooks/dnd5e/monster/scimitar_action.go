// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

const scimitarActionID = "scimitar"

// ScimitarConfig holds configuration for creating a scimitar action
type ScimitarConfig struct {
	ID          string `json:"id"`
	AttackBonus int    `json:"attack_bonus"` // e.g., +4
	DamageDice  string `json:"damage_dice"`  // e.g., "1d6+2"
	DamageBonus int    `json:"damage_bonus"` // flat bonus from STR/DEX
}

// ScimitarAction implements a melee weapon attack with a scimitar.
// Implements MonsterAction interface.
type ScimitarAction struct {
	id          string
	attackBonus int
	damageDice  string
	damageBonus int
}

// Ensure ScimitarAction implements MonsterAction
var _ MonsterAction = (*ScimitarAction)(nil)

// NewScimitarAction creates a scimitar action with the given config
func NewScimitarAction(config ScimitarConfig) *ScimitarAction {
	id := config.ID
	if id == "" {
		id = scimitarActionID
	}
	return &ScimitarAction{
		id:          id,
		attackBonus: config.AttackBonus,
		damageDice:  config.DamageDice,
		damageBonus: config.DamageBonus,
	}
}

// GetID implements core.Entity
func (s *ScimitarAction) GetID() string {
	return s.id
}

// GetType implements core.Entity
func (s *ScimitarAction) GetType() core.EntityType {
	return "monster-action"
}

// Cost returns the action economy cost (uses a standard action)
func (s *ScimitarAction) Cost() ActionCost {
	return CostAction
}

// ActionType returns the type of action for target selection
func (s *ScimitarAction) ActionType() ActionType {
	return TypeMeleeAttack
}

// Score returns how desirable this action is in the current situation.
// Higher when there's an adjacent enemy.
func (s *ScimitarAction) Score(_ *Monster, perception *PerceptionData) int {
	baseScore := 50

	// Bonus if target is adjacent (melee range)
	if perception.HasAdjacentEnemy() {
		baseScore += 20
	}

	return baseScore
}

// CanActivate checks if the action can be used
func (s *ScimitarAction) CanActivate(_ context.Context, _ core.Entity, input MonsterActionInput) error {
	// Need a target
	if input.Target == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "no target for scimitar attack")
	}

	// Target must be adjacent (5ft reach)
	closest := input.Perception.ClosestEnemy()
	if closest == nil || !closest.Adjacent {
		return rpgerr.New(rpgerr.CodeOutOfRange, "target not in melee range")
	}

	return nil
}

// Activate executes the scimitar attack
func (s *ScimitarAction) Activate(ctx context.Context, owner core.Entity, input MonsterActionInput) error {
	// Validate we can activate
	if err := s.CanActivate(ctx, owner, input); err != nil {
		return err
	}

	// Publish attack event - the combat system handles the actual resolution
	attackTopic := dnd5eEvents.AttackTopic.On(input.Bus)
	err := attackTopic.Publish(ctx, dnd5eEvents.AttackEvent{
		AttackerID: owner.GetID(),
		TargetID:   input.Target.GetID(),
		WeaponRef:  s.id,
		IsMelee:    true,
	})
	if err != nil {
		return rpgerr.Wrapf(err, "failed to publish attack event")
	}

	return nil
}
