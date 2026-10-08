// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// FeetPerCell is the grid's fixed real-world scale: every cell on every grid
// [tools/spatial] exposes — square (Chebyshev), hex (cube), or gridless — is
// 5 feet on a side (Kirk, rpg-project#254 review). This has always been true
// of this codebase's data (a plain melee weapon's reach, a monster's walking
// speed, a character's darkvision) and used to be re-derived, inconsistently
// worded, at three separate call sites (conditions' proneReachCells,
// FightingStyleProtectionCondition, SneakAttackCondition) rather than named
// once. This constant, and [CellsFromFeet] beside it, are that one place.
const FeetPerCell = 5

// CellsFromFeet converts a feet-denominated distance to cells, the unit
// [Encounter.Distance] and every grid Distance call answer in.
//
// THE ONE PLACE THIS CONVERSION HAPPENS (Kirk, rpg-project#254 review):
// "convert feet->cells once, at the point of comparison, in one exported
// helper" — not at every producer of a feet-denominated fact guessing at a
// conversion. session's reach gate and this module's own monster-turn
// movement budget are the two call sites that need it; both compare a
// feet-denominated authored fact (an action's reach, a member's speed)
// against a cell-denominated grid Distance, and both call this rather than
// re-deriving FeetPerCell locally.
//
// Rounds down (integer division): 5e's own stat blocks author reach and
// speed as exact multiples of 5, so this never actually truncates a real
// value — the floor exists so a caller handed something else (a partial
// remainder of movement budget, say) gets the conservative answer rather
// than a cell it cannot fully afford.
func CellsFromFeet(feet int) int {
	return feet / FeetPerCell
}

// SecondsPerRound is the world clock's real-world scale: one unit of the
// world clock is one round, and a round is six seconds (PHB p. 189). Every
// advance in worldtime.go is in rounds; this is what a round is worth.
const SecondsPerRound = 6

// RoundsPerHour is an hour on the world clock, in the rounds it counts —
// THE ONE PLACE an hour becomes rounds, for [FeetPerCell]'s reason. A short
// rest advances the world clock by exactly this ([Encounter.RecordRest]); a
// caller that needs an hour asks for it by kind and never counts rounds
// itself.
const RoundsPerHour = 60 * 60 / SecondsPerRound
