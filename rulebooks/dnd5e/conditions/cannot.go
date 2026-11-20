// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// TakenAwayInput is one condition a sheet holds, asked what it takes away
// from its holder.
type TakenAwayInput struct {
	Condition dnd5eEvents.ConditionBehavior
}

// TakenAwayOutput is everything the condition takes away while held, each
// once, in the order its parts list it. Empty means it takes nothing away.
type TakenAwayOutput struct {
	Taken []dndfx.Taken
}

// TakenAway answers what one held condition takes away from its holder: the
// union of its current cannot parts. Nothing is cached; a removed condition is
// simply no longer asked.
//
// Errors: a nil input, a nil condition or a nil ref (CodeInvalidArgument), an
// unregistered ref (wrapping [ErrUnknownCondition]), and a cannot part whose
// list is empty, unknown or repeated.
func TakenAway(in *TakenAwayInput) (*TakenAwayOutput, error) {
	if in == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "taken away requires input")
	}
	if in.Condition == nil || in.Condition.Ref() == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "taken away requires a condition with a ref")
	}
	if _, err := lookup(in.Condition.Ref().String()); err != nil {
		return nil, err
	}
	_, outputs := heldOutputs(in.Condition)
	out := &TakenAwayOutput{}
	for _, output := range outputs {
		answer, err := dndfx.CannotOf(&dndfx.CannotInput{Output: output})
		if err != nil {
			return nil, err
		}
		for _, what := range answer.Taken {
			if !slices.Contains(out.Taken, what) {
				out.Taken = append(out.Taken, what)
			}
		}
	}
	return out, nil
}

// CannotMoveInput is a creature's held conditions.
type CannotMoveInput struct {
	Conditions []dnd5eEvents.ConditionBehavior
}

// CannotMoveOutput says whether the creature may not move by its own will,
// and what forbids it.
type CannotMoveOutput struct {
	// CannotMove is true when any held condition takes moving away.
	CannotMove bool
	// Sources are the names of the held conditions that take it away, in the
	// order the sheet holds them. Empty when the creature may move.
	Sources []string
}

// CannotMove is the ONE question asked of a creature for whether it may move
// by its own will: every held condition that takes moving away, by name. A
// creature holding nothing may move. It is worked out from the held
// conditions each time it is asked; nothing is stored but the conditions.
//
// It says nothing about moves made ON the creature (a push, a carry): those
// are not its own and are not taken.
//
// Errors: a nil input, or whatever [TakenAway] refuses for a held condition.
func CannotMove(in *CannotMoveInput) (*CannotMoveOutput, error) {
	if in == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "cannot move requires input")
	}
	out := &CannotMoveOutput{}
	for _, held := range in.Conditions {
		taken, err := TakenAway(&TakenAwayInput{Condition: held})
		if err != nil {
			return nil, err
		}
		if slices.Contains(taken.Taken, dndfx.TakenMove) {
			name, _ := heldOutputs(held)
			out.CannotMove = true
			out.Sources = append(out.Sources, name)
		}
	}
	return out, nil
}
