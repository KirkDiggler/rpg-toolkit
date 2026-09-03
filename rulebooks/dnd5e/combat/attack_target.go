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
// can target. It is the compatibility surface for callers that can express
// only kind and the current [IsDown] answer; richer callers should use
// [ClassifyLifeState] and [ParticipationFor].
func CanBeAttackTarget(kind CombatantKind, down bool) bool {
	state := ClassifyLifeState(LifeStateInput{Kind: kind, Down: down})
	return ParticipationFor(state).AttackTarget
}
