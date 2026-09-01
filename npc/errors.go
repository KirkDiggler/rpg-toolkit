// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package npc

import "errors"

var (
	// ErrNoData reports a nil Data load request.
	ErrNoData = errors.New("npc: data is required")

	// ErrNoRef reports an NPC without a content ref.
	ErrNoRef = errors.New("npc: ref is required")

	// ErrInvalidRef reports an NPC with a malformed content ref.
	ErrInvalidRef = errors.New("npc: ref is invalid")

	// ErrNoDisplayName reports an NPC without a display name.
	ErrNoDisplayName = errors.New("npc: display name is required")

	// ErrEmptyCapability reports an empty capability label.
	ErrEmptyCapability = errors.New("npc: capability must not be empty")

	// ErrNoCombatPolicy reports an NPC without a combat policy.
	ErrNoCombatPolicy = errors.New("npc: combat policy is required")

	// ErrUnknownCombatPolicy reports an unsupported combat policy.
	ErrUnknownCombatPolicy = errors.New("npc: combat policy is unknown")

	// ErrNoObservationPolicy reports an NPC without an observation policy.
	ErrNoObservationPolicy = errors.New("npc: observation policy is required")

	// ErrUnknownObservationPolicy reports an unsupported observation policy.
	ErrUnknownObservationPolicy = errors.New("npc: observation policy is unknown")

	// ErrNoDispositionPolicy reports an NPC without a disposition policy.
	ErrNoDispositionPolicy = errors.New("npc: disposition policy is required")

	// ErrUnknownDispositionPolicy reports an unsupported disposition policy.
	ErrUnknownDispositionPolicy = errors.New("npc: disposition policy is unknown")

	// ErrNoMovementPolicy reports an NPC without a movement policy.
	ErrNoMovementPolicy = errors.New("npc: movement policy is required")

	// ErrUnknownMovementPolicy reports an unsupported movement policy.
	ErrUnknownMovementPolicy = errors.New("npc: movement policy is unknown")
)
