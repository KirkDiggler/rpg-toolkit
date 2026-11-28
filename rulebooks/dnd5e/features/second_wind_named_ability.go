// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/combat"
	dnd5eCombat "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/healing"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// SecondWindParameters are the values Second Wind takes.
type SecondWindParameters struct {
	Level int `json:"level"`
}

// secondWindMinLevel is the least level Second Wind takes. The entry's check
// enforces it and its values report it, so the two cannot drift.
const secondWindMinLevel = 1

// secondWindAbility is Second Wind's entry in the table of named abilities.
var secondWindAbility = namedAbility{
	word:  "second wind",
	ref:   func() core.Ref { return *refs.Features.SecondWind() },
	label: "Second Wind",
	values: []AbilityValue{{
		Word:     "level",
		Required: true,
		Min:      secondWindMinLevel,
		NoMax:    true,
		Label:    "regains 1d10 hit points plus {value}",
	}},
	check: func(raw json.RawMessage) error {
		_, err := decodeSecondWindParameters(raw)
		return err
	},
	describe: describeSecondWind,
}

func decodeSecondWindParameters(raw json.RawMessage) (SecondWindParameters, error) {
	var params SecondWindParameters
	if len(raw) == 0 {
		return params, fmt.Errorf("second wind requires parameters, for example {\"level\": 1}")
	}
	if err := jsonv2.Unmarshal(raw, &params, jsonv2.RejectUnknownMembers(true)); err != nil {
		return params, fmt.Errorf("second wind parameters: %w", err)
	}
	if params.Level < secondWindMinLevel {
		return params, fmt.Errorf("second wind level %d must be at least %d", params.Level, secondWindMinLevel)
	}
	return params, nil
}

func describeSecondWind(in *describeInput) (*actions.Definition, error) {
	params, err := decodeSecondWindParameters(in.parameters)
	if err != nil {
		return nil, err
	}
	return &actions.Definition{
		Ref:         *refs.Features.SecondWind(),
		Name:        "Second Wind",
		Description: fmt.Sprintf("Regain 1d10 hit points plus %d", params.Level),
		Cost: &dnd5eCombat.SpendProfile{
			Slots: map[combat.ActionType]int{combat.ActionBonus: 1},
			Pools: in.pools,
		},
		Effect: &actions.EffectProfile{
			To: actions.EffectSelf,
			Healing: &healing.Declaration{
				Dice: "1d10",
				Modifiers: []healing.Modifier{{
					Source: contributions.CloneSource(in.giver),
					Amount: params.Level,
				}},
			},
		},
	}, nil
}
