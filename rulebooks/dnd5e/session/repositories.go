// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// The repositories below are most of the host's integration surface: it
// implements them once and thereafter calls verbs with IDs, never holding a
// domain object. (The other piece is EventStream, which lives with the fan-out
// that uses it, in events.go.)
//
// They point OUTWARD, which is the distinction worth keeping even after the
// jargon goes: these are interfaces this package CALLS and the host IMPLEMENTS,
// the reverse of the usual direction. That inversion is what lets storage be
// swapped, mocked, or held in memory without this package knowing.
//
// Two rules shape every one of them.
//
// S12 — repositories are key-value. Every operation is get-by-id or put-by-id. No
// queries, no scans, no joins, no ordering. This is a constraint on this
// package, not a claim about the host's database: honouring it means a
// key-value store stays sufficient forever, and it makes it structurally
// impossible for a later wave to quietly require a relational one.
//
// S3 — repositories trade in data, not domain objects. They carry persistence
// shapes, and reconstitution happens inside this package where the laws live.
// This is also the one deliberate exception to S2's "no inner type crosses the
// boundary": EncounterRepository names encounter.EncounterData, because those
// are exactly the bytes the host persists. Data types are the slowest-moving
// surface in the toolkit and already carry their own compatibility discipline,
// whereas domain types are free to change under a compatible tag.

// SessionRepository persists session state.
//
// In this version that is a small record: an ID and the encounter the session
// points at. It grows as later waves add state that belongs to the session
// rather than the world — open interrupt windows, a frozen resolution,
// session-scoped NPCs — but a host implementing this today is storing two
// strings, and should not be led to expect otherwise.
//
// Required. Implementations must return an error satisfying errors.Is(err,
// ErrNotFound) when the ID is absent, and must never report success with no
// data: that is a contract violation and is rejected as ErrBadRepository
// rather than guessed at in either direction.
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

// CharacterRepository persists player characters.
//
// Required. This is the repository the deferral rule was waiting on: it is
// declared now because something finally calls it, and arriving with a caller
// is what settled its shape rather than a guess.
//
// It names character.Data, and that crosses the boundary on purpose. The
// distinction worth holding is between two kinds of type this package lets out:
//
//   - A PERSISTENCE SHAPE (encounter.EncounterData, interrupt.LedgerData) is
//     bytes the host round-trips and never builds. It promises REPLACEABILITY —
//     we can swap what is underneath and the host never notices.
//   - A CONTRACT TYPE (spatial.Position, character.Data) is shared vocabulary
//     the host constructs and reasons about. It promises the opposite: a change
//     to it is announced, not hidden.
//
// A character is a thing, not an implementation detail we would refactor
// without telling the host. Reading it as a grudging exception gets the intent
// backwards. The version-bump promise is unaffected because that promise was
// only ever about replaceability.
//
// What does NOT cross is *character.Character, the runtime object. It is loaded
// inside a verb, attached to that call's bus, acted on, converted back to data,
// and dropped. The boundary test rejects it, and should.
//
// SaveCharacter takes only the data because character.Data carries its own ID,
// the same asymmetry SaveSession has and for the same reason: passing a key
// alongside a payload that already contains one puts identity in two places.
//
// Save is declared now although the first wave to use this repository only
// reads. Adding a method to an interface the host has already implemented stops
// it compiling, so the reversible direction is to declare both up front — the
// same rule that governs Config fields.
//
// Implementations must return an error satisfying errors.Is(err, ErrNotFound)
// when the ID is absent, and must never report success with no data.
type CharacterRepository interface {
	// GetCharacter returns the character with the given ID, or an ErrNotFound
	// error if it does not exist.
	GetCharacter(ctx context.Context, id string) (*character.Data, error)

	// SaveCharacter writes the character, creating or replacing it wholesale.
	// It stays a whole-record write: the SDK is its only caller (through one
	// sheet store per verb, store.go), and every caller holds the guard the
	// character's seat names.
	SaveCharacter(ctx context.Context, data *character.Data) error
}

// Concurrency is a host responsibility. Stateless load-act-save does not
// prevent simultaneous requests from loading the same world and overwriting
// one another. Config.Locker supplies a separate SessionLocker capability;
// no locking methods are added to these key-value repository interfaces.
// The SDK holds that guard across the entire session-shaped operation,
// including reads, creation, writes and event delivery. Without it, callers
// must serialize externally. All writers of the same session must participate
// in the same coordination domain; a process-local locker cannot protect a
// different process or an out-of-band repository writer.
//
// Exclusion is not a transaction: a failed multi-repository save still carries
// the existing partial SaveReport. Character verbs (Equip, Unequip, LevelUp)
// act under the guard their character's seat names — the session's, or the
// character's own while unseated (seats.go) — so every writer of a character
// record holds the right guard. Durable exploration profiles shared across
// sessions still need their own host consistency policy.
