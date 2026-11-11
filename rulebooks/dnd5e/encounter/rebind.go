// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import "fmt"

// rebind.go is THE RESIDENT COPY'S REBIND (rpg-project#879 S6, R6,
// rpg-project#927): a host that keeps a loaded encounter in memory between
// verbs, as a cache of its stored checkpoint, hands the next verb's
// capabilities to the kept instance instead of loading it again.
//
// WHAT A LOAD BINDS, AND NOTHING ELSE. [LoadEncounter] copies every
// [Capabilities] member onto the encounter; Rebind replaces exactly those, by
// the same rules: [Capabilities.Validate] refuses a missing one, the
// concealment pair is required when the field declares a concealment, and the
// [Witness] is held only then. Everything else the instance holds is the run's
// state, which the kept copy already has, and which a load would have rebuilt
// from the checkpoint the copy wrote. [LoadEncounterInput.IDs] and
// [LoadEncounterInput.Sites] are not capabilities: the id source is bound into
// the record at load and the site is shared and read-only, so a kept copy keeps
// both.
//
// WHAT A CAPABILITY MAY HOLD ACROSS VERBS: nothing of the verb it was built
// for. A host builds its capabilities per verb (its context, its dice, its
// clock, its listeners, the session data it read) and rebinds them before the
// verb runs, so a capability built for one verb is never asked during another.
// The encounter keeps no reference to a capability anywhere but its own fields
// (every ask goes through them), so after Rebind returns the old ones are
// unreachable from the encounter.
//
// BETWEEN VERBS ONLY. Rebind refuses ([ErrVerbInFlight]) while an act is open
// or a driven turn or a world round is running: a capability swapped under a
// verb would answer half of it.

// RebindInput is the capabilities the next verb runs with.
type RebindInput struct {
	// Capabilities is every capability the encounter asks of its host,
	// validated exactly as [LoadEncounter] validates them.
	Capabilities
}

// Rebind binds a kept encounter to the next verb's capabilities, in place of
// a reload: see rebind.go.
//
// Errors (the encounter is unchanged on each): [ErrNilInput]; a missing
// capability by its own sentinel, as [Capabilities.Validate] returns it;
// [ErrNoCheckResolver] or [ErrNoWitness] when the field declares a
// concealment; [ErrVerbInFlight] while a verb runs.
func (e *Encounter) Rebind(in *RebindInput) error {
	if in == nil {
		return fmt.Errorf("rebind: %w", ErrNilInput)
	}
	if len(e.acts) > 0 || e.driving || e.worldThinking {
		return fmt.Errorf("rebind: %w", ErrVerbInFlight)
	}
	if err := in.Validate(); err != nil {
		return fmt.Errorf("rebind: %w", err)
	}
	concealed := e.field != nil && len(e.field.concealments) > 0
	if concealed {
		if err := in.validateConcealed(); err != nil {
			return fmt.Errorf("rebind: %w", err)
		}
	}

	// The same list LoadEncounter's struct literal binds, in its order.
	e.initiative = in.Initiative
	e.participation = in.Participation
	e.sight = in.Sight
	e.equipment = in.Equipment
	e.conditions = in.Conditions
	e.sheets = in.Sheets
	e.driver = in.Driver
	e.roller = in.Roller
	e.striker = in.Striker
	e.disengager = in.Disengager
	e.mover = in.Mover
	e.announcer = in.Announcer
	e.witness = nil
	if concealed {
		e.witness = in.Witness
	}
	e.checkResolver = in.CheckResolver
	return nil
}
