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

// Decider is the interface for monster intelligence. A decider receives ONLY
// its own holdings (the monster's own vision) and returns an Intent representing
// what the monster wants to do, or an error that aborts the pump atomically.
// The anti-wall-hack contract is structural: Decide receives exactly what the
// monster can see (its own observations), nothing more.
type Decider interface {
	// Decide takes the decider's own holdings (what it can see) and returns
	// an Intent (what to do) or an error that aborts the pump atomically.
	// If an error is returned, the Pump rolls back completely: the clock is not
	// advanced, no moves are executed, and no record beats are appended.
	Decide(view []intel.Holding) (Intent, error)
}
