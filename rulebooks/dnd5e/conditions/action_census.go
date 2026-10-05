// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// actionClass says how one loaded effect bears on its holder's own action.
type actionClass string

const (
	// actionAnswers marks an effect whose rule answers through
	// contributions.ActionAssessor.
	actionAnswers actionClass = "answers"
	// actionNotBearing marks an effect that does not bear on its holder's own
	// action. It yields no row.
	actionNotBearing actionClass = "not_bearing"
	// actionNotYetAnswering marks an effect that bears on the action but whose
	// rule cannot yet answer. It is shown unavailable, never dropped and never
	// shown as not applying.
	actionNotYetAnswering actionClass = "not_yet_answering"
)

// actionCensusEntry classifies one loader ref. Participation is empty for an
// effect that does not bear.
type actionCensusEntry struct {
	class         actionClass
	participation contributions.Participation
}

var (
	answersNow       = actionCensusEntry{class: actionAnswers, participation: contributions.ContributesNow}
	answersLater     = actionCensusEntry{class: actionAnswers, participation: contributions.LaterChoice}
	notYetAnswering  = actionCensusEntry{class: actionNotYetAnswering, participation: contributions.ContributesNow}
	notBearingAction = actionCensusEntry{class: actionNotBearing}
)

// actionCensus classifies every conditionLoaders key once, for the acting
// character's own loaded effects. A loader with no entry is an error at
// assessment, and TestEveryConditionLoaderIsClassified keeps the two key sets
// equal.
var actionCensus = map[string]actionCensusEntry{
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
	refs.Conditions.InFog().String():                   notBearingAction,
	refs.Conditions.UnarmoredDefense().String():        notBearingAction,
	refs.Conditions.FightingStyleDefense().String():    notBearingAction,
	refs.Conditions.FightingStyleProtection().String(): notBearingAction,
	refs.Conditions.UnarmoredMovement().String():       notBearingAction,
	refs.Conditions.Disengaging().String():             notBearingAction,
	refs.Conditions.Dodging().String():                 notBearingAction,
	refs.Conditions.Unconscious().String():             notBearingAction,
	refs.Conditions.OpportunityAttack().String():       notBearingAction,
	refs.Conditions.BladeWard().String():               notBearingAction,
	refs.Conditions.Commanded().String():               notBearingAction,
	refs.Conditions.Concentrating().String():           notBearingAction,
	refs.Conditions.FaerieFire().String():              notBearingAction,
	refs.Conditions.ShieldOfFaith().String():           notBearingAction,
	refs.Conditions.Guided().String():                  notBearingAction,
	refs.Conditions.Resistance().String():              notBearingAction,
	refs.Conditions.GuidingBolt().String():             notBearingAction,
	refs.Spells.Shield().String():                      notBearingAction,
	// Only bars receiving another Sanctuary.
	refs.Conditions.SanctuaryImmune().String(): notBearingAction,
}

// targetCensus classifies every conditionLoaders key once more, for a
// condition the TARGET of an attack holds: whether it bears on an attack
// against its holder. An answering entry has a rule in targetHeldRules, keyed
// by the same ref. TestEveryConditionLoaderIsClassifiedForTargetHeld keeps the
// key sets equal.
var targetCensus = map[string]actionCensusEntry{
	// Answer from the frame, through a rule keyed by reference.
	refs.Conditions.FaerieFire().String():  answersNow,
	refs.Conditions.GuidingBolt().String(): answersNow,
	refs.Conditions.Dodging().String():     answersNow,
	refs.Conditions.Prone().String():       answersNow,
	refs.Conditions.Sanctuary().String():   answersNow,
	refs.Conditions.Hidden().String():      answersNow,

	// Bear on an attack against the holder, but cannot yet answer.
	// Attacks against a reckless holder have advantage; flagged to follow up.
	refs.Conditions.RecklessAttack().String(): notYetAnswering,
	// Resistance as the defender, through the damage chain.
	refs.Conditions.Raging().String():    notYetAnswering,
	refs.Conditions.BladeWard().String(): notYetAnswering,
	// Armour class, already folded into the target's AC.
	refs.Conditions.ShieldOfFaith().String():        notYetAnswering,
	refs.Conditions.UnarmoredDefense().String():     notYetAnswering,
	refs.Conditions.FightingStyleDefense().String(): notYetAnswering,

	// Do not bear on an attack against the holder.
	// The Shield reaction bears only once its holder chooses it after the
	// roll, and the status catalogue deliberately holds no description for
	// it (TestDisplayCatalogExcludesShieldSpell), so it shows no row here.
	refs.Spells.Shield().String(): notBearingAction,
	// Owns no rule; fog acts through the general sight rule (R20).
	refs.Conditions.InFog().String(): notBearingAction,
	// No handler acts on an attack against an unconscious holder.
	refs.Conditions.Unconscious().String():                      notBearingAction,
	refs.Features.SneakAttack().String():                        notBearingAction,
	refs.Conditions.Blessed().String():                          notBearingAction,
	refs.Conditions.Baned().String():                            notBearingAction,
	refs.Conditions.Inspired().String():                         notBearingAction,
	refs.Conditions.Helped().String():                           notBearingAction,
	refs.Conditions.TrueStrike().String():                       notBearingAction,
	refs.Conditions.ViciousMockery().String():                   notBearingAction,
	refs.Conditions.ImprovedCritical().String():                 notBearingAction,
	refs.Conditions.FightingStyleArchery().String():             notBearingAction,
	refs.Conditions.DivineFavor().String():                      notBearingAction,
	refs.Conditions.BrutalCritical().String():                   notBearingAction,
	refs.Conditions.Shillelagh().String():                       notBearingAction,
	refs.Conditions.MartialArts().String():                      notBearingAction,
	refs.Conditions.FightingStyleDueling().String():             notBearingAction,
	refs.Conditions.FightingStyleGreatWeaponFighting().String(): notBearingAction,
	refs.Conditions.FightingStyleTwoWeaponFighting().String():   notBearingAction,
	refs.Conditions.FightingStyleProtection().String():          notBearingAction,
	refs.Conditions.UnarmoredMovement().String():                notBearingAction,
	refs.Conditions.Disengaging().String():                      notBearingAction,
	refs.Conditions.OpportunityAttack().String():                notBearingAction,
	refs.Conditions.Commanded().String():                        notBearingAction,
	refs.Conditions.Concentrating().String():                    notBearingAction,
	refs.Conditions.Guided().String():                           notBearingAction,
	refs.Conditions.Resistance().String():                       notBearingAction,
	refs.Conditions.SanctuaryImmune().String():                  notBearingAction,
}

// TargetBearingRefs lists, sorted, every loader ref the target census classes
// as answering or not yet answering — the conditions that yield a row when the
// target of an attack holds them. It is a read of the census, not a second
// list.
func TargetBearingRefs() []string {
	return bearingRefs(targetCensus)
}

// ActionBearingRefs lists, sorted, every loader ref the action census classes
// as answering or not yet answering — the conditions that yield an effect row
// for their holder's own action. It is a read of the census, not a second
// list: a test that must cover every bearing condition reads this rather than
// restating its size.
func ActionBearingRefs() []string {
	return bearingRefs(actionCensus)
}

// bearingRefs lists, sorted, a census's refs that are not classed as not
// bearing.
func bearingRefs(census map[string]actionCensusEntry) []string {
	bearing := make([]string, 0, len(census))
	for ref, entry := range census {
		if entry.class != actionNotBearing {
			bearing = append(bearing, ref)
		}
	}
	slices.Sort(bearing)
	return bearing
}
