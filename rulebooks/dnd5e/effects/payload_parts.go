// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
)

func damagePartOutput(decl PartDeclaration) (generic.Output, error) {
	out := generic.Output{Kind: KindDamageBonus, Params: jsontext.Value(decl.Params), When: decl.When, Amount: decl.Amount}
	if decl.OnTerminal != generic.EndPart {
		return generic.Output{}, fmt.Errorf("damage payload requires local part scope")
	}
	if err := checkDamage(out); err != nil {
		return generic.Output{}, err
	}
	if err := validateFrameGuard(decl.When, []machine.FactName{FactAbility, FactMelee, FactWeaponPool}); err != nil {
		return generic.Output{}, err
	}
	return out, nil
}
func advantagePartOutput(decl PartDeclaration) (generic.Output, error) {
	out := generic.Output{Kind: KindAdvantage, Params: jsontext.Value(decl.Params), When: decl.When, Amount: decl.Amount}
	if decl.OnTerminal != generic.EndPart {
		return generic.Output{}, fmt.Errorf("advantage payload requires local part scope")
	}
	if err := checkAdvantage(out); err != nil {
		return generic.Output{}, err
	}
	if err := validateFrameGuard(decl.When, []machine.FactName{FactAbility}); err != nil {
		return generic.Output{}, err
	}
	return out, nil
}
func resistancePartOutput(decl PartDeclaration) (generic.Output, error) {
	out := generic.Output{Kind: KindResistance, Params: jsontext.Value(decl.Params), When: decl.When, Amount: decl.Amount}
	if decl.OnTerminal != generic.EndPart || decl.When != nil {
		return generic.Output{}, fmt.Errorf("resistance payload requires local scope and no guard")
	}
	if err := checkResistance(out); err != nil {
		return generic.Output{}, err
	}
	return out, nil
}
func payloadRecipe(in *BindPartInput, out generic.Output) (boundPart, error) {
	raw, err := json.Marshal(out)
	if err != nil {
		return boundPart{}, err
	}
	return boundPart{recipe: machine.Recipe{Ref: in.Ref, Initial: partStateActive, Bag: machine.Bag{}, States: map[string]machine.State{partStateActive: {Outputs: []jsontext.Value{raw}}}}}, nil
}
func bindDamagePart(in *BindPartInput) (boundPart, error) {
	out, err := damagePartOutput(in.Declaration)
	if err != nil {
		return boundPart{}, err
	}
	return payloadRecipe(in, out)
}
func bindAdvantagePart(in *BindPartInput) (boundPart, error) {
	out, err := advantagePartOutput(in.Declaration)
	if err != nil {
		return boundPart{}, err
	}
	return payloadRecipe(in, out)
}
func validateDamagePart(decl PartDeclaration) error { _, err := damagePartOutput(decl); return err }
func validateAdvantagePart(decl PartDeclaration) error {
	_, err := advantagePartOutput(decl)
	return err
}
func inspectDamagePart(in *InspectPartInput) (*PartInfo, error) {
	if err := validateDamagePart(in.Declaration); err != nil {
		return nil, err
	}
	return inspectPayloadState(in.Snapshot)
}
func inspectAdvantagePart(in *InspectPartInput) (*PartInfo, error) {
	if err := validateAdvantagePart(in.Declaration); err != nil {
		return nil, err
	}
	return inspectPayloadState(in.Snapshot)
}
func inspectPayloadState(state generic.Snapshot) (*PartInfo, error) {
	if state.State != partStateActive || state.Bag == nil || len(state.Bag) != 0 {
		return nil, fmt.Errorf("payload requires active state and an explicit empty bag")
	}
	return &PartInfo{}, nil
}
