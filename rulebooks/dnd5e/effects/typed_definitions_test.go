// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

type typedDefinitionsSuite struct{ suite.Suite }

func TestTypedDefinitionsSuite(t *testing.T) { suite.Run(t, new(typedDefinitionsSuite)) }

func typedRollSpec() dndfx.PartSpec {
	return dndfx.PartSpec{Key: "roll", Effect: dndfx.AddToRoll{
		Amount:    dndfx.SignedDice{Notation: "1d4", Polarity: dndfx.Positive},
		Rolls:     []contributions.RollKind{contributions.RollKindAttack, contributions.RollKindSavingThrow},
		Selection: generic.SelectionRule{Mode: generic.OldestApplicable, Group: "bless"},
	}, OnTerminal: generic.EndPart}
}

func (s *typedDefinitionsSuite) TestGoAndJSONCanonicalEquality() {
	out, err := dndfx.CompileParts(&dndfx.CompilePartsInput{Parts: []dndfx.PartSpec{
		typedRollSpec(), {Key: "rest", Effect: dndfx.EndOn{Beat: dndfx.BeatRest, Reason: "rest"}, OnTerminal: generic.EndAttachment},
	}})
	s.Require().NoError(err)
	raw, err := json.Marshal(out.Parts)
	s.Require().NoError(err)
	s.JSONEq(`[{"key":"roll","kind":"add_to_roll","amount":{"dice":"1d4"},"params":{"polarity":"positive","rolls":["attack","saving_throw"],"selection":{"mode":"oldest_applicable","group":"bless"}},"on_terminal":"part"},{"key":"rest","kind":"end_on","params":{"beat":"rest","reason":"rest"},"on_terminal":"attachment"}]`, string(raw))
	var authored []dndfx.PartDeclaration
	s.Require().NoError(json.Unmarshal(raw, &authored))
	for _, part := range authored {
		s.NoError(dndfx.ValidatePart(&dndfx.ValidatePartInput{Declaration: part}))
	}
}

func (s *typedDefinitionsSuite) TestNilAndTypedNilNeverPanic() {
	for _, effect := range []dndfx.Definition{nil, (*dndfx.AddToRoll)(nil), (*dndfx.EndOn)(nil), (*dndfx.Concentration)(nil)} {
		s.NotPanics(func() {
			out, err := dndfx.CompileParts(&dndfx.CompilePartsInput{Parts: []dndfx.PartSpec{{Key: "invalid", Effect: effect, OnTerminal: generic.EndPart}}})
			s.Error(err)
			s.Nil(out)
		})
	}
}

func (s *typedDefinitionsSuite) TestMalformedAndDuplicatePartsRefuse() {
	out, err := dndfx.CompileParts(nil)
	s.Require().Error(err)
	s.Nil(out)
	for _, bad := range []dndfx.PartSpec{
		{Key: "", Effect: dndfx.Concentration{}, OnTerminal: generic.EndApplication},
		{Key: "bad", Effect: dndfx.AddToRoll{}, OnTerminal: generic.EndPart},
		{Key: "bad", Effect: dndfx.Resistance{}, OnTerminal: generic.EndPart},
		{Key: "bad", Effect: dndfx.Advantage{On: dndfx.AdvantageOnSavingThrows, Side: dndfx.AttackByHolder}, OnTerminal: generic.EndPart},
		{Key: "bad", Effect: dndfx.EndOn{Beat: dndfx.BeatRest, Reason: "rest"}, OnTerminal: generic.EndPart},
		typedRollSpec(),
	} {
		result, err := dndfx.CompileParts(&dndfx.CompilePartsInput{Parts: []dndfx.PartSpec{typedRollSpec(), bad}})
		s.Error(err)
		s.Nil(result, "a malformed later entry returns no partial parts")
	}
}

func (s *typedDefinitionsSuite) TestReturnedValuesAreOwned() {
	steps := map[int]int{1: 2, 9: 3, 16: 4}
	guard := abilityGuard("str")
	types := []damage.Type{damage.Slashing}
	input := &dndfx.CompilePartsInput{Parts: []dndfx.PartSpec{
		{Key: "damage", Effect: dndfx.DamageBonus{Amount: generic.Amount{ByClassLevel: &generic.ClassSteps{Class: "barbarian", Steps: steps}}, When: guard}, OnTerminal: generic.EndPart},
		{Key: "resistance", Effect: dndfx.Resistance{Types: types}, OnTerminal: generic.EndPart},
	}}
	out, err := dndfx.CompileParts(input)
	s.Require().NoError(err)
	steps[1] = 99
	guard.Fact.Value = "dex"
	types[0] = damage.Fire
	s.Equal(2, out.Parts[0].Amount.ByClassLevel.Steps[1])
	s.Equal("str", out.Parts[0].When.Fact.Value)
	s.JSONEq(`{"types":["slashing"]}`, string(out.Parts[1].Params))
	outputs, err := dndfx.CompileOutputs(&dndfx.CompileOutputsInput{Parts: out.Parts})
	s.Require().NoError(err)
	out.Parts[0].Amount.ByClassLevel.Steps[1] = 88
	out.Parts[0].When.Fact.Value = "wis"
	out.Parts[1].Params[0] = '!'
	s.Equal(2, outputs.Outputs[0].Amount.ByClassLevel.Steps[1])
	s.Equal("str", outputs.Outputs[0].When.Fact.Value)
	s.JSONEq(`{"types":["slashing"]}`, string(outputs.Outputs[1].Params))
}

func (s *typedDefinitionsSuite) TestCompileOutputsRejectsMaintenance() {
	parts, err := dndfx.CompileParts(&dndfx.CompilePartsInput{Parts: []dndfx.PartSpec{typedRollSpec(), {Key: "hold", Effect: dndfx.Concentration{}, OnTerminal: generic.EndApplication}}})
	s.Require().NoError(err)
	out, err := dndfx.CompileOutputs(&dndfx.CompileOutputsInput{Parts: parts.Parts})
	s.Require().Error(err)
	s.Nil(out, "maintenance cannot be silently omitted from a payload-only recipe")
	out, err = dndfx.CompileOutputs(nil)
	s.Error(err)
	s.Nil(out)
	out, err = dndfx.CompileOutputs(&dndfx.CompileOutputsInput{Parts: []dndfx.PartDeclaration{parts.Parts[0], parts.Parts[0]}})
	s.Error(err, "raw canonical declarations must not bypass duplicate-key validation")
	s.Nil(out)
}

func (s *typedDefinitionsSuite) TestRageLeafOutputsUseExistingMeanings() {
	bonus := 2
	strength := abilityGuard("str")
	parts, err := dndfx.CompileParts(&dndfx.CompilePartsInput{Parts: []dndfx.PartSpec{
		{Key: "damage", Effect: dndfx.DamageBonus{Amount: generic.Amount{Fixed: &bonus}, When: strength}, OnTerminal: generic.EndPart},
		{Key: "resistance", Effect: dndfx.Resistance{Types: []damage.Type{damage.Bludgeoning, damage.Piercing, damage.Slashing}}, OnTerminal: generic.EndPart},
		{Key: "save", Effect: dndfx.Advantage{On: dndfx.AdvantageOnSavingThrows, When: strength}, OnTerminal: generic.EndPart},
		{Key: "check", Effect: dndfx.Advantage{On: dndfx.AdvantageOnAbilityChecks, When: strength}, OnTerminal: generic.EndPart},
	}})
	s.Require().NoError(err)
	outputs, err := dndfx.CompileOutputs(&dndfx.CompileOutputsInput{Parts: parts.Parts})
	s.Require().NoError(err)
	s.Require().Len(outputs.Outputs, 4)
	s.Equal(dndfx.KindDamageBonus, outputs.Outputs[0].Kind)
	s.Equal(2, *outputs.Outputs[0].Amount.Fixed)
	s.Equal(dndfx.KindResistance, outputs.Outputs[1].Kind)
	s.JSONEq(`{"types":["bludgeoning","piercing","slashing"]}`, string(outputs.Outputs[1].Params))
	s.Equal(dndfx.KindAdvantage, outputs.Outputs[2].Kind)
	s.JSONEq(`{"on":"saving_throw"}`, string(outputs.Outputs[2].Params))
	s.Equal(dndfx.KindAdvantage, outputs.Outputs[3].Kind)
	s.JSONEq(`{"on":"ability_check"}`, string(outputs.Outputs[3].Params))
	for _, i := range []int{0, 2, 3} {
		s.Equal("str", outputs.Outputs[i].When.Fact.Value)
	}
	maintenance, err := dndfx.CompileParts(&dndfx.CompilePartsInput{Parts: []dndfx.PartSpec{
		{Key: "hold", Effect: dndfx.Concentration{}, OnTerminal: generic.EndApplication},
		{Key: "empty", Effect: dndfx.UntilEmpty{}, OnTerminal: generic.EndApplication},
	}})
	s.Require().NoError(err)
	s.Equal(dndfx.KindUntilEmpty, maintenance.Parts[1].Kind)
}

func (s *typedDefinitionsSuite) TestBareAddToRollDoesNotEndOnRest() {
	compiled, err := dndfx.CompileParts(&dndfx.CompilePartsInput{Parts: []dndfx.PartSpec{typedRollSpec()}})
	s.Require().NoError(err)
	in := partInput("roll", dndfx.KindAddToRoll, "")
	in.Declaration = compiled.Parts[0]
	bound, err := dndfx.BindPart(in)
	s.Require().NoError(err)
	composed, err := generic.Compose(bound.Part.Compose)
	s.Require().NoError(err)
	ctx, bus := context.Background(), events.NewEventBus()
	s.Require().NoError(composed.Apply(ctx, bus))
	defer func() { s.NoError(composed.Remove(ctx, bus)) }()
	s.Require().NoError(dndevents.RestTopic.On(bus).Publish(ctx, dndevents.RestEvent{CharacterID: in.HolderID}))
	s.True(composed.IsApplied())
	s.Require().Len(composed.CurrentOutputs(), 1)
	answer, err := dndfx.DescribeRoll(&dndfx.DescribeRollInput{Output: composed.CurrentOutputs()[0], Origin: in.Origin, DisplayName: in.DisplayName, Request: &dndevents.DescribeRollContributionsInput{Kind: contributions.RollKindAttack}})
	s.Require().NoError(err)
	s.True(answer.Applicable)
	s.Equal("1d4", answer.Contributions[0].Dice)
}
