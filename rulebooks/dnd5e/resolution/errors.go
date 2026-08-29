// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import "errors"

var (
	// ErrNilInput indicates a caller defect: Resolve was handed no input at all.
	ErrNilInput = errors.New("resolution: nil input")

	// ErrNoInitiative indicates an interaction given no way to order a fight
	// that starts while it runs. The composition requires one to load at all.
	ErrNoInitiative = errors.New("resolution: no initiative roller")

	// ErrNoRoller indicates an interaction given no dice.
	//
	// It used to default to real randomness, which made a missing capability
	// look like a working one and put untestable rolls into results that
	// seemed fine. Refused at the door instead.
	ErrNoRoller = errors.New("resolution: no roller")

	// ErrNoStanding indicates an interaction given no way to find out who is
	// standing. The composition requires one to load at all, the same way it
	// requires an initiative roller.
	//
	// Nothing in this package consults it. The world is loaded here and read
	// back out as data, and no verb that refreshes sight runs in between — so
	// this is a capability carried ACROSS rather than used, exactly like
	// Deciders. Carried rather than invented, because a package that answered
	// "nobody is down" on the caller's behalf would be deciding a rule it
	// cannot see (rpg-toolkit#1079).
	ErrNoStanding = errors.New("resolution: no standing capability")

	// ErrNoSight indicates an interaction given no way to find out how far
	// anybody can see. The composition requires one to load at all, the same
	// way it requires an initiative roller and a standing capability.
	//
	// Nothing in this package consults it, for ErrNoStanding's reason and by a
	// nameable mechanism: the composition asks how far somebody can see only
	// where it rebuilds percepts, and nothing on the load-act-save path this
	// package walks reaches that. Carried rather than invented — a package
	// that answered "sixty feet" on the caller's behalf would be deciding a
	// rule about light it cannot see (rpg-toolkit#1111, rpg-toolkit#1033).
	ErrNoSight = errors.New("resolution: no sight capability")

	// ErrNoTurnDriver indicates an interaction given no way to decide what an
	// unplayed member does when a fight's clock lands on their turn. The
	// composition requires one to load at all, the same way it requires an
	// initiative roller, a standing capability and a sight capability
	// (rpg-toolkit#1162).
	//
	// Nothing in this package consults it, for ErrNoStanding's reason: the
	// world is loaded here, one interaction runs, and it is read back out as
	// data — this package never calls EndTurn, form, Transfer or Exit, so the
	// question this capability answers is never put. Carried rather than
	// invented, because a package that answered "it passes" on the caller's
	// behalf would be deciding a rule it cannot see, the same way ErrNoStanding
	// and ErrNoSight already refuse to guess.
	ErrNoTurnDriver = errors.New("resolution: no turn driver capability")

	// ErrNoMachine indicates an interaction with nothing to resolve. Distinct
	// from a machine that finishes immediately, which is legal.
	ErrNoMachine = errors.New("resolution: no machine")

	// ErrBadParticipant indicates a participant that is not one sheet: neither
	// a character nor a monster, both at once, or sharing an ID with another.
	ErrBadParticipant = errors.New("resolution: bad participant")

	// ErrBadStep indicates a step this package did not construct, or one it has
	// no case for. Sealed vocabularies are only sealed if the driver says so.
	ErrBadStep = errors.New("resolution: unrecognized step")

	// ErrNoSaver indicates a saving throw naming a participant that was not
	// passed in. Silently rolling without the saver's modifier would be a
	// wrong answer wearing a right one's clothes.
	ErrNoSaver = errors.New("resolution: saver is not a participant")

	// ErrBadGate indicates a save gate that does not describe a save anyone
	// could make — no ability to roll, no DC source, a DC nobody can fail. The
	// declaration is content, so this is content being wrong rather than a
	// caller being wrong, and it says which.
	ErrBadGate = errors.New("resolution: invalid save gate")

	// ErrBadWorld indicates world data this package cannot make sense of — a
	// member placed in a room the field does not contain, or a position the
	// room refuses. The world is the caller's to hand over intact.
	ErrBadWorld = errors.New("resolution: world data is unusable")

	// ErrBadAction indicates an invalid or unsupported shared action definition.
	ErrBadAction = errors.New("resolution: invalid action definition")

	// ErrBadBoundary indicates a boundary set this package cannot announce:
	// empty, a crossing with no subject, or a kind this build has no topic
	// for. The last is the load-bearing one — the composition's boundary
	// kinds and this package's topic table are the same set by
	// construction, so a kind added on one side is REFUSED here rather than
	// silently published as nothing.
	ErrBadBoundary = errors.New("resolution: invalid boundary set")

	// ErrOutOfRange indicates a valid attack whose target lies beyond its delivery.
	ErrOutOfRange = errors.New("resolution: target is out of range")

	// ErrBadAttack indicates an attack cannot be resolved from its shared
	// definition, such as damage notation the dice parser refuses or a roller
	// that does not return a requested pair.
	ErrBadAttack = errors.New("resolution: unreadable attack")

	// ErrNoCombatant indicates a strike naming somebody who was not passed in.
	ErrNoCombatant = errors.New("resolution: combatant is not a participant")

	// ErrBadCost indicates a cost that does not describe a price anybody could
	// be charged — one that names no payer, or one whose profile is keyed to a
	// currency no ledger holds. Refused at the door before the world is loaded.
	//
	// Kept distinct from [ErrCannotPay] on purpose, and this is the whole reason
	// there are two: A MALFORMED PROFILE MUST NOT REACH A PLAYER AS "OUT OF
	// ACTIONS". One is content or wiring being wrong and wants a developer; the
	// other is an actor who spent what they had and wants a different verb. A
	// single sentinel would send the first one looking at the wrong sheet.
	ErrBadCost = errors.New("resolution: invalid cost")

	// ErrNoPayer indicates a cost naming somebody this cast cannot charge —
	// an ID that was not passed in, or a monster, whose economy belongs to
	// whoever runs its turn rather than to its sheet.
	ErrNoPayer = errors.New("resolution: payer has no ledger in this cast")

	// ErrCannotPay indicates an actor who cannot afford what they declared: no
	// economy to spend from, not enough of a currency, or a precondition the
	// ledger does not hold. The gate's own refusal is wrapped rather than
	// replaced, so which currency ran out survives the crossing.
	ErrCannotPay = errors.New("resolution: cost cannot be paid")

	// ErrBadMovement indicates a step nobody could announce: no mover, a step
	// that goes nowhere, or a missing capability. Wiring being wrong, never a
	// rule saying no — a member who may not walk is refused by the composition
	// long before a step reaches this package.
	//
	// A MOVEMENT WITH NO WAY TO RESOLVE A REACTION IS MALFORMED, not free.
	// Accepting one would publish triggers and drop them on the floor, which is
	// the exact defect this machine exists to undo: an opportunity attack that
	// fires into nothing looks identical to one that never fired.
	ErrBadMovement = errors.New("resolution: invalid movement")

	// ErrBadActivation indicates an activation nobody could run: no member, no
	// ability ref or an unusable one, a member who is not in the cast, a
	// monster (whose abilities are driven rather than declared), or a target
	// that was never passed in. Content or wiring being wrong.
	//
	// Kept distinct from [ErrActivationRefused] for [ErrBadCost]'s reason, and
	// it is the same failure wearing a different hat: A MALFORMED ACTIVATION
	// MUST NOT REACH A PLAYER AS "YOU ARE ALREADY RAGING". One wants a
	// developer; the other wants the player to pick a different verb.
	ErrBadActivation = errors.New("resolution: invalid activation")

	// ErrActivationRefused indicates an ability that could have run and said
	// no: the slot is spent, the charges are gone, the barbarian is already
	// raging, the fighter is at full hit points.
	//
	// It exists because the sheet does not raise one. Character.ActivateAbility
	// answers every refusal as (output{Success:false}, nil) — a SUCCESSFUL call
	// carrying a false — so without this the interaction would finish, report
	// Done, and save dirty sheets for something that never happened. The
	// ability's own words are wrapped rather than replaced, so "no rage charges
	// remaining" survives the crossing the way a shortfall's currency does.
	ErrActivationRefused = errors.New("resolution: ability refused to activate")

	// ErrRecurrenceUnsupported indicates a gate asking for a repeat save this
	// package cannot yet run. Refusing is the point: treating "save again at
	// the end of each of your turns" as a single save would produce a
	// paralysis nobody ever shakes off, and it would look like it worked.
	ErrRecurrenceUnsupported = errors.New("resolution: save gate recurrence not supported yet")
)
