// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later
package effects_test

import (
	"context"
	"encoding/json"
	"encoding/json/jsontext"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	fx "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/stretchr/testify/suite"
)

type attackAdvantageSuite struct{ suite.Suite }

func TestAttackAdvantageSuite(t *testing.T) { suite.Run(t, new(attackAdvantageSuite)) }
func attackPart(side string) dndfx.PartDeclaration {
	return dndfx.PartDeclaration{Key: side, Kind: dndfx.KindAdvantage, Params: json.RawMessage(`{"on":"attack","side":"` + side + `"}`), OnTerminal: fx.EndPart}
}
func (s *attackAdvantageSuite) bound(decl dndfx.PartDeclaration) (*fx.Composed, events.EventBus, *dndfx.BindPartInput) {
	in := partInput(decl.Key, decl.Kind, "")
	in.Declaration = decl
	in.HolderID = "holder"
	in.Address = dndevents.ConditionAddress{MemberID: "holder", ConditionRef: in.Ref.String(), SourceID: "payload"}
	part, err := dndfx.BindPart(in)
	s.Require().NoError(err)
	composed, err := fx.Compose(part.Part.Compose)
	s.Require().NoError(err)
	bus := events.NewEventBus()
	s.Require().NoError(composed.Apply(context.Background(), bus))
	s.T().Cleanup(func() { s.NoError(composed.Remove(context.Background(), bus)) })
	return composed, bus, in
}
func attackDetails(c *fx.Composed, in *dndfx.BindPartInput) contributions.HeldEffectDetail {
	return contributions.HeldEffectDetail{MemberID: in.HolderID, Held: contributions.HeldCondition{Ref: in.Address.ConditionRef, SourceID: in.Address.SourceID}, ApplicationID: in.ApplicationID, DisplayName: in.DisplayName, OriginRef: in.Origin.Ref, OriginID: in.Origin.EntityID, Outputs: []contributions.ConfiguredOutput{{ID: c.CurrentOutputs()[0].ID(), Definition: c.CurrentOutputs()[0].Definition()}}}
}
func (s *attackAdvantageSuite) fold(bus events.EventBus, f contributions.Frame) (dndevents.AttackChainEvent, error) {
	target, _ := f.Target.Get()
	e := dndevents.AttackChainEvent{AttackerID: f.Actor, TargetID: target, Frame: f}
	ctx := context.Background()
	c, err := dndevents.AttackChain.On(bus).PublishWithChain(ctx, e, events.NewStagedChain[dndevents.AttackChainEvent](combat.ModifierStages))
	if err != nil {
		return e, err
	}
	return c.Execute(ctx, e)
}
func (s *attackAdvantageSuite) TestByHolderAndAgainstHolderDoNotSwapActors() {
	for _, side := range []string{"by_holder", "against_holder"} {
		s.Run(side, func() {
			c, bus, in := s.bound(attackPart(side))
			f := attackFrame(0)
			if side == "against_holder" {
				f.Actor = "enemy"
				f.Target = contributions.Known("holder")
				detail := attackDetails(c, in)
				f.Held = []contributions.MemberHeld{{Member: "holder", Conditions: []contributions.HeldCondition{detail.Held}}}
				f.HeldEffects = []contributions.HeldEffectDetail{detail}
			}
			out, err := s.fold(bus, f)
			s.Require().NoError(err)
			s.Require().Len(out.AdvantageSources, 1)
			s.Equal(f.Actor, out.Frame.Actor)
			info, err := dndfx.Assess(&dndfx.AssessInput{Output: c.CurrentOutputs()[0], Query: dndfx.Query{Action: &f}, Origin: in.Origin, DisplayName: in.DisplayName, Attachment: &dndfx.AttachmentIdentity{Address: in.Address, ApplicationID: in.ApplicationID}})
			s.Require().NoError(err)
			s.Equal(contributions.StateApplies, info.Effect.State)
			if side == "by_holder" {
				f.Actor = "other"
			} else {
				f.Target = contributions.Known("other")
			}
			out, err = s.fold(bus, f)
			s.Require().NoError(err)
			s.Empty(out.AdvantageSources)
		})
	}
}
func (s *attackAdvantageSuite) TestMissingSideAndUnsupportedSaveSideRefused() {
	for _, params := range []string{`{"on":"attack"}`, `{"on":"attack","side":""}`, `{"on":"attack","side":null}`, `{"on":"attack","side":"other"}`, `{"on":"saving_throw","side":""}`, `{"on":"ability_check","side":null}`} {
		_, err := dndfx.ValidatePartTemplate(&dndfx.ValidatePartTemplateInput{Template: dndfx.PartTemplate{Kind: dndfx.KindAdvantage, Params: json.RawMessage(params)}})
		s.Error(err, params)
	}
	in := partInput("against", dndfx.KindAdvantage, "")
	in.Declaration = attackPart("against_holder")
	_, err := dndfx.BindPart(in)
	s.Error(err, "a part cannot defer missing local identity until Apply")
	vocabulary, err := dndfx.Bind(&dndfx.BindInput{Origin: in.Origin, DisplayName: in.DisplayName})
	s.Require().NoError(err)
	raw := []byte(`{"kind":"advantage","params":{"on":"attack","side":"against_holder"}}`)
	_, err = fx.Compose(fx.ComposeInput{Recipe: machine.Recipe{Ref: core.Ref{Module: "example", Type: "conditions", ID: "raw"}, Initial: "active", States: map[string]machine.State{"active": {Outputs: []jsontext.Value{jsontext.Value(raw)}}}}, HolderID: "holder", MachineVocabulary: vocabulary.Machine, OutputVocabulary: vocabulary.Outputs})
	s.ErrorIs(err, fx.ErrInvalidOutput)
}
func (s *attackAdvantageSuite) TestOpportunityAndAbilityFacts() {
	decl := attackPart("by_holder")
	decl.When = &machine.Guard{All: []machine.Guard{*abilityGuard("str"), {Fact: &machine.FactLeaf{Name: dndfx.FactOpportunity, Op: machine.OpEq, Value: false}}}}
	_, bus, _ := s.bound(decl)
	f := attackFrame(0)
	f.Action.Opportunity = contributions.Known(false)
	out, err := s.fold(bus, f)
	s.Require().NoError(err)
	s.Len(out.AdvantageSources, 1)
	f.Action.Opportunity = contributions.Known(true)
	out, err = s.fold(bus, f)
	s.Require().NoError(err)
	s.Empty(out.AdvantageSources)
	f.Action.Opportunity = contributions.Known(false)
	f.Action.Ability = contributions.Known(abilities.DEX)
	out, err = s.fold(bus, f)
	s.Require().NoError(err)
	s.Empty(out.AdvantageSources)
	f.Action.Ability = contributions.Unknown[abilities.Ability]()
	_, err = s.fold(bus, f)
	s.ErrorIs(err, contributions.ErrRuleCannotAnswer)
}
func (s *attackAdvantageSuite) TestEndReportRetainsOutputsThenDropsThem() {
	decl := attackPart("against_holder")
	in := partInput(decl.Key, decl.Kind, "")
	in.Declaration = decl
	in.Address = partOwner()
	payload, err := dndfx.BindPart(in)
	s.Require().NoError(err)
	end, err := dndfx.BindPart(partInput("end", dndfx.KindEndOn, `{"beat":"rest","reason":"rest"}`))
	s.Require().NoError(err)
	var assembly *fx.Assembly
	observed := 0
	assembly, err = fx.Assemble(fx.AssemblyInput{Key: "end-report", Parts: []fx.PartInput{payload.Part, end.Part}, Lifecycle: fx.AssemblyLifecycle{End: func(_ context.Context, _ events.EventBus, _ fx.AssemblyEnd) error {
		for _, current := range assembly.CurrentOutputs() {
			if current.PartKey != decl.Key {
				continue
			}
			projection, err := dndfx.ProjectPartOutput(&dndfx.ProjectPartOutputInput{Declaration: decl, Output: current.Output})
			s.Require().NoError(err)
			s.True(projection.AgainstHolder)
			observed++
		}
		return nil
	}}})
	s.Require().NoError(err)
	bus := events.NewEventBus()
	s.Require().NoError(assembly.Apply(context.Background(), bus))
	s.Require().NoError(dndevents.RestTopic.On(bus).Publish(context.Background(), dndevents.RestEvent{CharacterID: "maintainer"}))
	s.Equal(1, observed)
	s.Empty(assembly.CurrentOutputs())
}

func (s *attackAdvantageSuite) TestAttackSupportPreservesSaveShapedDamage() {
	helper := new(payloadPartsSuite)
	helper.SetT(s.T())
	_, bus := helper.apply(damagePart(fixedBonus(2)))
	f := attackFrame(0)
	f.Action.Roll = contributions.Known(contributions.RollKindSavingThrow)
	damageHelper := new(vocabularySuite)
	damageHelper.SetT(s.T())
	out, err := damageHelper.foldDamage(bus, f)
	s.Require().NoError(err)
	s.Equal(12, damageTotal(out))
}
