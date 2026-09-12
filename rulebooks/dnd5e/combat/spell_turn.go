// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat

import (
	"errors"
	"fmt"
	"strings"
)

// SpellCastingTime describes the casting time, independently of a spell's
// price. A free spell still has a casting time and obeys the same-turn rule.
type SpellCastingTime string

const (
	// SpellCastingAction is a casting time of one action.
	SpellCastingAction SpellCastingTime = "action"
	// SpellCastingBonusAction is a casting time of one bonus action.
	SpellCastingBonusAction SpellCastingTime = "bonus_action"
	// SpellCastingReaction is a casting time of one reaction.
	SpellCastingReaction SpellCastingTime = "reaction"
	// SpellCastingOther covers casting times that are not one action, bonus
	// action, or reaction. These do not qualify for the cantrip exception.
	SpellCastingOther SpellCastingTime = "other"
)

// SpellCasting declares the facts used by the 2014 bonus-action spell rule.
// Level is the spell's level (zero for a cantrip), not the slot used to pay.
type SpellCasting struct {
	Level int              `json:"level"`
	Time  SpellCastingTime `json:"time"`
}

// Validate refuses unknown casting times and spell levels outside zero to nine.
func (c SpellCasting) Validate() error {
	if c.Level < 0 || c.Level > 9 {
		return fmt.Errorf("spell level must be between 0 and 9")
	}
	switch c.Time {
	case SpellCastingAction, SpellCastingBonusAction, SpellCastingReaction, SpellCastingOther:
		return nil
	default:
		return fmt.Errorf("unknown spell casting time %q", c.Time)
	}
}

// ErrBonusActionSpell is a conflict with the 2014 same-turn casting rule.
var ErrBonusActionSpell = errors.New("a bonus-action spell allows only other one-action cantrips on the same turn")

// SpellTurnState is one caster's serializable casting history for one turn.
// Turn is an opaque identity supplied by the composition. It must change at
// EVERY creature's turn, including across encounters; a round number or the
// caster's action-economy refill number is not sufficient. Queries never reset
// action slots or reactions. The zero value represents no recorded casts.
type SpellTurnState struct {
	Turn             string `json:"turn,omitempty"`
	BonusActionSpell bool   `json:"bonus_action_spell,omitempty"`
	OtherSpell       bool   `json:"other_spell,omitempty"`
}

// AfterCast returns the history that committing this cast would produce.
// It is pure: callers may use it for offers and preflight, and store the result
// only after successful payment. Refused declarations must not become history.
// A different explicit turn starts fresh history without refilling any resource.
func (s SpellTurnState) AfterCast(turn string, cast SpellCasting) (SpellTurnState, error) {
	if strings.TrimSpace(turn) == "" {
		return s, fmt.Errorf("spell casting requires an explicit turn identity")
	}
	if err := cast.Validate(); err != nil {
		return s, err
	}
	next := s
	if next.Turn != turn {
		next = SpellTurnState{Turn: turn}
	}
	cantripAction := cast.Level == 0 && cast.Time == SpellCastingAction
	bonus := cast.Time == SpellCastingBonusAction
	if (next.BonusActionSpell && !cantripAction) || (bonus && next.OtherSpell) {
		return s, ErrBonusActionSpell
	}
	if bonus {
		next.BonusActionSpell = true
	} else if !cantripAction {
		next.OtherSpell = true
	}
	return next, nil
}
