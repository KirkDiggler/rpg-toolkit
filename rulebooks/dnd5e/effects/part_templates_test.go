// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later
package effects_test

import (
	"encoding/json"
	"testing"

	fx "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	"github.com/stretchr/testify/suite"
)

type partTemplateSuite struct{ suite.Suite }

func TestPartTemplateSuite(t *testing.T) { suite.Run(t, new(partTemplateSuite)) }
func fixedBonus(n int) *fx.Amount        { return &fx.Amount{Fixed: &n} }
func abilityGuard(value any) *machine.Guard {
	return &machine.Guard{Fact: &machine.FactLeaf{Name: dndfx.FactAbility, Op: machine.OpEq, Value: value}}
}

func (s *partTemplateSuite) TestClassifiesThroughKindEntry() {
	for _, tc := range []struct {
		template      dndfx.PartTemplate
		class         dndfx.PartClass
		concentration bool
	}{
		{dndfx.PartTemplate{Kind: dndfx.KindDamageBonus, Amount: fixedBonus(2)}, dndfx.PartClassBenefit, false},
		{dndfx.PartTemplate{Kind: dndfx.KindAdvantage, Params: json.RawMessage(`{"on":"saving_throw"}`), When: abilityGuard("str")}, dndfx.PartClassBenefit, false},
		{dndfx.PartTemplate{Kind: dndfx.KindResistance, Params: json.RawMessage(`{"types":["fire"]}`)}, dndfx.PartClassBenefit, false},
		{dndfx.PartTemplate{Kind: dndfx.KindConcentration}, dndfx.PartClassMaintenance, true},
		{dndfx.PartTemplate{Kind: dndfx.KindDuration, Params: json.RawMessage(`{"turn_ends":10}`)}, dndfx.PartClassMaintenance, false},
		{dndfx.PartTemplate{Kind: dndfx.KindUntilEmpty}, dndfx.PartClassMaintenance, false},
		{dndfx.PartTemplate{Kind: dndfx.KindEndOn, Params: json.RawMessage(`{"beat":"rest","reason":"rest"}`)}, dndfx.PartClassMaintenance, false},
		{dndfx.PartTemplate{Kind: dndfx.KindRollModifier, Params: json.RawMessage(`{"dice":"1d4","group":"test","rolls":["attack"]}`)}, dndfx.PartClassBenefit, false},
	} {
		s.Run(string(tc.template.Kind), func() {
			out, err := dndfx.ValidatePartTemplate(&dndfx.ValidatePartTemplateInput{Template: tc.template})
			s.Require().NoError(err)
			s.Equal(tc.class, out.Class)
			s.Equal(tc.concentration, out.Capabilities.Concentration)
		})
	}
}

func (s *partTemplateSuite) TestRejectsUnknownFieldsKindsFactsAndStateGuards() {
	_, err := dndfx.ValidatePartTemplate(nil)
	s.Error(err)
	for _, template := range []dndfx.PartTemplate{
		{Kind: "unknown"},
		{Kind: dndfx.KindDamageBonus, Amount: fixedBonus(2), Params: json.RawMessage(`{"stage":"final"}`)},
		{Kind: dndfx.KindDamageBonus, Amount: fixedBonus(2), When: &machine.Guard{Fact: &machine.FactLeaf{Name: "unknown", Op: machine.OpEq, Value: true}}},
		{Kind: dndfx.KindDamageBonus, Amount: fixedBonus(2), When: abilityGuard("STR")},
		{Kind: dndfx.KindDamageBonus, Amount: fixedBonus(2), When: abilityGuard(true)},
		{Kind: dndfx.KindDamageBonus, Amount: fixedBonus(2), When: &machine.Guard{Flag: "hidden"}},
		{Kind: dndfx.KindDamageBonus, Amount: fixedBonus(2), When: &machine.Guard{Bag: &machine.BagLeaf{Key: "x", Op: machine.OpEq, Value: 1}}},
		{Kind: dndfx.KindDamageBonus, Amount: fixedBonus(2), When: &machine.Guard{Rounds: &machine.RoundsLeaf{WentBackwards: true}}},
		{Kind: dndfx.KindDamageBonus, Amount: fixedBonus(2), When: &machine.Guard{ClassLevel: &machine.ClassLevelLeaf{Class: "barbarian", Op: machine.OpGte, N: 1}}},
		{Kind: dndfx.KindDamageBonus, Amount: fixedBonus(2), When: &machine.Guard{}},
		{Kind: dndfx.KindAdvantage, Params: json.RawMessage(`{"on":"saving_throw"}`), When: &machine.Guard{Fact: &machine.FactLeaf{Name: dndfx.FactMelee, Op: machine.OpEq, Value: true}}},
	} {
		_, err = dndfx.ValidatePartTemplate(&dndfx.ValidatePartTemplateInput{Template: template})
		s.Error(err, "%+v", template)
	}
}

func (s *partTemplateSuite) TestRejectsUnsupportedAmountOrWhen() {
	for _, template := range []dndfx.PartTemplate{
		{Kind: dndfx.KindDamageBonus},
		{Kind: dndfx.KindDamageBonus, Amount: &fx.Amount{Dice: "1d4"}},
		{Kind: dndfx.KindAdvantage, Params: json.RawMessage(`{"on":"ability_check"}`), Amount: fixedBonus(2)},
		{Kind: dndfx.KindResistance, Params: json.RawMessage(`{"types":["fire"]}`), When: abilityGuard("str")},
		{Kind: dndfx.KindDuration, Params: json.RawMessage(`{"turn_ends":2}`), Amount: fixedBonus(2)},
		{Kind: dndfx.KindConcentration, When: abilityGuard("str")},
		{Kind: dndfx.KindRollModifier, Params: json.RawMessage(`{"dice":"1d4","group":"test","rolls":["attack"]}`), Amount: fixedBonus(2)},
	} {
		_, err := dndfx.ValidatePartTemplate(&dndfx.ValidatePartTemplateInput{Template: template})
		s.Error(err)
		declaration := dndfx.PartDeclaration{Key: "test", Kind: template.Kind, Params: template.Params, When: template.When, Amount: template.Amount, OnTerminal: fx.EndApplication}
		s.Error(dndfx.ValidatePart(&dndfx.ValidatePartInput{Declaration: declaration}))
	}
}
