// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions_test

import (
	"context"
	"encoding/json"
	"testing"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/definitions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/stretchr/testify/suite"
)

type applicationDefinitionSuite struct{ suite.Suite }

func TestApplicationDefinitionSuite(t *testing.T) { suite.Run(t, new(applicationDefinitionSuite)) }

func applicationDefinitionFixture() conditions.ApplicationDefinition {
	return conditions.ApplicationDefinition{
		Name: "Independent ward", Coordinator: conditions.DefinitionUser,
		Definition: effects.Composition{Members: []effects.Member{
			{Key: "payload", Definition: effects.Resistance{Types: []damage.Type{damage.Slashing}}},
			{Key: "upkeep", Definition: effects.Concentration{}},
			{Key: "expiry", Definition: effects.RoundDuration{Rounds: 3, ExpiredReason: "expired", ResetReason: "clock_reset"}},
		}},
		Placements: []conditions.ApplicationPlacement{
			{Key: "ward", Path: []string{"payload"}, Attachment: conditions.RecipientAttachment, OnTerminal: generic.EndPart},
			{Key: "hold", Path: []string{"upkeep"}, Attachment: conditions.CoordinatorAttachment, OnTerminal: generic.EndApplication},
			{Key: "limit", Path: []string{"expiry"}, Attachment: conditions.RecipientAttachment, OnTerminal: generic.EndAttachment},
		},
	}
}
func (s *applicationDefinitionSuite) compile(def conditions.ApplicationDefinition) *conditions.CompiledApplicationDefinition {
	out, err := conditions.CompileApplicationDefinition(&conditions.CompileApplicationDefinitionInput{Definition: def})
	s.Require().NoError(err)
	return out.Definition
}
func (s *applicationDefinitionSuite) bind(def *conditions.CompiledApplicationDefinition, target string) *conditions.BindDefinitionOutput {
	out, err := conditions.BindApplicationDefinition(&conditions.BindApplicationDefinitionInput{Definition: def, UserID: "A", RecipientID: target, ApplicationID: `["recipient","use"]`, Origin: definitionOrigin()})
	s.Require().NoError(err)
	return out
}
func (s *applicationDefinitionSuite) TestDirectValuesNeedNoCatalogueAndPreserveAttachmentRoles() {
	for _, role := range []conditions.DefinitionRole{conditions.DefinitionUser, conditions.DefinitionRecipient} {
		for _, target := range []string{"A", "B"} {
			def := applicationDefinitionFixture()
			def.Coordinator = role
			compiled := s.compile(def)
			bound := s.bind(compiled, target)
			expected := "A"
			if role == conditions.DefinitionRecipient {
				expected = target
			}
			s.Equal(expected, bound.CoordinatorID)
			s.Require().Len(bound.Spec.CoordinatorParts, 1)
			s.Equal(effects.KindConcentration, bound.Spec.CoordinatorParts[0].Kind)
			s.Require().Len(bound.Spec.Recipients, 1)
			recipient := bound.Spec.Recipients[0]
			s.Equal(target, recipient.MemberID)
			s.NotEqual(`["recipient","use"]`, recipient.AddressSourceID, "self-use must not collapse the coordinator and recipient addresses")
			s.Equal(conditions.LifetimeShared, recipient.Lifetime)
			s.Require().Len(recipient.Parts, 2)
			s.Equal("ward", recipient.Parts[0].Key)
			s.Equal(generic.EndAttachment, recipient.Parts[1].OnTerminal, "recipient expiry is explicit, not forced into coordinator upkeep")
			holders := []conditions.ParticipantConditions{{MemberID: "A"}}
			if target != "A" {
				holders = append(holders, conditions.ParticipantConditions{MemberID: target})
			}
			prepared, err := conditions.PrepareApplication(&conditions.PrepareApplicationInput{ID: `["recipient","use"]`, CoordinatorID: bound.CoordinatorID, Origin: definitionOrigin(), Spec: bound.Spec, Existing: holders})
			s.Require().NoError(err)
			owner, err := conditions.CreateApplication(&conditions.CreateApplicationInput{Prepared: prepared})
			s.Require().NoError(err)
			payload, err := conditions.CreateRecipient(&conditions.CreateRecipientInput{Prepared: prepared, SpecIndex: 0})
			s.Require().NoError(err)
			s.False(owner.Condition.IsApplied())
			s.False(payload.Condition.IsApplied())
		}
	}
}
func (s *applicationDefinitionSuite) TestCompilationOwnsValuesAndBindingReturnsIndependentSpecs() {
	def := applicationDefinitionFixture()
	compiled := s.compile(def)
	def.Placements[0].Path[0] = "absent"
	def.Definition.(effects.Composition).Members[0].Definition.(effects.Resistance).Types[0] = damage.Fire
	first := s.bind(compiled, "B")
	s.JSONEq(`{"types":["slashing"]}`, string(first.Spec.Recipients[0].Parts[0].Params))
	first.Spec.Recipients[0].Parts[0].Params[0] = '!'
	first.Spec.CoordinatorParts[0].Key = "changed"
	again := s.bind(compiled, "B")
	s.JSONEq(`{"types":["slashing"]}`, string(again.Spec.Recipients[0].Parts[0].Params))
	s.Equal("hold", again.Spec.CoordinatorParts[0].Key)
}
func (s *applicationDefinitionSuite) TestPlacementRefusesIncompleteOrImpossibleCustody() {
	for name, change := range map[string]func(*conditions.ApplicationDefinition){
		"name":                             func(d *conditions.ApplicationDefinition) { d.Name = " " },
		"role":                             func(d *conditions.ApplicationDefinition) { d.Coordinator = "actor" },
		"missing definition":               func(d *conditions.ApplicationDefinition) { d.Definition = nil },
		"unknown attachment":               func(d *conditions.ApplicationDefinition) { d.Placements[0].Attachment = "holder" },
		"unknown path":                     func(d *conditions.ApplicationDefinition) { d.Placements[0].Path = []string{"absent"} },
		"uncovered":                        func(d *conditions.ApplicationDefinition) { d.Placements = d.Placements[:2] },
		"overlap":                          func(d *conditions.ApplicationDefinition) { d.Placements[0].Path = nil },
		"duplicate key":                    func(d *conditions.ApplicationDefinition) { d.Placements[1].Key = d.Placements[0].Key },
		"recipient cannot end application": func(d *conditions.ApplicationDefinition) { d.Placements[2].OnTerminal = generic.EndApplication },
		"coordinator cannot abandon members": func(d *conditions.ApplicationDefinition) {
			d.Placements[2].Attachment = conditions.CoordinatorAttachment
		},
		"concentration cannot move to recipient": func(d *conditions.ApplicationDefinition) { d.Placements[1].Attachment = conditions.RecipientAttachment },
		"two concentration claims": func(d *conditions.ApplicationDefinition) {
			d.Definition.(effects.Composition).Members[2].Definition = effects.Concentration{}
			d.Placements[2].Attachment = conditions.CoordinatorAttachment
			d.Placements[2].OnTerminal = generic.EndApplication
		},
		"no implicit upkeep": func(d *conditions.ApplicationDefinition) {
			d.Definition = effects.Resistance{Types: []damage.Type{damage.Slashing}}
			d.Placements = []conditions.ApplicationPlacement{{Key: "root", Attachment: conditions.RecipientAttachment, OnTerminal: generic.EndPart}}
		},
		"empty composition remains host-invalid": func(d *conditions.ApplicationDefinition) {
			d.Definition = effects.Composition{Members: []effects.Member{}}
			d.Placements = nil
		},
	} {
		s.Run(name, func() {
			def := applicationDefinitionFixture()
			change(&def)
			out, err := conditions.CompileApplicationDefinition(&conditions.CompileApplicationDefinitionInput{Definition: def})
			s.Error(err)
			s.Nil(out)
		})
	}
	out, err := conditions.CompileApplicationDefinition(nil)
	s.Error(err)
	s.Nil(out)
}
func (s *applicationDefinitionSuite) TestGroupMountsUseAuthoredOrderNotMountOrder() {
	def := applicationDefinitionFixture()
	def.Definition = effects.Composition{Members: []effects.Member{
		{Key: "bundle", Definition: effects.Composition{Members: []effects.Member{{Key: "second", Definition: effects.Resistance{Types: []damage.Type{damage.Cold}}}, {Key: "first", Definition: effects.Resistance{Types: []damage.Type{damage.Fire}}}}}},
		{Key: "hold", Definition: effects.Concentration{}},
	}}
	def.Placements = []conditions.ApplicationPlacement{
		{Key: "maintainer", Path: []string{"hold"}, Attachment: conditions.CoordinatorAttachment, OnTerminal: generic.EndApplication},
		{Key: "grant", Path: []string{"bundle"}, Attachment: conditions.RecipientAttachment, OnTerminal: generic.EndPart},
	}
	bound := s.bind(s.compile(def), "B")
	s.Equal("grant/second", bound.Spec.Recipients[0].Parts[0].Key)
	s.Equal("grant/first", bound.Spec.Recipients[0].Parts[1].Key)
}
func (s *applicationDefinitionSuite) TestRecipientExpiryUsesExistingGenerationCleanupAfterReload() {
	w, err := newDefinitionWorld()
	s.Require().NoError(err)
	defer func() { s.NoError(w.close()) }()
	def := applicationDefinitionFixture()
	def.Definition.(effects.Composition).Members[1].Definition = effects.Composition{Members: []effects.Member{
		{Key: "concentration", Definition: effects.Concentration{}},
		{Key: "until-empty", Definition: effects.UntilEmpty{}},
	}}
	bound := s.bind(s.compile(def), "B")
	prepared, err := conditions.PrepareApplication(&conditions.PrepareApplicationInput{ID: `["recipient","use"]`, CoordinatorID: bound.CoordinatorID, Origin: definitionOrigin(), Spec: bound.Spec, Existing: bindingParticipants(w.sheets)})
	s.Require().NoError(err)
	s.Require().NoError(w.apply(prepared))
	s.Require().NoError(dndevents.TurnEndTopic.On(w.bus).Publish(context.Background(), dndevents.TurnEndEvent{SubjectID: "B", Round: 4}))
	s.Require().NoError(w.reload())
	s.Len(w.holder("A").GetConditions(), 1)
	s.Len(w.holder("B").GetConditions(), 1)
	s.Require().NoError(dndevents.TurnEndTopic.On(w.bus).Publish(context.Background(), dndevents.TurnEndEvent{SubjectID: "B", Round: 6}))
	s.Empty(w.holder("B").GetConditions(), "recipient-local expiry removes its exact delivered generation")
	s.Empty(w.holder("A").GetConditions(), "the existing last-member lifecycle ends the owner")
}

func (s *applicationDefinitionSuite) TestBorrowedUpkeepRestoreCannotRenewActivityGrace() {
	raging, err := definitions.Raging(&definitions.RagingInput{DamageBonus: definitions.RagingDamageBonus{BarbarianLevel: true}})
	s.Require().NoError(err)
	def := conditions.ApplicationDefinition{Name: "Active ward", Coordinator: conditions.DefinitionUser,
		Definition: effects.Composition{Members: []effects.Member{{Key: "ward", Definition: raging.Benefits.PhysicalResistance}, {Key: "upkeep", Definition: raging.Maintenance}}},
		Placements: []conditions.ApplicationPlacement{{Key: "ward", Path: []string{"ward"}, Attachment: conditions.RecipientAttachment, OnTerminal: generic.EndPart}, {Key: "upkeep", Path: []string{"upkeep"}, Attachment: conditions.CoordinatorAttachment, OnTerminal: generic.EndApplication}},
	}
	bound := s.bind(s.compile(def), "B")
	prepared, err := conditions.PrepareApplication(&conditions.PrepareApplicationInput{ID: `["recipient","use"]`, CoordinatorID: bound.CoordinatorID, Origin: definitionOrigin(), Spec: bound.Spec, Existing: []conditions.ParticipantConditions{{MemberID: "A"}, {MemberID: "B"}}})
	s.Require().NoError(err)
	owner, err := conditions.CreateApplication(&conditions.CreateApplicationInput{Prepared: prepared})
	s.Require().NoError(err)
	w, err := newDefinitionWorld()
	s.Require().NoError(err)
	defer func() { s.NoError(w.close()) }()
	s.Require().NoError(owner.Condition.Apply(context.Background(), w.bus))
	s.Require().NoError(dndevents.TurnEndTopic.On(w.bus).Publish(context.Background(), dndevents.TurnEndEvent{SubjectID: "A", Round: 4}))
	raw, err := owner.Condition.ToJSON()
	s.Require().NoError(err)
	s.Require().NoError(owner.Condition.Remove(context.Background(), w.bus))
	var data conditions.LinkedConditionData
	s.Require().NoError(json.Unmarshal(raw, &data))
	activity := data.Runtime.Parts.Parts["upkeep/combat-activity"]
	activity.State = "fresh"
	data.Runtime.Parts.Parts["upkeep/combat-activity"] = activity
	raw, err = json.Marshal(&data)
	s.Require().NoError(err)
	loaded, err := conditions.LoadJSON(raw)
	s.Require().Error(err, "independent saved upkeep must agree on whether its first holder end happened")
	s.Nil(loaded)
}

func (s *applicationDefinitionSuite) TestBindingRefusesMissingIdentitiesAndUncompiledValues() {
	compiled := s.compile(applicationDefinitionFixture())
	for _, change := range []func(*conditions.BindApplicationDefinitionInput){
		func(i *conditions.BindApplicationDefinitionInput) { i.Definition = nil },
		func(i *conditions.BindApplicationDefinitionInput) {
			i.Definition = &conditions.CompiledApplicationDefinition{}
		},
		func(i *conditions.BindApplicationDefinitionInput) { i.UserID = "" },
		func(i *conditions.BindApplicationDefinitionInput) { i.RecipientID = "" },
		func(i *conditions.BindApplicationDefinitionInput) { i.ApplicationID = "" },
		func(i *conditions.BindApplicationDefinitionInput) { i.Origin.EntityID = "" },
	} {
		in := conditions.BindApplicationDefinitionInput{Definition: compiled, UserID: "A", RecipientID: "B", ApplicationID: "use", Origin: definitionOrigin()}
		change(&in)
		out, err := conditions.BindApplicationDefinition(&in)
		s.Error(err)
		s.Nil(out)
	}
	out, err := conditions.BindApplicationDefinition(nil)
	s.Error(err)
	s.Nil(out)
}
