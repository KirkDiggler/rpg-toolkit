// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// The ports below are the host's entire integration surface. It implements
// them once and thereafter calls verbs with IDs, never holding a domain object.
//
// Two rules shape every one of them.
//
// S12 — ports are key-value. Every operation is get-by-id or put-by-id. No
// queries, no scans, no joins, no ordering. This is a constraint on this
// package, not a claim about the host's database: honouring it means a
// key-value store stays sufficient forever, and it makes it structurally
// impossible for a later wave to quietly require a relational one.
//
// S3 — ports trade in data, not domain objects. They carry persistence shapes,
// and reconstitution happens inside this package where the laws live. This is
// also the one deliberate exception to S2's "no inner type crosses the
// boundary": EncounterRepository names encounter.EncounterData, because those
// are exactly the bytes the host persists. Data types are the slowest-moving
// surface in the toolkit and already carry their own compatibility discipline,
// whereas domain types are free to change under a compatible tag.

// SessionRepository persists session state: the encounter a session points at,
// its open interrupt windows, and any frozen resolution.
//
// Required. Implementations must return an error satisfying errors.Is(err,
// ErrNotFound) when the ID is absent.
type SessionRepository interface {
	// GetSession returns the session with the given ID, or an ErrNotFound
	// error if it does not exist.
	GetSession(ctx context.Context, id string) (*SessionData, error)

	// SaveSession writes the session, creating or replacing it wholesale.
	SaveSession(ctx context.Context, data *SessionData) error
}

// EncounterRepository persists encounters — the world a session plays in.
//
// Required. Separate from SessionRepository because they are different data
// types with different lifetimes (S13), and because keeping them apart leaves
// each one's storage strategy up to the host: an encounter held in memory on a
// live server and checkpointed periodically is invisible from here, which it
// could not be if the two shared a blob.
type EncounterRepository interface {
	// GetEncounter returns the encounter with the given ID, or an ErrNotFound
	// error if it does not exist.
	GetEncounter(ctx context.Context, id string) (*encounter.EncounterData, error)

	// SaveEncounter writes the encounter, creating or replacing it wholesale.
	SaveEncounter(ctx context.Context, id string, data *encounter.EncounterData) error
}

// EventStream delivers events to the host for multiplayer fan-out.
//
// Optional: a single-player setup, a test, or a headless simulation constructs
// without one and simply produces no stream.
//
// Events are already projected per audience when they arrive here — who may
// see what is a rule, decided inside this package where perception lives, not
// a delivery concern the host is expected to re-derive. A host that filtered
// events itself would be reimplementing visibility, and its first mistake
// would leak something a player has not perceived.
//
// Publishing is best-effort by contract: a failure here is reported but does
// not fail the verb, because the story log remains the source of truth and a
// client that misses an event can notice the gap and re-query. Implementations
// should therefore not block indefinitely.
type EventStream interface {
	// Publish delivers a batch of already-projected events.
	Publish(ctx context.Context, events []Event) error
}

// Concurrency: two doors deliberately left open.
//
// Every verb loads, acts and saves (S4), so nothing is cached between calls and
// a stale in-memory world can never overwrite a fresh one. What that does NOT
// prevent is two genuinely simultaneous requests against one session: both load
// the same world, both apply their change, both save, and the later write wins
// while the earlier action vanishes. No failure is required for this — only two
// players acting at the same moment.
//
// It is unaddressed on purpose. The game is turn-based, a host can serialise
// per session trivially, and committing to a concurrency model before real
// contention exists would be guessing. But the doors are worth keeping open,
// and they do not cost the same:
//
// PESSIMISTIC — free to add later. Locking must NOT arrive as a method on
// SessionRepository: adding one to an existing interface stops every host
// compiling. It arrives as a separate optional capability the manager
// type-asserts for, the way spatial.BoundaryAwareRoom already works in this
// codebase:
//
//	type SessionLocker interface {
//	    LockSession(ctx context.Context, id string) (release func(), err error)
//	}
//
// Hosts that implement it get serialised sessions; hosts that do not keep
// working exactly as they do today. Nothing needs to exist for this to remain
// possible, which is why nothing does.
//
// OPTIMISTIC — cheaper if decided early. A version on the blob, with SaveSession
// rejecting a stale write, needs the field to have been there all along;
// retrofitting means a data migration and a change to what SaveSession is
// permitted to do. Deliberately not added yet: a version nobody increments is
// worse than no version, because it reads as a guarantee it does not provide,
// and a migration on a pre-1.0 module whose consumer has not adopted it is
// about as cheap as migrations get.
