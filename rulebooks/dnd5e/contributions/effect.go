// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package contributions

import "errors"

// ErrRuleCannotAnswer is wrapped by execution when a rule is handed a frame it
// cannot read or answers Depends. The action fails; it is never treated as the
// rule not applying, because a frame missing a fact would otherwise switch the
// rule off silently.
var ErrRuleCannotAnswer = errors.New("rule cannot answer from this frame")

// PrimaryWeaponPool names the marked primary weapon pool, not every damage
// component or the entire attack.
const PrimaryWeaponPool = "weapon:primary"

// DamageChange is an unresolved sourced change on an exact pool. Exactly one of
// Fixed or Dice is present; a fixed zero remains a real sourced amount.
type DamageChange struct {
	PoolID string
	Source Source
	Fixed  *int
	Dice   string
}

// Answer is a rule's whole reply to one frame: its decision, how it takes
// part, and what it contributes as data. Benefit, Damage and Roll are
// non-empty only when the decision is Applies. It holds no total.
type Answer struct {
	Decision      Decision
	Participation Participation
	Benefit       string
	Damage        []DamageChange
	Roll          []DiceContribution
}

// AssessActionInput hands a rule the frame it answers from.
type AssessActionInput struct {
	Frame Frame
}

// AssessActionOutput carries the rule's answer.
type AssessActionOutput struct {
	Answer Answer
}

// ActionAssessor is implemented by a rule that answers from a frame alone. It
// reads nothing but the frame and its own persisted state; it does not query
// the world, roll, spend, publish or change state. A nil input or an invalid
// frame is an error. Execution and information call the same method.
type ActionAssessor interface {
	AssessAction(*AssessActionInput) (*AssessActionOutput, error)
}

// EffectState is how one effect bears on an action, as the player sees it.
type EffectState string

const (
	// StateApplies means the effect's rule applies.
	StateApplies EffectState = "applies"
	// StateDoesNotApply means the rule does not apply, for its stated reason.
	StateDoesNotApply EffectState = "does_not_apply"
	// StateDepends means the rule needs a fact not yet known.
	StateDepends EffectState = "depends"
	// StateUnavailable means the effect's rule cannot yet answer at all. It is
	// never shown as not applying.
	StateUnavailable EffectState = "unavailable"
)

// Effect is one row of effect information for an action: which effect, what
// it does, whether it applies here and why. ID is unique within one listing.
// Benefit is empty unless the state is StateApplies. Rows never grant or
// refuse an action and are never folded into a total.
type Effect struct {
	ID            string
	Source        Source
	Description   string
	State         EffectState
	Reason        string
	Participation Participation
	Benefit       string
}
