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
	Name        string          `json:"name"`         // e.g., "shortsword", "greataxe"
	AttackBonus int             `json:"attack_bonus"` // e.g., +4
	Damage      []damage.Damage `json:"damage"`
	// Reach is in FEET (Kirk, rpg-project#254 review) — typically 5 (one
	// cell, the standard melee reach) or 10 (the Reach property, two
	// cells). This comment previously said "in hexes", which was the
	// actual bug: the data (skeleton's shortsword at 5, for instance) was
	// always authored correctly, in feet.
	Reach int `json:"reach"`
}

// MeleeAction implements a generic melee weapon attack.
type MeleeAction struct {
	name        string
	attackBonus int
	damage      []damage.Damage
	reach       int
}

// Ensure MeleeAction implements MonsterAction
var _ monster.MonsterAction = (*MeleeAction)(nil)

// NewMeleeAction creates a melee action with the given config
func NewMeleeAction(config MeleeConfig) (*MeleeAction, error) {
	if err := damage.Validate(config.Damage); err != nil {
		return nil, rpgerr.Wrap(err, "invalid melee action damage")
	}

	return &MeleeAction{
		name:        config.Name,
		attackBonus: config.AttackBonus,
		damage:      copyDamage(config.Damage),
		reach:       config.Reach,
	}, nil
}

func copyDamage(pools []damage.Damage) []damage.Damage {
	copy := make([]damage.Damage, len(pools))
	for i, pool := range pools {
		copy[i] = pool
		copy[i].Properties = append([]damage.Property(nil), pool.Properties...)
	}
	return copy
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
		Damage:      copyDamage(m.damage),
		Reach:       m.reach,
	}
	configJSON, _ := json.Marshal(config)

	return monster.ActionData{
		Ref:    *refs.MonsterActions.Melee(),
		Config: configJSON,
	}
}
