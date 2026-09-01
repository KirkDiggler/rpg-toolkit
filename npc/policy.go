// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package npc

// Capability is an opaque label for behavior another package may route on.
type Capability string

const (
	// CapabilityVendor marks an NPC as vendor-like. The vendor behavior itself
	// is owned by the rulebook or host package that interprets this label.
	CapabilityVendor Capability = "vendor"
)

// CombatPolicy is authored combat participation intent.
type CombatPolicy string

const (
	// CombatPolicyNonCombatant means the NPC is authored as outside combat by
	// default. Runtime systems still own enforcement.
	CombatPolicyNonCombatant CombatPolicy = "non_combatant"
)

// ObservationPolicy is authored observation intent.
type ObservationPolicy string

const (
	// ObservationPolicySubjectOnly means other participants may observe this
	// NPC, but the NPC does not receive its own observation state from content
	// alone.
	ObservationPolicySubjectOnly ObservationPolicy = "subject_only"

	// ObservationPolicyObserver means a runtime may give this NPC its own
	// observation state when the runtime supports that.
	ObservationPolicyObserver ObservationPolicy = "observer"
)

// DispositionPolicy is authored default stance, not pairwise hostility.
type DispositionPolicy string

const (
	// DispositionPolicyNeutral means no authored ally/enemy stance is asserted.
	DispositionPolicyNeutral DispositionPolicy = "neutral"
)

// MovementPolicy is authored movement-occupancy intent.
type MovementPolicy string

const (
	// MovementPolicyBlocking maps to today's binary spatial occupancy as a
	// movement blocker.
	MovementPolicyBlocking MovementPolicy = "blocking"

	// MovementPolicyPassable maps to today's binary spatial occupancy as
	// non-blocking.
	MovementPolicyPassable MovementPolicy = "passable"
)
