// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import "github.com/KirkDiggler/rpg-toolkit/dice"

// Capabilities is every capability an encounter asks of its host. It is the one
// list: [SetupInput] and [LoadEncounterInput] embed it, and a capability the
// encounter gains is added here and to nothing else.
//
// Capabilities are never persisted. [EncounterData] carries none of them, so
// every [LoadEncounter] supplies its own.
type Capabilities struct {
	// Initiative rolls the order a bubble forms in when trigger detection
	// starts a fight (rpg-toolkit#964). REQUIRED — trigger detection runs from
	// first light, so a fight can start before the caller does anything, and
	// an encounter that cannot order one is a misconfiguration. Refused at the
	// door without it (ErrNoInitiative), never guarded where it is used.
	Initiative InitiativeRoller

	// Standing answers who is down and who participates. REQUIRED (nil is
	// ErrNoStanding), and typed [StandingWithParticipation] so the compiler,
	// not a runtime assertion, refuses a Standing-only value (rpg-toolkit#1958).
	// Play consults the richer assessment only; nothing defaults to everyone
	// active. The standing consult runs from first light too: a scene can open
	// with a body already on the floor, and an encounter that cannot ask would
	// start fights with corpses (rpg-toolkit#1033).
	Standing StandingWithParticipation

	// Sight reports how far each member can see, in cells (rpg-toolkit#1111).
	// REQUIRED, for the same reason Standing is: the consult runs at every
	// sight refresh including first light, so an encounter that cannot ask
	// cannot build a percept. Refused at the door (ErrNoSight). There is no
	// default — a number meaning "everyone sees this far" would be this module
	// inventing a rule 5e does not have, since sight is per-creature and
	// per-light-source.
	Sight Sight

	// Equipment reports what each member is holding (rpg-toolkit#1615). REQUIRED,
	// for the same reason Sight is: the consult runs at every sight refresh
	// including first light, so an encounter that cannot ask cannot snapshot a
	// complete percept. Refused at the door (ErrNoEquipment). There is no
	// default — empty hands for everybody would be this module inventing
	// testimony, and the difference between "no hands to observe" and "observed
	// empty" is a distinction only the rulebook can draw. Typed
	// [EquipmentWithConditions] so the compiler, not a runtime assertion,
	// requires it to answer Conditions too (rpg-toolkit#1958).
	Equipment EquipmentWithConditions

	// Sheets reports each member's speed, actions and targeting
	// (rpg-project#538). REQUIRED: a fight can form at first light and drive
	// an unplayed member, whose movement budget and reach are read from this
	// answer at that moment, and a walk on the world clock is paced from it.
	// Refused at the door (ErrNoSheets), never defaulted. This composition
	// stores none of these facts; see [Sheets].
	Sheets Sheets

	// Driver decides what a member with no player does when it is given time
	// — its turn in a fight, or a round of the world (rpg-toolkit#1162,
	// rpg-project#465). REQUIRED: a fight can form at first light with an
	// unplayed member first in initiative, so an encounter that cannot answer
	// this would stall before its caller does anything. Refused at the door
	// (ErrNoTurnDriver), with no default — see ADR-0043.
	Driver Driver

	// Roller is THE WORLD'S DIE: the shared dice every pick this composition
	// makes is rolled through — a creature's `time` table on its turn and on
	// a round of the world, and a faction's temperament mix at the door
	// (table.go, rpg-project#465).
	//
	// OPTIONAL, AND REFUSED LOUDLY AT THE ROLL when it is absent
	// ([ErrNoRoller]) — the shape Initiative already has. A scene with no
	// table and no mix rolls nothing, and requiring a die at every door would
	// make every caller declare one it never uses.
	//
	// IT CANNOT BE PER VERB, which is why it is here rather than on an input.
	// The round site raises the world clock from inside EndTurn, which takes
	// no die, and a creature's `time` pick happens there.
	Roller dice.Roller

	// CheckResolver resolves an authored find check when a member searches
	// (rpg-toolkit#1371). REQUIRED exactly when the field declares a
	// [ConcealmentInput], and refused there at the door (ErrNoCheckResolver):
	// a concealment exists to be searched for, and this module may not roll
	// the find itself (rpg-toolkit#1033). Unread, and legally nil, for a
	// field that hides nothing. At Load the answer depends on the persisted
	// field, so [LoadEncounter] refuses it in its body rather than in
	// [Capabilities.Validate].
	CheckResolver CheckResolver

	// Witness answers who currently perceives a concealment's door standing
	// open (rpg-toolkit#1371). REQUIRED under exactly the same rule as
	// CheckResolver, refused at the same door (ErrNoWitness): perception's
	// reach is the host's light-and-sight truth, never this module's guess.
	Witness Witness

	Actors
}

// Actors are the capabilities through which an encounter calls its host to act.
type Actors struct {
	// Striker resolves and records a member's attack when a [Driver]
	// returns an [Attack] intent (rpg-project#254). REQUIRED, for the same
	// reason Driver is and at the same door: a fight can form with an
	// unplayed member ready to swing the moment it forms, so an encounter
	// that cannot resolve that swing would stall or silently drop it.
	// Refused at the door (ErrNoStriker). There is no default — see
	// [Striker]'s own doc.
	Striker Striker

	// Mover announces a member's step before the encounter takes it, so
	// whatever reacts to movement can (rpg-project#316). REQUIRED, for the
	// same reason Striker is and at the same door: a fight can form with an
	// unplayed member ready to walk the moment it forms, and a step nothing
	// observed is a reaction that silently never happened. Refused at the
	// door (ErrNoMover). There is no default — see [Mover]'s own doc.
	Mover Mover

	// Announcer publishes the temporal boundaries a clock advance crossed —
	// a turn ending, a fight forming. REQUIRED, refused at the door
	// (ErrNoAnnouncer). There is no default, and the reason it cannot have
	// one is in [Announcer]'s own doc: a silent Announcer and a missing one
	// look identical, and one of them is the bug.
	Announcer Announcer
}

// Validate refuses a missing capability every encounter asks, in this order:
// ErrNoInitiative, ErrNoStanding, ErrNoSight, ErrNoEquipment, ErrNoSheets,
// ErrNoTurnDriver, ErrNoStriker, ErrNoMover, ErrNoAnnouncer. The sentinel is
// returned bare; the door that called wraps it with its own name.
//
// It does not check Roller, CheckResolver or Witness: Roller is optional and
// refused at the roll, and the concealment pair is required only where the
// field is known (Setup and Load).
func (c *Capabilities) Validate() error {
	switch {
	case c.Initiative == nil:
		return ErrNoInitiative
	case c.Standing == nil:
		return ErrNoStanding
	case c.Sight == nil:
		return ErrNoSight
	case c.Equipment == nil:
		return ErrNoEquipment
	case c.Sheets == nil:
		return ErrNoSheets
	case c.Driver == nil:
		return ErrNoTurnDriver
	case c.Striker == nil:
		return ErrNoStriker
	case c.Mover == nil:
		return ErrNoMover
	case c.Announcer == nil:
		return ErrNoAnnouncer
	}
	return nil
}

// validateConcealed refuses a nil CheckResolver (ErrNoCheckResolver), then a
// nil Witness (ErrNoWitness). Setup and Load call it exactly when the field
// declares a concealment.
func (c *Capabilities) validateConcealed() error {
	if c.CheckResolver == nil {
		return ErrNoCheckResolver
	}
	if c.Witness == nil {
		return ErrNoWitness
	}
	return nil
}

// RefusingActors returns actors that each refuse by name: [RefusingStriker],
// [RefusingMover] and [RefusingAnnouncer].
func RefusingActors() Actors {
	return Actors{
		Striker:   RefusingStriker{},
		Mover:     RefusingMover{},
		Announcer: RefusingAnnouncer{},
	}
}

// RefusingCapabilities returns the one value for a world compiled or previewed
// and never played (rpg-toolkit#1956, rpg-toolkit#1958).
//
// A host that builds a world before anybody is in it, or loads one only to
// inspect, preview its atlas or re-serialize it, has no sheets to answer from
// and no turn to drive. This is the one place that knows which capabilities an
// encounter requires, so a capability added to [Capabilities] is stood in here
// and the host's call does not change. A host that hand-wrote its stand-ins
// broke the moment [Conditions] was added: every launch failed with
// ErrNoConditions.
//
// The value serves [NewEncounter] and [LoadEncounter] alike. Roller is left
// nil, because a roll is not a thing a world nobody is in does; a host that
// plays the world supplies its own capabilities, and any it CAN answer it
// overwrites on the returned value. Each member answers only what is true of
// a world nobody is in, and refuses every other ask by name:
//
//   - Standing: nobody is down, as an empty list — literally true of a world
//     with nobody in it.
//   - Participation: an empty ask gets an empty assessment. A non-empty ask
//     REFUSES with ErrRefusingParticipation: participation has no "not
//     observed" answer, and saying a member is conscious and in contact would
//     be a confident answer nobody observed. [LoadEncounter] never asks it, so
//     a world WITH members loads, and the refusal fires only if the host goes
//     on to play it. Never asked in play: capabilities are not persisted and
//     every load supplies its own.
//   - Initiative: REFUSES with ErrRefusingInitiative. Initiative has no "not
//     observed" answer, and any order this could give — the order asked is
//     the forming members sorted by ID — would be a confident wrong answer
//     deciding a fight. With zero sight no fight can form here, so reaching
//     it means a host changed the setup and tried to play a compiled world.
//   - Sight: zero cells for every member; no sighting is written.
//   - Equipment and Conditions: nil for every member — nothing to observe,
//     never empty hands or an empty set ([UnobservedEquipment]).
//   - Sheets: an empty ask gets an empty answer; a non-empty ask REFUSES with
//     ErrRefusingSheets, Participation's shape and for its reason. A speed or
//     a reach has no "not observed" answer — zero is a real speed — and a
//     compiled world has no sheets behind its members. Asked only when a walk
//     is paced, a turn budgeted or a driver's view built, none of which
//     compiling a world does. [LoadEncounter] never asks it either.
//   - Driver: [RefusingDriver]. A world may hold members, and a silent pass
//     would turn a host that drove a compiled world into a board of idle
//     monsters where a refusal names the bug.
//   - Striker, Mover, Announcer: [RefusingStriker], [RefusingMover],
//     [RefusingAnnouncer] (see [RefusingActors]).
//   - CheckResolver: [RefusingCheckResolver]; a check is only rolled through
//     an explicit Search, which nothing compiling a world does.
//   - Witness: [NobodyPerceives]. It answers rather than refuses, because
//     first light asks it for an authored concealed door that stands open even
//     with no members (legal content, rpg-api#887). Both it and CheckResolver
//     are installed whether or not the data declares a concealment; the
//     encounter holds them only when it does.
func RefusingCapabilities() Capabilities {
	return Capabilities{
		Initiative:    refusingInitiative{},
		Standing:      nobodyDown{},
		Sight:         zeroSight{},
		Equipment:     UnobservedEquipment{},
		Sheets:        noSheets{},
		Driver:        RefusingDriver{},
		CheckResolver: RefusingCheckResolver{},
		Witness:       NobodyPerceives{},
		Actors:        RefusingActors(),
	}
}
