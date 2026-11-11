// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/events"
	fx "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

func (s *partsSuite) TestRollModifierDescribesDomainsWithoutRolling() {
	for _, subtract := range []bool{false, true} {
		params := `{"dice":"1d4","group":"bless","rolls":["attack","saving_throw"]}`
		if subtract {
			params = `{"dice":"1d4","subtract":true,"group":"bane","rolls":["attack","saving_throw"]}`
		}
		input := partInput("dice", dndfx.KindRollModifier, params)
		input.Declaration.OnTerminal = fx.EndPart
		input.HolderID = "recipient"
		part := s.bind(input)
		assembly, err := fx.Assemble(fx.AssemblyInput{Key: "recipient-instance", Parts: []fx.PartInput{part}})
		s.Require().NoError(err)
		outputs := assembly.CurrentOutputs()
		s.Require().Len(outputs, 1)
		out := outputs[0].Output
		for _, kind := range []contributions.RollKind{contributions.RollKindAttack, contributions.RollKindSavingThrow} {
			described, err := dndfx.DescribeRoll(&dndfx.DescribeRollInput{
				Output: out, Request: &dndevents.DescribeRollContributionsInput{Kind: kind},
				Origin: input.Origin, DisplayName: input.DisplayName,
			})
			s.Require().NoError(err)
			s.True(described.Applicable)
			s.Require().Len(described.Contributions, 1)
			s.Equal("1d4", described.Contributions[0].Dice)
			s.Equal(subtract, described.Contributions[0].Subtract)
			s.Equal(input.Origin.Ref.String(), described.Contributions[0].Source.Ref.String())
			s.Equal("maintainer", described.Contributions[0].Source.SourceID)
			if subtract {
				s.Equal("bane", described.Group)
			} else {
				s.Equal("bless", described.Group)
			}
		}
		for _, request := range []*dndevents.DescribeRollContributionsInput{nil, {Kind: "ability_check"}, {Kind: "damage"}} {
			out, err := dndfx.DescribeRoll(&dndfx.DescribeRollInput{Output: out, Request: request, Origin: input.Origin, DisplayName: input.DisplayName})
			s.Require().NoError(err)
			s.False(out.Applicable)
			s.Empty(out.Contributions)
		}
		s.False(assembly.IsApplied(), "description does not need an interaction")
		ctx, bus := context.Background(), events.NewEventBus()
		s.Require().NoError(assembly.Apply(ctx, bus))
		s.Require().NoError(assembly.Remove(ctx, bus))
	}
}

func (s *partsSuite) TestRollModifierKeepsSaverAndOutgoingActorDistinct() {
	input := partInput("dice", dndfx.KindRollModifier, `{"dice":"1d4","group":"bless","rolls":["attack","saving_throw"]}`)
	input.Declaration.OnTerminal = fx.EndPart
	input.HolderID = "holder"
	part := s.bind(input)
	composed, err := fx.Compose(part.Compose)
	s.Require().NoError(err)
	bound := composed.CurrentOutputs()[0]
	frame := attackFrame(0)
	frame.Action.Roll = contributions.Known(contributions.RollKindSavingThrow)
	answer, err := dndfx.Assess(&dndfx.AssessInput{Query: dndfx.Query{Action: &frame}, Output: bound, Origin: input.Origin, DisplayName: input.DisplayName})
	s.Require().NoError(err)
	s.Equal(contributions.StateDoesNotApply, answer.Effect.State, "casting a save-shaped action is not making its saving throw")
	answer, err = dndfx.Assess(&dndfx.AssessInput{Query: dndfx.Query{Roll: &dndfx.RollQuery{MemberID: "other", Kind: contributions.RollKindSavingThrow}}, Output: bound, Origin: input.Origin, DisplayName: input.DisplayName})
	s.Require().NoError(err)
	s.Equal(contributions.StateDoesNotApply, answer.Effect.State)
	rows, err := composed.Rows(dndfx.Query{Roll: &dndfx.RollQuery{MemberID: "holder", Kind: contributions.RollKindSavingThrow}})
	s.Require().NoError(err)
	s.Require().Len(rows, 1)
	s.Require().NotNil(rows[0].Amount)
	s.Equal("1d4", rows[0].Amount.Dice)
	s.Nil(rows[0].Amount.Scalar)
}

func (s *partsSuite) TestRollModifierRefusesMalformedBindingsAndInspectsCurrentState() {
	input := partInput("dice", dndfx.KindRollModifier, `{"dice":"1d4","group":"bless","rolls":["attack"]}`)
	input.Declaration.OnTerminal = fx.EndPart
	part := s.bind(input)
	composed, err := fx.Compose(part.Compose)
	s.Require().NoError(err)
	_, err = dndfx.DescribeRoll(&dndfx.DescribeRollInput{Output: composed.CurrentOutputs()[0]})
	s.Error(err, "even a nil-request description must not accept missing attribution")
	info, err := dndfx.InspectPart(&dndfx.InspectPartInput{Declaration: input.Declaration, Snapshot: composed.Snapshot()})
	s.Require().NoError(err)
	s.False(info.Concentration)
	s.Nil(info.Duration)
	for _, params := range []string{
		`{"dice":"bad","group":"bless","rolls":["attack"]}`,
		`{"dice":"1d4","group":"","rolls":["attack"]}`,
		`{"dice":"1d4","group":"bless","rolls":[]}`,
		`{"dice":"1d4","group":"bless","rolls":["attack","attack"]}`,
		`{"dice":"1d4","group":"bless","rolls":["damage"]}`,
	} {
		input.Declaration.Params = json.RawMessage(params)
		got, err := dndfx.BindPart(input)
		s.Error(err)
		s.Nil(got)
	}
}
