// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat

// CombatantKind distinguishes the two sheet kinds whose zero-hit-point states
// have different meanings in the current rulebook.
type CombatantKind string

const (
	// CombatantKindCharacter is a player character, unconscious rather than
	// defeated at zero hit points in the current death rules.
	CombatantKindCharacter CombatantKind = "character"

	// CombatantKindMonster is a monster, defeated at zero hit points when
	// [IsDown] reports it down.
	CombatantKindMonster CombatantKind = "monster"
)

// CanBeAttackTarget reports whether a combatant remains a creature an Attack
// can target. A down character is unconscious and remains targetable; a down
// monster is defeated and does not. Unknown kinds fail closed.
//
// down must be the current [IsDown] answer rather than a hit-point comparison,
// so exceptions such as Undead Fortitude can evolve there without changing
// this rule or its callers.
func CanBeAttackTarget(kind CombatantKind, down bool) bool {
	switch kind {
	case CombatantKindCharacter:
		return true
	case CombatantKindMonster:
		return !down
	default:
		return false
	}
}
