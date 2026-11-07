// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	fx "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

type vocabularySuite struct{ suite.Suite }

func (s *vocabularySuite) origin() dndevents.ConditionOrigin {
	return dndevents.ConditionOrigin{Ref: core.Ref{Module: "example", Type: "items", ID: "source"}, EntityID: "item-17"}
}

func (s *vocabularySuite) recipe(amount fx.Amount) machine.Recipe {
	fact := func(name machine.FactName, value any) machine.Guard {
		return machine.Guard{Fact: &machine.FactLeaf{Name: name, Op: machine.OpEq, Value: value}}
	}
	strength := fact("action.ability", string(abilities.STR))
	outputs := []fx.Output{
		{Kind: "damage_bonus", When: &machine.Guard{All: []machine.Guard{fact("action.melee", true), fact("action.weapon_pool", true), strength}}, Amount: &amount},
		{Kind: "resistance", Params: jsontext.Value(`{"types":["bludgeoning","piercing","slashing"]}`)},
		{Kind: "advantage", When: &strength, Params: jsontext.Value(`{"on":"saving_throw"}`)},
		{Kind: "advantage", When: &strength, Params: jsontext.Value(`{"on":"ability_check"}`)},
	}
	encoded := make([]jsontext.Value, 0, len(outputs))
	for _, out := range outputs {
		raw, err := json.Marshal(out)
		s.Require().NoError(err)
		encoded = append(encoded, raw)
	}
	return machine.Recipe{Ref: core.Ref{Module: "example", Type: "conditions", ID: "bundle"}, Initial: "active", States: map[string]machine.State{
		"active": {Outputs: encoded},
	}}
}

func (s *vocabularySuite) compose(amount fx.Amount) (*fx.Composed, events.EventBus) {
	bound, err := dndfx.Bind(&dndfx.BindInput{Origin: s.origin(), DisplayName: "Authored bundle"})
	s.Require().NoError(err)
	c, err := fx.Compose(fx.ComposeInput{Recipe: s.recipe(amount), MachineVocabulary: bound.Machine, OutputVocabulary: bound.Outputs, HolderID: "holder"})
	s.Require().NoError(err)
	bus := events.NewEventBus()
	s.Require().NoError(c.Apply(context.Background(), bus))
	s.T().Cleanup(func() { s.NoError(c.Remove(context.Background(), bus)) })
	return c, bus
}

func attackFrame(level int) contributions.Frame {
	levels := contributions.KnownClassLevels()
	if level > 0 {
		levels = contributions.KnownClassLevels(contributions.ClassLevel{Class: classes.Barbarian, Levels: level})
	}
	return contributions.Frame{Actor: "holder", ActorClassLevels: levels, Target: contributions.Known("target"), Action: contributions.ActionFacts{
		Roll: contributions.Known(contributions.RollKindAttack), Ability: contributions.Known(abilities.STR), Melee: contributions.Known(true), WeaponPool: contributions.Known(true),
	}}
}

func (s *vocabularySuite) foldDamage(bus events.EventBus, frame contributions.Frame) (*dndevents.DamageChainEvent, error) {
	base := 10
	source := core.Ref{Module: "example", Type: "actions", ID: "strike"}
	in := dndevents.DamageChainInput{AttackerID: frame.Actor, TargetID: "target", Frame: frame, WeaponDamageType: damage.Slashing, Components: []dndevents.DamageComponent{{Source: dndevents.DamageSourceWeapon, DamageType: damage.Slashing, Roll: dndevents.RollComponent{Source: dndevents.RollSource{Ref: &source, Name: "Strike"}, Modifier: &base}}}}
	ctx := context.Background()
	chain, err := dndevents.DamageChain.On(bus).PublishWithChain(ctx, dndevents.NewDamageChainEvent(in), events.NewStagedChain[*dndevents.DamageChainEvent](combat.ModifierStages))
	if err != nil {
		return nil, err
	}
	return chain.Execute(ctx, dndevents.NewDamageChainEvent(in))
}

func damageTotal(out *dndevents.DamageChainEvent) int {
	total := 0
	for _, component := range out.Components {
		if component.Roll.Modifier != nil {
			total += *component.Roll.Modifier
		}
	}
	return total
}

func (s *vocabularySuite) TestDamageBonusUsesRealFrameAndOrigin() {
	cases := []struct {
		name   string
		level  int
		amount fx.Amount
		want   int
	}{
		{"fixed without class", 0, fx.Amount{Fixed: new(3)}, 13},
		{"level one", 1, fx.Amount{ByClassLevel: &fx.ClassSteps{Class: "barbarian", Steps: map[int]int{1: 2, 9: 3, 16: 4}}}, 12},
		{"level nine", 9, fx.Amount{ByClassLevel: &fx.ClassSteps{Class: "barbarian", Steps: map[int]int{1: 2, 9: 3, 16: 4}}}, 13},
		{"level sixteen", 16, fx.Amount{ByClassLevel: &fx.ClassSteps{Class: "barbarian", Steps: map[int]int{1: 2, 9: 3, 16: 4}}}, 14},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			_, bus := s.compose(tc.amount)
			out, err := s.foldDamage(bus, attackFrame(tc.level))
			s.Require().NoError(err)
			s.Equal(tc.want, damageTotal(out))
			s.Equal("item-17", out.Components[1].Roll.Source.SourceID)
			s.Equal("Authored bundle", out.Components[1].Roll.Source.Name)
			s.Equal("example:conditions:bundle", out.Components[1].Roll.Source.Ref.String())
		})
	}
}

func (s *vocabularySuite) TestDamageKnownNegativesAndUnknownFacts() {
	_, bus := s.compose(fx.Amount{Fixed: new(3)})
	for _, which := range []string{"dexterity", "ranged", "nonweapon", "other holder"} {
		s.Run(which, func() {
			frame := attackFrame(0)
			switch which {
			case "dexterity":
				frame.Action.Ability = contributions.Known(abilities.DEX)
			case "ranged":
				frame.Action.Melee = contributions.Known(false)
			case "nonweapon":
				frame.Action.WeaponPool = contributions.Known(false)
			case "other holder":
				frame.Actor = "someone-else"
			}
			out, err := s.foldDamage(bus, frame)
			s.Require().NoError(err)
			s.Equal(10, damageTotal(out))
		})
	}
	frame := attackFrame(0)
	frame.Action.Ability = contributions.Unknown[abilities.Ability]()
	_, err := s.foldDamage(bus, frame)
	s.ErrorIs(err, contributions.ErrRuleCannotAnswer)
}

func (s *vocabularySuite) TestResistanceAndAdvantagesUseTheirOwnDomains() {
	c, bus := s.compose(fx.Amount{Fixed: new(3)})
	ctx := context.Background()
	for _, tc := range []struct {
		kind   damage.Type
		target string
		want   int
	}{{damage.Slashing, "holder", 4}, {damage.Fire, "holder", 9}, {damage.Slashing, "other", 9}} {
		amount := 9
		ref := core.Ref{Module: "example", Type: "actions", ID: "incoming"}
		frame := attackFrame(0)
		frame.Actor = "enemy"
		frame.Target = contributions.Known(tc.target)
		sent, err := dndevents.NewIncomingDamageEvent(dndevents.IncomingDamageInput{TargetID: tc.target, SourceID: "enemy", Frame: frame, Dealt: []dndevents.DamageComponent{{Source: dndevents.DamageSourceWeapon, DamageType: tc.kind, Roll: dndevents.RollComponent{Source: dndevents.RollSource{Ref: &ref, Name: "Incoming"}, Modifier: &amount}}}})
		s.Require().NoError(err)
		chain, err := dndevents.IncomingDamageChain.On(bus).PublishWithChain(ctx, sent.Clone(), events.NewStagedChain[*dndevents.IncomingDamageEvent](combat.ModifierStages))
		s.Require().NoError(err)
		folded, err := chain.Execute(ctx, sent.Clone())
		s.Require().NoError(err)
		s.Require().NoError(folded.CheckUnaltered(sent))
		settled, err := combat.SettleDamage(&combat.SettleDamageInput{Dealt: sent.Dealt(), Reductions: folded.Reductions, Multipliers: folded.Multipliers})
		s.Require().NoError(err)
		s.Require().Len(settled.Types, 1)
		s.Equal(tc.want, settled.Types[0].Taken)
		if tc.want == 4 {
			s.Require().NotNil(settled.Types[0].DecidedBy)
			s.Equal("item-17", settled.Types[0].DecidedBy.Source.SourceID)
		}
	}
	for _, tc := range []struct {
		ability abilities.Ability
		want    bool
	}{{abilities.STR, true}, {abilities.DEX, false}} {
		event := &dndevents.SavingThrowChainEvent{SaverID: "holder", Ability: tc.ability, DC: 10}
		chain, err := dndevents.SavingThrowChain.On(bus).PublishWithChain(ctx, event, events.NewStagedChain[*dndevents.SavingThrowChainEvent](combat.ModifierStages))
		s.Require().NoError(err)
		out, err := chain.Execute(ctx, event)
		s.Require().NoError(err)
		s.Equal(tc.want, len(out.AdvantageSources) > 0)
	}
	for _, tc := range []struct {
		skill skills.Skill
		want  bool
	}{{skills.Athletics, true}, {skills.Stealth, false}} {
		event := &dndevents.AbilityCheckChainEvent{CheckerID: "holder", Skill: tc.skill, DC: 10}
		chain, err := dndevents.AbilityCheckChain.On(bus).PublishWithChain(ctx, event, events.NewStagedChain[*dndevents.AbilityCheckChainEvent](combat.ModifierStages))
		s.Require().NoError(err)
		out, err := chain.Execute(ctx, event)
		s.Require().NoError(err)
		s.Equal(tc.want, len(out.AdvantageSources) > 0)
	}
	frame := attackFrame(0)
	for index, out := range c.CurrentOutputs() {
		assessed, err := dndfx.Assess(&dndfx.AssessInput{Query: dndfx.Query{Action: &frame}, Output: out, Origin: s.origin(), DisplayName: "Authored bundle"})
		s.Require().NoError(err)
		want := contributions.StateDoesNotApply
		if index == 0 {
			want = contributions.StateApplies
		}
		s.Equal(want, assessed.Effect.State, "Strength attacks do not receive save/check advantage or incoming resistance")
	}
}

func (s *vocabularySuite) TestKindValidationAndQueryRefusals() {
	_, err := dndfx.Bind(nil)
	s.Require().Error(err)
	invalid := s.origin()
	invalid.EntityID = ""
	_, err = dndfx.Bind(&dndfx.BindInput{Origin: invalid, DisplayName: "Authored bundle"})
	s.Require().Error(err)
	bound, err := dndfx.Bind(&dndfx.BindInput{Origin: s.origin(), DisplayName: "Authored bundle"})
	s.Require().NoError(err)
	for _, out := range []fx.Output{
		{Kind: "damage_bonus"},
		{Kind: "resistance", Params: jsontext.Value(`{"types":["made-up"]}`)},
		{Kind: "advantage", Params: jsontext.Value(`{"on":"attack"}`)},
		{Kind: "advantage", Params: jsontext.Value(`{"on":"saving_throw","unknown":true}`)},
	} {
		raw, err := json.Marshal(out)
		s.Require().NoError(err)
		recipe := machine.Recipe{Ref: core.Ref{Module: "example", Type: "conditions", ID: "invalid"}, Initial: "active", States: map[string]machine.State{"active": {Outputs: []jsontext.Value{raw}}}}
		c, err := fx.Compose(fx.ComposeInput{Recipe: recipe, MachineVocabulary: bound.Machine, OutputVocabulary: bound.Outputs, HolderID: "holder"})
		s.ErrorIs(err, fx.ErrInvalidOutput)
		s.Nil(c)
	}
	c, _ := s.compose(fx.Amount{Fixed: new(3)})
	frame := attackFrame(0)
	_, err = dndfx.Assess(&dndfx.AssessInput{Query: dndfx.Query{Action: &frame, Save: &dndevents.SavingThrowChainEvent{SaverID: "holder", Ability: abilities.STR}}, Output: c.CurrentOutputs()[0], Origin: s.origin(), DisplayName: "Authored bundle"})
	s.Error(err, "ambiguous context cannot choose a domain implicitly")
	described, err := dndfx.Assess(&dndfx.AssessInput{Output: c.CurrentOutputs()[0], Origin: s.origin(), DisplayName: "Authored bundle"})
	s.Require().NoError(err)
	s.Equal(contributions.StateDepends, described.Effect.State)
	s.Empty(described.Effect.Benefit)
	s.NotEmpty(described.Effect.Description)
}

func TestRulebookEffectVocabulary(t *testing.T) { suite.Run(t, new(vocabularySuite)) }
