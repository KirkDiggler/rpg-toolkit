// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package perception

import "errors"

// Sentinel errors — the module's error vocabulary. All returned errors wrap
// exactly one of these; callers dispatch with errors.Is. Observe validates in
// this order, before any mutation: ErrNoReach, ErrNoChannel, ErrNoSubject,
// ErrNoObserver, ErrDuplicateSubject, ErrDuplicateObserver (R8).
var (
	// ErrNoReach reports a Pass with a nil Reach. Reach is the whole of the
	// physics this package delegates to the caller, so a pass cannot proceed
	// without one.
	ErrNoReach = errors.New("nil reach")
	// ErrNoChannel reports a Pass with an empty Channel.
	ErrNoChannel = errors.New("empty channel")
	// ErrNoSubject reports a Presence with an empty ID, or an On query
	// against an empty subject.
	ErrNoSubject = errors.New("empty subject")
	// ErrNoObserver reports an empty observer ID.
	ErrNoObserver = errors.New("empty observer")
	// ErrDuplicateSubject reports two Presences in one Pass sharing an ID.
	// Rule 6's determinism promise depends on presences forming a set once
	// sorted by ID; intel's own dedupe is last-wins over a slice, and which
	// duplicate survives an unstable sort depends on input order. Rather
	// than leave that to chance, a duplicate ID is a caller bug, rejected
	// before any mutation.
	ErrDuplicateSubject = errors.New("duplicate presence id")
	// ErrDuplicateObserver reports the same observer named twice in one
	// Pass's Observers — also a caller bug, for the same reason.
	ErrDuplicateObserver = errors.New("duplicate observer")
	// ErrNotHeld reports that an observer holds nothing on a subject. This
	// is On's routine "nothing there" answer, not a wiring fault — unlike
	// the six validation sentinels above, it is not caught before mutation,
	// it IS the result. On translates intel's own not-held error into this
	// one so a caller can dispatch on it without importing play/intel: the
	// charter in doc.go says intel is never seen by callers, and a caller
	// forced to check errors.Is(err, intel.ErrNotHeld) would make that
	// false for exactly the error callers hit most often. Do not replace
	// this translation with a bare wrap.
	ErrNotHeld = errors.New("nothing held on subject")
)
