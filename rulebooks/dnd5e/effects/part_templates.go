// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"encoding/json"
	"encoding/json/jsontext"
	"fmt"
	"slices"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
)

// PartClass distinguishes selectable benefits from lifecycle policy. The kind
// entry owns this classification; callers do not maintain another kind table.
type PartClass string

const (
	// PartClassBenefit describes a recipient contribution.
	PartClassBenefit PartClass = "benefit"
	// PartClassMaintenance describes coordinator-owned upkeep or ending policy.
	PartClassMaintenance PartClass = "maintenance"
)

// PartTemplate is reusable authored meaning without a key, scope or participant.
// Each kind validates which optional fields it accepts.
type PartTemplate struct {
	Kind   generic.KindName `json:"kind"`
	Params json.RawMessage  `json:"params,omitempty"`
	When   *machine.Guard   `json:"when,omitempty"`
	Amount *generic.Amount  `json:"amount,omitempty"`
}

// Clone detaches all nested template values without binding an instance.
func (p PartTemplate) Clone() PartTemplate {
	out := contributions.ConfiguredOutput{Definition: generic.Output{Kind: p.Kind, Params: jsontext.Value(p.Params), When: p.When, Amount: p.Amount}}.Clone().Definition
	return PartTemplate{Kind: out.Kind, Params: json.RawMessage(out.Params), When: out.When, Amount: out.Amount}
}

// Clone detaches a declaration's data while retaining its local identity/scope.
func (p PartDeclaration) Clone() PartDeclaration {
	out := (PartTemplate{Kind: p.Kind, Params: p.Params, When: p.When, Amount: p.Amount}).Clone()
	p.Params, p.When, p.Amount = out.Params, out.When, out.Amount
	return p
}

// ValidatePartTemplateInput supplies an unbound template, not fake runtime IDs.
type ValidatePartTemplateInput struct{ Template PartTemplate }

// ValidatePartTemplateOutput reports kind-owned capabilities after validation.
type ValidatePartTemplateOutput struct {
	Class        PartClass
	Capabilities PartCapabilities
}

// ValidatePartTemplate refuses unknown kinds, malformed parameters and unsupported
// fields without constructing a runtime, attaching, paying or reading the world.
func ValidatePartTemplate(in *ValidatePartTemplateInput) (*ValidatePartTemplateOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("part template is required")
	}
	t := in.Template
	def, ok := kindTable[t.Kind]
	if !ok || def.validatePart == nil || def.class == "" {
		return nil, fmt.Errorf("unsupported effect template %q", t.Kind)
	}
	scope := generic.EndPart
	if def.class == PartClassMaintenance {
		scope = generic.EndApplication
	}
	decl := PartDeclaration{Kind: t.Kind, Params: t.Params, When: t.When, Amount: t.Amount, OnTerminal: scope}
	if err := validatePartDefinition(decl, def); err != nil {
		return nil, err
	}
	result := &ValidatePartTemplateOutput{Class: def.class}
	if def.describePart != nil {
		info, err := def.describePart(decl)
		if err != nil {
			return nil, err
		}
		result.Capabilities = info.capabilities
	}
	return result, nil
}

func validatePartDefinition(decl PartDeclaration, def kindDefinition) error {
	if def.payload == nil && (decl.When != nil || decl.Amount != nil) {
		return fmt.Errorf("part %q does not accept when or amount", decl.Kind)
	}
	return def.validatePart(decl)
}

// The generic machine owns guard evaluation. This validates only the authored
// frame-fact subset and the selected kind's available fact domains.
func validateFrameGuard(g *machine.Guard, allowed []machine.FactName) error {
	if g == nil {
		return nil
	}
	count := 0
	if len(g.All) > 0 {
		count++
	}
	if len(g.Any) > 0 {
		count++
	}
	if g.Not != nil {
		count++
	}
	if g.Fact != nil {
		count++
	}
	if count != 1 || g.Bag != nil || g.Flag != "" || g.Rounds != nil || g.ClassLevel != nil {
		return fmt.Errorf("authored guard requires one frame-fact or boolean node")
	}
	for _, list := range [][]machine.Guard{g.All, g.Any} {
		for i := range list {
			if err := validateFrameGuard(&list[i], allowed); err != nil {
				return err
			}
		}
	}
	if g.Not != nil {
		return validateFrameGuard(g.Not, allowed)
	}
	if g.Fact == nil {
		return nil
	}
	fact := g.Fact
	if !slices.Contains(allowed, fact.Name) {
		return fmt.Errorf("unsupported frame fact %q for this effect", fact.Name)
	}
	spec, ok := machineVocabulary().Facts[fact.Name]
	if !ok || fact.Op != machine.OpEq {
		return fmt.Errorf("unsupported comparison for frame fact %q", fact.Name)
	}
	switch spec.Kind {
	case machine.KindBool:
		if _, ok := fact.Value.(bool); !ok {
			return fmt.Errorf("fact %q requires a bool", fact.Name)
		}
	case machine.KindString:
		value, ok := fact.Value.(string)
		if !ok {
			return fmt.Errorf("fact %q requires a string", fact.Name)
		}
		if fact.Name == FactAbility && value != "" {
			if _, err := abilities.GetByID(value); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unsupported authored fact type %q", spec.Kind)
	}
	return nil
}
