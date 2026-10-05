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
	refs.Conditions.Raging().String():               answersNow,
	refs.Features.SneakAttack().String():            answersNow,
	refs.Conditions.Blessed().String():              answersNow,
	refs.Conditions.Baned().String():                answersNow,
	refs.Conditions.Inspired().String():             answersLater,
	refs.Conditions.Prone().String():                answersNow,
	refs.Conditions.Hidden().String():               answersNow,
	refs.Conditions.Helped().String():               answersNow,
	refs.Conditions.TrueStrike().String():           answersNow,
	refs.Conditions.ViciousMockery().String():       answersNow,
	refs.Conditions.ImprovedCritical().String():     answersNow,
	refs.Conditions.FightingStyleArchery().String(): answersNow,
	refs.Conditions.DivineFavor().String():          answersNow,
	refs.Conditions.BrutalCritical().String():       answersNow,

	// Bear on the holder's attack, but cannot yet answer: each needs a fact
	// the frame does not carry.
	// Which held weapon the attack uses.
	refs.Conditions.Shillelagh().String():  notYetAnswering,
	refs.Conditions.MartialArts().String(): notYetAnswering,
	// The weapon's grip: two-handed, and what the other hand holds.
	refs.Conditions.FightingStyleDueling().String():             notYetAnswering,
	refs.Conditions.FightingStyleGreatWeaponFighting().String(): notYetAnswering,
	// Whether the attack is the two-weapon off-hand attack.
	refs.Conditions.FightingStyleTwoWeaponFighting().String(): notYetAnswering,
	// Whether the attack is an opportunity attack.
	refs.Conditions.RecklessAttack().String(): notYetAnswering,
	// Attacking ends the ward, which resolution decides; no rule here holds
	// that predicate for information to share.
	refs.Conditions.Sanctuary().String(): notYetAnswering,
	// Who can see whom through the fog.
	refs.Conditions.InFog().String(): notYetAnswering,

	// Do not bear on the holder's own attack.
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

// ActionBearingRefs lists, sorted, every loader ref the action census classes
// as answering or not yet answering — the conditions that yield an effect row
// for their holder's own action. It is a read of the census, not a second
// list: a test that must cover every bearing condition reads this rather than
// restating its size.
func ActionBearingRefs() []string {
	bearing := make([]string, 0, len(actionCensus))
	for ref, entry := range actionCensus {
		if entry.class != actionNotBearing {
			bearing = append(bearing, ref)
		}
	}
	slices.Sort(bearing)
	return bearing
}
