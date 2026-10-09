// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// This file is the only one in the package that composes
// [encounter.Capabilities] or [resolution.Input]; land_only_test.go holds it
// to that.
//
// BUILT AT EACH USE, NEVER CACHED ON THE SCOPE. Every capability reads the
// scope's standing at the moment of the call, and adopting a world replaces
// the roster those seams answer from ([Manager.adopt]). A value built once and
// kept would answer for the roster the verb opened with.

// writeCapabilities binds every capability to scope, read from scope.standing
// at the moment of the call.
//
// Every write verb's world is loaded with it ([Manager.openScope],
// [Manager.adopt]), so there is one answer to which sheets, which brain and
// which die a world on this call reads:
//
//   - Sight and Sheets are one [sheetSeam] built beside the standing, so a
//     member placed mid-verb is one whose sheet both can find.
//   - Driver is the compelled wrapper over the verb's own driver: a
//     compulsion is read off a sheet and can leave a condition behind, so the
//     thing that takes a commanded member's turn is built where the sheets are
//     and can save into this scope. See [compelledDriver].
//   - Roller is THE WORLD'S DIE, this session's shared dice, because a write
//     verb can advance a clock and a clock that advances gives creatures time
//     (rpg-project#465).
//   - CheckResolver and Witness are the concealment pair bound to this scope;
//     the encounter requires them exactly when the field carries concealed
//     structure, and they read scope.enc only at consult time.
//   - The actors are bound to this same scope and read scope.enc only from
//     inside a later call, well after the load that hands them over has
//     landed (rpg-project#254).
func (m *Manager) writeCapabilities(ctx context.Context, scope *writeScope) encounter.Capabilities {
	return encounter.Capabilities{
		Initiative:    m.initiative,
		Standing:      scope.standing,
		Sight:         sheetsBeside(scope.standing),
		Equipment:     equipmentBeside(scope.standing),
		Sheets:        sheetsBeside(scope.standing),
		Driver:        m.compelledDriverFor(ctx, scope),
		Roller:        m.encounterDice(),
		CheckResolver: m.checkResolverFor(scope),
		Witness:       witnessSeam{scope: scope},
		Actors: encounter.Actors{
			Striker:   strikerSeam{m: m, scope: scope},
			Mover:     moverSeam{m: m, scope: scope},
			Announcer: announcerSeam{m: m, scope: scope},
		},
	}
}

// readCapabilities serves a read that advances no clock ([Manager.loadWorld]).
//
// The standing seams answer as they do for a write. The driver is the
// session's own plain seam, never a compelled one: a read advances no clock,
// so it is never consulted, and a compelled driver here would have no scope to
// save what an obeyed word leaves behind.
//
// NO DIE, the check resolver and witness refuse or perceive nobody, and every
// actor refuses by name. A read never gives a creature time, rolls a check,
// refreshes sight, drives a turn, walks anybody or crosses a boundary, so any
// of those reached on a read path fails loudly at the point of failure rather
// than quietly doing what nobody asked (encounter.ErrNoRoller,
// encounter.RefusingActors).
func (m *Manager) readCapabilities(standing standingSeam, driver encounter.Driver) encounter.Capabilities {
	return encounter.Capabilities{
		Initiative:    m.initiative,
		Standing:      standing,
		Sight:         sheetsBeside(standing),
		Equipment:     equipmentBeside(standing),
		Sheets:        sheetsBeside(standing),
		Driver:        driver,
		CheckResolver: encounter.RefusingCheckResolver{},
		Witness:       encounter.NobodyPerceives{},
		Actors:        encounter.RefusingActors(),
	}
}

// resolutionAsk is what a verb or seam brings to a resolution: the world as
// data, the cast, the interaction and its price. Everything else on the input
// is this package's to compose.
type resolutionAsk struct {
	World        encounter.EncounterData
	Participants []resolution.Participant
	Machine      resolution.Machine
	// Cost is nil for a free action, which is the common case.
	Cost *resolution.Cost
}

// resolutionInput is the only constructor of [resolution.Input] in this
// package: [Manager.writeCapabilities] over scope with the actors replaced by
// [resolution.Actors].
//
// Resolution carries the capabilities to the world it loads and consults none
// but the roller: it calls no encounter verb, so no driver, check resolver,
// witness or actor is ever asked inside an interaction. The actors are
// resolution's own refusing value because an actor asked there would be a
// second interaction opened from inside the first. A seam passes its own
// scope, which is the scope of the verb that called it.
func (m *Manager) resolutionInput(ctx context.Context, scope *writeScope, ask resolutionAsk) *resolution.Input {
	capabilities := m.writeCapabilities(ctx, scope)
	capabilities.Actors = resolution.Actors
	return &resolution.Input{
		World:        ask.World,
		Participants: ask.Participants,
		Machine:      ask.Machine,
		Cost:         ask.Cost,
		Capabilities: capabilities,
	}
}
