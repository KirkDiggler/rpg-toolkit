// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// applySightAttackModifiers folds the generic 5e sight rule into an attack:
// attacking an unseen target has disadvantage; an unseen attacker has advantage.
// Both directions are read from the strike's execution frame — the attacker→
// target and target→attacker pairs' Sees — which [strikeMachine.attackRollFrame]
// fills from the installed visibility, so the rule and every other attack-chain
// rule read sight from one place. No spell or effect name is consulted here.
//
// Errors: either direction unknown, wrapping [contributions.ErrRuleCannotAnswer]
// — at execution an undecided sight fails the strike rather than counting as
// seen (R13).
func applySightAttackModifiers(
	frame contributions.Frame, event *dnd5eEvents.AttackChainEvent, attackerID, targetID string, sourceRef *core.Ref,
) error {
	attackerSees, known := frame.Pair(attackerID, targetID).Sees.Get()
	if !known {
		return fmt.Errorf("sight from %q to %q: %w: unknown on the execution frame",
			attackerID, targetID, contributions.ErrRuleCannotAnswer)
	}
	targetSees, known := frame.Pair(targetID, attackerID).Sees.Get()
	if !known {
		return fmt.Errorf("sight from %q to %q: %w: unknown on the execution frame",
			targetID, attackerID, contributions.ErrRuleCannotAnswer)
	}
	if !attackerSees {
		event.DisadvantageSources = append(event.DisadvantageSources, dnd5eEvents.AttackModifierSource{
			SourceRef: cloneVisibilityRef(sourceRef), SourceID: attackerID,
			Reason: "attacker cannot see target",
		})
	}
	if !targetSees {
		event.AdvantageSources = append(event.AdvantageSources, dnd5eEvents.AttackModifierSource{
			SourceRef: cloneVisibilityRef(sourceRef), SourceID: targetID,
			Reason: "target cannot see attacker",
		})
	}
	return nil
}

func cloneVisibilityRef(ref *core.Ref) *core.Ref {
	if ref == nil {
		return nil
	}
	clone := *ref
	return &clone
}
