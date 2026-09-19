// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/core"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
)

// applySightAttackModifiers folds the generic 5e sight rule into an attack:
// attacking an unseen target has disadvantage; an unseen attacker has advantage.
// The encounter supplies the facts; no spell or effect name is consulted here.
// When no visibility model is installed, the helper leaves the event unchanged
// so non-spatial callers retain their established behaviour.
func applySightAttackModifiers(ctx context.Context, event *dnd5eEvents.AttackChainEvent, attackerID, targetID string, rangeFeet int, sourceRef *core.Ref) {
	if event == nil {
		return
	}
	sight, ok := gamectx.Visibility(ctx)
	if !ok {
		return
	}
	addSightAttackModifiers(sight, event, attackerID, targetID, rangeFeet, sourceRef)
}

func addSightAttackModifiers(sight gamectx.VisibilityProvider, event *dnd5eEvents.AttackChainEvent, attackerID, targetID string, rangeFeet int, sourceRef *core.Ref) {
	attackerSees, attackerKnown := sight.SeesWithin(attackerID, targetID, rangeFeet)
	targetSees, targetKnown := sight.SeesWithin(targetID, attackerID, rangeFeet)
	if attackerKnown && !attackerSees {
		event.DisadvantageSources = append(event.DisadvantageSources, dnd5eEvents.AttackModifierSource{
			SourceRef: cloneVisibilityRef(sourceRef), SourceID: attackerID,
			Reason: "attacker cannot see target",
		})
	}
	if targetKnown && !targetSees {
		event.AdvantageSources = append(event.AdvantageSources, dnd5eEvents.AttackModifierSource{
			SourceRef: cloneVisibilityRef(sourceRef), SourceID: targetID,
			Reason: "target cannot see attacker",
		})
	}
}

func cloneVisibilityRef(ref *core.Ref) *core.Ref {
	if ref == nil {
		return nil
	}
	clone := *ref
	return &clone
}
