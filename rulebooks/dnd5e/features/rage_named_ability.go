// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"bytes"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/combat"
	dnd5eCombat "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// RageParameters are the values Rage takes. DamageBonus is written one of two
// ways: a whole number of at least 1 (an item or a stat block writes this), or
// exactly the string "barbarian level" (the barbarian's own feature writes
// this, since only she has barbarian levels to read).
type RageParameters struct {
	DamageBonus json.RawMessage `json:"damage_bonus"`
}

// rageMinDamageBonus is the least a written damage bonus is. The entry's
// check enforces it and its values report it, so the two cannot drift.
const rageMinDamageBonus = 1

// rageByLevel is the one string the damage bonus may be written as: it reads
// the holder's barbarian level when she strikes.
const rageByLevel = "barbarian level"

// rageAbility is Rage's entry in the table of named abilities. It charges a
// bonus action, so it is not counted.
var rageAbility = namedAbility{
	word:  "rage",
	ref:   func() core.Ref { return *refs.Features.Rage() },
	label: "Rage",
	values: []AbilityValue{{
		Word:     "damage_bonus",
		Required: true,
		Min:      rageMinDamageBonus,
		NoMax:    true,
		Label:    "adds {value} to the damage of each melee weapon hit made with strength",
	}},
	check: func(raw json.RawMessage) error {
		_, err := decodeRageParameters(raw)
		return err
	},
	describe: describeRage,
}

// decodeRageParameters judges Rage's values strictly and returns the bindings
// the raging status is built from.
func decodeRageParameters(raw json.RawMessage) (conditions.RagingBindings, error) {
	var none conditions.RagingBindings
	if len(bytes.TrimSpace(raw)) == 0 {
		return none, fmt.Errorf("rage requires parameters, for example {\"damage_bonus\": 2}")
	}
	var params RageParameters
	if err := jsonv2.Unmarshal(raw, &params, jsonv2.RejectUnknownMembers(true)); err != nil {
		return none, fmt.Errorf("rage parameters: %w", err)
	}
	written := bytes.TrimSpace(params.DamageBonus)
	if len(written) == 0 {
		return none, fmt.Errorf("rage requires damage_bonus, for example {\"damage_bonus\": 2}")
	}
	refuse := fmt.Errorf("rage damage_bonus %s must be at least %d (a whole number), or %q",
		written, rageMinDamageBonus, rageByLevel)
	if written[0] == '"' {
		var text string
		if err := jsonv2.Unmarshal(written, &text); err != nil || text != rageByLevel {
			return none, refuse
		}
		return conditions.RagingBindings{DamageBonus: conditions.RagingDamageBonus{BarbarianLevel: true}}, nil
	}
	var amount int
	if err := jsonv2.Unmarshal(written, &amount); err != nil || amount < rageMinDamageBonus {
		return none, refuse
	}
	return conditions.RagingBindings{DamageBonus: conditions.RagingDamageBonus{Fixed: &amount}}, nil
}

func describeRage(in *describeInput) (*actions.Definition, error) {
	bindings, err := decodeRageParameters(in.parameters)
	if err != nil {
		return nil, err
	}
	parameters, err := json.Marshal(bindings)
	if err != nil {
		return nil, fmt.Errorf("encoding rage bindings: %w", err)
	}
	description := (&Rage{}).Description()
	if bindings.DamageBonus.Fixed != nil {
		description = fmt.Sprintf("Enter a rage. While raging you add %d to the damage of melee weapon hits made with Strength, "+
			"have advantage on Strength checks and saving throws, and resist bludgeoning, piercing and slashing damage.",
			*bindings.DamageBonus.Fixed)
	}
	return &actions.Definition{
		Ref:         *refs.Features.Rage(),
		Name:        "Rage",
		Description: description,
		Cost: &dnd5eCombat.SpendProfile{
			Slots: map[combat.ActionType]int{combat.ActionBonus: 1},
			Pools: in.pools,
		},
		Effect: &actions.EffectProfile{
			To: actions.EffectSelf,
			Condition: &actions.ConditionApplication{
				Ref:        *refs.Conditions.Raging(),
				Parameters: parameters,
			},
		},
	}, nil
}
