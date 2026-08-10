// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package intel

import "errors"

// Sentinel errors — the module's error vocabulary (design: Errors).
// All returned errors wrap exactly one of these; callers dispatch with
// errors.Is. Messages are user-facing.
var (
	// ErrNilInput reports a nil *XxxInput. Caller defect, dedicated sentinel.
	ErrNilInput = errors.New("nil input")
	// ErrNoObserver reports an empty observer ID.
	ErrNoObserver = errors.New("empty observer")
	// ErrNoChannel reports an empty channel identifier — the vocabulary
	// is open, but the identifier is required.
	ErrNoChannel = errors.New("empty channel")
	// ErrNoSubject reports a report or query with an empty subject.
	ErrNoSubject = errors.New("empty subject")
	// ErrNotHeld reports that the observer holds nothing on that subject.
	ErrNotHeld = errors.New("nothing held on subject")
	// ErrInvalidData reports persisted state rejected by LoadIntel (design R9).
	ErrInvalidData = errors.New("invalid intel data")
)
