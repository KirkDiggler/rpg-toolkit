// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"fmt"
	"sort"
)

// ErrBadSessionLock means a locker reported success without a usable release.
var ErrBadSessionLock = errors.New("session locker returned no release")

// SessionLocker is host-owned exclusion for a complete session operation.
// Managers sharing storage must share a coordination domain. Implementations
// must be concurrency-safe, honor cancellation while waiting, and retain
// exclusion until Release is called, including after the request is canceled.
// They must not expire a held lock while the operation can still write.
//
// The SDK calls this before repository access for every public operation with a
// Session input, including reads and StartSession, and releases after the last
// save/delivery or error. A callback invoked during that operation must not
// synchronously re-enter a Manager operation for the same session.
//
// It also holds the CHARACTER guard (rpg-project#542, "The seat"): the
// exclusion an unseated character's sheet is written under, and the second
// half of the pair a seat changes under. The SDK always takes the guards in
// one order — a session first, then characters in id order — and never holds
// a character's guard while waiting for a session's, so a host implementing
// both with ordinary mutexes cannot deadlock through the SDK.
//
// This does not make multiple repository writes transactional. A host with
// multiple processes needs a shared coordinator; an in-process mutex alone
// does not provide cross-process exclusion.
type SessionLocker interface {
	// LockSession acquires exclusion or returns an error without a held guard.
	LockSession(context.Context, *LockSessionInput) (*LockSessionOutput, error)

	// LockCharacter acquires one character's exclusion or returns an error
	// without a held guard. The same contract as LockSession: honor
	// cancellation while waiting, keep the guard until Release, never expire
	// it while the operation can still write.
	LockCharacter(context.Context, *LockCharacterInput) (*LockCharacterOutput, error)
}

// LockSessionInput identifies the session whose complete operation is guarded.
type LockSessionInput struct {
	// Session is the storage identity, not an observer or character ID.
	Session string
}

// LockSessionOutput holds a successfully acquired guard. Release is required;
// the SDK invokes it exactly once, even when an operation fails or panics.
// Release must not require a live request context.
type LockSessionOutput struct {
	Release func()
}

// acquireSession leaves argument validation to the operation when no session
// was supplied. Without a locker the host retains its external-serialization
// obligation; no implicit lock or background process is created here.
func (m *Manager) acquireSession(ctx context.Context, id string) (func(), error) {
	var held *LockSessionOutput
	if m.locker != nil && id != "" {
		var err error
		held, err = m.locker.LockSession(ctx, &LockSessionInput{Session: id})
		if err != nil {
			return nil, fmt.Errorf("lock session %q: %w", id, err)
		}
		if held == nil || held.Release == nil {
			return nil, fmt.Errorf("lock session %q: %w", id, ErrBadSessionLock)
		}
	}
	return func() {
		if held != nil {
			held.Release()
		}
	}, nil
}

// LockCharacterInput identifies the character whose guard is taken.
type LockCharacterInput struct {
	// Character is the character id, the same key the character and seat
	// repositories use.
	Character string
}

// LockCharacterOutput holds a successfully acquired character guard. Release
// is required; the SDK invokes it exactly once. Release must not require a
// live request context.
type LockCharacterOutput struct {
	Release func()
}

// acquireCharacters takes the guards of the given characters, each once and
// in id order, and returns one release that gives them back in reverse. A
// failure gives back whatever was already taken. Without a locker the host
// retains its external-serialization obligation, as for sessions.
func (m *Manager) acquireCharacters(ctx context.Context, characters ...string) (func(), error) {
	ordered := uniqueSorted(characters)
	var releases []func()
	releaseAll := func() {
		for i := len(releases) - 1; i >= 0; i-- {
			releases[i]()
		}
	}
	if m.locker == nil {
		return releaseAll, nil
	}
	for _, character := range ordered {
		held, err := m.locker.LockCharacter(ctx, &LockCharacterInput{Character: character})
		if err != nil {
			releaseAll()
			return nil, fmt.Errorf("lock character %q: %w", character, err)
		}
		if held == nil || held.Release == nil {
			releaseAll()
			return nil, fmt.Errorf("lock character %q: %w", character, ErrBadSessionLock)
		}
		releases = append(releases, held.Release)
	}
	return releaseAll, nil
}

// acquireCharactersFor takes, for a verb already holding its session's guard,
// the guards of the given characters it does not hold yet, and remembers them
// on the scope until the returned release gives them back. A guard the verb
// already holds is never asked for twice, so a host's non-reentrant lock is
// safe.
func (m *Manager) acquireCharactersFor(ctx context.Context, scope *writeScope, characters ...string) (func(), error) {
	var wanted []string
	for _, character := range uniqueSorted(characters) {
		if !scope.heldCharacters[character] {
			wanted = append(wanted, character)
		}
	}
	release, err := m.acquireCharacters(ctx, wanted...)
	if err != nil {
		return nil, err
	}
	if scope.heldCharacters == nil {
		scope.heldCharacters = map[string]bool{}
	}
	for _, character := range wanted {
		scope.heldCharacters[character] = true
	}
	return func() {
		for _, character := range wanted {
			delete(scope.heldCharacters, character)
		}
		release()
	}, nil
}

// uniqueSorted is the id order guards are taken in, each id once.
func uniqueSorted(ids []string) []string {
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
