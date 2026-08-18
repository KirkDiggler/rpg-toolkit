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

	// ErrBadAttack indicates an action this build cannot read as an attack.
	// Refused by name rather than treated as a generic swing, because guessing
	// an attack bonus is how a stat block starts lying again.
	ErrBadAttack = errors.New("resolution: unreadable attack")

	// ErrNoCombatant indicates a strike naming somebody who was not passed in.
	ErrNoCombatant = errors.New("resolution: combatant is not a participant")

	// ErrRecurrenceUnsupported indicates a gate asking for a repeat save this
	// package cannot yet run. Refusing is the point: treating "save again at
	// the end of each of your turns" as a single save would produce a
	// paralysis nobody ever shakes off, and it would look like it worked.
	ErrRecurrenceUnsupported = errors.New("resolution: save gate recurrence not supported yet")
)
