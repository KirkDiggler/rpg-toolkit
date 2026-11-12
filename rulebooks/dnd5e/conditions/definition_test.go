// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later
package conditions_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	fx "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/stretchr/testify/suite"
)

const definitionFixture = `{
 "parts":{
  "damage":{"kind":"damage_bonus","amount":{"fixed":2},"when":{"fact":{"name":"action.ability","op":"eq","value":"str"}}},
  "ward":{"kind":"resistance","params":{"types":["slashing"]}},
  "hold":{"kind":"concentration"},
  "activity":{"kind":"combat_activity","params":{"skip_first":true}}
 },
 "benefit_sets":{"rage":["damage","ward"]},
 "definitions":{
  "ring":{"name":"Rage ring","coordinator":"user","maintenance":[{"key":"hold","part":"hold"}],"benefits":[{"key":"rage","set":"rage"}]},
  "scroll":{"name":"Rage scroll","coordinator":"recipient","maintenance":[{"key":"activity","part":"activity"}],"benefits":[{"key":"rage","set":"rage"}]}
 }
}`

type definitionCompilerSuite struct{ suite.Suite }

func TestDefinitionCompilerSuite(t *testing.T) { suite.Run(t, new(definitionCompilerSuite)) }
func (s *definitionCompilerSuite) compile(raw string) *conditions.CompiledDefinitions {
	out, err := conditions.CompileDefinitions(&conditions.CompileDefinitionsInput{Data: json.RawMessage(raw)})
	s.Require().NoError(err)
	return out.Definitions
}
func definitionOrigin() dndevents.ConditionOrigin {
	return dndevents.ConditionOrigin{Ref: core.Ref{Module: "example", Type: "items", ID: "ring"}, EntityID: "item-1"}
}
func (s *definitionCompilerSuite) bind(defs *conditions.CompiledDefinitions, id, user, target, application string) *conditions.BindDefinitionOutput {
	out, err := conditions.BindDefinition(&conditions.BindDefinitionInput{Definitions: defs, DefinitionID: id, UserID: user, RecipientID: target, ApplicationID: application, Origin: definitionOrigin()})
	s.Require().NoError(err)
	return out
}
func (s *definitionCompilerSuite) TestExpandsOneSetIntoRingAndScroll() {
	defs := s.compile(definitionFixture)
	ring := s.bind(defs, "ring", "A", "B", "ring-use")
	scroll := s.bind(defs, "scroll", "A", "B", "scroll-use")
	s.Equal("A", ring.CoordinatorID)
	s.Equal("B", scroll.CoordinatorID)
	s.Require().Len(ring.Spec.Recipients, 1)
	parts := ring.Spec.Recipients[0].Parts
	s.Require().Len(parts, 2)
	s.Equal("rage/damage", parts[0].Key)
	s.Equal("rage/ward", parts[1].Key)
	s.Equal(2, *parts[0].Amount.Fixed)
	s.Equal(fx.EndPart, parts[0].OnTerminal)
	s.Equal(fx.EndApplication, ring.Spec.CoordinatorParts[0].OnTerminal)
	s.Equal(parts, scroll.Spec.Recipients[0].Parts)
	s.Equal(dndfx.KindConcentration, ring.Spec.CoordinatorParts[0].Kind)
	s.Equal(dndfx.KindCombatActivity, scroll.Spec.CoordinatorParts[0].Kind)
}
func (s *definitionCompilerSuite) TestStrictWholeLibraryValidation() {
	for name, raw := range map[string]string{
		"unknown field":       strings.Replace(definitionFixture, `"name":"Rage ring"`, `"name":"Rage ring","surprise":true`, 1),
		"duplicate json":      strings.Replace(definitionFixture, `"fixed":2`, `"fixed":2,"fixed":3`, 1),
		"unknown ref":         strings.Replace(definitionFixture, `"part":"hold"`, `"part":"missing"`, 1),
		"nested set":          strings.Replace(definitionFixture, `"rage":["damage","ward"]`, `"rage":["another"],"another":["damage"]`, 1),
		"maintenance benefit": strings.Replace(definitionFixture, `"rage":["damage","ward"]`, `"rage":["hold"]`, 1),
		"benefit maintenance": strings.Replace(definitionFixture, `"part":"hold"`, `"part":"damage"`, 1),
		"duplicate leaf":      strings.Replace(definitionFixture, `"rage":["damage","ward"]`, `"rage":["damage","damage"]`, 1),
		"missing name":        strings.Replace(definitionFixture, `"name":"Rage ring"`, `"name":" "`, 1),
		"missing role":        strings.Replace(definitionFixture, `"coordinator":"user"`, `"coordinator":""`, 1),
		"bad role":            strings.Replace(definitionFixture, `"coordinator":"user"`, `"coordinator":"caster"`, 1),
		"both selectors":      strings.Replace(definitionFixture, `"key":"hold","part":"hold"`, `"key":"hold","part":"hold","set":"rage"`, 1),
		"bad key":             strings.Replace(definitionFixture, `"key":"hold"`, `"key":"a/b"`, 1),
		"missing benefits":    strings.Replace(definitionFixture, `"benefits":[{"key":"rage","set":"rage"}]`, `"benefits":[]`, 1),
		"bad unused":          strings.Replace(definitionFixture, `"parts":{`, `"parts":{"unused":{"kind":"unknown"},`, 1),
		"bad unused set":      strings.Replace(definitionFixture, `"benefit_sets":{`, `"benefit_sets":{"unused":["missing"],`, 1),
		"two concentrations":  strings.Replace(definitionFixture, `"parts":{`, `"parts":{"hold2":{"kind":"concentration"},`, 1),
	} {
		if name == "two concentrations" {
			raw = strings.Replace(raw, `"maintenance":[{"key":"hold","part":"hold"}]`, `"maintenance":[{"key":"hold","part":"hold"},{"key":"hold2","part":"hold2"}]`, 1)
		}
		s.Run(name, func() {
			out, err := conditions.CompileDefinitions(&conditions.CompileDefinitionsInput{Data: json.RawMessage(raw)})
			s.Error(err)
			s.Nil(out)
		})
	}
	for _, raw := range []string{"", `null`, `{}`, `{"parts":{},"definitions":{}}`} {
		out, err := conditions.CompileDefinitions(&conditions.CompileDefinitionsInput{Data: json.RawMessage(raw)})
		s.Error(err)
		s.Nil(out)
	}
	out, err := conditions.CompileDefinitions(nil)
	s.Error(err)
	s.Nil(out)
}
func (s *definitionCompilerSuite) TestUnparameterizedMaintenanceCannotClaimAnAmount() {
	raw := strings.Replace(definitionFixture, `"kind":"concentration"`, `"kind":"concentration","amount":{"fixed":2}`, 1)
	_, err := conditions.CompileDefinitions(&conditions.CompileDefinitionsInput{Data: json.RawMessage(raw)})
	s.Require().Error(err)
	s.Contains(err.Error(), "hold")
	s.Contains(err.Error(), "amount")
}
func (s *definitionCompilerSuite) TestValidateReferenceNeedsNoFakeActors() {
	defs := s.compile(definitionFixture)
	s.NoError(conditions.ValidateDefinitionReference(&conditions.ValidateDefinitionReferenceInput{Definitions: defs, DefinitionID: "ring"}))
	s.Error(conditions.ValidateDefinitionReference(&conditions.ValidateDefinitionReferenceInput{Definitions: defs, DefinitionID: "absent"}))
	s.Error(conditions.ValidateDefinitionReference(nil))
}
func (s *definitionCompilerSuite) TestBindingRolesAndSelfUse() {
	defs := s.compile(definitionFixture)
	for _, id := range []string{"ring", "scroll"} {
		for _, target := range []string{"A", "B"} {
			out := s.bind(defs, id, "A", target, `["recipient","odd-use"]`)
			want := "A"
			if id == "scroll" {
				want = target
			}
			s.Equal(want, out.CoordinatorID)
			recipient := out.Spec.Recipients[0]
			s.Equal(target, recipient.MemberID)
			s.Equal(conditions.LifetimeShared, recipient.Lifetime)
			s.NotEqual(`["recipient","odd-use"]`, recipient.AddressSourceID)
			s.NotEqual("item-1", recipient.AddressSourceID)
			again := s.bind(defs, id, "A", target, `["recipient","odd-use"]`)
			s.Equal(recipient.AddressSourceID, again.Spec.Recipients[0].AddressSourceID)
		}
	}
}
func (s *definitionCompilerSuite) TestBindingDoesNotSpendAttachOrAlias() {
	bytes := []byte(definitionFixture)
	compiled, err := conditions.CompileDefinitions(&conditions.CompileDefinitionsInput{Data: bytes})
	s.Require().NoError(err)
	for i := range bytes {
		bytes[i] = ' '
	}
	out := s.bind(compiled.Definitions, "ring", "A", "B", "use")
	// There is no world/budget/bus in this interface; pure preflight still succeeds
	// with explicitly supplied participants after binding, without any attachment.
	prepared, err := conditions.PrepareApplication(&conditions.PrepareApplicationInput{ID: "use", CoordinatorID: out.CoordinatorID, Spec: out.Spec, Origin: definitionOrigin(), Existing: []conditions.ParticipantConditions{{MemberID: "A"}, {MemberID: "B"}}})
	s.Require().NoError(err)
	made, err := conditions.CreateRecipient(&conditions.CreateRecipientInput{Prepared: prepared, SpecIndex: 0})
	s.Require().NoError(err)
	s.False(made.Condition.IsApplied())
	for _, modify := range []func(*conditions.BindDefinitionInput){func(i *conditions.BindDefinitionInput) { i.UserID = "" }, func(i *conditions.BindDefinitionInput) { i.RecipientID = "" }, func(i *conditions.BindDefinitionInput) { i.ApplicationID = "" }, func(i *conditions.BindDefinitionInput) { i.Origin.EntityID = "" }, func(i *conditions.BindDefinitionInput) { i.Origin.Ref = core.Ref{} }, func(i *conditions.BindDefinitionInput) { i.Definitions = nil }} {
		in := conditions.BindDefinitionInput{Definitions: compiled.Definitions, DefinitionID: "ring", UserID: "A", RecipientID: "B", ApplicationID: "use", Origin: definitionOrigin()}
		modify(&in)
		result, err := conditions.BindDefinition(&in)
		s.Error(err)
		s.Nil(result)
	}
}
func (s *definitionCompilerSuite) TestReturnedSpecMutationCannotChangeLibrary() {
	defs := s.compile(definitionFixture)
	a := s.bind(defs, "ring", "A", "B", "first")
	a.Spec.Recipients[0].Parts[0].When.Fact.Value = "dex"
	*a.Spec.Recipients[0].Parts[0].Amount.Fixed = 99
	a.Spec.Recipients[0].Parts[1].Params[0] = '['
	a.Spec.CoordinatorParts[0].Kind = "bad"
	b := s.bind(defs, "ring", "A", "B", "second")
	s.Equal("str", b.Spec.Recipients[0].Parts[0].When.Fact.Value)
	s.Equal(2, *b.Spec.Recipients[0].Parts[0].Amount.Fixed)
	s.JSONEq(`{"types":["slashing"]}`, string(b.Spec.Recipients[0].Parts[1].Params))
	s.Equal(dndfx.KindConcentration, b.Spec.CoordinatorParts[0].Kind)
}
