// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// IntentForAnswerInput pairs an already-selected answer with the selecting
// actor's view. The view supplies budgets and observer-local targets, not another
// table to roll. Answer must have passed its source/selection validation.
type IntentForAnswerInput struct {
	View   MonsterView
	Answer Answer
}

// IntentForAnswerOutput is the existing execution intent for that answer. It
// reports neither execution success nor arrival; only the executor knows those.
type IntentForAnswerOutput struct {
	Intent TurnIntent
}

// IntentForAnswer translates one chosen answer without selecting or rolling.
// Selectors resolve only against View, and movement keeps the ordinary budget,
// cause and targeting semantics. Nil input returns ErrNilInput; a row without a
// supported action or spoken line wraps ErrBadAnswer. No world state is changed.
func IntentForAnswer(in *IntentForAnswerInput) (*IntentForAnswerOutput, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	intent, err := (TableDriver{}).intentFor(in.View, in.Answer)
	if err != nil {
		return nil, err
	}
	return &IntentForAnswerOutput{Intent: intent}, nil
}
