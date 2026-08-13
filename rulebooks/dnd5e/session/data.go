// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

// SessionData is the persistent representation of a session.
//
// It holds only session state. The encounter is referenced by ID rather than
// embedded: clock, intel and record ride inside an encounter because they are
// parts of it with no independent lifetime, but an encounter is something a
// session points at. Wrapping it would weld the encounter's storage strategy
// to the session's and would rewrite every room and member on a save that only
// opened a window.
type SessionData struct {
	// ID identifies this session.
	ID string `json:"id"`

	// Encounter is the ID of the encounter this session plays in.
	Encounter string `json:"encounter"`
}
