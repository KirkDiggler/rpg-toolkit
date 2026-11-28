// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/healing"
)

// EffectTarget names whom an effect lands on.
type EffectTarget string

// EffectSelf is the one who uses the action, the only target this version
// knows. What lands on that user is a heal or a named status.
const EffectSelf EffectTarget = "self"

// EffectProfile is the body of an action that does something to its user
// without a weapon and without a spell: what a description of an ability that
// is neither an attack nor a cast says it does. It does exactly one thing, a
// heal (Healing) or a named status (Condition); nothing here runs either.
type EffectProfile struct {
	// To is whom it lands on. EffectSelf, the one who uses it, is the only
	// value this version knows.
	To EffectTarget `json:"to"`
	// Healing is an immediate hit point consequence, the declaration a
	// healing spell carries. One of the two things an effect can do.
	Healing *healing.Declaration `json:"healing,omitempty"`
	// Condition is a named status put on the user: built from its ref and
	// values where it is run, never here. Rage is the first: the raging
	// condition, with its bonus read from the barbarian's level.
	Condition *ConditionApplication `json:"condition,omitempty"`
}

// Validate reports whether the effect lands on someone this version knows and
// does exactly one valid thing: a heal or a status on its user.
func (p EffectProfile) Validate() error {
	if p.To != EffectSelf {
		return fmt.Errorf("effect target %q is unknown; the only target is %q", p.To, EffectSelf)
	}
	switch {
	case p.Healing == nil && p.Condition == nil:
		return fmt.Errorf("effect must do exactly one thing: a heal or a status, and it has neither")
	case p.Healing != nil && p.Condition != nil:
		return fmt.Errorf("effect must do exactly one thing: it has both a heal and a status")
	}
	if p.Healing != nil {
		if err := p.Healing.Validate(); err != nil {
			return fmt.Errorf("effect healing is invalid: %w", err)
		}
		return nil
	}
	// The status lands on the user, so there is no one to save and no one
	// else to bind into it.
	if p.Condition.Save != nil {
		return fmt.Errorf("effect status is invalid: an effect on its user is not saved against")
	}
	if p.Condition.CounterpartKey != "" {
		return fmt.Errorf("effect status is invalid: an effect on its user names no counterpart")
	}
	if err := p.Condition.Validate(); err != nil {
		return fmt.Errorf("effect status is invalid: %w", err)
	}
	return nil
}

// Clone returns a deep copy whose healing declaration and status do not alias
// the original's.
func (p EffectProfile) Clone() EffectProfile {
	clone := p
	if p.Healing != nil {
		declaration := p.Healing.Clone()
		clone.Healing = &declaration
	}
	if p.Condition != nil {
		condition := p.Condition.Clone()
		clone.Condition = &condition
	}
	return clone
}
