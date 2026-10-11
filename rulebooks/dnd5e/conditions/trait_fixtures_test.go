// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// foldTraitIncoming runs the target step's incoming fold over dealt, an attack
// by attackerID on targetID, and returns the folded event with combat's
// settlement of it. The trait tests read both.
func foldTraitIncoming(
	ctx context.Context, bus events.EventBus, attackerID, targetID string, dealt []dnd5eEvents.DamageComponent,
) (*dnd5eEvents.IncomingDamageEvent, *combat.SettleDamageOutput, error) {
	got, err := foldIncoming(ctx, bus, testAttackFrame(attackerID, targetID), dealt)
	if err != nil {
		return nil, nil, err
	}
	return got.Folded, got.Settlement, nil
}
