// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/weaponattack"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// bearing is how a condition type bears on one kind of action: its holder's
// own action, or an attack against its holder. The zero value does not bear.
type bearing struct {
	kind          bearingKind
	participation contributions.Participation
}

// bearingKind says whether a bearing yields a row and whether that row can
// answer.
type bearingKind uint8

const (
	// bearingNone yields no row.
	bearingNone bearingKind = iota
	// bearingAnswers answers through a rule.
	bearingAnswers
	// bearingNotYetAnswering bears but cannot yet answer: it is shown as an
	// unavailable row, never dropped and never shown as not applying.
	bearingNotYetAnswering
)

var (
	// notBearing does not bear on the action. It yields no row.
	notBearing = bearing{}
	// answersNow answers through a rule and joins the action's own
	// calculation.
	answersNow = bearing{kind: bearingAnswers, participation: contributions.ContributesNow}
	// answersLater answers through a rule and is offered later; it is never
	// shown as already applied.
	answersLater = bearing{kind: bearingAnswers, participation: contributions.LaterChoice}
	// notYetAnswering bears on the action but its rule cannot yet answer.
	notYetAnswering = bearing{kind: bearingNotYetAnswering, participation: contributions.ContributesNow}
)

// descriptor is the one entry for one condition type. It is declared beside
// the type, listed in [descriptors], and read through the registry: no other
// table keyed by a condition ref exists.
type descriptor struct {
	// ref is the one ref the type names itself by: what Ref returns, what
	// ToJSON writes, and the registry key (ref.String()).
	ref *core.Ref

	// display is the type's status-visible name and detail. Name is never
	// empty. Detail is required when either bearing is not notBearing.
	//
	// A Detail is read by more than its holder: the holder's status view shows
	// it, and so does an attacker's target candidate when the target holds it
	// (R18). So an effect that bears on attacks against its holder is
	// described in reader-neutral third person ("A prone creature's
	// attacks…"), never in the holder's "you".
	display Display

	// load restores a stored blob. Required.
	load func(json.RawMessage) (dnd5eEvents.ConditionBehavior, error)

	// create builds the type from a ref, its config and its holder. Nil for a
	// type nothing builds from a ref: a free reaction, a stat-block trait,
	// a condition its own feature constructs.
	create func(*CreateFromRefInput) (dnd5eEvents.ConditionBehavior, error)

	// ownAction is the bearing on the holder's own action.
	ownAction bearing

	// againstHolder is the bearing on an attack against the holder.
	againstHolder bearing

	// heldRule builds the rule that answers for an attack against the holder.
	// Present exactly when againstHolder answers.
	heldRule heldRuleFor

	// attackOverride is the type's die-or-ability offer at attack assembly.
	// Nil for a type that makes none.
	attackOverride func(dnd5eEvents.ConditionBehavior, *weaponattack.OverrideInput) (*weaponattack.OverrideOutput, error)
}

// loaderOf is the one load body: a zero value of T reads the blob. A failure
// is wrapped with the type's ref so the caller can tell which load refused.
func loaderOf[T any, P interface {
	*T
	dnd5eEvents.ConditionBehavior
	loadJSON(json.RawMessage) error
}]() func(json.RawMessage) (dnd5eEvents.ConditionBehavior, error) {
	return func(data json.RawMessage) (dnd5eEvents.ConditionBehavior, error) {
		condition := P(new(T))
		if err := condition.loadJSON(data); err != nil {
			return nil, rpgerr.Wrapf(err, "failed to load %s", condition.Ref().String())
		}
		return condition, nil
	}
}
