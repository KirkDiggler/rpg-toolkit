// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"context"
	"encoding/json/jsontext"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
)

type amountResultSuite struct{ suite.Suite }

func TestAmountResultSuite(t *testing.T) { suite.Run(t, new(amountResultSuite)) }

const queryKind effects.KindName = "test_roll_description"

func queryInput(key string) effects.ComposeInput {
	return effects.ComposeInput{
		InstanceKey: key, HolderID: "recipient",
		Recipe: machine.Recipe{
			Ref: core.Ref{Module: "test", Type: "effects", ID: "dice"}, Initial: "active", Bag: machine.Bag{},
			States: map[string]machine.State{"active": {Outputs: []jsontext.Value{
				output(effects.Output{Kind: queryKind, Amount: &effects.Amount{Dice: "1d4"}}),
			}}},
		},
		OutputVocabulary: effects.OutputVocabulary{
			Kinds: map[effects.KindName]effects.KindSpec{queryKind: {Execution: effects.KindQuery, Assess: row}},
			Amounts: map[effects.AmountName]effects.AmountFn{
				effects.AmountDice: func(_ any, a effects.Amount) (effects.AmountResult, error) {
					return effects.AmountResult{Dice: a.Dice}, nil
				},
			},
		},
	}
}

// An eager scalar conversion cannot pass an assertion on the unresolved term.
func (s *amountResultSuite) TestDiceIsNeverEvaluatedByInspection() {
	composed, err := effects.Compose(queryInput("source-a"))
	s.Require().NoError(err)
	for range 3 {
		rows, rowErr := composed.Rows(nil)
		s.Require().NoError(rowErr)
		s.Require().Len(rows, 1)
		s.Require().NotNil(rows[0].Amount)
		s.Equal("1d4", rows[0].Amount.Dice)
		s.Nil(rows[0].Amount.Scalar)
		_, scalarErr := rows[0].Amount.ScalarValue()
		s.Error(scalarErr)
	}
	s.False(composed.IsApplied(), "information does not attach anything")
}

func (s *amountResultSuite) TestScalarZeroIsPresentAndDiceRefusesScalarRead() {
	zero := 0
	value := effects.AmountResult{Scalar: &zero}
	s.Require().NoError(value.Validate())
	got, err := value.ScalarValue()
	s.Require().NoError(err)
	s.Zero(got)
	_, err = (effects.AmountResult{Dice: "1d4"}).ScalarValue()
	s.Error(err)
}

func (s *amountResultSuite) TestInvalidUnionIsRefused() {
	one := 1
	for _, value := range []effects.AmountResult{{}, {Scalar: &one, Dice: "1d4"}, {Dice: " "}} {
		s.Error(value.Validate())
		_, err := value.ScalarValue()
		s.Error(err)
		input := queryInput("invalid-result")
		input.OutputVocabulary.Amounts[effects.AmountDice] = func(any, effects.Amount) (effects.AmountResult, error) {
			return value, nil
		}
		composed, err := effects.Compose(input)
		s.Require().NoError(err)
		_, err = composed.Rows(nil)
		s.Error(err, "the Bound boundary must validate evaluator output")
	}
}

func (s *amountResultSuite) TestEvaluatedAmountIsOwned() {
	value := 3
	input := queryInput("scalar-result")
	input.OutputVocabulary.Amounts[effects.AmountDice] = func(any, effects.Amount) (effects.AmountResult, error) {
		return effects.AmountResult{Scalar: &value}, nil
	}
	composed, err := effects.Compose(input)
	s.Require().NoError(err)
	answer, err := composed.CurrentOutputs()[0].Amount(nil)
	s.Require().NoError(err)
	*answer.Scalar = 50
	again, err := composed.CurrentOutputs()[0].Amount(nil)
	s.Require().NoError(err)
	s.Equal(3, *again.Scalar)
}

type subscriptionCounter struct {
	events.EventBus
	subscribed int
}

func (b *subscriptionCounter) Subscribe(ctx context.Context, topic events.Topic, handler any) (string, error) {
	b.subscribed++
	return b.EventBus.Subscribe(ctx, topic, handler)
}

func (s *amountResultSuite) TestQueryKindHasNoSubscriptionAndStillRequiresAssessment() {
	input := queryInput("query")
	composed, err := effects.Compose(input)
	s.Require().NoError(err)
	bus := &subscriptionCounter{EventBus: events.NewEventBus()}
	s.Require().NoError(composed.Apply(context.Background(), bus))
	s.Zero(bus.subscribed)
	s.Require().NoError(composed.Remove(context.Background(), bus))
	s.False(composed.IsApplied())
	kind := input.OutputVocabulary.Kinds[queryKind]
	kind.Assess = nil
	input.OutputVocabulary.Kinds[queryKind] = kind
	_, err = effects.Compose(input)
	s.Error(err)
}

func (s *amountResultSuite) TestSubscribedKindStillRequiresAttach() {
	input := queryInput("refused")
	kind := input.OutputVocabulary.Kinds[queryKind]
	kind.Execution = effects.KindSubscribed
	input.OutputVocabulary.Kinds[queryKind] = kind
	_, err := effects.Compose(input)
	s.Error(err)
	kind.Execution = effects.KindQuery
	kind.Attach = damageBonusKind.Attach
	input.OutputVocabulary.Kinds[queryKind] = kind
	_, err = effects.Compose(input)
	s.Error(err, "a query-only kind cannot hide a subscription")
	kind.Execution = effects.KindExecution("unknown")
	input.OutputVocabulary.Kinds[queryKind] = kind
	_, err = effects.Compose(input)
	s.Error(err)
}

func (s *amountResultSuite) TestInstanceKeysSeparateEqualRecipeOutputs() {
	a, err := effects.Compose(queryInput("a"))
	s.Require().NoError(err)
	b, err := effects.Compose(queryInput("b"))
	s.Require().NoError(err)
	s.NotEqual(a.CurrentOutputs()[0].ID(), b.CurrentOutputs()[0].ID())
	restored, err := effects.Compose(queryInput("a"))
	s.Require().NoError(err)
	s.Equal(a.CurrentOutputs()[0].ID(), restored.CurrentOutputs()[0].ID())
}
