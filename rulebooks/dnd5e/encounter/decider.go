// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// Intent represents a decision made by a decider in response to their
// Snapshot. The anti-wall-hack contract: a decider receives ONLY its own
// Snapshot (own room, own position, own holdings), never the full
// encounter state or another member's live truth. Intent is a sealed
// type (unexported marker method).
type Intent interface {
	isIntent()
}

// IntentMoveTo represents a decision to step to a specific cell.
type IntentMoveTo struct {
	// To is the cell the member intends to step to, in DUNGEON-ABSOLUTE
	// space — the same frame Snapshot.Position and every sight payload
	// speak, so a decider can compare where it stands with where it last
	// saw something and subtract one from the other (rpg-toolkit#1044).
	//
	// A cell in the room the member is already in, or the far side of a
	// doorway they are standing at: both are ordinary steps, and the
	// composition decides which mechanism carries them out. An unreachable
	// or unowned cell is skipped in silence rather than aborting the pump.
	To spatial.Position
}

// isIntent marks IntentMoveTo as an Intent.
func (i IntentMoveTo) isIntent() {}

// IntentHold represents a decision to stay in place (do nothing this tick).
type IntentHold struct{}

// isIntent marks IntentHold as an Intent.
func (i IntentHold) isIntent() {}

// There was an IntentTraverse here, naming a connection to cross. It retired
// with rpg-toolkit#1044: an intent now names a CELL in dungeon-absolute space,
// and W3 makes a doorway's two endpoints adjacent absolute cells — so crossing
// one is an ordinary intended step, and which mechanism carries it out is the
// composition's bookkeeping rather than a decision a monster makes. See stepTo.

// Snapshot is a decider's complete input: where it stands, plus its OWN held
// intel. The anti-wall-hack contract (C2) extends to placement, not just
// holdings — a decider learns where IT stands, never where any other member
// stands except through Holdings' sighted percepts. Static field topology is
// NOT part of Snapshot; give it to a decider at construction time instead —
// Snapshot carries only what changes tick to tick.
type Snapshot struct {
	// Position is where the decider stands, in DUNGEON-ABSOLUTE space.
	//
	// There was a Room beside this, and the position was local to it. Both
	// halves were needed then and neither is now (rpg-toolkit#1044): a
	// decider that knows its own cell on the map, and reads its targets'
	// cells on the same map, has nothing left to reconcile. Keeping the room
	// would keep the frame the reshape exists to remove — and it was never
	// really identity a monster needed, only the key to a coordinate system.
	Position spatial.Position

	// Holdings is the decider's own held intel — exactly what HeldBy
	// returns for this member, nothing more (C2).
	Holdings []intel.Holding
}

// Decider is the interface for monster intelligence. A decider receives ONLY
// its own Snapshot (own room, own position, own holdings — never another
// member's live truth) and returns an Intent representing what the monster
// wants to do, or an error that aborts the pump atomically. The anti-wall-
// hack contract is structural: Decide receives exactly what the monster
// knows about itself and can see, nothing more.
type Decider interface {
	// Decide takes the decider's own Snapshot and returns an Intent (what
	// to do) or an error that aborts the pump atomically. If an error is
	// returned, the Pump rolls back completely: the clock is not advanced,
	// no moves are executed, and no record beats are appended.
	Decide(snap Snapshot) (Intent, error)
}
