// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster

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

// SetTargeting sets the monster's targeting strategy
func (m *Monster) SetTargeting(strategy TargetingStrategy) {
	m.targeting = strategy
}

// Targeting returns the monster's current targeting strategy
func (m *Monster) Targeting() TargetingStrategy {
	return m.targeting
}
