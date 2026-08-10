// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package record

import "errors"

// Sentinel errors — the module's error vocabulary (design: Errors).
// All returned errors wrap exactly one of these; callers dispatch with
// errors.Is. Messages are user-facing.
var (
	// ErrNilInput reports a nil *XxxInput. Caller defect, dedicated sentinel.
	ErrNilInput = errors.New("nil input")
	// ErrBadAudience reports an audience with empty IDs or duplicates.
	ErrBadAudience = errors.New("invalid audience")
	// ErrBadTag reports a tag (or filter) key that is empty.
	ErrBadTag = errors.New("invalid tag key")
	// ErrNoPayload reports a nil payload (empty non-nil is legal).
	ErrNoPayload = errors.New("nil payload")
	// ErrBadSeq reports a trim point beyond NextSeq — you cannot forget the future.
	ErrBadSeq = errors.New("sequence out of range")
	// ErrNoViewer reports an empty viewer ID.
	ErrNoViewer = errors.New("empty viewer")
	// ErrInvalidData reports persisted state rejected by LoadLog (design R9).
	ErrInvalidData = errors.New("invalid log data")
)
