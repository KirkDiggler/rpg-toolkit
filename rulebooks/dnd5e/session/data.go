// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
)

// SessionData is the persistent representation of a session.
//
// It holds only session state. The encounter is referenced by ID rather than
// embedded: clock, intel and record ride inside an encounter because they are
// parts of it with no independent lifetime, but an encounter is something a
// session points at. Wrapping it would weld the encounter's storage strategy
// to the session's and would rewrite every room and member on a save that only
// touched session state.
type SessionData struct {
	// ID identifies this session.
	ID string `json:"id"`

	// Encounter is the ID of the encounter this session plays in.
	Encounter string `json:"encounter"`

	// There was a "windows" key here, holding a ledger of open interrupt
	// windows and the frozen resolution waiting on each. Nothing opens a window
	// any more (rpg-toolkit#964 slice 2), so the field retired with its
	// producer. A stored session that still carries the key unmarshals with it
	// ignored, which is why removing it needs no migration — the same property
	// that let it arrive without one.

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

	// Streams is each ever-member's delivered-stream cursor — the persisted
	// half of per-recipient dense numbering (stream.go's whole account of
	// why it must persist and what it survives). Keyed by member ID.
	//
	// It lives HERE, not in EncounterData, by the split the composition
	// already draws: the record numbers the story globally and is complete
	// at that; who has been DELIVERED what is a fact about the table.
	//
	// A session written before this field unmarshals to nil, which reads as
	// "no number was ever issued" — the one legal seeding (stream.go).
	Streams map[string]StreamCursor `json:"streams,omitempty"`
}
