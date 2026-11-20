// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"fmt"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
)

func checkAmount(amount *generic.Amount) error {
	if amount == nil {
		return fmt.Errorf("damage bonus requires an amount")
	}
	name, err := amount.Name()
	if err != nil {
		return err
	}
	switch name {
	case generic.AmountFixed:
		return nil
	case generic.AmountByClassLevel:
		steps := amount.ByClassLevel
		if _, err := classes.GetByID(steps.Class); err != nil {
			return err
		}
		if len(steps.Steps) == 0 {
			return fmt.Errorf("class amount requires level steps")
		}
		for level := range steps.Steps {
			if level < 1 {
				return fmt.Errorf("class step must be positive, got %d", level)
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported amount %q", name)
	}
}

func fixedAmount(_ any, amount generic.Amount) (generic.AmountResult, error) {
	if amount.Fixed == nil {
		return generic.AmountResult{}, fmt.Errorf("fixed amount is missing")
	}
	value := *amount.Fixed
	return generic.AmountResult{Scalar: &value}, nil
}

// checkAbilityModifier refuses an ability-modifier amount naming an ability
// the rulebook does not have.
func checkAbilityModifier(amount *generic.Amount) error {
	if amount == nil || amount.AbilityModifier == nil {
		return fmt.Errorf("ability modifier amount is missing")
	}
	if _, err := abilities.GetByID(amount.AbilityModifier.Ability); err != nil {
		return fmt.Errorf("ability modifier names no ability this rulebook has: %w", err)
	}
	return nil
}

// abilityModifierAmount is the holder's modifier for the named ability, read
// from the holder's scores on the frame; negative is a real answer. A frame
// with no holder, or a holder whose sheet states no score for the ability,
// cannot answer: a missing score is not a score of 0, so it never becomes -5.
func abilityModifierAmount(frame any, amount generic.Amount) (generic.AmountResult, error) {
	if err := checkAbilityModifier(&amount); err != nil {
		return generic.AmountResult{}, err
	}
	q, err := queryFrom(frame)
	if err != nil {
		return generic.AmountResult{}, err
	}
	if q.Holder == nil {
		return generic.AmountResult{}, fmt.Errorf("%w: an ability modifier needs the holder's scores", contributions.ErrRuleCannotAnswer)
	}
	ability := abilities.Ability(amount.AbilityModifier.Ability)
	if _, stated := q.Holder.Scores[ability]; !stated {
		return generic.AmountResult{}, fmt.Errorf("%w: %s states no %s score", contributions.ErrRuleCannotAnswer, q.Holder.MemberID, ability)
	}
	n := q.Holder.Scores.Modifier(ability)
	return generic.AmountResult{Scalar: &n}, nil
}

func classAmount(frame any, amount generic.Amount) (generic.AmountResult, error) {
	q, err := queryFrom(frame)
	if err != nil {
		return generic.AmountResult{}, err
	}
	if q.Action == nil || amount.ByClassLevel == nil {
		return generic.AmountResult{}, fmt.Errorf("%w: class amount requires an action frame", contributions.ErrRuleCannotAnswer)
	}
	steps := amount.ByClassLevel
	level, known := q.Action.ActorClassLevels.Of(classes.Class(steps.Class))
	if !known || level < 1 {
		return generic.AmountResult{}, fmt.Errorf("%w: no %s levels", contributions.ErrRuleCannotAnswer, steps.Class)
	}
	best, value := 0, 0
	for threshold, n := range steps.Steps {
		if threshold <= level && threshold > best {
			best, value = threshold, n
		}
	}
	if best == 0 {
		return generic.AmountResult{}, fmt.Errorf("%w: no class step for level %d", contributions.ErrRuleCannotAnswer, level)
	}
	return generic.AmountResult{Scalar: &value}, nil
}
