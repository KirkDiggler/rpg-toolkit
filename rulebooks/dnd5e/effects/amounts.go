// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"fmt"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
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

func fixedAmount(_ any, amount generic.Amount) (int, error) {
	if amount.Fixed == nil {
		return 0, fmt.Errorf("fixed amount is missing")
	}
	return *amount.Fixed, nil
}

func classAmount(frame any, amount generic.Amount) (int, error) {
	q, err := queryFrom(frame)
	if err != nil {
		return 0, err
	}
	if q.Action == nil || amount.ByClassLevel == nil {
		return 0, fmt.Errorf("%w: class amount requires an action frame", contributions.ErrRuleCannotAnswer)
	}
	steps := amount.ByClassLevel
	level, known := q.Action.ActorClassLevels.Of(classes.Class(steps.Class))
	if !known || level < 1 {
		return 0, fmt.Errorf("%w: no %s levels", contributions.ErrRuleCannotAnswer, steps.Class)
	}
	best, value := 0, 0
	for threshold, n := range steps.Steps {
		if threshold <= level && threshold > best {
			best, value = threshold, n
		}
	}
	if best == 0 {
		return 0, fmt.Errorf("%w: no class step for level %d", contributions.ErrRuleCannotAnswer, level)
	}
	return value, nil
}
