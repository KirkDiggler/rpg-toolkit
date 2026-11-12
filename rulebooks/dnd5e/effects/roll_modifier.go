// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"slices"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// KindRollModifier describes signed dice for selected roll domains. Selection
// and rolling remain the recipient/native roll path's job, not a subscription.
const KindRollModifier generic.KindName = "roll_modifier"

type rollModifierParams struct {
	Group    string                   `json:"group"`
	Rolls    []contributions.RollKind `json:"rolls"`
	Subtract bool                     `json:"subtract,omitempty"`
}

type rollPartParams struct {
	Dice     string                   `json:"dice"`
	Group    string                   `json:"group"`
	Rolls    []contributions.RollKind `json:"rolls"`
	Subtract bool                     `json:"subtract,omitempty"`
}

// DescribeRollInput requests one bound contribution through the same meaning
// used by information. A nil Request describes no applicable roll.
type DescribeRollInput struct {
	Output      *generic.Bound
	Request     *dndevents.DescribeRollContributionsInput
	Origin      dndevents.ConditionOrigin
	DisplayName string
}

// DescribeRollOutput carries selection metadata and unresolved contributions.
// It contains no die face and causes no roll, attachment or state mutation.
type DescribeRollOutput struct {
	Applicable    bool
	Group         string
	Contributions []dndevents.DiceContribution
}

func rollMeaning(raw jsontext.Value) (meaning, error) {
	params, err := decodeParams[rollModifierParams](raw)
	if err != nil {
		return meaning{}, err
	}
	if strings.TrimSpace(params.Group) == "" || len(params.Rolls) == 0 {
		return meaning{}, fmt.Errorf("roll modifier requires a group and roll domains")
	}
	seen := map[contributions.RollKind]bool{}
	for _, roll := range params.Rolls {
		if (roll != contributions.RollKindAttack && roll != contributions.RollKindSavingThrow) || seen[roll] {
			return meaning{}, fmt.Errorf("unsupported or repeated roll domain %q", roll)
		}
		seen[roll] = true
	}
	return meaning{domain: domainRoll, description: "Modifies the recipient's selected attack rolls or saving throws with unresolved dice."}, nil
}

func checkRollModifier(out generic.Output) error {
	if _, err := rollMeaning(out.Params); err != nil {
		return err
	}
	if out.Amount == nil {
		return fmt.Errorf("roll modifier requires dice")
	}
	name, err := out.Amount.Name()
	if err != nil {
		return err
	}
	if name != generic.AmountDice {
		return fmt.Errorf("roll modifier requires a dice amount")
	}
	_, err = dice.ParseNotation(out.Amount.Dice)
	return err
}

func rollPartOutput(declaration PartDeclaration) (generic.Output, error) {
	if declaration.OnTerminal != generic.EndPart {
		return generic.Output{}, fmt.Errorf("roll modifier payload requires local part scope")
	}
	params, err := decodeParams[rollPartParams](jsontext.Value(declaration.Params))
	if err != nil {
		return generic.Output{}, err
	}
	rawParams, err := json.Marshal(rollModifierParams{Group: params.Group, Rolls: params.Rolls, Subtract: params.Subtract})
	if err != nil {
		return generic.Output{}, err
	}
	out := generic.Output{Kind: KindRollModifier, Params: rawParams, Amount: &generic.Amount{Dice: params.Dice}}
	if err := checkRollModifier(out); err != nil {
		return generic.Output{}, err
	}
	return out, nil
}

func bindRollModifierPart(in *BindPartInput) (boundPart, error) {
	out, err := rollPartOutput(in.Declaration)
	if err != nil {
		return boundPart{}, err
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return boundPart{}, err
	}
	return boundPart{recipe: machine.Recipe{Ref: in.Ref, Initial: partStateActive, Bag: machine.Bag{},
		States: map[string]machine.State{partStateActive: {Outputs: []jsontext.Value{raw}}}}}, nil
}

func inspectRollModifierPart(in *InspectPartInput) (*PartInfo, error) {
	if _, err := rollPartOutput(in.Declaration); err != nil {
		return nil, err
	}
	if in.Snapshot.State != partStateActive || in.Snapshot.Bag == nil || len(in.Snapshot.Bag) != 0 {
		return nil, fmt.Errorf("roll modifier requires active state and an explicit empty bag")
	}
	return &PartInfo{}, nil
}

func diceAmount(_ any, amount generic.Amount) (generic.AmountResult, error) {
	if _, err := dice.ParseNotation(amount.Dice); err != nil {
		return generic.AmountResult{}, err
	}
	return generic.AmountResult{Dice: amount.Dice}, nil
}

// rollContext distinguishes an actual saver from a caster declaring a save-shaped
// outgoing action. Unsupported known contexts return an empty kind, not a save.
func rollContext(q Query) (string, contributions.RollKind, bool) {
	switch {
	case q.Roll != nil:
		return q.Roll.MemberID, q.Roll.Kind, true
	case q.Save != nil:
		return q.Save.SaverID, contributions.RollKindSavingThrow, true
	case q.Action != nil:
		kind, known := q.Action.Action.Roll.Get()
		if kind != contributions.RollKindAttack {
			kind = ""
		}
		return q.Action.Actor, kind, known
	case q.Check != nil:
		return q.Check.CheckerID, "", true
	case q.Incoming != nil:
		return q.Incoming.TargetID(), "", true
	default:
		return "", "", false
	}
}

func assessRollModifier(in *AssessInput, m meaning) (assessment, error) {
	var result assessment
	if err := validateBinding(&BindInput{Origin: in.Origin, DisplayName: in.DisplayName}); err != nil {
		return result, err
	}
	if err := in.Query.Validate(); err != nil {
		return result, err
	}
	params, err := decodeParams[rollModifierParams](in.Output.Params())
	if err != nil {
		return result, err
	}
	sourceRef := in.Origin.Ref
	result.effect = contributions.Effect{ID: in.Output.ID(), RollGroup: params.Group, Source: contributions.Source{Ref: in.Output.Host().Ref, Name: in.DisplayName, SourceID: in.Origin.EntityID}, Description: m.description, Participation: contributions.ContributesNow}
	subject, kind, known := rollContext(in.Query)
	if !known {
		result.effect.State = contributions.StateDepends
		result.effect.Reason = "A recipient roll is needed to assess this effect"
		return result, nil
	}
	if subject != in.Output.Host().Holder || !slices.Contains(params.Rolls, kind) {
		result.effect.State = contributions.StateDoesNotApply
		result.effect.Reason = "This effect applies only to its recipient's declared roll domains"
		return result, nil
	}
	verdict, reason := in.Output.Answer(in.Query)
	result.effect.Reason = reason
	if verdict != machine.Applies {
		result.effect.State = contributions.StateDepends
		if verdict == machine.DoesNotApply {
			result.effect.State = contributions.StateDoesNotApply
		}
		return result, nil
	}
	value, err := in.Output.Amount(in.Query)
	if err != nil {
		return result, err
	}
	if value.Dice == "" {
		return result, fmt.Errorf("roll modifier did not describe dice")
	}
	result.value = &value
	result.effect.State = contributions.StateApplies
	sign := "+"
	if params.Subtract {
		sign = "−"
	}
	noun := "attack roll"
	if kind == contributions.RollKindSavingThrow {
		noun = "saving throw"
	}
	result.effect.Benefit = sign + value.Dice + " to the " + noun
	result.roll = []dndevents.DiceContribution{{Dice: value.Dice, Subtract: params.Subtract,
		Source: dndevents.RollSource{Ref: &sourceRef, Name: in.DisplayName, SourceID: in.Origin.EntityID}}}
	return result, nil
}

// DescribeRoll returns unresolved signed dice with the same applicability used
// by Assess. It refuses malformed output/source/request data and never rolls.
func DescribeRoll(in *DescribeRollInput) (*DescribeRollOutput, error) {
	if in == nil || in.Output == nil || in.Output.Kind() != KindRollModifier {
		return nil, fmt.Errorf("roll description requires a roll-modifier output")
	}
	if err := validateBinding(&BindInput{Origin: in.Origin, DisplayName: in.DisplayName}); err != nil {
		return nil, err
	}
	if _, err := rollMeaning(in.Output.Params()); err != nil {
		return nil, err
	}
	params, err := decodeParams[rollModifierParams](in.Output.Params())
	if err != nil {
		return nil, err
	}
	result := &DescribeRollOutput{Group: params.Group}
	if in.Request == nil {
		return result, nil
	}
	answer, err := assess(&AssessInput{Output: in.Output, Origin: in.Origin, DisplayName: in.DisplayName,
		Query: Query{Roll: &RollQuery{MemberID: in.Output.Host().Holder, Kind: in.Request.Kind}}})
	if err != nil {
		return nil, err
	}
	result.Applicable = answer.effect.State == contributions.StateApplies
	result.Contributions = answer.roll
	return result, nil
}
