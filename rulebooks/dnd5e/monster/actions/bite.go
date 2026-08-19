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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

const monsterActionEntityType core.EntityType = "monster-action"

// BiteConfig holds configuration for creating a bite action. The knockdown is
// optional and lives in SaveGate.
type BiteConfig struct {
	AttackBonus int             `json:"attack_bonus"` // e.g., +4
	Damage      []damage.Damage `json:"damage"`

	// SaveGate is what the bite's consequence can be contested with, if
	// anything: the wolf declares a STR save against DC 11 or be knocked prone
	// (ADR-0039). Nil means the bite just bites.
	SaveGate *saves.SaveGate `json:"save_gate,omitempty"`
}

// BiteAction is a melee bite attack that may carry a rider its target can
// contest.
//
// The rider is a [saves.SaveGate] and is optional: the wolf's bite declares a
// STR save against DC 11 or be knocked prone, while another creature's could
// name a different ability, a different DC, or half damage instead of nothing
// (ADR-0039). Nil means the bite just bites.
//
// What the gate is not is behaviour. This action declares what can be
// contested; imposing the consequence, and running the save that gates it, is
// resolution's job — which is what makes "can I resist this bite, and how?"
// answerable from the stat block before anything runs.
type BiteAction struct {
	attackBonus int
	damage      []damage.Damage
	saveGate    *saves.SaveGate
}

// Ensure BiteAction implements MonsterAction
var _ monster.MonsterAction = (*BiteAction)(nil)

// NewBiteAction creates a bite action with the given config.
func NewBiteAction(config BiteConfig) (*BiteAction, error) {
	if err := damage.Validate(config.Damage); err != nil {
		return nil, rpgerr.Wrap(err, "invalid bite action damage")
	}

	return &BiteAction{
		attackBonus: config.AttackBonus,
		damage:      copyDamage(config.Damage),
		saveGate:    config.SaveGate,
	}, nil
}

// SaveGate returns what this bite's consequence can be contested with, or nil
// if it carries none. Data, not behaviour: whoever imposes the consequence asks
// for the declaration and turns it into a save.
func (b *BiteAction) SaveGate() *saves.SaveGate {
	return b.saveGate
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

	// Extra bonus for knockdown potential — now read from the declaration
	// rather than assumed. A bite with no gate knocks nobody down, and used to
	// score as though it did.
	if b.saveGate != nil {
		baseScore += 10
	}

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

// ToData converts the action to its serializable form.
func (b *BiteAction) ToData() monster.ActionData {
	config := BiteConfig{
		AttackBonus: b.attackBonus,
		Damage:      copyDamage(b.damage),
		SaveGate:    b.saveGate,
	}
	configJSON, _ := json.Marshal(config)

	return monster.ActionData{
		Ref:    *refs.MonsterActions.Bite(),
		Config: configJSON,
	}
}
