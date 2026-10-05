// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package integration

import (
	"context"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
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

// framed sets the damage event's execution frame the way resolution builds it
// from authoritative state: the action facts from the swing, and a pair from
// the target to every other entity the room places, measured on the room's
// grid and stanced from the cast. Without a room the frame carries no pairs.
func framed(ctx context.Context, event *dnd5eEvents.DamageChainEvent) *dnd5eEvents.DamageChainEvent {
	weaponPool := false
	for _, component := range event.Components {
		if component.Source == dnd5eEvents.DamageSourceWeapon &&
			component.HasProperty(damage.AddsAttackAbilityModifier) {
			weaponPool = true
		}
	}
	frame := contributions.Frame{
		Actor:  event.AttackerID,
		Target: contributions.Known(event.TargetID),
		Action: contributions.ActionFacts{
			Roll:       contributions.Known(contributions.RollKindAttack),
			Ability:    contributions.Known(event.AbilityUsed),
			Melee:      contributions.Known(event.IsMelee),
			WeaponPool: contributions.Known(weaponPool),
			Advantage:  contributions.Known(event.HasAdvantage),
		},
		Complete: true,
	}
	room, hasRoom := gamectx.Room(ctx)
	cast, hasCast := gamectx.CastOf(ctx)
	if hasRoom {
		targetPos, placed := room.GetEntityPosition(event.TargetID)
		for id := range room.GetAllEntities() {
			pos, ok := room.GetEntityPosition(id)
			if !placed || !ok || id == event.TargetID {
				continue
			}
			pair := contributions.PairFacts{
				From: event.TargetID, To: id,
				DistanceCells: contributions.Known(room.GetGrid().Distance(targetPos, pos)),
			}
			if hasCast {
				// Two placed members with no stance have no side, never neutral.
				pair.Stance = contributions.Known(contributions.StanceNone)
				if stance, ok := cast.StanceBetween(event.TargetID, id); ok {
					pair.Stance = contributions.Known(stance)
				}
			}
			frame.Pairs = append(frame.Pairs, pair)
		}
	}
	event.Frame = frame
	return event
}
