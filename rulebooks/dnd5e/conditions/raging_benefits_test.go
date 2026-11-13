// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type ragingBenefitsSuite struct{ suite.Suite }

func TestRagingBenefitsSuite(t *testing.T) { suite.Run(t, new(ragingBenefitsSuite)) }

func (s *ragingBenefitsSuite) TestSharedNativeOutputs() {
	fixed := 5
	bundle, err := RagingBenefits(&RagingBenefitsInput{DamageBonus: RagingDamageBonus{Fixed: &fixed}})
	s.Require().NoError(err)
	compiled, err := dndfx.CompileParts(&dndfx.CompilePartsInput{Parts: bundle.Parts})
	s.Require().NoError(err)
	outputs, err := dndfx.CompileOutputs(&dndfx.CompileOutputsInput{Parts: compiled.Parts})
	s.Require().NoError(err)
	native, err := ragingRecipe(generic.Amount{Fixed: &fixed})
	s.Require().NoError(err)
	s.Require().Len(native.States[ragingFresh].Outputs, 4)
	s.Require().Len(outputs.Outputs, 4)
	for i, raw := range native.States[ragingFresh].Outputs {
		authored, err := json.Marshal(outputs.Outputs[i])
		s.Require().NoError(err)
		s.JSONEq(string(raw), string(authored))
	}
	s.Equal(5, *outputs.Outputs[0].Amount.Fixed)
	s.JSONEq(`{"types":["bludgeoning","piercing","slashing"]}`, string(outputs.Outputs[1].Params))
	s.JSONEq(`{"on":"saving_throw"}`, string(outputs.Outputs[2].Params))
	s.JSONEq(`{"on":"ability_check"}`, string(outputs.Outputs[3].Params))
}

func (s *ragingBenefitsSuite) boundDamage(choice RagingDamageBonus) *generic.Bound {
	bundle, err := RagingBenefits(&RagingBenefitsInput{DamageBonus: choice})
	s.Require().NoError(err)
	compiled, err := dndfx.CompileParts(&dndfx.CompilePartsInput{Parts: bundle.Parts})
	s.Require().NoError(err)
	bound, err := dndfx.BindPart(&dndfx.BindPartInput{Declaration: compiled.Parts[0], Ref: *refs.Conditions.LinkedEffects(), HolderID: "barb", ApplicationID: "grant", DisplayName: "Rage benefits", Origin: dndevents.ConditionOrigin{Ref: *refs.Features.Rage(), EntityID: "barb"}})
	s.Require().NoError(err)
	composed, err := generic.Compose(bound.Part.Compose)
	s.Require().NoError(err)
	s.Require().Len(composed.CurrentOutputs(), 1)
	return composed.CurrentOutputs()[0]
}

func (s *ragingBenefitsSuite) TestFixedAndLiveScaling() {
	fixed := 2
	fixedOutput := s.boundDamage(RagingDamageBonus{Fixed: &fixed})
	frame := barbarianFrame()
	frame.ActorClassLevels = contributions.UnknownClassLevels()
	amount, err := fixedOutput.Amount(dndfx.Query{Action: &frame})
	s.Require().NoError(err)
	s.Require().NotNil(amount.Scalar)
	s.Equal(2, *amount.Scalar)
	live := s.boundDamage(RagingDamageBonus{BarbarianLevel: true})
	for _, tc := range []struct{ level, want int }{{1, 2}, {9, 3}, {16, 4}} {
		framed := barbarianFrameAt(tc.level)
		amount, err = live.Amount(dndfx.Query{Action: &framed})
		s.Require().NoError(err)
		s.Require().NotNil(amount.Scalar)
		s.Equal(tc.want, *amount.Scalar)
	}
	_, err = live.Amount(dndfx.Query{Action: &frame})
	s.Error(err, "live class scaling cannot invent a missing level")
}

func (s *ragingBenefitsSuite) TestNoMaintenanceInBenefits() {
	out, err := RagingBenefits(&RagingBenefitsInput{DamageBonus: RagingDamageBonus{BarbarianLevel: true}})
	s.Require().NoError(err)
	parts, err := dndfx.CompileParts(&dndfx.CompilePartsInput{Parts: out.Parts})
	s.Require().NoError(err)
	for _, part := range parts.Parts {
		checked, err := dndfx.ValidatePartTemplate(&dndfx.ValidatePartTemplateInput{Template: dndfx.PartTemplate{Kind: part.Kind, Params: part.Params, When: part.When, Amount: part.Amount}})
		s.Require().NoError(err)
		s.Equal(dndfx.PartClassBenefit, checked.Class)
		s.Equal(generic.EndPart, part.OnTerminal)
		s.False(checked.Capabilities.Concentration)
		s.False(checked.Capabilities.Duration)
	}
}

func (s *ragingBenefitsSuite) TestInputMutationCannotChangeAnotherBundle() {
	bonus := 2
	a, err := RagingBenefits(&RagingBenefitsInput{DamageBonus: RagingDamageBonus{Fixed: &bonus}})
	s.Require().NoError(err)
	b, err := RagingBenefits(&RagingBenefitsInput{DamageBonus: RagingDamageBonus{Fixed: &bonus}})
	s.Require().NoError(err)
	bonus = 99
	damageEffect := a.Parts[0].Effect.(dndfx.DamageBonus)
	s.Equal(2, *damageEffect.Amount.Fixed)
	*damageEffect.Amount.Fixed = 88
	saveEffect := a.Parts[2].Effect.(dndfx.Advantage)
	saveEffect.When.Fact.Value = "dex"
	checkEffect := a.Parts[3].Effect.(dndfx.Advantage)
	s.Equal("str", checkEffect.When.Fact.Value, "changing one leaf must not rewrite its sibling")
	compiled, err := dndfx.CompileParts(&dndfx.CompilePartsInput{Parts: b.Parts})
	s.Require().NoError(err)
	s.Equal(2, *compiled.Parts[0].Amount.Fixed)
	s.Equal("str", compiled.Parts[2].When.Fact.Value)
}

func (s *ragingBenefitsSuite) TestInvalidStrategyRefuses() {
	out, err := RagingBenefits(nil)
	s.Error(err)
	s.Nil(out)
	zero, negative, positive := 0, -1, 2
	for _, choice := range []RagingDamageBonus{{}, {Fixed: &zero}, {Fixed: &negative}, {Fixed: &positive, BarbarianLevel: true}} {
		out, err = RagingBenefits(&RagingBenefitsInput{DamageBonus: choice})
		s.Error(err)
		s.Nil(out)
	}
}
