// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// intPtr returns a pointer to v, so a present zero modifier stays present.
func intPtr(v int) *int { return &v }

// testDiceTrace builds a self-consistent dice trace for one pool of faces:
// the notation, die size, original/final rolls, and authoritative subtotal all
// describe the same physical pool. Final rolls are a clone, so originals stay
// immutable no matter what a condition under test writes.
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

// testAttackFrame is a complete, valid execution frame for attacker's attack
// roll on target with no pairs: the minimum a frame-reading handler accepts.
func testAttackFrame(attacker, target string) contributions.Frame {
	return contributions.Frame{
		Actor:    attacker,
		Target:   contributions.Known(target),
		Action:   contributions.ActionFacts{Roll: contributions.Known(contributions.RollKindAttack)},
		Complete: true,
	}
}

// withEventFrame sets the event's frame from its own fields, the facts
// resolution settles from the assembled attack: ability, melee, the marked
// weapon pool and effective advantage, all known, plus any pairs supplied.
func withEventFrame(event *dnd5eEvents.DamageChainEvent, pairs ...contributions.PairFacts) *dnd5eEvents.DamageChainEvent {
	frame := testAttackFrame(event.AttackerID, event.TargetID)
	frame.Action.Ability = contributions.Known(event.AbilityUsed)
	frame.Action.Melee = contributions.Known(event.IsMelee)
	frame.Action.WeaponPool = contributions.Known(primaryWeaponComponent(event) != nil)
	frame.Action.Advantage = contributions.Known(event.HasAdvantage)
	frame.Pairs = pairs
	event.Frame = frame
	return event
}
