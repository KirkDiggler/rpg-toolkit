// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// HeldBonusInput is one condition a sheet holds, asked what it adds to one of
// the sheet's numbers.
type HeldBonusInput struct {
	Condition dnd5eEvents.ConditionBehavior
}

// HeldBonusOutput is the whole number the condition adds while held: the sum
// of its current parts of the asked kind. Zero means it adds nothing.
type HeldBonusOutput struct {
	Amount int
}

// HitPointMaximumBonus answers how much one held condition raises its
// holder's hit point maximum. The sheet sums this over its conditions each
// time it reads its maximum; nothing is cached, so a removed condition is
// simply no longer counted.
//
// Errors: a nil input, a nil condition or a nil ref (CodeInvalidArgument), an
// unregistered ref (wrapping [ErrUnknownCondition]), and a part whose amount
// is not a fixed positive whole number.
func HitPointMaximumBonus(in *HeldBonusInput) (*HeldBonusOutput, error) {
	return heldBonus(in, "hit point maximum bonus", dndfx.HitPointMaximumOf)
}

// InitiativeBonus answers how much one held condition adds to its holder's
// initiative. Errors as [HitPointMaximumBonus].
func InitiativeBonus(in *HeldBonusInput) (*HeldBonusOutput, error) {
	return heldBonus(in, "initiative bonus", dndfx.InitiativeBonusOf)
}

func heldBonus(in *HeldBonusInput, noun string, read func(*dndfx.HeldBonusInput) (*dndfx.HeldBonusOutput, error)) (*HeldBonusOutput, error) {
	if in == nil {
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "%s requires input", noun)
	}
	if in.Condition == nil || in.Condition.Ref() == nil {
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "%s requires a condition with a ref", noun)
	}
	if _, err := lookup(in.Condition.Ref().String()); err != nil {
		return nil, err
	}
	var outputs []*generic.Bound
	switch held := in.Condition.(type) {
	case *LinkedCondition:
		for _, part := range held.assembly.CurrentOutputs() {
			outputs = append(outputs, part.Output)
		}
	case *ComposedCondition:
		outputs = held.runtime.CurrentOutputs()
	}
	out := &HeldBonusOutput{}
	for _, output := range outputs {
		answer, err := read(&dndfx.HeldBonusInput{Output: output})
		if err != nil {
			return nil, err
		}
		out.Amount += answer.Amount
	}
	return out, nil
}
