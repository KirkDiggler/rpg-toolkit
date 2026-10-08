// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monstertraits

import (
	"context"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// intPtr returns a pointer to v, so a present zero modifier stays present.
func intPtr(v int) *int { return &v }

// testDiceTrace builds a self-consistent dice trace for one pool of faces.
func testDiceTrace(dieSize int, faces ...int) *dnd5eEvents.DiceTrace {
	subtotal := 0
	for _, face := range faces {
		subtotal += face
	}
	return &dnd5eEvents.DiceTrace{
		Notation:      dice.SimplePool(len(faces), dieSize, 0).Notation(),
		DieSize:       dieSize,
		OriginalRolls: faces,
		FinalRolls:    slices.Clone(faces),
		Subtotal:      subtotal,
	}
}

// foldIncoming runs the target step's incoming fold over dealt, an attack by
// attackerID on targetID, refuses a fold that altered what it was handed, and
// returns the folded event with combat's settlement of it.
func foldIncoming(
	ctx context.Context, bus events.EventBus, attackerID, targetID string, dealt []dnd5eEvents.DamageComponent,
) (*dnd5eEvents.IncomingDamageEvent, *combat.SettleDamageOutput, error) {
	sent, err := dnd5eEvents.NewIncomingDamageEvent(dnd5eEvents.IncomingDamageInput{
		TargetID: targetID, SourceID: attackerID, Dealt: dealt,
		Frame: contributions.Frame{
			Actor:  attackerID,
			Target: contributions.Known(targetID),
			Action: contributions.ActionFacts{Roll: contributions.Known(contributions.RollKindAttack)},
		},
	})
	if err != nil {
		return nil, nil, err
	}
	chain := events.NewStagedChain[*dnd5eEvents.IncomingDamageEvent](combat.ModifierStages)
	modified, err := dnd5eEvents.IncomingDamageChain.On(bus).PublishWithChain(ctx, sent.Clone(), chain)
	if err != nil {
		return nil, nil, err
	}
	folded, err := modified.Execute(ctx, sent.Clone())
	if err != nil {
		return nil, nil, err
	}
	if err := folded.CheckUnaltered(sent); err != nil {
		return nil, nil, err
	}
	settled, err := combat.SettleDamage(&combat.SettleDamageInput{
		Dealt: sent.Dealt(), Reductions: folded.Reductions, Multipliers: folded.Multipliers,
	})
	if err != nil {
		return nil, nil, err
	}
	return folded, settled, nil
}
