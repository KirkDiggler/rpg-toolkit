// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat

// DefaultMeleeReach is the default melee reach for most combatants in grid units.
// In D&D 5e with 5ft squares, this is 1 unit (5 feet).
// Reach weapons extend this to 2 units (10 feet).
//
// It outlived movement.go, which rpg-project#319 Phase 6 deleted around it.
// The reason it is not dead with the rest is
// conditions.OpportunityAttackCondition.reach, which names it as a
// compile-time witness that its own local constant still agrees with this one
// — the conditions package cannot import a value it needs without an import
// cycle, so it asserts the agreement instead. That witness is worth more than
// it looks: rpg-toolkit#1255 shipped a reach constant that disagreed with the
// one a neighbouring condition's comment claimed to share, and nothing caught
// it because every value involved was plausible.
const DefaultMeleeReach = 1.0
