// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"github.com/KirkDiggler/rpg-toolkit/play/interrupt"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
)

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

	// Windows is the ledger of open interrupt windows: who owes an answer,
	// what they may choose, and the frozen resolution waiting on it.
	//
	// It lives here rather than in EncounterData deliberately. An encounter
	// cannot interpret or validate a suspended walk, so holding one would spend
	// the aggregate purity hardened in the anchoring wave on state it does not
	// understand. This struct is ours, so the cost of being wrong is confined
	// to this module.
	//
	// A session written before v0.2.0 has no windows key; it unmarshals to the
	// zero LedgerData, which LoadLedger reads as an idle ledger. Older sessions
	// therefore load unchanged rather than needing a migration.
	Windows interrupt.LedgerData `json:"windows,omitempty"`

	// NPCs are the sheets of members that were instantiated from code rather
	// than loaded from a host repository — monsters, today.
	//
	// They live in the session rather than behind a repository because they
	// are session-scoped: a skeleton spawned into this fight has no existence
	// outside it, and nothing durable to look up. Adding an NPCRepository
	// later is a compatible change; removing one a host had implemented is
	// not, so the reversible direction is to wait until something durable
	// actually exists.
	//
	// They live here rather than in EncounterData for the same reason Windows
	// does, and it is the split the composition already draws: the encounter
	// is the world and holds placement, the session is the table and holds
	// the sheets. A member's position is the encounter's business; its hit
	// points are not.
	//
	// The stored sheet is what gets rehydrated, NOT the ref it was built
	// from. A skeleton that has taken damage is no longer the catalog
	// skeleton, so re-running the constructor on load would silently heal it.
	// monster.Data carries its own Ref, so nothing is lost by storing the
	// instance instead of the recipe.
	//
	// A session written before this field has no npcs key and unmarshals to
	// nil, which reads as "no NPCs" — so older sessions load unchanged.
	NPCs []monster.Data `json:"npcs,omitempty"`
}
