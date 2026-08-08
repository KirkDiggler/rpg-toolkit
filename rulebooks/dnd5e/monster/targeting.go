// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster

import "fmt"

// TargetingStrategy defines how a monster selects targets from available enemies
type TargetingStrategy int

// Targeting strategy constants
const (
	// TargetClosest selects the nearest enemy (default behavior)
	TargetClosest TargetingStrategy = iota
	// TargetLowestHP focuses fire on wounded enemies
	TargetLowestHP
	// TargetLowestAC attacks the enemy with lowest armor class
	TargetLowestAC
)

// Author-facing targeting strategy labels — the vocabulary ParseTargetingStrategy
// accepts and String() returns. "lowest-health" (not "lowest-hp") is deliberate:
// it matches the authored dungeonspec YAML vocabulary. The decision-rationale
// ref segment (see Ref) is spelled differently ("lowest-hp") to match the
// ActionResolvedEvent.TargetRationale vocabulary from rpg-toolkit#895 — the two
// are not interchangeable.
const (
	targetingLabelClosest      = "closest"
	targetingLabelLowestHealth = "lowest-health"
	targetingLabelLowestAC     = "lowest-ac"
)

// SetTargeting sets the monster's targeting strategy
func (m *Monster) SetTargeting(strategy TargetingStrategy) {
	m.targeting = strategy
}

// Targeting returns the monster's current targeting strategy
func (m *Monster) Targeting() TargetingStrategy {
	return m.targeting
}

// ParseTargetingStrategy parses the author-facing targeting vocabulary
// ("closest" | "lowest-health" | "lowest-ac") into a TargetingStrategy. Any
// other value, including the empty string, is rejected — callers that want
// TargetClosest as a default must not call this on an absent/omitted value.
func ParseTargetingStrategy(s string) (TargetingStrategy, error) {
	switch s {
	case targetingLabelClosest:
		return TargetClosest, nil
	case targetingLabelLowestHealth:
		return TargetLowestHP, nil
	case targetingLabelLowestAC:
		return TargetLowestAC, nil
	default:
		return 0, fmt.Errorf("invalid targeting strategy %q (must be %q, %q, or %q)",
			s, targetingLabelClosest, targetingLabelLowestHealth, targetingLabelLowestAC)
	}
}

// String returns the author-facing label for the strategy ("closest" |
// "lowest-health" | "lowest-ac"), the inverse of ParseTargetingStrategy. An
// unrecognized value (never produced by this package) falls back to
// "closest" rather than panicking or returning garbage.
func (s TargetingStrategy) String() string {
	switch s {
	case TargetLowestHP:
		return targetingLabelLowestHealth
	case TargetLowestAC:
		return targetingLabelLowestAC
	case TargetClosest:
		fallthrough
	default:
		return targetingLabelClosest
	}
}

// Ref returns the canonical decision-rationale ref for this targeting
// strategy (rpg-toolkit#895), threaded into
// events.ActionResolvedEvent.TargetRationale on the NPC attack path. Note the
// "lowest-hp" segment here intentionally differs from String()'s
// "lowest-health" author-facing label — see the package-level doc comment.
func (s TargetingStrategy) Ref() string {
	switch s {
	case TargetLowestHP:
		return "dnd5e:targeting:lowest-hp"
	case TargetLowestAC:
		return "dnd5e:targeting:lowest-ac"
	case TargetClosest:
		fallthrough
	default:
		return "dnd5e:targeting:closest"
	}
}
