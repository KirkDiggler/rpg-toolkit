// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package integration

import (
	"context"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
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

// swing is what a test says about one swing: the facts resolution settles
// from the assembled attack and puts on the frame. The events carry none of
// these; only the frame does. A test fixture.
type swing struct {
	WeaponRef        *core.Ref
	IsMelee          bool
	AbilityUsed      abilities.Ability
	AbilityModifier  int
	IsOffHandAttack  bool
	TwoHanded        bool
	OffHandWeaponRef *core.Ref
	HasAdvantage     bool
}

// swungDamage records the swing's action facts on the damage event's frame;
// framed adds the pairs when the test folds it.
func swungDamage(event *dnd5eEvents.DamageChainEvent, sw swing) *dnd5eEvents.DamageChainEvent {
	weaponPool := false
	for _, component := range event.Components {
		if component.Source == dnd5eEvents.DamageSourceWeapon &&
			component.HasProperty(damage.AddsAttackAbilityModifier) {
			weaponPool = true
		}
	}
	frame := contributions.Frame{
		Actor:    event.AttackerID,
		Target:   contributions.Known(event.TargetID),
		Action:   weaponFacts(sw.WeaponRef, sw.TwoHanded, sw.OffHandWeaponRef != nil),
		Complete: true,
	}
	frame.Action.Ability = contributions.Known(sw.AbilityUsed)
	frame.Action.AbilityModifier = contributions.Known(sw.AbilityModifier)
	frame.Action.Melee = contributions.Known(sw.IsMelee)
	frame.Action.WeaponPool = contributions.Known(weaponPool)
	frame.Action.Advantage = contributions.Known(sw.HasAdvantage)
	frame.Action.OffHandAttack = contributions.Known(sw.IsOffHandAttack)
	event.Frame = frame
	return event
}

// framed completes the damage event's execution frame the way resolution
// builds it from authoritative state: the swing's action facts (a swing with
// none when swungDamage never set them), and a pair from the target to every
// other entity the room places, measured on the room's grid and stanced from
// the cast. Without a room the frame carries no pairs.
func framed(ctx context.Context, event *dnd5eEvents.DamageChainEvent) *dnd5eEvents.DamageChainEvent {
	if event.Frame.Actor == "" {
		swungDamage(event, swing{})
	}
	frame := event.Frame
	frame.Pairs = nil
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
				// No stance between two MEMBERS of the cast is no side, never
				// neutral; a placed entity the cast does not hold stays unknown.
				members := cast.Members()
				if stance, ok := cast.StanceBetween(event.TargetID, id); ok {
					pair.Stance = contributions.Known(stance)
				} else if slices.Contains(members, event.TargetID) && slices.Contains(members, id) {
					pair.Stance = contributions.Known(contributions.StanceNone)
				}
			}
			frame.Pairs = append(frame.Pairs, pair)
		}
	}
	event.Frame = frame
	return event
}

// swungAttack frames an attack event from the swing, with advantage unknown
// because the chain has not folded. A test fixture: production frames come
// from resolution alone.
func swungAttack(event dnd5eEvents.AttackChainEvent, sw swing) dnd5eEvents.AttackChainEvent {
	event.Frame = contributions.Frame{
		Actor:    event.AttackerID,
		Target:   contributions.Known(event.TargetID),
		Action:   weaponFacts(sw.WeaponRef, false, false),
		Complete: true,
	}
	event.Frame.Action.Melee = contributions.Known(sw.IsMelee)
	event.Frame.Action.WeaponPool = contributions.Known(sw.WeaponRef != nil)
	event.Frame.Action.Opportunity = contributions.Known(false)
	return event
}

// framedAttack keeps the frame swungAttack set, or frames a swing with no
// weapon facts.
func framedAttack(event dnd5eEvents.AttackChainEvent) dnd5eEvents.AttackChainEvent {
	if event.Frame.Actor != "" {
		return event
	}
	return swungAttack(event, swing{})
}

// weaponFacts reads the weapon facts from a weapon ref through the catalogue:
// no ref is an attack with no weapon.
func weaponFacts(weaponRef *core.Ref, twoHanded, otherWeapon bool) contributions.ActionFacts {
	facts := contributions.ActionFacts{
		Roll:            contributions.Known(contributions.RollKindAttack),
		Ability:         contributions.Known(abilities.Ability("")),
		AbilityModifier: contributions.Known(0),
		Weapon:          contributions.Known(""),
		WeaponSlot:      contributions.Known(""),
		Finesse:         contributions.Known(false),
		RangedWeapon:    contributions.Known(false),
		TwoHanded:       contributions.Known(twoHanded),
		OffHandWeapon:   contributions.Known(otherWeapon),
		OffHandAttack:   contributions.Known(false),
		Opportunity:     contributions.Known(false),
	}
	if weaponRef == nil {
		return facts
	}
	facts.Weapon = contributions.Known(weaponRef.String())
	weapon, err := weapons.GetByID(weapons.WeaponID(weaponRef.ID))
	if err != nil {
		// Like resolution: a weapon the catalogue does not hold is unread.
		facts.Finesse = contributions.Unknown[bool]()
		facts.RangedWeapon = contributions.Unknown[bool]()
		return facts
	}
	facts.Finesse = contributions.Known(weapon.HasProperty(weapons.PropertyFinesse))
	facts.RangedWeapon = contributions.Known(weapon.IsRanged())
	return facts
}
