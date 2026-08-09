// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package clock

import "github.com/KirkDiggler/rpg-toolkit/core"

// MilestoneKind names a temporal boundary a clock verb crossed.
type MilestoneKind string

// The closed v1 milestone kind set (design: Milestone).
const (
	// TurnStarted marks Subject's turn beginning.
	TurnStarted MilestoneKind = "turn_started"
	// TurnEnded marks Subject's turn ending.
	TurnEnded MilestoneKind = "turn_ended"
	// RoundStarted marks a new round beginning on a Turn clock.
	RoundStarted MilestoneKind = "round_started"
	// Ticked marks a world-clock advance driven by Subject.
	Ticked MilestoneKind = "ticked"
	// MemberJoined marks Subject joining a clock.
	MemberJoined MilestoneKind = "member_joined"
	// MemberLeft marks Subject leaving a clock.
	MemberLeft MilestoneKind = "member_left"
	// Merged marks two Turn clocks combining.
	Merged MilestoneKind = "merged"
	// Dissolved marks a Turn clock emptying at fight end.
	Dissolved MilestoneKind = "dissolved"
)

// Milestone is a temporal boundary returned — never published — by the
// verb that caused it, in causal order (design R4). Turn-emitted
// milestones carry the clock's Round at the moment of emission.
type Milestone struct {
	Kind    MilestoneKind
	Subject core.EntityID // zero when not about a specific entity
	Round   int           // Turn clocks only; zero otherwise
}
