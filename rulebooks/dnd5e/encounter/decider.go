// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// Intent represents a decision made by a decider in response to their view.
// The anti-wall-hack contract: a decider receives ONLY its own holdings,
// never the full encounter state. Intent is a sealed type (unexported marker method).
type Intent interface {
	isIntent()
}

// IntentMoveTo represents a decision to move to a specific position.
type IntentMoveTo struct {
	// To is the target position the member intends to move to.
	To spatial.Position
}

// isIntent marks IntentMoveTo as an Intent.
func (i IntentMoveTo) isIntent() {}

// IntentHold represents a decision to stay in place (do nothing this tick).
type IntentHold struct{}

// isIntent marks IntentHold as an Intent.
func (i IntentHold) isIntent() {}

// IntentTraverse represents a decision to cross a connection into the room
// on its other side. Preconditions match the Traverse verb exactly: the
// decider's own Snapshot.Room/Position (see Snapshot) must be standing
// exactly on one of the named connection's two endpoints. An illegal
// traverse intent (unknown connection, or not at the threshold) does not
// abort the pump — it follows the same silent-skip contract Pump already
// applies to a spatially-rejected IntentMoveTo: no beat, no room/position
// change for that member, everything else in the pump proceeds normally.
type IntentTraverse struct {
	// Connection is the ID of the connection to traverse.
	Connection string
}

// isIntent marks IntentTraverse as an Intent.
func (i IntentTraverse) isIntent() {}

// Snapshot is a decider's complete input: their OWN current placement
// (room and position) plus their OWN held intel. The anti-wall-hack
// contract (C2) extends to placement, not just holdings — a decider
// learns where IT stands, never where any other member stands except
// through Holdings' sighted percepts. Static field topology (e.g. a
// connections list, for a pursuit decider that needs to know where the
// doors are) is NOT part of Snapshot; give it to a decider at
// construction time instead — Snapshot itself carries only what changes
// tick to tick.
type Snapshot struct {
	// Room is the decider's own current room ID.
	Room string

	// Position is the decider's own current position within Room.
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
