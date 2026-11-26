// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"fmt"
	"maps"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	dnd5eCombat "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/healing"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// DescribeFromRefInput asks for a named ability with the values its giver supplies.
type DescribeFromRefInput struct {
	// Ref names the ability. Only refs.Features.SecondWind() is known in this version.
	Ref core.Ref
	// Parameters are the ability's values as JSON, decoded strictly.
	// Second Wind takes {"level": n}, n at least 1.
	Parameters json.RawMessage
	// Giver is who supplies the values. It is named on the roll's modifier:
	// a class ("Fighter level"), an item ("Ring level").
	Giver dnd5eEvents.RollSource
	// Pools are the giver's counts that pay, beside what the ability itself
	// costs. May be empty: a giver that keeps its own count pays elsewhere.
	Pools map[coreResources.ResourceKey]int
}

// ValidateParametersInput names an ability and the values to check.
type ValidateParametersInput struct {
	Ref        core.Ref
	Parameters json.RawMessage
}

// SecondWindParameters are the values Second Wind takes.
type SecondWindParameters struct {
	Level int `json:"level"`
}

// DescribeFromRef returns the action description the gate runs for a named
// ability, with the values its giver supplies. Every giver of Second Wind, a
// class feature or an item, asks here; the meaning is written once.
func DescribeFromRef(in *DescribeFromRefInput) (*actions.Definition, error) {
	if in == nil {
		return nil, fmt.Errorf("describing an ability requires input")
	}
	params, err := decodeParameters(in.Ref, in.Parameters)
	if err != nil {
		return nil, err
	}
	if err := validateGiver(in.Giver); err != nil {
		return nil, err
	}
	pools := make(map[coreResources.ResourceKey]int, len(in.Pools))
	for key, amount := range in.Pools {
		if key == "" {
			return nil, fmt.Errorf("second wind pool key is empty")
		}
		if amount < 1 {
			return nil, fmt.Errorf("second wind pool %q amount %d must be at least 1", key, amount)
		}
		pools[key] = amount
	}
	if len(pools) == 0 {
		pools = nil
	}
	return &actions.Definition{
		Ref:         *refs.Features.SecondWind(),
		Name:        "Second Wind",
		Description: fmt.Sprintf("Regain 1d10 hit points plus %d", params.Level),
		Cost: &dnd5eCombat.SpendProfile{
			Slots: map[combat.ActionType]int{combat.ActionBonus: 1},
			Pools: pools,
		},
		Effect: &actions.EffectProfile{
			To: actions.EffectSelf,
			Healing: &healing.Declaration{
				Dice: "1d10",
				Modifiers: []healing.Modifier{{
					Source: contributions.CloneSource(in.Giver),
					Amount: params.Level,
				}},
			},
		},
	}, nil
}

// ValidateParameters checks values with the same contract DescribeFromRef uses.
// It refuses the ref and the parameters; the giver and pools are not its concern.
func ValidateParameters(in *ValidateParametersInput) error {
	if in == nil {
		return fmt.Errorf("validating ability parameters requires input")
	}
	_, err := decodeParameters(in.Ref, in.Parameters)
	return err
}

// secondWindMinLevel is the least level Second Wind takes. decodeParameters
// enforces it and [AbilityWords] reports it, so the two cannot drift.
const secondWindMinLevel = 1

func decodeParameters(ref core.Ref, raw json.RawMessage) (SecondWindParameters, error) {
	var params SecondWindParameters
	if ref != *refs.Features.SecondWind() {
		return params, fmt.Errorf("no described ability %s", ref.String())
	}
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

func validateGiver(giver dnd5eEvents.RollSource) error {
	if giver.Ref == nil {
		return fmt.Errorf("second wind giver requires a Ref")
	}
	if giver.Label == "" {
		return fmt.Errorf("second wind giver requires a Label")
	}
	if giver.SourceID == "" {
		return fmt.Errorf("second wind giver requires a SourceID")
	}
	return nil
}

// namedAbilities are the described abilities by the plain name an author
// writes. Every ref here is one DescribeFromRef and ValidateParameters know.
var namedAbilities = map[string]func() core.Ref{
	"second wind": func() core.Ref { return *refs.Features.SecondWind() },
}

// NamedAbility returns the ref of a described ability by the plain name an
// author writes: "second wind". The name is matched exactly: lower case,
// single spaces.
func NamedAbility(name string) (core.Ref, bool) {
	ref, ok := namedAbilities[name]
	if !ok {
		return core.Ref{}, false
	}
	return ref(), true
}

// NamedAbilities lists the plain names, sorted, for a refusal to offer.
func NamedAbilities() []string {
	return slices.Sorted(maps.Keys(namedAbilities))
}
