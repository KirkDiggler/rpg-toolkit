// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior_test

import (
	"encoding/json/v2"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

type parameterSuite struct{ suite.Suite }

// TestParameterSuite checks the producer-facing scalar contract, not a sampler.
func TestParameterSuite(t *testing.T) { suite.Run(t, new(parameterSuite)) }

func parameterInt(n int) *int { return &n }

func (s *parameterSuite) TestBoundsAreInclusiveAndValuesStayTyped() {
	schema := []behavior.ParameterSpec{{Name: "rounds", Kind: machine.KindInt, Required: true,
		Min: parameterInt(1), Max: parameterInt(4)}}
	for _, value := range []int{1, 4} {
		out, err := behavior.CheckParameters(&behavior.CheckParametersInput{
			Schema: schema, Values: map[string]any{"rounds": value},
		})
		s.Require().NoError(err)
		s.Equal(map[string]any{"rounds": value}, out.Values)
	}
	for _, value := range []any{0, 5, 1.0, "1", true, nil} {
		out, err := behavior.CheckParameters(&behavior.CheckParametersInput{
			Schema: schema, Values: map[string]any{"rounds": value},
		})
		s.ErrorIs(err, behavior.ErrBadParameter)
		s.ErrorContains(err, "rounds")
		s.Nil(out)
	}
}

func (s *parameterSuite) TestFalseZeroAndEmptyAreNotMissing() {
	out, err := behavior.CheckParameters(&behavior.CheckParametersInput{
		Schema: []behavior.ParameterSpec{
			{Name: "enabled", Kind: machine.KindBool, Required: true},
			{Name: "count", Kind: machine.KindInt, Required: true, Min: parameterInt(0)},
			{Name: "label", Kind: machine.KindString, Required: true},
			{Name: "optional", Kind: machine.KindString},
		},
		Values: map[string]any{"enabled": false, "count": 0, "label": ""},
	})
	s.Require().NoError(err)
	s.Equal(map[string]any{"enabled": false, "count": 0, "label": ""}, out.Values)
	_, present := out.Values["optional"]
	s.False(present, "absence is not an implicit default")
}

func (s *parameterSuite) TestRequiredUnknownAndChoiceRefusals() {
	schema := []behavior.ParameterSpec{{Name: "kind", Kind: machine.KindString, Required: true,
		Choices: []string{"clang", "voice"}}}
	for _, tc := range []struct {
		name   string
		values map[string]any
		field  string
	}{
		{"missing", nil, "kind"},
		{"unsupported choice", map[string]any{"kind": "boom"}, "kind"},
		{"unknown parameter", map[string]any{"kind": "clang", "typo": 3}, "typo"},
		{"wrong kind", map[string]any{"kind": 1}, "kind"},
	} {
		s.Run(tc.name, func() {
			out, err := behavior.CheckParameters(&behavior.CheckParametersInput{Schema: schema, Values: tc.values})
			s.ErrorIs(err, behavior.ErrBadParameter)
			s.ErrorContains(err, tc.field)
			s.Nil(out)
		})
	}
	for _, value := range []string{"clang", "voice"} {
		out, err := behavior.CheckParameters(&behavior.CheckParametersInput{
			Schema: schema, Values: map[string]any{"kind": value},
		})
		s.Require().NoError(err)
		s.Equal(value, out.Values["kind"])
	}
}

func (s *parameterSuite) TestSchemaRefusesAmbiguousOrImpossibleConstraints() {
	for _, tc := range []struct {
		name   string
		schema []behavior.ParameterSpec
	}{
		{"empty name", []behavior.ParameterSpec{{Kind: machine.KindInt}}},
		{"duplicate names", []behavior.ParameterSpec{
			{Name: "n", Kind: machine.KindInt}, {Name: "n", Kind: machine.KindBool},
		}},
		{"unknown kind", []behavior.ParameterSpec{{Name: "n", Kind: "float"}}},
		{"reversed bounds", []behavior.ParameterSpec{
			{Name: "n", Kind: machine.KindInt, Min: parameterInt(4), Max: parameterInt(1)},
		}},
		{"bounds on string", []behavior.ParameterSpec{{Name: "n", Kind: machine.KindString, Min: parameterInt(1)}}},
		{"choices on bool", []behavior.ParameterSpec{{Name: "n", Kind: machine.KindBool, Choices: []string{"true"}}}},
		{"duplicate choices", []behavior.ParameterSpec{{Name: "n", Kind: machine.KindString, Choices: []string{"a", "a"}}}},
	} {
		s.Run(tc.name, func() {
			out, err := behavior.CheckParameters(&behavior.CheckParametersInput{Schema: tc.schema})
			s.ErrorIs(err, behavior.ErrBadParameter)
			s.Nil(out)
		})
	}
}

func (s *parameterSuite) TestResultDoesNotAliasProducerValues() {
	values := map[string]any{"n": 2}
	out, err := behavior.CheckParameters(&behavior.CheckParametersInput{
		Schema: []behavior.ParameterSpec{{Name: "n", Kind: machine.KindInt}}, Values: values,
	})
	s.Require().NoError(err)
	values["n"] = 99
	s.Equal(2, out.Values["n"])
	out.Values["n"] = 3
	s.Equal(99, values["n"])
}

func (s *parameterSuite) TestMetadataRoundTripPreservesItsConstraints() {
	schema := []behavior.ParameterSpec{{Name: "rounds", Description: "Investigation window", Unit: "rounds",
		Kind: machine.KindInt, Required: true, Min: parameterInt(1), Max: parameterInt(4)}}
	data, err := json.Marshal(schema)
	s.Require().NoError(err)
	var loaded []behavior.ParameterSpec
	s.Require().NoError(json.Unmarshal(data, &loaded))
	_, err = behavior.CheckParameters(&behavior.CheckParametersInput{
		Schema: loaded, Values: map[string]any{"rounds": 5},
	})
	s.ErrorIs(err, behavior.ErrBadParameter, "loading metadata must not erase the upper bound")
	out, err := behavior.CheckParameters(&behavior.CheckParametersInput{
		Schema: loaded, Values: map[string]any{"rounds": 4},
	})
	s.Require().NoError(err)
	s.Equal(4, out.Values["rounds"])
}

func (s *parameterSuite) TestNilAndEmptyInputsHaveDifferentMeanings() {
	out, err := behavior.CheckParameters(nil)
	s.ErrorIs(err, behavior.ErrNilInput)
	s.Nil(out)
	out, err = behavior.CheckParameters(&behavior.CheckParametersInput{})
	s.Require().NoError(err)
	s.NotNil(out)
	s.Empty(out.Values)
}
