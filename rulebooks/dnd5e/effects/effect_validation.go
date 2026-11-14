// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"encoding/json/jsontext"
	"fmt"
	"slices"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
)

// ValidateEffectInput supplies unbound configured meaning, without a placement.
type ValidateEffectInput struct{ Effect EffectSpec }

// ValidateEffectOutput reports kind-owned placement capabilities. Returned scope
// values are detached from the kind table; validation does not choose a scope.
type ValidateEffectOutput struct{ Capabilities PartCapabilities }

// ValidateEffect validates intrinsic configuration through the single kind
// table. Nil input, unknown kinds or malformed/unsupported fields are errors.
// It invents no holder, key, application or terminal policy to validate content.
func ValidateEffect(in *ValidateEffectInput) (*ValidateEffectOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("effect definition input is required")
	}
	kind, ok := kindTable[in.Effect.Kind]
	if !ok || kind.validateEffect == nil {
		return nil, fmt.Errorf("unsupported effect kind %q", in.Effect.Kind)
	}
	if err := kind.validateEffect(in.Effect); err != nil {
		return nil, err
	}
	caps := kind.capabilities
	caps.AllowedScopes = slices.Clone(caps.AllowedScopes)
	return &ValidateEffectOutput{Capabilities: caps}, nil
}

func effectOutput(spec EffectSpec) generic.Output {
	return generic.Output{Kind: spec.Kind, Params: jsontext.Value(spec.Params), When: spec.When, Amount: spec.Amount}
}
func validateDamageEffect(spec EffectSpec) error {
	if err := checkDamage(effectOutput(spec)); err != nil {
		return err
	}
	return validateFrameGuard(spec.When, []machine.FactName{FactAbility, FactMelee, FactWeaponPool})
}
func validateResistanceEffect(spec EffectSpec) error {
	if spec.When != nil {
		return fmt.Errorf("resistance does not accept a guard")
	}
	return checkResistance(effectOutput(spec))
}
func validateAdvantageEffect(spec EffectSpec) error {
	if err := checkAdvantage(effectOutput(spec)); err != nil {
		return err
	}
	meaning, err := advantageMeaning(jsontext.Value(spec.Params))
	if err != nil {
		return err
	}
	return validateFrameGuard(spec.When, advantageFacts(meaning))
}
func validateAddToRollEffect(spec EffectSpec) error { return checkAddToRoll(effectOutput(spec)) }
func validateMaintenanceFields(spec EffectSpec) error {
	if spec.When != nil || spec.Amount != nil {
		return fmt.Errorf("effect %q does not accept when or amount", spec.Kind)
	}
	return nil
}
func validateUnparameterizedEffect(spec EffectSpec) error {
	if err := validateMaintenanceFields(spec); err != nil {
		return err
	}
	if len(spec.Params) == 0 {
		return nil
	}
	_, err := decodeParams[struct{}](jsontext.Value(spec.Params))
	return err
}
func validateDurationEffect(spec EffectSpec) error {
	if err := validateMaintenanceFields(spec); err != nil {
		return err
	}
	_, err := parseDurationParams(jsontext.Value(spec.Params))
	return err
}
func validateActivityEffect(spec EffectSpec) error {
	if err := validateMaintenanceFields(spec); err != nil {
		return err
	}
	_, err := decodeParams[activityParams](jsontext.Value(spec.Params))
	return err
}
func validateEndOnEffect(spec EffectSpec) error {
	if err := validateMaintenanceFields(spec); err != nil {
		return err
	}
	_, err := parseEndOnParams(jsontext.Value(spec.Params))
	return err
}
