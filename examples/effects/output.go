// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"encoding/json/jsontext"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/examples/effects/machine"
)

// KindName names an output kind the rulebook supplies in an
// OutputVocabulary.
type KindName string

// AmountName names an amount function; it is the JSON name of the one field
// an Amount sets.
type AmountName string

const (
	// AmountFixed names Amount.Fixed.
	AmountFixed AmountName = "fixed"
	// AmountDice names Amount.Dice.
	AmountDice AmountName = "dice"
	// AmountByClassLevel names Amount.ByClassLevel.
	AmountByClassLevel AmountName = "by_class_level"
	// AmountAbilityModifier names Amount.AbilityModifier.
	AmountAbilityModifier AmountName = "ability_modifier"
)

// Output is one contribution a state makes while the machine sits in it:
// the kind that attaches it, an optional guard over the frame, an optional
// amount, and the kind's own params, which the kind validates. A recipe
// carries outputs as opaque JSON on its states; Compose decodes them.
type Output struct {
	When   *machine.Guard `json:"when,omitzero"`
	Amount *Amount        `json:"amount,omitzero"`
	Kind   KindName       `json:"kind"`
	Params jsontext.Value `json:"params,omitzero"`
}

// Amount is a value an output contributes. Exactly one field is set, and
// the OutputVocabulary must hold an AmountFn under its name.
type Amount struct {
	Fixed           *int        `json:"fixed,omitzero"`
	Dice            string      `json:"dice,omitzero"`
	ByClassLevel    *ClassSteps `json:"by_class_level,omitzero"`
	AbilityModifier bool        `json:"ability_modifier,omitzero"`
}

// ClassSteps maps a class level to a value: the highest step at or below
// the level applies ({1: 2, 9: 3, 16: 4}).
type ClassSteps struct {
	Class string      `json:"class"`
	Steps map[int]int `json:"steps"`
}

// Name returns the name of the one field a sets, or an error when it sets
// none or more than one.
func (a *Amount) Name() (AmountName, error) {
	var names []AmountName
	if a.Fixed != nil {
		names = append(names, AmountFixed)
	}
	if a.Dice != "" {
		names = append(names, AmountDice)
	}
	if a.ByClassLevel != nil {
		names = append(names, AmountByClassLevel)
	}
	if a.AbilityModifier {
		names = append(names, AmountAbilityModifier)
	}
	if len(names) != 1 {
		return "", fmt.Errorf("amount sets %d fields, want exactly 1", len(names))
	}
	return names[0], nil
}
