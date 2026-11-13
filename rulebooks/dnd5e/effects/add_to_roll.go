// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"encoding/json/jsontext"
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/rolls"
)

// KindAddToRoll supplies signed additive dice to selected roll domains.
// The recipient selects contributions before the owning operation rolls them.
const KindAddToRoll generic.KindName = "add_to_roll"

// Polarity is the sign of an additive amount, separate from physical dice faces.
// The zero value is invalid; definition authors must select its direction.
type Polarity string

const (
	// Positive adds the resolved amount to the roll.
	Positive Polarity = "positive"
	// Negative subtracts the resolved amount without negating die faces.
	Negative Polarity = "negative"
)

type addToRollParams struct {
	Polarity  Polarity                 `json:"polarity"`
	Rolls     []contributions.RollKind `json:"rolls"`
	Selection generic.SelectionRule    `json:"selection"`
}

// DescribeRollInput requests one bound contribution through the same meaning
// used by information. A nil Request describes no applicable roll.
type DescribeRollInput struct {
	Output      *generic.Bound
	Request     *dndevents.DescribeRollContributionsInput
	Origin      dndevents.ConditionOrigin
	DisplayName string
}

// DescribeRollOutput carries an explicit selection rule and unresolved terms.
// It contains no die face and causes no roll, attachment or state mutation.
type DescribeRollOutput struct {
	Applicable    bool
	Selection     generic.SelectionRule
	Contributions []dndevents.DiceContribution
}

func addToRollMeaning(raw jsontext.Value) (meaning, error) {
	params, err := decodeParams[addToRollParams](raw)
	if err != nil {
		return meaning{}, err
	}
	if params.Polarity != Positive && params.Polarity != Negative {
		return meaning{}, fmt.Errorf("add-to-roll requires positive or negative polarity")
	}
	if err := generic.ValidateSelectionRule(&generic.ValidateSelectionRuleInput{Rule: params.Selection}); err != nil {
		return meaning{}, err
	}
	if len(params.Rolls) == 0 {
		return meaning{}, fmt.Errorf("add-to-roll requires roll domains")
	}
	seen := map[contributions.RollKind]bool{}
	for _, roll := range params.Rolls {
		if (roll != contributions.RollKindAttack && roll != contributions.RollKindSavingThrow) || seen[roll] {
			return meaning{}, fmt.Errorf("unsupported or repeated roll domain %q", roll)
		}
		seen[roll] = true
	}
	return meaning{domain: domainRoll, description: "Adds a signed unresolved amount to the recipient's selected attack rolls or saving throws."}, nil
}

func checkAddToRoll(out generic.Output) error {
	if _, err := addToRollMeaning(out.Params); err != nil {
		return err
	}
	if out.When != nil {
		return fmt.Errorf("add-to-roll does not yet accept an authored guard")
	}
	if out.Amount == nil {
		return fmt.Errorf("add-to-roll requires a dice amount")
	}
	name, err := out.Amount.Name()
	if err != nil {
		return err
	}
	if name != generic.AmountDice {
		return fmt.Errorf("add-to-roll requires a dice amount")
	}
	_, err = rolls.ParseContributionDice(&rolls.ParseContributionDiceInput{Notation: out.Amount.Dice})
	return err
}

func addToRollPartOutput(decl PartDeclaration) (generic.Output, error) {
	if decl.OnTerminal != generic.EndPart {
		return generic.Output{}, fmt.Errorf("add-to-roll payload requires local part scope")
	}
	out := generic.Output{Kind: KindAddToRoll, Params: jsontext.Value(decl.Params), When: decl.When, Amount: decl.Amount}
	if err := checkAddToRoll(out); err != nil {
		return generic.Output{}, err
	}
	return out, nil
}

func bindAddToRollPart(in *BindPartInput) (boundPart, error) {
	out, err := addToRollPartOutput(in.Declaration)
	if err != nil {
		return boundPart{}, err
	}
	return payloadRecipe(in, out)
}

func inspectAddToRollPart(in *InspectPartInput) (*PartInfo, error) {
	if _, err := addToRollPartOutput(in.Declaration); err != nil {
		return nil, err
	}
	return inspectPayloadState(in.Snapshot)
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

func assessAddToRoll(in *AssessInput, m meaning) (assessment, error) {
	var result assessment
	if err := validateBinding(&BindInput{Origin: in.Origin, DisplayName: in.DisplayName}); err != nil {
		return result, err
	}
	if err := in.Query.Validate(); err != nil {
		return result, err
	}
	params, err := decodeParams[addToRollParams](in.Output.Params())
	if err != nil {
		return result, err
	}
	sourceRef := in.Origin.Ref
	rule := params.Selection
	result.effect = contributions.Effect{ID: in.Output.ID(), Selection: &rule, Source: contributions.Source{Ref: in.Output.Host().Ref, Name: in.DisplayName, SourceID: in.Origin.EntityID}, Description: m.description, Participation: contributions.ContributesNow}
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
		return result, fmt.Errorf("add-to-roll did not describe dice")
	}
	result.value = &value
	result.effect.State = contributions.StateApplies
	sign := "+"
	if params.Polarity == Negative {
		sign = "−"
	}
	noun := "attack roll"
	if kind == contributions.RollKindSavingThrow {
		noun = "saving throw"
	}
	result.effect.Benefit = sign + value.Dice + " to the " + noun
	result.roll = []dndevents.DiceContribution{{Dice: value.Dice, Subtract: params.Polarity == Negative, Source: dndevents.RollSource{Ref: &sourceRef, Name: in.DisplayName, SourceID: in.Origin.EntityID}}}
	return result, nil
}

// DescribeRoll returns unresolved signed dice and explicit selection metadata
// through the same applicability as Assess. It refuses malformed output/source
// data and never rolls. A known nonmatching roll produces no contributions.
func DescribeRoll(in *DescribeRollInput) (*DescribeRollOutput, error) {
	if in == nil || in.Output == nil || in.Output.Kind() != KindAddToRoll {
		return nil, fmt.Errorf("roll description requires an add-to-roll output")
	}
	if err := validateBinding(&BindInput{Origin: in.Origin, DisplayName: in.DisplayName}); err != nil {
		return nil, err
	}
	if _, err := addToRollMeaning(in.Output.Params()); err != nil {
		return nil, err
	}
	params, err := decodeParams[addToRollParams](in.Output.Params())
	if err != nil {
		return nil, err
	}
	result := &DescribeRollOutput{Selection: params.Selection}
	if in.Request == nil {
		return result, nil
	}
	answer, err := assess(&AssessInput{Output: in.Output, Origin: in.Origin, DisplayName: in.DisplayName, Query: Query{Roll: &RollQuery{MemberID: in.Output.Host().Holder, Kind: in.Request.Kind}}})
	if err != nil {
		return nil, err
	}
	result.Applicable = answer.effect.State == contributions.StateApplies
	result.Contributions = answer.roll
	return result, nil
}
