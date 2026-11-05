// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior_test

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	arrivedA            machine.Beat     = "arrived:a"
	arrivedB            machine.Beat     = "arrived:b"
	noiseKind           machine.FactName = "heard.kind"
	investigationRounds                  = "investigation_rounds"
)

// patrolComposition is authored fixture content, not a coordinator. It binds
// one validated scalar into ordinary recipe data. The real Composer owns all
// progress, precedence, picking and reload. The caller would execute a chosen
// movement and deliver arrival inputs; these tests make no physical-walk claim.
func patrolComposition(rounds int) (behavior.ComposerConfig, error) {
	checked, err := behavior.CheckParameters(&behavior.CheckParametersInput{
		Schema: []behavior.ParameterSpec{{Name: investigationRounds, Kind: machine.KindInt,
			Required: true, Unit: "rounds", Min: parameterInt(1), Max: parameterInt(4)}},
		Values: map[string]any{investigationRounds: rounds},
	})
	if err != nil {
		return behavior.ComposerConfig{}, err
	}
	window, ok := checked.Values[investigationRounds].(int)
	if !ok {
		return behavior.ComposerConfig{}, fmt.Errorf("validated investigation window is not an int")
	}
	vocab := compositionVocabulary()
	vocab.Beats[arrivedA], vocab.Beats[arrivedB] = machine.BeatSpec{}, machine.BeatSpec{}
	vocab.Facts = map[machine.FactName]machine.FactSpec{
		noiseKind: {Kind: machine.KindString, Read: func(value any) (any, bool) {
			facts, known := value.(map[string]any)
			if !known {
				return nil, false
			}
			kind, present := facts[string(noiseKind)]
			return kind, present
		}},
	}
	investigate := behavior.InstanceSpec{
		ID: "investigate",
		Recipe: machine.Recipe{
			Ref: core.Ref{Module: "test", Type: "behaviors", ID: "investigation"}, Initial: "idle",
			States: map[string]machine.State{
				"idle": {Transitions: []machine.Transition{{On: composeHeard,
					When: &machine.Guard{Fact: &machine.FactLeaf{Name: noiseKind, Op: machine.OpEq, Value: "clang"}},
					To:   "active", Do: []machine.Act{{Anchor: true}}, Reason: "investigate the clang",
				}}},
				"active": {Transitions: []machine.Transition{
					{On: composeTick, When: &machine.Guard{Rounds: &machine.RoundsLeaf{Op: machine.OpGte, N: window}},
						To: "idle", Reason: "investigation complete"},
					{On: composeTick, Do: []machine.Act{{Pick: "look"}}},
				}},
			},
		},
		Table: behavior.Table{"look": {
			{Weight: 1, Toward: &behavior.Selector{At: &spatial.Position{X: 8, Y: 1}}},
			{Weight: 1, Toward: &behavior.Selector{At: &spatial.Position{X: 8, Y: -1}}},
		}},
	}
	patrol := behavior.InstanceSpec{
		ID: "patrol",
		Recipe: machine.Recipe{
			Ref: core.Ref{Module: "test", Type: "behaviors", ID: "patrol"}, Initial: "leg-a",
			States: map[string]machine.State{
				"leg-a": {Transitions: []machine.Transition{
					{On: composeTick, Do: []machine.Act{{Pick: "leg-a"}}},
					{On: arrivedA, To: "leg-b", Reason: "first waypoint reached"},
				}},
				"leg-b": {Transitions: []machine.Transition{
					{On: composeTick, Do: []machine.Act{{Pick: "leg-b"}}},
					{On: arrivedB, To: "leg-a", Reason: "second waypoint reached"},
				}},
			},
		},
		Table: behavior.Table{
			"leg-a": {{Weight: 1, Toward: &behavior.Selector{At: &spatial.Position{X: 0, Y: 0}}}},
			"leg-b": {{Weight: 1, Toward: &behavior.Selector{At: &spatial.Position{X: 4, Y: 0}}}},
		},
	}
	return behavior.ComposerConfig{Instances: []behavior.InstanceSpec{investigate, patrol}, Vocabulary: vocab}, nil
}

type patrolCompositionSuite struct{ suite.Suite }

// TestPatrolCompositionSuite joins real parameter validation, machines,
// precedence, weighted selection and reload around one authored continuation.
func TestPatrolCompositionSuite(t *testing.T) { suite.Run(t, new(patrolCompositionSuite)) }

func (s *patrolCompositionSuite) TestInvestigationYieldsBackToThePendingLeg() {
	for _, tc := range []struct {
		name   string
		face   int
		want   spatial.Position
		reload bool
	}{
		{"first branch", 1, spatial.Position{X: 8, Y: 1}, false},
		{"second branch across reload", 101, spatial.Position{X: 8, Y: -1}, true},
	} {
		s.Run(tc.name, func() {
			config, err := patrolComposition(2)
			s.Require().NoError(err)
			composer, err := behavior.NewComposer(&config)
			s.Require().NoError(err)
			die := &exactDie{steps: []rollStep{
				{sides: 100, face: 1}, {sides: 100, face: 1},
				{sides: 200, face: tc.face}, {sides: 100, face: 1},
			}}
			out, err := composer.Decide(context.Background(), &behavior.DecideInput{
				On: composeTick, Event: uint64(0), Facts: behavior.Facts{CanMove: true}, Die: die,
			})
			s.Require().NoError(err)
			s.Equal("patrol", out.Instance)
			s.Equal(spatial.Position{X: 0, Y: 0}, *out.Pick.Answer.Toward.At)

			_, err = composer.Observe(&behavior.ObserveInput{Instance: out.Instance, On: arrivedA})
			s.Require().NoError(err)
			out, err = composer.Decide(context.Background(), &behavior.DecideInput{
				On: composeTick, Event: uint64(1), Now: 1, Facts: behavior.Facts{CanMove: true}, Die: die,
			})
			s.Require().NoError(err)
			s.Equal(spatial.Position{X: 4, Y: 0}, *out.Pick.Answer.Toward.At)
			pending := composer.ToData().Instances[1]

			_, err = composer.Observe(&behavior.ObserveInput{
				On: composeHeard, Env: map[string]any{string(noiseKind): "clang"}, Now: 1,
			})
			s.Require().NoError(err)
			s.Equal(2, die.used, "activation rolls nothing")
			out, err = composer.Decide(context.Background(), &behavior.DecideInput{
				On: composeTick, Event: uint64(2), Now: 2, Facts: behavior.Facts{CanMove: true}, Die: die,
			})
			s.Require().NoError(err)
			s.Equal("investigate", out.Instance)
			s.Equal(tc.want, *out.Pick.Answer.Toward.At)
			s.Equal(200, out.Pick.Of)
			s.Equal(tc.face, out.Pick.Roll)
			s.Equal([]string{"patrol"}, out.Deferred)
			s.Equal(pending, composer.ToData().Instances[1], "investigation did not advance or reset patrol")

			if tc.reload {
				raw, err := json.Marshal(composer.ToData())
				s.Require().NoError(err)
				var data behavior.ComposerData
				s.Require().NoError(json.Unmarshal(raw, &data))
				composer, err = behavior.LoadComposer(&behavior.LoadComposerInput{Config: config, Data: data})
				s.Require().NoError(err)
				s.Equal(3, die.used, "reload does not replay activation or selection")
			}
			out, err = composer.Decide(context.Background(), &behavior.DecideInput{
				On: composeTick, Event: uint64(3), Now: 3, Facts: behavior.Facts{CanMove: true}, Die: die,
			})
			s.Require().NoError(err)
			s.Equal("patrol", out.Instance)
			s.Equal(behavior.AnswerKey("leg-b"), out.Pick.Key)
			s.Equal(spatial.Position{X: 4, Y: 0}, *out.Pick.Answer.Toward.At,
				"continue the pending leg, not the initial leg or a geometrically inferred replacement")
			s.Equal("idle", composer.ToData().Instances[0].Blob.State)
			s.Equal(4, die.used)
		})
	}
}

func (s *patrolCompositionSuite) TestIrrelevantOrUnknownSoundDoesNotTakeThePatrolOpportunity() {
	for _, facts := range []any{map[string]any{string(noiseKind): "voice"}, nil} {
		config, err := patrolComposition(2)
		s.Require().NoError(err)
		composer, err := behavior.NewComposer(&config)
		s.Require().NoError(err)
		_, err = composer.Observe(&behavior.ObserveInput{On: composeHeard, Env: facts, Now: 1})
		s.Require().NoError(err)
		die := &exactDie{steps: []rollStep{{sides: 100, face: 1}}}
		out, err := composer.Decide(context.Background(), &behavior.DecideInput{
			On: composeTick, Event: uint64(2), Now: 2, Facts: behavior.Facts{CanMove: true}, Die: die,
		})
		s.Require().NoError(err)
		s.Equal("patrol", out.Instance)
		s.Equal(1, die.used)
	}
}

func (s *patrolCompositionSuite) TestUnavailableMovementDoesNotSpendProgressOrRoll() {
	config, err := patrolComposition(2)
	s.Require().NoError(err)
	composer, err := behavior.NewComposer(&config)
	s.Require().NoError(err)
	_, err = composer.Observe(&behavior.ObserveInput{
		On: composeHeard, Env: map[string]any{string(noiseKind): "clang"}, Now: 1,
	})
	s.Require().NoError(err)
	before := composer.ToData()
	die := &exactDie{}
	out, err := composer.Decide(context.Background(), &behavior.DecideInput{
		On: composeTick, Event: uint64(2), Now: 2, Die: die,
	})
	s.Require().NoError(err)
	s.Equal("investigate", out.Instance)
	s.True(out.Pick.Answer.Hold)
	s.Equal(-1, out.Pick.Entry)
	s.Zero(die.used)
	s.Equal(before, composer.ToData())
}

func (s *patrolCompositionSuite) TestMetadataParameterChangesTheExecutedRecipe() {
	for _, tc := range []struct {
		rounds   int
		instance string
		sides    int
	}{
		{2, "patrol", 100},
		{3, "investigate", 200},
	} {
		config, err := patrolComposition(tc.rounds)
		s.Require().NoError(err)
		composer, err := behavior.NewComposer(&config)
		s.Require().NoError(err)
		_, err = composer.Observe(&behavior.ObserveInput{
			On: composeHeard, Env: map[string]any{string(noiseKind): "clang"}, Now: 1,
		})
		s.Require().NoError(err)
		die := &exactDie{steps: []rollStep{{sides: tc.sides, face: 1}}}
		out, err := composer.Decide(context.Background(), &behavior.DecideInput{
			On: composeTick, Event: uint64(3), Now: 3, Facts: behavior.Facts{CanMove: true}, Die: die,
		})
		s.Require().NoError(err)
		s.Equal(tc.instance, out.Instance)
		s.Equal(1, die.used)
	}
	_, err := patrolComposition(0)
	s.ErrorIs(err, behavior.ErrBadParameter)
	_, err = patrolComposition(5)
	s.ErrorIs(err, behavior.ErrBadParameter)
}
