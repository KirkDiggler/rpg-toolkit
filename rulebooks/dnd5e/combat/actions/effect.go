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
// knows.
const EffectSelf EffectTarget = "self"

// EffectProfile is the body of an action that does something to someone
// without a weapon and without a spell: what a description of an ability
// that is neither an attack nor a cast says it does.
type EffectProfile struct {
	// To is whom it lands on. EffectSelf, the one who uses it, is the only
	// value this version knows.
	To EffectTarget `json:"to"`
	// Healing is an immediate hit point consequence, the declaration a
	// healing spell carries. The only thing an effect can do in this version.
	Healing *healing.Declaration `json:"healing,omitempty"`
}

// Validate reports whether the effect lands on someone this version knows and
// does exactly one valid thing.
func (p EffectProfile) Validate() error {
	if p.To != EffectSelf {
		return fmt.Errorf("effect target %q is unknown; the only target is %q", p.To, EffectSelf)
	}
	if p.Healing == nil {
		return fmt.Errorf("effect must do exactly one thing: healing is absent")
	}
	if err := p.Healing.Validate(); err != nil {
		return fmt.Errorf("effect healing is invalid: %w", err)
	}
	return nil
}

// Clone returns a deep copy whose healing declaration does not alias the
// original's.
func (p EffectProfile) Clone() EffectProfile {
	clone := p
	if p.Healing != nil {
		declaration := p.Healing.Clone()
		clone.Healing = &declaration
	}
	return clone
}
