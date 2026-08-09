// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package clock

import "errors"

// Sentinel errors — the module's state vocabulary (design: Errors).
// All returned errors wrap exactly one of these; callers dispatch with
// errors.Is. Messages are user-facing.
var (
	// ErrIdle reports a clock with no order set / nothing to act on.
	ErrIdle = errors.New("clock is idle")
	// ErrNotActive reports that the named actor is not the active entity.
	ErrNotActive = errors.New("not the active entity")
	// ErrNotMember reports an entity that is not in this clock.
	ErrNotMember = errors.New("entity is not a member of this clock")
	// ErrDuplicateMember reports an entity already present.
	ErrDuplicateMember = errors.New("entity is already a member of this clock")
	// ErrBadPosition reports an insert position outside [0, len].
	ErrBadPosition = errors.New("position out of range")
	// ErrBadOrder reports an empty order or a merge order that is not a
	// permutation of the union of both member sets.
	ErrBadOrder = errors.New("invalid order")
	// ErrBadAmount reports a non-positive spend or negative displacement.
	ErrBadAmount = errors.New("invalid amount")
	// ErrInsufficientBudget reports a spend exceeding the member's budget.
	ErrInsufficientBudget = errors.New("insufficient budget")
	// ErrInvalidData reports persisted state rejected by LoadTurn/LoadTick (design R9).
	ErrInvalidData = errors.New("invalid clock data")
)
