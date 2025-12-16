// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// MultiattackConfig holds configuration for creating a multiattack action
type MultiattackConfig struct {
	Attacks []string `json:"attacks"` // Names of actions to chain, e.g., ["bite", "claw", "claw"]
}

// MultiattackAction executes multiple attacks in sequence.
// This is typically used for boss monsters.
type MultiattackAction struct {
	attacks []string
}

// Ensure MultiattackAction implements MonsterAction
var _ monster.MonsterAction = (*MultiattackAction)(nil)

// NewMultiattackAction creates a multiattack action with the given config
func NewMultiattackAction(config MultiattackConfig) *MultiattackAction {
	return &MultiattackAction{
		attacks: config.Attacks,
	}
}

// GetID implements core.Entity
func (m *MultiattackAction) GetID() string {
	return "multiattack"
}

// GetType implements core.Entity
func (m *MultiattackAction) GetType() core.EntityType {
	return monsterActionEntityType
}

// Cost returns the action economy cost (uses a standard action for all attacks)
func (m *MultiattackAction) Cost() monster.ActionCost {
	return monster.CostAction
}

// ActionType returns the type of action for target selection
func (m *MultiattackAction) ActionType() monster.ActionType {
	return monster.TypeMeleeAttack // Most multiattacks are melee-based
}

// Score returns how desirable this action is in the current situation.
// Multiattack is always highly desirable for boss monsters.
func (m *MultiattackAction) Score(_ *monster.Monster, perception *monster.PerceptionData) int {
	baseScore := 80 // High score for multiattack

	// Extra bonus if adjacent to enemy (most multiattacks are melee)
	if perception.HasAdjacentEnemy() {
		baseScore += 10
	}

	return baseScore
}

// CanActivate checks if the action can be used
func (m *MultiattackAction) CanActivate(_ context.Context, owner core.Entity, input monster.MonsterActionInput) error {
	// Owner must be a Monster so we can access its actions
	monsterOwner, ok := owner.(*monster.Monster)
	if !ok {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "owner must be a monster")
	}

	// Need a target for the attacks
	if input.Target == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "no target for multiattack")
	}

	// Verify all sub-actions exist
	monsterActions := monsterOwner.Actions()
	for _, attackName := range m.attacks {
		found := false
		for _, action := range monsterActions {
			if action.GetID() == attackName {
				found = true
				break
			}
		}
		if !found {
			return rpgerr.New(rpgerr.CodeNotFound, "sub-action not found: "+attackName)
		}
	}

	return nil
}

// Activate executes the multiattack
func (m *MultiattackAction) Activate(ctx context.Context, owner core.Entity, input monster.MonsterActionInput) error {
	// Validate we can activate
	if err := m.CanActivate(ctx, owner, input); err != nil {
		return err
	}

	// Owner must be a Monster
	monsterOwner, ok := owner.(*monster.Monster)
	if !ok {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "owner must be a monster")
	}

	// Execute each attack in sequence
	monsterActions := monsterOwner.Actions()
	for _, attackName := range m.attacks {
		// Find the action
		var subAction monster.MonsterAction
		for _, action := range monsterActions {
			if action.GetID() == attackName {
				subAction = action
				break
			}
		}

		if subAction == nil {
			// Skip missing actions (shouldn't happen after CanActivate)
			continue
		}

		// Execute the sub-action
		// Note: We don't check CanActivate for each sub-action since multiattack
		// is a single action that chains attacks. If one fails, we continue with the rest.
		_ = subAction.Activate(ctx, owner, input)
	}

	return nil
}

// ToData converts the action to its serializable form
func (m *MultiattackAction) ToData() monster.ActionData {
	config := MultiattackConfig{
		Attacks: m.attacks,
	}
	configJSON, _ := json.Marshal(config)

	return monster.ActionData{
		Ref:    *refs.MonsterActions.Multiattack(),
		Config: configJSON,
	}
}
