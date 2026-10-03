// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"fmt"
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
// This does not coordinate character-only operations or make multiple repository
// writes transactional. A host with multiple processes needs a shared coordinator;
// an in-process mutex alone does not provide cross-process exclusion.
type SessionLocker interface {
	// LockSession acquires exclusion or returns an error without a held guard.
	LockSession(context.Context, *LockSessionInput) (*LockSessionOutput, error)
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
