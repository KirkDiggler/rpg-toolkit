// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
)

// One sheet store per verb (rpg-project#542, "One sheet store per verb").
//
// Every character record a verb reads or writes goes through the verb's one
// [sheetStore]. It is the only code in this package that calls the host's
// [CharacterRepository]; no seam, builder or machine reaches the repository
// on its own. TestOnlyTheStoreCallsTheCharacterRepository holds that by
// parsing this package's sources.
//
// # Read per ask, write through
//
// The store holds no copy between asks: every load is a repository read and
// every save is a repository write. The repository is the one copy, and the
// guard the verb acts under (the session's for a seated character, the
// character's own for an unseated one) makes this verb its only writer. A
// verb-scoped cache is a deferred ruling (R10) and stays write-through when it
// comes.
//
// # One answer for an absent sheet
//
// A record the repository does not hold is [ErrNoCharacter], wherever it is
// asked for. A repository that reports success with no data, or answers with
// a different character than the one asked for, has broken its contract and
// is [ErrBadRepository]. Callers that tolerate a sheetless member (a candidate
// list peeking at a monster ally, a perception snapshot of a body nobody can
// read) branch on ErrNoCharacter themselves, in the open.
//
// # The report is kept in one place
//
// Every save records its aggregate on the scope the store reports into
// ([writeScope.noteCharacterWritten]); a failed save returns a [SaveError]
// naming what landed and the aggregate that did not (S6). No call site builds
// a report by hand.

// sheetStore is one verb's reader and writer of character records.
//
// The zero value is unusable; build one with [Manager.sheetsFor].
type sheetStore struct {
	// repo is the host's character repository.
	repo CharacterRepository

	// scope is where saves are recorded. Nil for a store that only reads
	// (a read verb, a seam built for a compile-only load); saving through
	// such a store is refused rather than left unreported.
	scope *writeScope
}

// sheetsFor builds the store a verb reads and writes character records
// through. scope is the verb's report; nil builds a read-only store.
func (m *Manager) sheetsFor(scope *writeScope) sheetStore {
	return sheetStore{repo: m.characters, scope: scope}
}

// load reads one stored sheet and checks that the repository kept its side of
// the contract.
//
// role names the part the member plays in the verb — "attacker",
// "participant", plain "character" — so a caller told which record is missing
// knows which member of the roster to go and look at. The sentinel is not
// negotiable: ErrNoCharacter when the repository does not hold the id,
// ErrBadRepository when it reports success with no data or with another
// character's record.
//
// A sheet returned under the wrong id is perfectly loadable — the caller would
// attach SOMEBODY, nothing would error, and the character who was asked for
// would never have been in the interaction at all — which is why the id is
// checked here rather than trusted.
func (s sheetStore) load(ctx context.Context, role, id string) (*character.Data, error) {
	data, err := s.repo.GetCharacter(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("%s %q: %w", role, id, ErrNoCharacter)
		}
		return nil, fmt.Errorf("%s %q: %w", role, id, err)
	}
	if data == nil {
		return nil, fmt.Errorf(
			"%s %q: GetCharacter reported success with no data: %w", role, id, ErrBadRepository)
	}
	if data.ID != id {
		return nil, fmt.Errorf(
			"%s %q: GetCharacter returned %q instead: %w", role, id, data.ID, ErrBadRepository)
	}
	return data, nil
}

// save writes one authoritative character record and records the aggregate on
// the verb's report.
//
// A failed write is a [SaveError] whose report names every aggregate this
// verb already made durable and this character as the one that did not land;
// the repository's own error stays matchable through it. A read-only store
// refuses to save at all: a write nobody reports is the one write S6 exists
// to prevent.
func (s sheetStore) save(ctx context.Context, data *character.Data) error {
	if data == nil {
		return fmt.Errorf("saving character: no record: %w", ErrBadCharacter)
	}
	if s.scope == nil {
		return fmt.Errorf("saving character %q through a read-only sheet store: %w", data.ID, ErrInvalidSession)
	}
	if err := s.repo.SaveCharacter(ctx, data); err != nil {
		return &SaveError{
			Report: SaveReport{
				Written: append([]string(nil), s.scope.written...),
				Failed:  []string{"character:" + data.ID},
			},
			Err: fmt.Errorf("saving character: %w", err),
		}
	}
	s.scope.noteCharacterWritten(data.ID)
	return nil
}
