// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// actionCensus classifies every conditionLoaders key once, for the acting
// character's own loaded effects. A loader with no entry is an error at
// assessment, and TestEveryConditionLoaderIsClassified keeps the two key sets
// equal.
var actionCensus = map[string]censusEntry{
	// Answer from the frame.
	refs.Conditions.Raging().String():                           answersNow,
	refs.Features.SneakAttack().String():                        answersNow,
	refs.Conditions.Blessed().String():                          answersNow,
	refs.Conditions.Baned().String():                            answersNow,
	refs.Conditions.Inspired().String():                         answersLater,
	refs.Conditions.Prone().String():                            answersNow,
	refs.Conditions.Hidden().String():                           answersNow,
	refs.Conditions.Helped().String():                           answersNow,
	refs.Conditions.TrueStrike().String():                       answersNow,
	refs.Conditions.ViciousMockery().String():                   answersNow,
	refs.Conditions.ImprovedCritical().String():                 answersNow,
	refs.Conditions.FightingStyleArchery().String():             answersNow,
	refs.Conditions.DivineFavor().String():                      answersNow,
	refs.Conditions.BrutalCritical().String():                   answersNow,
	refs.Conditions.Shillelagh().String():                       answersNow,
	refs.Conditions.MartialArts().String():                      answersNow,
	refs.Conditions.FightingStyleDueling().String():             answersNow,
	refs.Conditions.FightingStyleGreatWeaponFighting().String(): answersNow,
	refs.Conditions.FightingStyleTwoWeaponFighting().String():   answersNow,
	refs.Conditions.RecklessAttack().String():                   answersNow,

	// Bear on the holder's attack, but cannot yet answer.
	// Attacking ends the ward, which resolution decides; no rule here holds
	// that predicate for information to share.
	refs.Conditions.Sanctuary().String(): notYetAnswering,

	// Do not bear on the holder's own attack.
	// Owns no rule; fog acts through the general sight rule (R20).
	refs.Conditions.InFog().String():                   notBearing,
	refs.Conditions.UnarmoredDefense().String():        notBearing,
	refs.Conditions.FightingStyleDefense().String():    notBearing,
	refs.Conditions.FightingStyleProtection().String(): notBearing,
	refs.Conditions.UnarmoredMovement().String():       notBearing,
	refs.Conditions.Disengaging().String():             notBearing,
	refs.Conditions.Dodging().String():                 notBearing,
	refs.Conditions.Unconscious().String():             notBearing,
	refs.Conditions.OpportunityAttack().String():       notBearing,
	refs.Conditions.BladeWard().String():               notBearing,
	refs.Conditions.Commanded().String():               notBearing,
	refs.Conditions.Concentrating().String():           notBearing,
	refs.Conditions.FaerieFire().String():              notBearing,
	refs.Conditions.ShieldOfFaith().String():           notBearing,
	refs.Conditions.Guided().String():                  notBearing,
	refs.Conditions.Resistance().String():              notBearing,
	refs.Conditions.GuidingBolt().String():             notBearing,
	refs.Spells.Shield().String():                      notBearing,
	// Only bars receiving another Sanctuary.
	refs.Conditions.SanctuaryImmune().String(): notBearing,
}

// ActionBearingRefs lists, sorted, every loader ref the action census classes
// as answering or not yet answering — the conditions that yield an effect row
// for their holder's own action. It is a read of the census, not a second
// list: a test that must cover every bearing condition reads this rather than
// restating its size.
func ActionBearingRefs() []string {
	return bearingRefs(actionCensus)
}
