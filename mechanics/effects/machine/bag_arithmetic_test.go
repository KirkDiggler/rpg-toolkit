// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine_test

import (
	"context"
	"encoding/json/v2"
	"math"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
)

type bagArithmeticSuite struct{ suite.Suite }

func TestBagArithmeticSuite(t *testing.T) { suite.Run(t, new(bagArithmeticSuite)) }

func countdownRecipe() machine.Recipe {
	return machine.Recipe{
		Ref:     core.Ref{Module: "test", Type: "machine", ID: "counter"},
		Initial: "active", Bag: machine.Bag{"remaining": 2},
		States: map[string]machine.State{
			"active": {Transitions: []machine.Transition{
				{On: beatTurnEnd, About: selSubject, When: &machine.Guard{Bag: &machine.BagLeaf{Key: "remaining", Op: machine.OpEq, Value: 1}}, To: "ended", Reason: "expired"},
				{On: beatTurnEnd, About: selSubject, Do: []machine.Act{{Add: &machine.AddAct{Key: "remaining", Delta: -1}}}},
			}},
			"ended": {Terminal: true},
		},
	}
}

// Missing either the comparison or the delta must break the second-beat expiry.
func (s *bagArithmeticSuite) TestCounterGuardAndDeltaRoundTrip() {
	recipe := countdownRecipe()
	raw, err := json.Marshal(recipe)
	s.Require().NoError(err)
	var decoded machine.Recipe
	s.Require().NoError(json.Unmarshal(raw, &decoded))
	s.Equal(recipe, decoded)
	program, err := decoded.Compile(rageVocabulary())
	s.Require().NoError(err)
	m := machine.New(machine.Config{Program: program, HolderID: holder})
	bus := events.NewEventBus()
	ctx := context.Background()
	s.Require().NoError(m.Apply(ctx, bus))
	s.Require().NoError(turnEndTopic.On(bus).Publish(ctx, turnEnd{SubjectID: otherHolder}))
	s.Equal(2, m.Bag()["remaining"], "another holder does not advance the counter")
	s.Require().NoError(turnEndTopic.On(bus).Publish(ctx, turnEnd{SubjectID: holder}))
	s.Equal(1, m.Bag()["remaining"])
	s.Equal("active", m.State())
	s.Require().NoError(turnEndTopic.On(bus).Publish(ctx, turnEnd{SubjectID: holder}))
	s.Equal("ended", m.State())
	s.False(m.IsApplied())
	s.Equal(2, recipe.Bag["remaining"], "running never rewrites authored defaults")
}

func (s *bagArithmeticSuite) TestRefusesUnknownWrongTypedAndMultipleActForms() {
	for _, tc := range []struct {
		name   string
		change func(*machine.Recipe)
	}{
		{"unknown comparison", func(r *machine.Recipe) { r.States["active"].Transitions[0].When.Bag.Key = "missing" }},
		{"wrong comparison operand", func(r *machine.Recipe) { r.States["active"].Transitions[0].When.Bag.Value = "one" }},
		{"unknown delta key", func(r *machine.Recipe) { r.States["active"].Transitions[1].Do[0].Add.Key = "missing" }},
		{"wrong delta key type", func(r *machine.Recipe) { r.Bag["remaining"] = false }},
		{"two act forms", func(r *machine.Recipe) {
			r.States["active"].Transitions[1].Do[0].Set = &machine.SetAct{Key: "remaining", Value: 1}
		}},
		{"two guard forms", func(r *machine.Recipe) { r.States["active"].Transitions[0].When.Flag = "remaining" }},
	} {
		s.Run(tc.name, func() {
			recipe := countdownRecipe()
			tc.change(&recipe)
			_, err := recipe.Compile(rageVocabulary())
			s.Error(err)
		})
	}
}

// Overflow after an earlier write must neither wrap nor publish partial state.
func (s *bagArithmeticSuite) TestOverflowDoesNotPartiallyWriteOrNotify() {
	for _, tc := range []struct {
		name  string
		start int
		delta int
	}{
		{"overflow", math.MaxInt, 1},
		{"underflow", math.MinInt, -1},
	} {
		s.Run(tc.name, func() {
			recipe := countdownRecipe()
			recipe.Bag = machine.Bag{"remaining": tc.start, "marked": false}
			state := recipe.States["active"]
			state.Transitions = []machine.Transition{{On: beatTurnEnd, About: selSubject, To: "ended", Reason: "wrong", Do: []machine.Act{
				{Set: &machine.SetAct{Key: "marked", Value: true}},
				{Add: &machine.AddAct{Key: "remaining", Delta: tc.delta}},
			}}}
			recipe.States["active"] = state
			program, err := recipe.Compile(rageVocabulary())
			s.Require().NoError(err)
			m := machine.New(machine.Config{Program: program, HolderID: holder})
			changed, transitioned := false, false
			m.OnChanged(func(context.Context) error { changed = true; return nil })
			m.OnTransition(func(context.Context, machine.Transitioned) error { transitioned = true; return nil })
			bus := events.NewEventBus()
			ctx := context.Background()
			s.Require().NoError(m.Apply(ctx, bus))
			s.Error(turnEndTopic.On(bus).Publish(ctx, turnEnd{SubjectID: holder}))
			s.Equal(tc.start, m.Bag()["remaining"])
			s.Equal(false, m.Bag()["marked"])
			s.Equal("active", m.State())
			s.False(changed)
			s.False(transitioned)
			s.Require().NoError(m.Remove(ctx, bus))
		})
	}
}

func (s *bagArithmeticSuite) TestBagUnknownIsNotFalse() {
	program, err := countdownRecipe().Compile(rageVocabulary())
	s.Require().NoError(err)
	guard, err := program.CompileGuard(&machine.Guard{Bag: &machine.BagLeaf{Key: "remaining", Op: machine.OpGte, Value: 1}}, "")
	s.Require().NoError(err)
	for _, tc := range []struct {
		bag  machine.Bag
		want machine.Verdict
	}{
		{nil, machine.Depends},
		{machine.Bag{"remaining": "unknown"}, machine.Depends},
		{machine.Bag{"remaining": 0}, machine.DoesNotApply},
		{machine.Bag{"remaining": 1}, machine.Applies},
	} {
		got, _ := guard.Answer(nil, tc.bag, nil)
		s.Equal(tc.want, got)
	}
	s.Equal([]string{"remaining"}, machine.GuardKeys(&machine.Guard{Bag: &machine.BagLeaf{Key: "remaining", Op: machine.OpEq, Value: 1}}))
}
