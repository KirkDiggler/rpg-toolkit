// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions_test

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

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

// swungDamage frames a damage event from the swing, unless the test already
// set a frame.
func swungDamage(event *dnd5eEvents.DamageChainEvent, sw swing) *dnd5eEvents.DamageChainEvent {
	if event.Frame.Actor != "" {
		return event
	}
	pool := false
	for _, component := range event.Components {
		if component.Source == dnd5eEvents.DamageSourceWeapon &&
			component.HasProperty(damage.AddsAttackAbilityModifier) {
			pool = true
		}
	}
	event.Frame = contributions.Frame{
		Actor:    event.AttackerID,
		Target:   contributions.Known(event.TargetID),
		Action:   externalWeaponFacts(sw.WeaponRef, sw.TwoHanded, sw.OffHandWeaponRef != nil),
		Complete: true,
	}
	event.Frame.Action.Ability = contributions.Known(sw.AbilityUsed)
	event.Frame.Action.AbilityModifier = contributions.Known(sw.AbilityModifier)
	event.Frame.Action.Melee = contributions.Known(sw.IsMelee)
	event.Frame.Action.WeaponPool = contributions.Known(pool)
	event.Frame.Action.Advantage = contributions.Known(sw.HasAdvantage)
	event.Frame.Action.OffHandAttack = contributions.Known(sw.IsOffHandAttack)
	return event
}

// swungAttack frames an attack event from the swing, with advantage unknown
// because the chain has not folded, unless the test already set a frame.
func swungAttack(event dnd5eEvents.AttackChainEvent, sw swing) dnd5eEvents.AttackChainEvent {
	if event.Frame.Actor != "" {
		return event
	}
	event.Frame = contributions.Frame{
		Actor:    event.AttackerID,
		Target:   contributions.Known(event.TargetID),
		Action:   externalWeaponFacts(sw.WeaponRef, false, false),
		Complete: true,
	}
	event.Frame.Action.Melee = contributions.Known(sw.IsMelee)
	event.Frame.Action.WeaponPool = contributions.Known(sw.WeaponRef != nil)
	event.Frame.Action.Opportunity = contributions.Known(false)
	return event
}

// framedDamage frames a damage event that names no swing facts, unless the
// test already set a frame. A test fixture: production frames come from
// resolution alone.
func framedDamage(event *dnd5eEvents.DamageChainEvent) *dnd5eEvents.DamageChainEvent {
	return swungDamage(event, swing{})
}

// framedAttack frames an attack event that names no swing facts, unless the
// test already set a frame. A test fixture: production frames come from
// resolution alone.
func framedAttack(event dnd5eEvents.AttackChainEvent) dnd5eEvents.AttackChainEvent {
	return swungAttack(event, swing{})
}

// externalWeaponFacts reads the weapon facts from a weapon ref through the
// catalogue: no ref is an attack with no weapon.
func externalWeaponFacts(weaponRef *core.Ref, twoHanded, otherWeapon bool) contributions.ActionFacts {
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

// framedAgainst sets an attack event's attack-roll frame from its own fields
// and adds what resolution measures between the two: the attacker→target
// distance and sight, and the conditions the target holds. A test fixture.
func framedAgainst(
	event dnd5eEvents.AttackChainEvent, distance float64, sees bool, held ...contributions.HeldCondition,
) dnd5eEvents.AttackChainEvent {
	event = framedAttack(event)
	event.Frame.Pairs = []contributions.PairFacts{{
		From: event.AttackerID, To: event.TargetID,
		DistanceCells: contributions.Known(distance), Sees: contributions.Known(sees),
	}}
	event.Frame.Held = []contributions.MemberHeld{{Member: event.TargetID, Conditions: held}}
	return event
}
