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

// MeleeConfig holds configuration for creating a melee action
type MeleeConfig struct {
	Name        string      `json:"name"`         // e.g., "shortsword", "greataxe"
	AttackBonus int         `json:"attack_bonus"` // e.g., +4
	DamageDice  string      `json:"damage_dice"`  // e.g., "1d6+2"
	Reach       int         `json:"reach"`        // in hexes, typically 1 (5ft) or 2 (10ft reach)
	DamageType  damage.Type `json:"damage_type"`  // e.g., piercing, slashing
}

// MeleeAction implements a generic melee weapon attack.
// This generalizes ScimitarAction to work with any melee weapon.
type MeleeAction struct {
	name        string
	attackBonus int
	damageDice  string
	reach       int
	damageType  damage.Type
}

// Ensure MeleeAction implements MonsterAction
var _ monster.MonsterAction = (*MeleeAction)(nil)

// NewMeleeAction creates a melee action with the given config
func NewMeleeAction(config MeleeConfig) *MeleeAction {
	return &MeleeAction{
		name:        config.Name,
		attackBonus: config.AttackBonus,
		damageDice:  config.DamageDice,
		reach:       config.Reach,
		damageType:  config.DamageType,
	}
}

// GetID implements core.Entity
func (m *MeleeAction) GetID() string {
	return m.name
}

// GetType implements core.Entity
func (m *MeleeAction) GetType() core.EntityType {
	return monsterActionEntityType
}

// Cost returns the action economy cost (uses a standard action)
func (m *MeleeAction) Cost() monster.ActionCost {
	return monster.CostAction
}

// ActionType returns the type of action for target selection
func (m *MeleeAction) ActionType() monster.ActionType {
	return monster.TypeMeleeAttack
}

// Score returns how desirable this action is in the current situation.
// Higher when there's an adjacent enemy.
func (m *MeleeAction) Score(_ *monster.Monster, perception *monster.PerceptionData) int {
	baseScore := 50

	// Bonus if target is adjacent (melee range)
	if perception.HasAdjacentEnemy() {
		baseScore += 20
	}

	return baseScore
}

// CanActivate checks if the action can be used
func (m *MeleeAction) CanActivate(_ context.Context, _ core.Entity, input monster.MonsterActionInput) error {
	// Need a target
	if input.Target == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "no target for melee attack")
	}

	// Target must be within reach
	if input.Perception == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "no perception data")
	}

	// Check if target is within reach
	targetInReach := false
	for _, enemy := range input.Perception.Enemies {
		if enemy.Entity.GetID() == input.Target.GetID() {
			if enemy.Distance <= m.reach {
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

// Activate executes the melee attack
func (m *MeleeAction) Activate(ctx context.Context, owner core.Entity, input monster.MonsterActionInput) error {
	// Validate we can activate
	if err := m.CanActivate(ctx, owner, input); err != nil {
		return err
	}

	// Publish attack event - the combat system handles the actual resolution
	attackTopic := dnd5eEvents.AttackTopic.On(input.Bus)
	err := attackTopic.Publish(ctx, dnd5eEvents.AttackEvent{
		AttackerID: owner.GetID(),
		TargetID:   input.Target.GetID(),
		WeaponRef:  m.name,
		IsMelee:    true,
	})
	if err != nil {
		return rpgerr.Wrapf(err, "failed to publish attack event")
	}

	return nil
}

// ToData converts the action to its serializable form
func (m *MeleeAction) ToData() monster.ActionData {
	config := MeleeConfig{
		Name:        m.name,
		AttackBonus: m.attackBonus,
		DamageDice:  m.damageDice,
		Reach:       m.reach,
		DamageType:  m.damageType,
	}
	configJSON, _ := json.Marshal(config)

	return monster.ActionData{
		Ref:    *refs.MonsterActions.Melee(),
		Config: configJSON,
	}
}
