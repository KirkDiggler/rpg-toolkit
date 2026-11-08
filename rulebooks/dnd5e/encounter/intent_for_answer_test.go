// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type intentForAnswerSuite struct{ suite.Suite }

func TestIntentForAnswerSuite(t *testing.T) { suite.Run(t, new(intentForAnswerSuite)) }

// selectedAnswerDie permits exactly the authored d200 choice, face 101.
// Any additional call or different die is a test failure, not another choice.
type selectedAnswerDie struct{ calls int }

func (d *selectedAnswerDie) Roll(_ context.Context, sides int) (int, error) {
	d.calls++
	if d.calls != 1 || sides != 200 {
		return 0, fmt.Errorf("expected one d200, got call %d with d%d", d.calls, sides)
	}
	return 101, nil
}

func (s *intentForAnswerSuite) TestTranslateTheComposerSelectionWithoutAnotherRoll() {
	composer, err := behavior.NewComposer(&behavior.ComposerConfig{
		Vocabulary: machine.Vocabulary{Beats: map[machine.Beat]machine.BeatSpec{"time": {Timed: true}}},
		Instances: []behavior.InstanceSpec{{
			ID: "duty",
			Recipe: machine.Recipe{
				Ref: core.Ref{Module: "test", Type: "behaviors", ID: "choice"}, Initial: "watch",
				States: map[string]machine.State{"watch": {Transitions: []machine.Transition{
					{On: "time", Do: []machine.Act{{Pick: "leg"}}},
				}}},
			},
			Table: behavior.Table{"leg": {
				{Weight: 1, Toward: &behavior.Selector{At: &spatial.Position{X: 4}}},
				{Weight: 1, Toward: &behavior.Selector{At: &spatial.Position{X: 6}}},
			}},
		}},
	})
	s.Require().NoError(err)
	die := &selectedAnswerDie{}
	selected, err := composer.Decide(context.Background(), &behavior.DecideInput{
		On: "time", Now: 1, Facts: behavior.Facts{CanMove: true}, Die: die,
	})
	s.Require().NoError(err)
	s.Require().NotNil(selected.Pick)
	s.Equal(1, selected.Pick.Entry)
	out, err := encounter.IntentForAnswer(&encounter.IntentForAnswerInput{
		View: encounter.MonsterView{Self: "guard", Budget: encounter.TurnBudget{MovementFeet: 10},
			Table: encounter.Table{encounter.AnswerTime: {{Weight: 1, Hold: true}}}},
		Answer: selected.Pick.Answer,
	})
	s.Require().NoError(err)
	s.Require().NotNil(out)
	walk, ok := out.Intent.(encounter.Routed)
	s.Require().True(ok)
	s.Equal(encounter.MoveToward, walk.Policy)
	s.Equal(spatial.Position{X: 6}, *walk.AnchorAt)
	s.Equal(core.Ref{Module: "encounter", Type: "table", ID: "toward"}, walk.Cause)
	s.Equal(1, die.calls, "translation consumes the chosen row, not the view's unrelated table")
	walk.AnchorAt.X = 99
	s.Equal(float64(6), selected.Pick.Answer.Toward.At.X, "the execution goal cannot rewrite the chosen answer")
}

func (s *intentForAnswerSuite) TestAlreadyThereAndNoMovementRemainDistinctPasses() {
	goal := spatial.Position{X: 4}
	for _, tc := range []struct {
		name   string
		view   encounter.MonsterView
		reason string
	}{
		{"already there", encounter.MonsterView{Position: goal, Budget: encounter.TurnBudget{MovementFeet: 10}}, "already there"},
		{"no movement", encounter.MonsterView{Position: goal}, ""},
	} {
		s.Run(tc.name, func() {
			out, err := encounter.IntentForAnswer(&encounter.IntentForAnswerInput{
				View: tc.view, Answer: encounter.Answer{Toward: &encounter.Selector{At: &goal}},
			})
			s.Require().NoError(err)
			pass, ok := out.Intent.(encounter.Pass)
			s.Require().True(ok)
			s.Equal(tc.reason, pass.Why)
		})
	}
}

func (s *intentForAnswerSuite) TestDashRetainsItsSelectedGoalAndChargeFlag() {
	goal := spatial.Position{X: 8}
	out, err := encounter.IntentForAnswer(&encounter.IntentForAnswerInput{
		View:   encounter.MonsterView{Self: "guard", SpeedFeet: 30, Budget: encounter.TurnBudget{MovementFeet: 10}},
		Answer: encounter.Answer{Dash: &encounter.Selector{At: &goal}},
	})
	s.Require().NoError(err)
	walk, ok := out.Intent.(encounter.Routed)
	s.Require().True(ok)
	s.True(walk.Dash)
	s.Equal(goal, *walk.AnchorAt)
	s.Equal(core.Ref{Module: "encounter", Type: "table", ID: "dash"}, walk.Cause)
}

func (s *intentForAnswerSuite) TestNilAndWordlessInputsRefuseButExplicitHoldIsValid() {
	out, err := encounter.IntentForAnswer(nil)
	s.Nil(out)
	s.ErrorIs(err, encounter.ErrNilInput)
	out, err = encounter.IntentForAnswer(&encounter.IntentForAnswerInput{})
	s.Nil(out)
	s.ErrorIs(err, encounter.ErrBadAnswer)
	out, err = encounter.IntentForAnswer(&encounter.IntentForAnswerInput{Answer: encounter.Answer{Hold: true}})
	s.Require().NoError(err)
	s.Equal(encounter.Pass{}, out.Intent)
}
