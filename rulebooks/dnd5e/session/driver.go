// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// Driver returns THE driver: every member with no player is driven by its own
// authored table, rolled through this session's shared dice
// (rpg-project#465, ideas/creature-table/design.md).
//
// Wire it as Config.TurnDriver. It holds nothing between turns, so one value
// serves every session this Manager serves — which is the difference from the
// pair it replaces.
//
// # What it replaced, and why one constructor instead of two
//
// `Behavior()` was a reference driver written in Go — attack the closest
// standing player, else close, else pass — and `Minded()` gave each member the
// preset its sheet named, "retaliator" or "coward", each with feel numbers
// nobody outside this repository could reach. Both are deleted. A creature's
// policy is the table an author wrote for it, and a streamer can open a table
// (design §7).
//
// So there is one driver now rather than a family of them, and nothing about
// a creature is chosen here: what it does was written by an author, laid over
// the rulebook's default for its kind, loaded by its temperament, and filtered
// by what it has seen and suffered. This constructor names the roll's owner
// and nothing else.
//
// # It is stateless, so Config.TurnDriver is the field
//
// `Minded()` had to be wired to Config.TurnDrivers because it remembered which
// preset each member was given, and one of them serving every session in a
// process meant two parties sharing a skeleton's brain (rpg-toolkit#1734). The
// table remembers nothing: a creature's whole memory is its own holdings, which
// live in the world rather than in the driver. Config.TurnDrivers remains for a
// host with a stateful driver of its own.
func Driver() TurnDriver {
	return tableDriver{}
}

// tableDriver NAMES the creature's table as this session's driver. It carries
// no behaviour of its own: [Manager.resolveTurnDriver] recognises it and builds
// the composition's own [encounter.TableDriver] around this session's dice.
//
// # Why a marker and not an adapter
//
// Every other driver crosses the boundary by projection: the composition's
// MonsterView flattens onto this package's twin on the way in, and this
// package's TurnIntent projects back on the way out ([turnDriverSeam]). The
// table driver CANNOT cross that way, and the reason is structural rather than
// a matter of taste.
//
//   - The twin view carries no table, no temperament and no deeds. A table
//     driver handed one would roll an empty table and hold, every turn,
//     silently — the worst possible failure for a creature that was given
//     orders.
//   - A pick's whole arithmetic travels back beside the intent
//     ([encounter.Decision]), and this package's TurnIntent has nowhere to put
//     it. Projected through the twin, every `time` roll would reach the story
//     log as a creature acting for no stated reason.
//
// Twinning the table's vocabulary to fix that would put the author's entire
// grammar — keys, conditions, selectors, weights, temperaments — on this
// package's exported surface for no host to read, and would still leave the
// arithmetic to re-derive. So the driver is named here and constructed one
// layer down, where the view it needs already exists.
//
// This is not the exception [basicSeam] used to refuse. That one was about a
// round trip being WASTEFUL for a driver that happened to live in the same
// repository. This one is about a round trip being LOSSY.
type tableDriver struct{}

// compile-time proof the marker satisfies the field it is wired to.
var _ TurnDriver = tableDriver{}

// Act always refuses, the way [refusingTurnDriver] does and for a kindred
// reason: reaching it means the marker was asked to drive a turn through the
// twin view instead of being recognised at [Manager.resolveTurnDriver], and a
// table rolled against a view with no table in it would hold forever while
// looking like a creature that chose to.
//
// Nothing in this package calls it. It exists because the marker has to
// satisfy [TurnDriver] to be wired at all, and a silent Pass here would turn
// this package's own wiring bug into a board of idle monsters.
func (tableDriver) Act(view MonsterView) (TurnIntent, error) {
	return nil, fmt.Errorf(
		"member %q: the creature's table is rolled on the composition's own view, never on this twin: %w",
		view.Self, ErrBadTurnOutcome)
}

// encounterDriverFor is the composition's own driver for one resolved session
// driver: the table's, built around the dice supplied here, or the projecting
// seam for anything a host wired itself.
//
// ONE PLACE THE SWAP HAPPENS, so "which driver is this verb using" keeps the
// single answer [Manager.resolveTurnDriver] was shaped to give it.
func encounterDriverFor(driver TurnDriver, roller *diceSeam) encounter.Driver {
	if _, ok := driver.(tableDriver); ok {
		// THE SESSION'S OWN DICE, not a roller of the driver's making: every
		// pick a creature makes is a roll the story log shows with the
		// creature as the die's entity (design §6, R7), and a driver holding
		// private randomness would make a run unreplayable.
		return encounter.TableDriver{Roller: roller}
	}

	return turnDriverSeam{driver: driver}
}
