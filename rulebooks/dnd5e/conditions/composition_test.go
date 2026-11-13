// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type compositionCompilerSuite struct{ suite.Suite }

func TestCompositionCompilerSuite(t *testing.T) { suite.Run(t, new(compositionCompilerSuite)) }

func typedWardSpec() CompositionSpec {
	return CompositionSpec{Name: "Ward", Coordinator: DefinitionUser,
		Benefits:    []dndfx.PartSpec{{Key: "ward", Effect: dndfx.Resistance{Types: []damage.Type{damage.Slashing}}, OnTerminal: generic.EndPart}},
		Maintenance: []dndfx.PartSpec{{Key: "hold", Effect: dndfx.Concentration{}, OnTerminal: generic.EndApplication}, {Key: "empty", Effect: dndfx.UntilEmpty{}, OnTerminal: generic.EndApplication}},
	}
}
func (s *compositionCompilerSuite) bind(definitions *CompiledDefinitions) *BindDefinitionOutput {
	out, err := BindDefinition(&BindDefinitionInput{Definitions: definitions, DefinitionID: "ring", UserID: "user", RecipientID: "recipient", ApplicationID: "application", Origin: dndevents.ConditionOrigin{Ref: *refs.Features.Rage(), EntityID: "user"}})
	s.Require().NoError(err)
	return out
}

func (s *compositionCompilerSuite) TestCodeAndLibraryBindEquivalentSpec() {
	code, err := CompileComposition(&CompileCompositionInput{ID: "ring", Spec: typedWardSpec()})
	s.Require().NoError(err)
	data, err := CompileDefinitions(&CompileDefinitionsInput{Data: json.RawMessage(`{
  "parts":{"ward-leaf":{"kind":"resistance","params":{"types":["slashing"]}},"hold-leaf":{"kind":"concentration"},"empty-leaf":{"kind":"until_empty"}},
  "definitions":{"ring":{"name":"Ward","coordinator":"user","maintenance":[{"key":"hold","part":"hold-leaf"},{"key":"empty","part":"empty-leaf"}],"benefits":[{"key":"ward","part":"ward-leaf"}]}}
 }`)})
	s.Require().NoError(err)
	fromCode, fromData := s.bind(code.Definitions), s.bind(data.Definitions)
	s.Equal(fromData, fromCode)
	s.Equal("user", fromCode.CoordinatorID)
	s.Equal("recipient", fromCode.Spec.Recipients[0].MemberID)
	s.Equal(dndfx.KindResistance, fromCode.Spec.Recipients[0].Parts[0].Kind)
	s.JSONEq(`{"types":["slashing"]}`, string(fromCode.Spec.Recipients[0].Parts[0].Params))
}

func (s *compositionCompilerSuite) TestRejectsMisplacedOrDuplicateParts() {
	out, err := CompileComposition(nil)
	s.Error(err)
	s.Nil(out)
	for _, tc := range []struct {
		name   string
		change func(*CompileCompositionInput)
	}{
		{"missing-id", func(in *CompileCompositionInput) { in.ID = "" }},
		{"missing-name", func(in *CompileCompositionInput) { in.Spec.Name = "" }},
		{"unknown-role", func(in *CompileCompositionInput) { in.Spec.Coordinator = "other" }},
		{"no-benefits", func(in *CompileCompositionInput) { in.Spec.Benefits = nil }},
		{"no-maintenance", func(in *CompileCompositionInput) { in.Spec.Maintenance = nil }},
		{"duplicate-benefit-key", func(in *CompileCompositionInput) { in.Spec.Benefits = append(in.Spec.Benefits, in.Spec.Benefits[0]) }},
		{"benefit-in-maintenance", func(in *CompileCompositionInput) { in.Spec.Maintenance = []dndfx.PartSpec{in.Spec.Benefits[0]} }},
		{"maintenance-in-benefits", func(in *CompileCompositionInput) { in.Spec.Benefits = []dndfx.PartSpec{in.Spec.Maintenance[0]} }},
		{"second-concentration", func(in *CompileCompositionInput) {
			in.Spec.Maintenance = append(in.Spec.Maintenance, dndfx.PartSpec{Key: "second-hold", Effect: dndfx.Concentration{}, OnTerminal: generic.EndApplication})
		}},
		{"local-instead-of-application", func(in *CompileCompositionInput) {
			in.Spec.Maintenance = []dndfx.PartSpec{{Key: "rest", Effect: dndfx.EndOn{Beat: dndfx.BeatRest, Reason: "rest"}, OnTerminal: generic.EndAttachment}}
		}},
	} {
		s.Run(tc.name, func() {
			in := &CompileCompositionInput{ID: "ring", Spec: typedWardSpec()}
			tc.change(in)
			out, err := CompileComposition(in)
			s.Error(err)
			s.Nil(out)
		})
	}
}

func (s *compositionCompilerSuite) TestCompilationIsPureAndOwned() {
	in := &CompileCompositionInput{ID: "ring", Spec: typedWardSpec()}
	compiled, err := CompileComposition(in)
	s.Require().NoError(err)
	in.Spec.Name = "Changed"
	resistance := in.Spec.Benefits[0].Effect.(dndfx.Resistance)
	resistance.Types[0] = damage.Fire
	in.Spec.Maintenance[0].Key = "mutated"
	first := s.bind(compiled.Definitions)
	s.Equal("Ward", first.Spec.Name)
	s.Equal("hold", first.Spec.CoordinatorParts[0].Key)
	s.JSONEq(`{"types":["slashing"]}`, string(first.Spec.Recipients[0].Parts[0].Params))
	first.Spec.Recipients[0].Parts[0].Params[0] = '!'
	second := s.bind(compiled.Definitions)
	s.JSONEq(`{"types":["slashing"]}`, string(second.Spec.Recipients[0].Parts[0].Params))
	s.NoError(ValidateDefinitionReference(&ValidateDefinitionReferenceInput{Definitions: compiled.Definitions, DefinitionID: "ring"}), "a compiled definition needs no actor/world lookup")
}

func (s *compositionCompilerSuite) TestMaintenanceChoiceDoesNotMutateBenefits() {
	spec := typedWardSpec()
	maintained, err := CompileComposition(&CompileCompositionInput{ID: "ring", Spec: spec})
	s.Require().NoError(err)
	spec.Maintenance = []dndfx.PartSpec{{Key: "next-turn", Effect: dndfx.EndOn{Beat: dndfx.BeatTurnStart, Reason: "turn_started"}, OnTerminal: generic.EndApplication}}
	nextTurn, err := CompileComposition(&CompileCompositionInput{ID: "ring", Spec: spec})
	s.Require().NoError(err)
	a, b := s.bind(maintained.Definitions), s.bind(nextTurn.Definitions)
	s.Equal(a.Spec.Recipients[0].Parts, b.Spec.Recipients[0].Parts)
	s.Equal(dndfx.KindConcentration, a.Spec.CoordinatorParts[0].Kind)
	s.Equal(dndfx.KindEndOn, b.Spec.CoordinatorParts[0].Kind)
	b.Spec.CoordinatorParts[0].Params[0] = '!'
	again := s.bind(maintained.Definitions)
	s.Equal(dndfx.KindConcentration, again.Spec.CoordinatorParts[0].Kind)
}
