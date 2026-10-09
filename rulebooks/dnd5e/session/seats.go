// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"fmt"
	"sort"
)

// The seat (rpg-project#542, "The seat").
//
// Which run, if any, holds a character decides how a character verb runs: an
// unseated character gets a plain sheet verb under its own guard; a seated
// one gets the verb inside its run, under the session's guard. The seat is
// the session's fact, kept in a record of its own keyed by character (S12,
// S13) rather than on the character record (R4).
//
// # Who writes it
//
// Launch and Join seat a character; Exit, End and the commit that closes a
// run clear it. Nothing else writes it. A seat changes only under both the
// session's guard and the character's guard, taken session first and then
// characters in id order, so a verb holding either guard reads a seat that
// cannot move beneath it.
//
// # Which order it is written in
//
// Seating is written BEFORE the run that holds the member is saved, and
// clearing AFTER the run that no longer holds it is saved. Both orders fail
// the same way when the second write never lands: the seat names a run the
// character is not in, and a seated verb then refuses loudly. The opposite
// orders would leave a member in a run with no seat, and an unseated verb
// would write its sheet under the wrong guard while the run writes it under
// the right one — the lost update the seat exists to prevent.
//
// # What it protects
//
// A session started before seats existed protects nothing; pre-alpha runs
// are relaunched, never migrated.

// SeatData is one character's seat: the session that holds it, or none.
type SeatData struct {
	// Character is the character the seat belongs to. It is the record's key.
	Character string

	// Session is the run holding the character. Empty means unseated: a seat
	// a run has cleared, written as a record rather than deleted so the
	// repository stays get-by-id and put-by-id (S12).
	Session string
}

// SeatRepository persists seats, keyed by character id.
//
// Required. It is a repository of its own because a seat is a different data
// type with a different lifetime from the session it names (S13): a session
// is one run, a seat is where one character is right now, and the question
// asked of it — "which run holds this character" — has a character id in it
// and no session id at all.
//
// Implementations must return an error satisfying errors.Is(err, ErrNotFound)
// for a character that was never seated, and must never report success with
// no data.
type SeatRepository interface {
	// GetSeat returns the seat recorded for the character, or an ErrNotFound
	// error if none was ever recorded.
	GetSeat(ctx context.Context, character string) (*SeatData, error)

	// SaveSeat writes the seat, creating or replacing it wholesale.
	SaveSeat(ctx context.Context, data *SeatData) error
}

// SeatInput names the character whose seat is asked for.
type SeatInput struct {
	// Character is the character to look up. Required.
	Character string
}

// SeatOutput is the seat a character holds.
type SeatOutput struct {
	// Seat is the character's live seat. It is a copy: changing it changes
	// nothing in the repository. Its Session is never empty.
	Seat *SeatData
}

// Seat answers which run holds a character, so a caller outside the run can
// read the fact the run keeps (rpg-project#548, "The seat is visible").
//
// The seat is a fact the character can read. Seat only reads: it never
// creates a seat, never moves one and never clears one, and it takes no
// guard, so it answers even while a verb is writing the seat. A caller that
// needs the seat to hold still takes the guard it names.
//
// Seat answers [ErrNoSeat] whenever the character holds no live seat, whether
// it was never seated or a run has cleared it. A successful answer always
// names a non-empty session.
//
// Errors: [ErrNilInput] for a nil input, [ErrNoCharacter] for an empty
// character, [ErrNoSeat] as above, [ErrBadRepository] for a repository that
// reports success with no data or another character's seat, and any other
// repository error wrapped.
func (m *Manager) Seat(ctx context.Context, in *SeatInput) (*SeatOutput, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	if in.Character == "" {
		return nil, fmt.Errorf("seat: no character named: %w", ErrNoCharacter)
	}
	held, err := m.seatOf(ctx, in.Character)
	if err != nil {
		return nil, err
	}
	if held == "" {
		return nil, fmt.Errorf("character %q: %w", in.Character, ErrNoSeat)
	}
	return &SeatOutput{Seat: &SeatData{Character: in.Character, Session: held}}, nil
}

// seatOf reads which session holds a character: its id, or empty when the
// character is unseated (never seated, or cleared).
//
// Returns ErrBadRepository for a repository that reports success with no data
// or with another character's seat, and any other repository error wrapped.
func (m *Manager) seatOf(ctx context.Context, character string) (string, error) {
	seat, err := m.seats.GetSeat(ctx, character)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return "", nil
		}
		return "", fmt.Errorf("seat of %q: %w", character, err)
	}
	if seat == nil {
		return "", fmt.Errorf("seat of %q: GetSeat reported success with no data: %w", character, ErrBadRepository)
	}
	if seat.Character != character {
		return "", fmt.Errorf("seat of %q: GetSeat returned %q instead: %w", character, seat.Character, ErrBadRepository)
	}
	return seat.Session, nil
}

// writeSeat records a character's seat and names the aggregate on the verb's
// report. session empty clears the seat.
//
// A failed write is a [SaveError] naming what this verb already made durable
// and the seat that did not land (S6).
func (m *Manager) writeSeat(ctx context.Context, scope *writeScope, character, session string) error {
	aggregate := "seat:" + character
	if err := m.seats.SaveSeat(ctx, &SeatData{Character: character, Session: session}); err != nil {
		return &SaveError{
			Report: SaveReport{
				Written: append([]string(nil), scope.written...),
				Failed:  []string{aggregate},
			},
			Err: fmt.Errorf("saving seat: %w", err),
		}
	}
	scope.noteWritten(aggregate)
	return nil
}

// seatIn seats a character in the scope's session under the guards the verb
// already holds. A character already seated here is left as it is — a rejoin
// writes nothing — and one seated in another run is refused with
// [ErrSeatedElsewhere] before anything is written.
func (m *Manager) seatIn(ctx context.Context, scope *writeScope, character string) error {
	current, err := m.seatOf(ctx, character)
	if err != nil {
		return err
	}
	switch current {
	case scope.session:
		return nil
	case "":
		return m.writeSeat(ctx, scope, character, scope.session)
	default:
		return fmt.Errorf("character %q is seated in session %q: %w", character, current, ErrSeatedElsewhere)
	}
}

// refuseSeatedElsewhere refuses, before anything is written, a character a
// different run holds.
func (m *Manager) refuseSeatedElsewhere(ctx context.Context, session, character string) error {
	current, err := m.seatOf(ctx, character)
	if err != nil {
		return err
	}
	if current != "" && current != session {
		return fmt.Errorf("character %q is seated in session %q: %w", character, current, ErrSeatedElsewhere)
	}
	return nil
}

// clearSeats clears the seats the scope's session holds for the given
// characters, taking each character's guard the verb does not already hold
// (the session's guard is already held, so the order stays session first).
// A seat naming a different run is left alone: this run's ending is not that
// run's business.
func (m *Manager) clearSeats(ctx context.Context, scope *writeScope, characters []string) error {
	if len(characters) == 0 {
		return nil
	}
	release, err := m.acquireCharactersFor(ctx, scope, characters...)
	if err != nil {
		return saveErrorAfterWrites(scope, "", err)
	}
	defer release()

	ordered := append([]string(nil), characters...)
	sort.Strings(ordered)
	for _, character := range ordered {
		current, err := m.seatOf(ctx, character)
		if err != nil {
			return saveErrorAfterWrites(scope, "", err)
		}
		if current != scope.session {
			continue
		}
		if err := m.writeSeat(ctx, scope, character, ""); err != nil {
			return err
		}
	}
	return nil
}

// actingGuard is the guard a character verb acts under and the seat it read
// under that guard.
type actingGuard struct {
	// session is the run holding the character, or empty when unseated.
	session string

	// release gives the guard back. Never nil.
	release func()
}

// guardBySeat takes the guard a character verb acts under, decided by the
// seat read UNDER that guard: an unseated character's own guard, or the
// session's guard for a seated one.
//
// The seat is peeked first, the matching guard taken, and the seat read
// again under it. A seat that moved in between (a launch seated the
// character, an exit released it) gives the guard back and tries again, so
// no verb ever holds a character's guard while waiting for a session's, and
// none acts on a seat that changed beneath it. Seat changes take both
// guards, so either guard alone freezes the seat it was taken for.
//
// Cancellation ends the attempt with the context's error.
func (m *Manager) guardBySeat(ctx context.Context, character string) (*actingGuard, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		peeked, err := m.seatOf(ctx, character)
		if err != nil {
			return nil, err
		}

		var release func()
		if peeked == "" {
			release, err = m.acquireCharacters(ctx, character)
		} else {
			release, err = m.acquireSession(ctx, peeked)
		}
		if err != nil {
			return nil, err
		}

		held, err := m.seatOf(ctx, character)
		if err != nil {
			release()
			return nil, err
		}
		if held == peeked {
			return &actingGuard{session: held, release: release}, nil
		}
		release()
	}
}
