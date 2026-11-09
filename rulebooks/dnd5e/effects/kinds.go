// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/events"
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
)

type kindDefinition struct {
	check   func(generic.Output) error
	meaning func(jsontext.Value) (meaning, error)
	attach  func(context.Context, events.EventBus, *generic.Bound, BindInput) (generic.Detach, error)
}

type meaning struct {
	domain      domain
	description string
	types       []damage.Type
}
type resistanceParams struct {
	Types []damage.Type `json:"types"`
}
type advantageParams struct {
	On domain `json:"on"`
}

type assessment struct {
	effect contributions.Effect
	amount int
	types  []damage.Type
}

// kindTable is the only kind registration: generic validation/attachment/rows
// and richer rulebook assessment all resolve through these same entries.
var kindTable = map[generic.KindName]kindDefinition{
	KindDamageBonus: {check: checkDamage, meaning: damageMeaning, attach: attachDamage},
	KindResistance:  {check: checkResistance, meaning: resistanceMeaning, attach: attachResistance},
	KindAdvantage:   {check: checkAdvantage, meaning: advantageMeaning, attach: attachAdvantage},
}

func decodeParams[T any](raw jsontext.Value) (T, error) {
	var value T
	if len(raw) == 0 {
		return value, fmt.Errorf("effect params are required")
	}
	if err := json.Unmarshal(raw, &value, json.RejectUnknownMembers(true)); err != nil {
		return value, err
	}
	return value, nil
}

func damageMeaning(raw jsontext.Value) (meaning, error) {
	if len(raw) > 0 {
		if _, err := decodeParams[struct{}](raw); err != nil {
			return meaning{}, err
		}
	}
	return meaning{domain: domainDamage, description: "Adds the configured damage bonus to matching outgoing damage."}, nil
}
func resistanceMeaning(raw jsontext.Value) (meaning, error) {
	p, err := decodeParams[resistanceParams](raw)
	if err != nil {
		return meaning{}, err
	}
	if len(p.Types) == 0 {
		return meaning{}, fmt.Errorf("resistance requires damage types")
	}
	seen := map[damage.Type]bool{}
	names := make([]string, 0, len(p.Types))
	for _, kind := range p.Types {
		if _, err := damage.GetByID(string(kind)); err != nil {
			return meaning{}, err
		}
		if seen[kind] {
			return meaning{}, fmt.Errorf("repeated resistance type %q", kind)
		}
		seen[kind] = true
		names = append(names, string(kind))
	}
	return meaning{domain: domainIncoming, description: "Resistance to " + strings.Join(names, ", ") + " damage.", types: p.Types}, nil
}
func advantageMeaning(raw jsontext.Value) (meaning, error) {
	p, err := decodeParams[advantageParams](raw)
	if err != nil {
		return meaning{}, err
	}
	switch p.On {
	case domainSave:
		return meaning{domain: domainSave, description: "Advantage on matching saving throws."}, nil
	case domainCheck:
		return meaning{domain: domainCheck, description: "Advantage on matching ability checks."}, nil
	default:
		return meaning{}, fmt.Errorf("unsupported advantage domain %q", p.On)
	}
}
func checkDamage(out generic.Output) error {
	if _, err := damageMeaning(out.Params); err != nil {
		return err
	}
	return checkAmount(out.Amount)
}
func checkResistance(out generic.Output) error {
	if out.Amount != nil {
		return fmt.Errorf("resistance does not take an amount")
	}
	_, err := resistanceMeaning(out.Params)
	return err
}
func checkAdvantage(out generic.Output) error {
	if out.Amount != nil {
		return fmt.Errorf("advantage does not take an amount")
	}
	_, err := advantageMeaning(out.Params)
	return err
}

// Assess returns one effect's explanation from the same domain, guard and amount
// evaluation used by execution. It refuses nil input/output, invalid provenance,
// an unknown kind, invalid params and malformed/ambiguous query contexts. Unknown
// required facts yield Depends with no applied benefit; no context describes only.
func Assess(in *AssessInput) (*AssessOutput, error) {
	result, err := assess(in)
	if err != nil {
		return nil, err
	}
	return &AssessOutput{Effect: result.effect}, nil
}

func assess(in *AssessInput) (assessment, error) {
	var result assessment
	if in == nil || in.Output == nil {
		return result, fmt.Errorf("effect assessment requires a bound output")
	}
	def, ok := kindTable[in.Output.Kind()]
	if !ok {
		return result, fmt.Errorf("unknown rulebook effect kind %q", in.Output.Kind())
	}
	m, err := def.meaning(in.Output.Params())
	if err != nil {
		return result, err
	}
	return assessMeaning(in, m)
}

// assessMeaning evaluates an already selected kind without redispatching through
// the registry. Execution and information share it, avoiding an initialization
// cycle between registry entries and their attachment functions.
func assessMeaning(in *AssessInput, m meaning) (assessment, error) {
	var result assessment
	if err := validateBinding(&BindInput{Origin: in.Origin, DisplayName: in.DisplayName}); err != nil {
		return result, err
	}
	if err := in.Query.Validate(); err != nil {
		return result, err
	}
	var err error
	host := in.Output.Host()
	result.effect = contributions.Effect{ID: in.Output.ID(), Source: contributions.Source{Ref: host.Ref, Name: in.DisplayName, SourceID: in.Origin.EntityID}, Description: m.description, Participation: contributions.ContributesNow}
	if in.Query.domain() == domainNone {
		result.effect.State = contributions.StateDepends
		result.effect.Reason = "An action or occurrence is needed to assess this effect"
		return result, nil
	}
	if in.Query.domain() != m.domain {
		result.effect.State = contributions.StateDoesNotApply
		result.effect.Reason = "This effect applies to " + string(m.domain) + ", not " + string(in.Query.domain())
		return result, nil
	}
	if in.Query.subject() != host.Holder {
		result.effect.State = contributions.StateDoesNotApply
		result.effect.Reason = "This effect applies to its recipient"
		return result, nil
	}
	if m.domain == domainIncoming {
		for _, kind := range in.Query.Incoming.DealtTypes() {
			if slices.Contains(m.types, kind) {
				result.types = append(result.types, kind)
			}
		}
		if len(result.types) == 0 {
			result.effect.State = contributions.StateDoesNotApply
			result.effect.Reason = "No matching damage type was dealt"
			return result, nil
		}
	}
	verdict, reason := in.Output.Answer(in.Query)
	result.effect.Reason = reason
	switch verdict {
	case machine.DoesNotApply:
		result.effect.State = contributions.StateDoesNotApply
		return result, nil
	case machine.Depends:
		result.effect.State = contributions.StateDepends
		return result, nil
	case machine.Applies:
		result.effect.State = contributions.StateApplies
	default:
		return result, fmt.Errorf("unknown applicability verdict %q", verdict)
	}
	if in.Output.HasAmount() {
		result.amount, err = in.Output.Amount(in.Query)
		if err != nil {
			if errors.Is(err, contributions.ErrRuleCannotAnswer) {
				result.effect.State = contributions.StateDepends
				result.effect.Reason = err.Error()
				return result, nil
			}
			return result, err
		}
	}
	switch in.Output.Kind() {
	case KindDamageBonus:
		result.effect.Benefit = fmt.Sprintf("%+d damage", result.amount)
	case KindResistance:
		result.effect.Benefit = "Resistance"
	case KindAdvantage:
		result.effect.Benefit = "Advantage"
	}
	return result, nil
}

func executeAssessment(in *AssessInput, m meaning) (assessment, error) {
	result, err := assessMeaning(in, m)
	if err != nil {
		return result, fmt.Errorf("%w: %w", contributions.ErrRuleCannotAnswer, err)
	}
	if result.effect.State == contributions.StateDepends {
		err = fmt.Errorf("%w: %s", contributions.ErrRuleCannotAnswer, result.effect.Reason)
	}
	return result, err
}
