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
// They must not expire a held lock while the operation can still write, with
// one exception: a SESSION lock may expire (a lease in a shared store, which
// cannot promise otherwise) when the host supplies a
// [FencedEncounterRepository]. Every write made under a session's guard (the
// world, the session record, the seats and the sheets of the characters it
// holds) then rides one fenced save naming the checkpoint version the
// operation loaded, so a writer whose lease ran out writes nothing and fails
// closed with [ErrStaleCheckpoint] instead of saving over its successor. The
// record's baseline lives beside that version: under
// [AppendingEncounterRepository] the save appends the record from the first
// seq the store does not hold, which the verb read with the version; under
// [PiecesEncounterRepository] the marks of the pieces the store holds sit
// beside it, read with the version and replaced only by a landed save. A
// refused save leaves all of them where they were. A
// character's own guard (an unseated character's sheet) is not fenced and
// must not expire while the operation can still write.
//
// The SDK calls this before repository access for every public operation with a
// Session input, including reads and Launch, and releases after the last
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
// THE RESIDENT COPY RIDES THE SAME GUARD (residency.go): a write verb takes
// its session's kept encounter when it opens and only the guard's release
// keeps one back, so a copy is never used by two verbs at once. A host whose
// lease can expire is still safe: the copy is checked against the store's
// checkpoint version before use, and the fence refuses a late writer's save.
//
// The locker guards the INSTANCE tier, one game's world, and nothing else:
// the SITE tier (Config.Sites, sites.go) is shared across games and read-only,
// so it takes no session's guard.
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
		// THE RESIDENT COPY IS KEPT HERE, OR NOWHERE (residency.go): the copy
		// the verb's final commit staged once its save landed and its
		// batches went out. Every other exit staged nothing, and the copy
		// the verb took on open is dropped with it: a stale save, a refused
		// verb, a commit error, a canceled context, a continuation that
		// failed after a pace commit, a panic in a host collaborator before
		// the stage, a run that closed. Kept before the guard is given back,
		// so the next verb on the session finds it.
		if id != "" {
			m.keepStaged(id)
		}
		// What a fenced verb wrote and did not save dies with its guard
		// (pendingWrites), before another verb can take the guard.
		if m.pending != nil && id != "" {
			m.pending.drop(id)
		}
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
// the guards of the given characters it does not hold yet, and remembers each
// on the scope until the returned release gives back the ones this call
// asked for.
//
// THE ID ORDER IS KEPT ACROSS THE WHOLE VERB, not only within one call: a
// verb holding carol's guard that comes to need bob's (a Join whose commit
// closes the run clears every seat) gives carol's back and takes bob's, then
// carol's again, so no verb ever waits on a lower id while holding a higher
// one. Giving a guard back is safe here because the session's guard is held
// throughout: every character it is taken for is seated in this run, and
// both its sheet and its seat change only under that session guard.
//
// A guard the verb already holds is never asked for twice, so a host's
// non-reentrant lock is safe.
func (m *Manager) acquireCharactersFor(ctx context.Context, scope *writeScope, characters ...string) (func(), error) {
	if scope.heldCharacters == nil {
		scope.heldCharacters = map[string]func(){}
	}
	var wanted []string
	for _, character := range uniqueSorted(characters) {
		if _, held := scope.heldCharacters[character]; !held {
			wanted = append(wanted, character)
		}
	}
	if len(wanted) == 0 {
		return func() {}, nil
	}

	// Every held guard above the lowest wanted id is given back first, so
	// the retake runs in one ascending pass.
	var retake []string
	for character := range scope.heldCharacters {
		if character > wanted[0] {
			retake = append(retake, character)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(retake)))
	for _, character := range retake {
		scope.heldCharacters[character]()
		delete(scope.heldCharacters, character)
	}

	// ALL OR NOTHING: a guard that cannot be taken gives back every guard
	// this pass took, retaken ones included, so a failed call leaves the verb
	// holding no character guard it would have to release in the wrong order.
	var taken []string
	for _, character := range uniqueSorted(append(append([]string(nil), wanted...), retake...)) {
		release, err := m.lockCharacter(ctx, character)
		if err != nil {
			for i := len(taken) - 1; i >= 0; i-- {
				scope.heldCharacters[taken[i]]()
				delete(scope.heldCharacters, taken[i])
			}
			return nil, err
		}
		scope.heldCharacters[character] = release
		taken = append(taken, character)
	}
	return func() {
		for i := len(wanted) - 1; i >= 0; i-- {
			if release, ok := scope.heldCharacters[wanted[i]]; ok {
				release()
				delete(scope.heldCharacters, wanted[i])
			}
		}
	}, nil
}

// lockCharacter takes one character's guard, or answers a release that does
// nothing when the host serializes externally.
func (m *Manager) lockCharacter(ctx context.Context, character string) (func(), error) {
	if m.locker == nil {
		return func() {}, nil
	}
	held, err := m.locker.LockCharacter(ctx, &LockCharacterInput{Character: character})
	if err != nil {
		return nil, fmt.Errorf("lock character %q: %w", character, err)
	}
	if held == nil || held.Release == nil {
		return nil, fmt.Errorf("lock character %q: %w", character, ErrBadSessionLock)
	}
	return held.Release, nil
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
