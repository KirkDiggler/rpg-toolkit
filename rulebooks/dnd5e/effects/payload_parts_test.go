// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later
package effects_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	fx "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/stretchr/testify/suite"
)

type payloadPartsSuite struct{ suite.Suite }

func TestPayloadPartsSuite(t *testing.T) { suite.Run(t, new(payloadPartsSuite)) }
func (s *payloadPartsSuite) apply(parts ...dndfx.PartDeclaration) (*fx.Assembly, events.EventBus) {
	input := fx.AssemblyInput{Key: "payloads"}
	for _, decl := range parts {
		in := partInput(decl.Key, decl.Kind, "")
		in.Declaration = decl
		in.HolderID = "holder"
		part, err := dndfx.BindPart(in)
		s.Require().NoError(err)
		input.Parts = append(input.Parts, part.Part)
	}
	a, err := fx.Assemble(input)
	s.Require().NoError(err)
	bus := events.NewEventBus()
	s.Require().NoError(a.Apply(context.Background(), bus))
	s.T().Cleanup(func() { s.NoError(a.Remove(context.Background(), bus)) })
	return a, bus
}
func damagePart(amount *fx.Amount) dndfx.PartDeclaration {
	return dndfx.PartDeclaration{Key: "bonus", Kind: dndfx.KindDamageBonus, OnTerminal: fx.EndPart, Amount: amount, When: &machine.Guard{All: []machine.Guard{
		*abilityGuard("str"), {Fact: &machine.FactLeaf{Name: dndfx.FactMelee, Op: machine.OpEq, Value: true}}, {Fact: &machine.FactLeaf{Name: dndfx.FactWeaponPool, Op: machine.OpEq, Value: true}},
	}}}
}
func (s *payloadPartsSuite) TestFixedDamageAndClassScalingUseExistingMeaning() {
	helper := new(vocabularySuite)
	helper.SetT(s.T())
	for _, tc := range []struct {
		level, want int
		amount      *fx.Amount
	}{
		{0, 12, fixedBonus(2)},
		{1, 12, &fx.Amount{ByClassLevel: &fx.ClassSteps{Class: "barbarian", Steps: map[int]int{1: 2, 9: 3, 16: 4}}}},
		{9, 13, &fx.Amount{ByClassLevel: &fx.ClassSteps{Class: "barbarian", Steps: map[int]int{1: 2, 9: 3, 16: 4}}}},
		{16, 14, &fx.Amount{ByClassLevel: &fx.ClassSteps{Class: "barbarian", Steps: map[int]int{1: 2, 9: 3, 16: 4}}}},
	} {
		_, bus := s.apply(damagePart(tc.amount))
		out, err := helper.foldDamage(bus, attackFrame(tc.level))
		s.Require().NoError(err)
		s.Equal(tc.want, damageTotal(out))
	}
	_, bus := s.apply(damagePart(fixedBonus(2)))
	for _, mutate := range []func(*contributions.Frame){
		func(f *contributions.Frame) { f.Action.Ability = contributions.Known(abilities.DEX) },
		func(f *contributions.Frame) { f.Action.Melee = contributions.Known(false) },
		func(f *contributions.Frame) { f.Action.WeaponPool = contributions.Known(false) },
	} {
		f := attackFrame(0)
		mutate(&f)
		out, err := helper.foldDamage(bus, f)
		s.Require().NoError(err)
		s.Equal(10, damageTotal(out))
	}
}
func (s *payloadPartsSuite) TestStrengthSaveAndCheckParts() {
	_, bus := s.apply(
		dndfx.PartDeclaration{Key: "save", Kind: dndfx.KindAdvantage, Params: json.RawMessage(`{"on":"saving_throw"}`), When: abilityGuard("str"), OnTerminal: fx.EndPart},
		dndfx.PartDeclaration{Key: "check", Kind: dndfx.KindAdvantage, Params: json.RawMessage(`{"on":"ability_check"}`), When: abilityGuard("str"), OnTerminal: fx.EndPart},
	)
	ctx := context.Background()
	for _, tc := range []struct {
		who     string
		ability abilities.Ability
		count   int
	}{{"holder", abilities.STR, 1}, {"holder", abilities.DEX, 0}, {"other", abilities.STR, 0}} {
		event := &dndevents.SavingThrowChainEvent{SaverID: tc.who, Ability: tc.ability, DC: 10}
		c, err := dndevents.SavingThrowChain.On(bus).PublishWithChain(ctx, event, events.NewStagedChain[*dndevents.SavingThrowChainEvent](combat.ModifierStages))
		s.Require().NoError(err)
		out, err := c.Execute(ctx, event)
		s.Require().NoError(err)
		s.Len(out.AdvantageSources, tc.count)
	}
	for _, tc := range []struct {
		skill skills.Skill
		count int
	}{{skills.Athletics, 1}, {skills.Acrobatics, 0}} {
		event := &dndevents.AbilityCheckChainEvent{CheckerID: "holder", Skill: tc.skill, DC: 10}
		c, err := dndevents.AbilityCheckChain.On(bus).PublishWithChain(ctx, event, events.NewStagedChain[*dndevents.AbilityCheckChainEvent](combat.ModifierStages))
		s.Require().NoError(err)
		out, err := c.Execute(ctx, event)
		s.Require().NoError(err)
		s.Len(out.AdvantageSources, tc.count)
	}
}
func (s *payloadPartsSuite) TestResistanceUsesOneExistingFold() {
	_, bus := s.apply(dndfx.PartDeclaration{Key: "one", Kind: dndfx.KindResistance, Params: json.RawMessage(`{"types":["slashing"]}`), OnTerminal: fx.EndPart}, dndfx.PartDeclaration{Key: "two", Kind: dndfx.KindResistance, Params: json.RawMessage(`{"types":["slashing"]}`), OnTerminal: fx.EndPart})
	n := 9
	ref := core.Ref{Module: "example", Type: "actions", ID: "hit"}
	frame := attackFrame(0)
	frame.Actor = "enemy"
	frame.Target = contributions.Known("holder")
	event, err := dndevents.NewIncomingDamageEvent(dndevents.IncomingDamageInput{TargetID: "holder", SourceID: "enemy", Frame: frame, Dealt: []dndevents.DamageComponent{{DamageType: damage.Slashing, Roll: dndevents.RollComponent{Source: dndevents.RollSource{Ref: &ref, Name: "Hit"}, Modifier: &n}}}})
	s.Require().NoError(err)
	ctx := context.Background()
	c, err := dndevents.IncomingDamageChain.On(bus).PublishWithChain(ctx, event.Clone(), events.NewStagedChain[*dndevents.IncomingDamageEvent](combat.ModifierStages))
	s.Require().NoError(err)
	folded, err := c.Execute(ctx, event.Clone())
	s.Require().NoError(err)
	s.Len(folded.Multipliers, 2)
	result, err := combat.SettleDamage(&combat.SettleDamageInput{Dealt: event.Dealt(), Multipliers: folded.Multipliers, Reductions: folded.Reductions})
	s.Require().NoError(err)
	s.Equal(4, result.Types[0].Taken)
}
func (s *payloadPartsSuite) TestDeclarationClonesOwnGuardsAndAmounts() {
	decl := damagePart(fixedBonus(2))
	cloned := decl.Clone()
	cloned.When.All[0].Fact.Value = "dex"
	*cloned.Amount.Fixed = 99
	s.Equal("str", decl.When.All[0].Fact.Value)
	s.Equal(2, *decl.Amount.Fixed)
	input := conditions.PrepareApplicationInput{ID: "application", CoordinatorID: "owner", Origin: dndevents.ConditionOrigin{Ref: core.Ref{Module: "example", Type: "items", ID: "ring"}, EntityID: "item-1"}, Existing: []conditions.ParticipantConditions{{MemberID: "owner"}, {MemberID: "holder"}}, Spec: conditions.ApplicationSpec{Name: "Bonus", CoordinatorParts: []dndfx.PartDeclaration{{Key: "empty", Kind: dndfx.KindUntilEmpty, OnTerminal: fx.EndApplication}}, Recipients: []conditions.RecipientSpec{{MemberID: "holder", Ref: refs.Conditions.LinkedEffects().String(), Name: "Bonus", AddressSourceID: "payload", Lifetime: conditions.LifetimeShared, Parts: []dndfx.PartDeclaration{decl}}}}}
	prepared, err := conditions.PrepareApplication(&input)
	s.Require().NoError(err)
	*decl.Amount.Fixed = 88
	decl.When.All[0].Fact.Value = "dex"
	made, err := conditions.CreateRecipient(&conditions.CreateRecipientInput{Prepared: prepared, SpecIndex: 0})
	s.Require().NoError(err)
	raw, err := made.Condition.ToJSON()
	s.Require().NoError(err)
	var saved conditions.LinkedConditionData
	s.Require().NoError(json.Unmarshal(raw, &saved))
	s.Equal(2, *saved.Bindings.Parts[0].Amount.Fixed)
	s.Equal("str", saved.Bindings.Parts[0].When.All[0].Fact.Value)
	*saved.Bindings.Parts[0].Amount.Fixed = 77
	again, err := made.Condition.ToJSON()
	s.Require().NoError(err)
	s.JSONEq(string(raw), string(again))
}
