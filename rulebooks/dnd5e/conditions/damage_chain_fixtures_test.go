// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
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

// swing is what a test says about one swing: the facts resolution settles
// from the assembled attack and puts on the frame — weapon, melee, ability and
// its modifier, grip, off-hand and effective advantage. The events carry none
// of these; only the frame does. A test fixture.
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
	frame := testAttackFrame(event.AttackerID, event.TargetID)
	frame.Action = fixtureWeaponFacts(sw.WeaponRef, sw.TwoHanded, sw.OffHandWeaponRef != nil)
	frame.Action.Ability = contributions.Known(sw.AbilityUsed)
	frame.Action.AbilityModifier = contributions.Known(sw.AbilityModifier)
	frame.Action.Melee = contributions.Known(sw.IsMelee)
	frame.Action.WeaponPool = contributions.Known(primaryWeaponComponent(event) != nil)
	frame.Action.Advantage = contributions.Known(sw.HasAdvantage)
	frame.Action.OffHandAttack = contributions.Known(sw.IsOffHandAttack)
	event.Frame = frame
	return event
}

// swungAttack frames an attack event from the swing — opportunity known false
// and advantage unknown because the chain has not folded — unless the test
// already set a frame.
func swungAttack(event dnd5eEvents.AttackChainEvent, sw swing) dnd5eEvents.AttackChainEvent {
	if event.Frame.Actor != "" {
		return event
	}
	frame := contributions.Frame{Actor: event.AttackerID, Target: contributions.Unknown[string](), Complete: true}
	if event.TargetID != "" {
		frame.Target = contributions.Known(event.TargetID)
	}
	frame.Action = fixtureWeaponFacts(sw.WeaponRef, false, false)
	frame.Action.Melee = contributions.Known(sw.IsMelee)
	frame.Action.WeaponPool = contributions.Known(sw.WeaponRef != nil)
	frame.Action.Opportunity = contributions.Known(false)
	event.Frame = frame
	return event
}

// withEventFrame frames the event as a swing with no weapon facts unless it
// is already framed, and sets the pairs supplied.
func withEventFrame(event *dnd5eEvents.DamageChainEvent, pairs ...contributions.PairFacts) *dnd5eEvents.DamageChainEvent {
	framedDamage(event)
	event.Frame.Pairs = pairs
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

// fixtureWeaponFacts reads the weapon facts from a weapon ref through the
// catalogue: no ref is an attack with no weapon.
func fixtureWeaponFacts(weaponRef *core.Ref, twoHanded, otherWeapon bool) contributions.ActionFacts {
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

// heldOf is the held condition a loaded condition stands for on member.
func heldOf(member string, condition dnd5eEvents.ConditionBehavior) contributions.HeldCondition {
	return heldAddress(member, condition)
}
