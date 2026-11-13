// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

type addToRollSuite struct{ suite.Suite }

func TestAddToRollSuite(t *testing.T) { suite.Run(t, new(addToRollSuite)) }

func addToRollBinding(params string) *dndfx.BindPartInput {
	in := partInput("roll", "add_to_roll", params)
	in.HolderID = "recipient"
	in.Declaration.OnTerminal = generic.EndPart
	in.Declaration.Amount = &generic.Amount{Dice: "1d4"}
	return in
}

func (s *addToRollSuite) TestPositiveNegativeDomains() {
	for _, tc := range []struct {
		polarity string
		subtract bool
	}{{"positive", false}, {"negative", true}} {
		s.Run(tc.polarity, func() {
			in := addToRollBinding(`{"polarity":"` + tc.polarity + `","rolls":["attack","saving_throw"],"selection":{"mode":"oldest_applicable","group":"test"}}`)
			part, err := dndfx.BindPart(in)
			s.Require().NoError(err)
			composed, err := generic.Compose(part.Part.Compose)
			s.Require().NoError(err)
			s.Require().Len(composed.CurrentOutputs(), 1)
			for _, kind := range []contributions.RollKind{contributions.RollKindAttack, contributions.RollKindSavingThrow} {
				out, err := dndfx.DescribeRoll(&dndfx.DescribeRollInput{Output: composed.CurrentOutputs()[0], Request: &dndevents.DescribeRollContributionsInput{Kind: kind}, Origin: in.Origin, DisplayName: in.DisplayName})
				s.Require().NoError(err)
				s.True(out.Applicable)
				s.Require().Len(out.Contributions, 1)
				s.Equal("1d4", out.Contributions[0].Dice)
				s.Equal(tc.subtract, out.Contributions[0].Subtract)
				s.Equal("maintainer", out.Contributions[0].Source.SourceID)
			}
			out, err := dndfx.DescribeRoll(&dndfx.DescribeRollInput{Output: composed.CurrentOutputs()[0], Request: &dndevents.DescribeRollContributionsInput{Kind: "ability_check"}, Origin: in.Origin, DisplayName: in.DisplayName})
			s.Require().NoError(err)
			s.False(out.Applicable)
			s.Empty(out.Contributions)
		})
	}
}

func (s *addToRollSuite) TestCanonicalAmountOnly() {
	valid := addToRollBinding(`{"polarity":"positive","rolls":["attack"],"selection":{"mode":"all_applicable"}}`)
	s.Require().NoError(dndfx.ValidatePart(&dndfx.ValidatePartInput{Declaration: valid.Declaration}))
	for _, tc := range []struct {
		name   string
		change func(*dndfx.PartDeclaration)
	}{
		{"params-dice", func(d *dndfx.PartDeclaration) {
			d.Params = json.RawMessage(`{"dice":"1d4","polarity":"positive","rolls":["attack"],"selection":{"mode":"all_applicable"}}`)
		}},
		{"missing-amount", func(d *dndfx.PartDeclaration) { d.Amount = nil }},
		{"scalar", func(d *dndfx.PartDeclaration) { n := 1; d.Amount = &generic.Amount{Fixed: &n} }},
		{"two-amounts", func(d *dndfx.PartDeclaration) { n := 1; d.Amount.Fixed = &n }},
		{"retired-kind", func(d *dndfx.PartDeclaration) { d.Kind = "roll_modifier" }},
	} {
		s.Run(tc.name, func() {
			decl := valid.Declaration.Clone()
			tc.change(&decl)
			s.Error(dndfx.ValidatePart(&dndfx.ValidatePartInput{Declaration: decl}))
		})
	}
}

func (s *addToRollSuite) TestRejectsUnsupportedOrMalformedDefinition() {
	valid := addToRollBinding(`{"polarity":"positive","rolls":["attack"],"selection":{"mode":"all_applicable"}}`)
	s.Require().NoError(dndfx.ValidatePart(&dndfx.ValidatePartInput{Declaration: valid.Declaration}))
	for _, params := range []string{
		`{"rolls":["attack"],"selection":{"mode":"all_applicable"}}`,
		`{"polarity":"unknown","rolls":["attack"],"selection":{"mode":"all_applicable"}}`,
		`{"polarity":"positive","rolls":[],"selection":{"mode":"all_applicable"}}`,
		`{"polarity":"positive","rolls":["attack","attack"],"selection":{"mode":"all_applicable"}}`,
		`{"polarity":"positive","rolls":["ability_check"],"selection":{"mode":"all_applicable"}}`,
		`{"polarity":"positive","rolls":["attack"]}`,
		`{"polarity":"positive","rolls":["attack"],"selection":{"mode":"oldest_applicable"}}`,
		`{"polarity":"positive","rolls":["attack"],"selection":{"mode":"all_applicable","group":"wrong"}}`,
		`{"polarity":"positive","rolls":["attack"],"selection":{"mode":"newest","group":"g"}}`,
		`{"polarity":"positive","rolls":["attack"],"selection":{"mode":"all_applicable"},"subtract":false}`,
	} {
		decl := valid.Declaration.Clone()
		decl.Params = json.RawMessage(params)
		s.Error(dndfx.ValidatePart(&dndfx.ValidatePartInput{Declaration: decl}), params)
	}
	for _, notation := range []string{"", "bad", "0d4", "1d0", "-1d4", "1d4+1", "1d4+1d6"} {
		decl := valid.Declaration.Clone()
		decl.Amount.Dice = notation
		s.Error(dndfx.ValidatePart(&dndfx.ValidatePartInput{Declaration: decl}), notation)
	}
	decl := valid.Declaration.Clone()
	decl.OnTerminal = generic.EndApplication
	s.Error(dndfx.ValidatePart(&dndfx.ValidatePartInput{Declaration: decl}))
	decl = valid.Declaration.Clone()
	decl.When = &machine.Guard{Fact: &machine.FactLeaf{Name: dndfx.FactMelee, Op: machine.OpEq, Value: true}}
	s.Error(dndfx.ValidatePart(&dndfx.ValidatePartInput{Declaration: decl}), "unsupported authored guards cannot be silently discarded")
}

func (s *addToRollSuite) TestSelectionDoesNotRoll() {
	in := addToRollBinding(`{"polarity":"positive","rolls":["attack"],"selection":{"mode":"all_applicable"}}`)
	in.Declaration.Amount.Dice = "2d6"
	part, err := dndfx.BindPart(in)
	s.Require().NoError(err)
	composed, err := generic.Compose(part.Part.Compose)
	s.Require().NoError(err)
	for range 2 {
		out, err := dndfx.DescribeRoll(&dndfx.DescribeRollInput{Output: composed.CurrentOutputs()[0], Request: &dndevents.DescribeRollContributionsInput{Kind: contributions.RollKindAttack}, Origin: in.Origin, DisplayName: in.DisplayName})
		s.Require().NoError(err)
		s.Require().Len(out.Contributions, 1)
		s.Equal("2d6", out.Contributions[0].Dice, "description retains the unresolved pool, not a face or total")
	}
	s.False(composed.IsApplied(), "description does not attach the instance")
}
