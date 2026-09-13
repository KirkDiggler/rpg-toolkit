// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior

import "errors"

// Sentinel errors — the module's error vocabulary. Every returned error wraps
// exactly one; callers dispatch with errors.Is (R13).
var (
	// ErrNoReader reports a game built without a Reader. Behaviour cannot
	// know where anything is without one, and a game that silently read
	// nothing would pass forever.
	ErrNoReader = errors.New("behavior: no reader")
	// ErrNoMind reports a turn for an actor nobody gave a mind.
	ErrNoMind = errors.New("behavior: the actor has no mind")
	// ErrNoSelf reports a turn for an actor nobody placed. It fails loudly:
	// an actor with no position would find nothing in reach and pass
	// forever, which would look like a very cautious monster rather than a
	// wiring fault.
	ErrNoSelf = errors.New("behavior: the actor has not been placed")
)
