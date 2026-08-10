// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package interrupt

import "errors"

// Sentinel errors — the module's error vocabulary (design: Errors).
// All returned errors wrap exactly one of these; callers dispatch with
// errors.Is. Messages are user-facing.
var (
	// ErrNilInput reports a nil *XxxInput. Caller defect, dedicated sentinel.
	ErrNilInput = errors.New("nil input")
	// ErrNoAudience reports an empty audience entity ID (Pose.Audience,
	// Answer.By, PendingFor.Audience).
	ErrNoAudience = errors.New("empty audience")
	// ErrNoOptions reports a Pose with no options — an unanswerable
	// window would deadlock the encounter (the liveness guard).
	ErrNoOptions = errors.New("no options offered")
	// ErrNoOption reports an empty option token (in Pose options or as
	// an Answer choice).
	ErrNoOption = errors.New("empty option")
	// ErrDuplicateOption reports a repeated option token within one window.
	ErrDuplicateOption = errors.New("duplicate option")
	// ErrNotOpen reports no open window with that ID — unknown, never
	// posed, or already answered (one answer per window).
	ErrNotOpen = errors.New("window not open")
	// ErrNotAudience reports an answerer who is not the window's audience.
	ErrNotAudience = errors.New("not the window's audience")
	// ErrNotOffered reports a choice that is not among the window's options.
	ErrNotOffered = errors.New("choice not offered")
	// ErrInvalidData reports persisted state rejected by LoadLedger (design R9).
	ErrInvalidData = errors.New("invalid ledger data")
)
