// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// targetCensus classifies every conditionLoaders key once more, for a
// condition the TARGET of an attack holds: whether it bears on an attack
// against its holder. An answering entry has a rule in targetHeldRules, keyed
// by the same ref. TestEveryConditionLoaderIsClassifiedForTargetHeld keeps the
// key sets equal.
var targetCensus = map[string]censusEntry{
	// Answer from the frame, through a rule keyed by reference.
	refs.Conditions.FaerieFire().String():     answersNow,
	refs.Conditions.GuidingBolt().String():    answersNow,
	refs.Conditions.Dodging().String():        answersNow,
	refs.Conditions.Prone().String():          answersNow,
	refs.Conditions.Sanctuary().String():      answersNow,
	refs.Conditions.Hidden().String():         answersNow,
	refs.Conditions.RecklessAttack().String(): answersNow,

	// Do not bear on an attack against the holder.
	// A target's armour class and resistances are not the attacker's to know
	// (R21): an effect whose only bearing on the attack is the target's AC or
	// its resistance produces no row for the attacker.
	refs.Conditions.Raging().String():               notBearing,
	refs.Conditions.BladeWard().String():            notBearing,
	refs.Conditions.ShieldOfFaith().String():        notBearing,
	refs.Conditions.UnarmoredDefense().String():     notBearing,
	refs.Conditions.FightingStyleDefense().String(): notBearing,
	refs.Spells.Shield().String():                   notBearing,

	// Owns no rule; fog acts through the general sight rule (R20).
	refs.Conditions.InFog().String(): notBearing,
	// No handler acts on an attack against an unconscious holder.
	refs.Conditions.Unconscious().String():                      notBearing,
	refs.Features.SneakAttack().String():                        notBearing,
	refs.Conditions.Blessed().String():                          notBearing,
	refs.Conditions.Baned().String():                            notBearing,
	refs.Conditions.Inspired().String():                         notBearing,
	refs.Conditions.Helped().String():                           notBearing,
	refs.Conditions.TrueStrike().String():                       notBearing,
	refs.Conditions.ViciousMockery().String():                   notBearing,
	refs.Conditions.ImprovedCritical().String():                 notBearing,
	refs.Conditions.FightingStyleArchery().String():             notBearing,
	refs.Conditions.DivineFavor().String():                      notBearing,
	refs.Conditions.BrutalCritical().String():                   notBearing,
	refs.Conditions.Shillelagh().String():                       notBearing,
	refs.Conditions.MartialArts().String():                      notBearing,
	refs.Conditions.FightingStyleDueling().String():             notBearing,
	refs.Conditions.FightingStyleGreatWeaponFighting().String(): notBearing,
	refs.Conditions.FightingStyleTwoWeaponFighting().String():   notBearing,
	refs.Conditions.FightingStyleProtection().String():          notBearing,
	refs.Conditions.UnarmoredMovement().String():                notBearing,
	refs.Conditions.Disengaging().String():                      notBearing,
	refs.Conditions.OpportunityAttack().String():                notBearing,
	refs.Conditions.Commanded().String():                        notBearing,
	refs.Conditions.Concentrating().String():                    notBearing,
	refs.Conditions.Guided().String():                           notBearing,
	refs.Conditions.Resistance().String():                       notBearing,
	refs.Conditions.SanctuaryImmune().String():                  notBearing,
}

// TargetBearingRefs lists, sorted, every loader ref the target census classes
// as answering or not yet answering — the conditions that yield a row when the
// target of an attack holds them. It is a read of the census, not a second
// list.
func TargetBearingRefs() []string {
	return bearingRefs(targetCensus)
}
