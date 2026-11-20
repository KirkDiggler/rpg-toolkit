// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine_test

import (
	jsonv1 "encoding/json"
	"encoding/json/v2"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
)

type scalarSuite struct{ suite.Suite }

func TestScalarSuite(t *testing.T) { suite.Run(t, new(scalarSuite)) }

// Each kind reads back only as itself, and the zero Scalar holds nothing.
func (s *scalarSuite) TestEachKindReadsBackAsItself() {
	b, ok := machine.BoolScalar(true).Bool()
	s.True(ok)
	s.True(b)
	n, ok := machine.IntScalar(-3).Int()
	s.True(ok)
	s.Equal(-3, n)
	text, ok := machine.StringScalar("str").Text()
	s.True(ok)
	s.Equal("str", text)
	_, ok = machine.StringScalar("3").Int()
	s.False(ok, "a string is not an int")
	s.True((machine.Scalar{}).IsZero())
	s.Nil((machine.Scalar{}).Value())
	for _, v := range []any{true, 7, "x"} {
		got, ok := machine.ScalarOf(v)
		s.True(ok)
		s.Equal(v, got.Value())
	}
	for _, v := range []any{nil, 1.5, int64(1), []any{}, map[string]any{}} {
		_, ok := machine.ScalarOf(v)
		s.False(ok, "%T is not a scalar", v)
	}
}

// Equality is the guard's eq: same kind and same value.
func (s *scalarSuite) TestEqualityIsKindAndValue() {
	s.Equal(machine.IntScalar(1), machine.IntScalar(1))
	s.NotEqual(machine.IntScalar(1), machine.IntScalar(2))
	s.NotEqual(machine.IntScalar(1), machine.BoolScalar(true))
	s.NotEqual(machine.StringScalar("1"), machine.IntScalar(1))
	a, b := machine.StringScalar("a"), machine.StringScalar("a")
	s.True(a == b, "comparable with ==")
}

// The JSON is today's bare value, both ways, through v2 and v1.
func (s *scalarSuite) TestJSONIsTheBareValue() {
	for text, want := range map[string]machine.Scalar{
		`true`: machine.BoolScalar(true), `false`: machine.BoolScalar(false),
		`3`: machine.IntScalar(3), `-12`: machine.IntScalar(-12), `0`: machine.IntScalar(0),
		`"str"`: machine.StringScalar("str"), `""`: machine.StringScalar(""),
	} {
		var got machine.Scalar
		s.Require().NoError(json.Unmarshal([]byte(text), &got), text)
		s.Equal(want, got, text)
		out, err := json.Marshal(got)
		s.Require().NoError(err)
		s.Equal(text, string(out))

		var old machine.Scalar
		s.Require().NoError(jsonv1.Unmarshal([]byte(text), &old), text)
		s.Equal(want, old, text)
		out, err = jsonv1.Marshal(old)
		s.Require().NoError(err)
		s.Equal(text, string(out))
	}
}

// Anything that is not a bare bool, whole number or string is refused at
// decoding, naming the leaf it was found in. This is where the compile-time
// refusal of a float, a container or a missing value now lives: the type
// cannot hold them.
func (s *scalarSuite) TestNonScalarsAreRefusedNamingTheLeaf() {
	for _, value := range []string{`null`, `1.5`, `3.0`, `1e3`, `99999999999999999999`, `[1]`, `{"x":1}`} {
		for leaf, doc := range map[string]string{
			"fact leaf": `{"fact":{"name":"level","op":"eq","value":` + value + `}}`,
			"bag leaf":  `{"bag":{"key":"turns_active","op":"eq","value":` + value + `}}`,
		} {
			var g machine.Guard
			err := json.Unmarshal([]byte(doc), &g)
			s.Require().Error(err, "%s %s", leaf, value)
			s.ErrorIs(err, machine.ErrInvalidScalar, "%s %s", leaf, value)
			s.Contains(err.Error(), leaf)
		}
		var a machine.Act
		err := json.Unmarshal([]byte(`{"set":{"key":"turns_active","value":`+value+`}}`), &a)
		s.Require().Error(err, "set %s", value)
		s.ErrorIs(err, machine.ErrInvalidScalar)
		s.Contains(err.Error(), "set act")
	}
	_, err := json.Marshal(machine.Scalar{})
	s.ErrorIs(err, machine.ErrInvalidScalar, "an unset value is not written as null")
}

// An unset leaf value is still refused when the recipe compiles.
func (s *scalarSuite) TestAnUnsetValueIsRefusedAtCompile() {
	r := rageRecipe()
	r.States["fresh"].Transitions[0].When = &machine.Guard{Fact: &machine.FactLeaf{Name: "action.melee", Op: machine.OpEq}}
	err := machine.Validate(r, rageVocabulary())
	s.Require().Error(err)
	s.Contains(err.Error(), "wants a bool value, got <nil>")
}

// Today's JSON round-trips byte for byte: the rage recipe and blob as the
// module encoded them before Scalar existed (testdata, written from main), and
// a recipe holding every leaf and act kind.
func (s *scalarSuite) TestFixturesRoundTripByteForByte() {
	golden, err := os.ReadFile("testdata/rage_recipe.json")
	s.Require().NoError(err)
	var recipe machine.Recipe
	s.Require().NoError(json.Unmarshal(golden, &recipe))
	again, err := json.Marshal(&recipe, json.Deterministic(true))
	s.Require().NoError(err)
	s.Equal(string(golden), string(again))
	literal := rageRecipe()
	fromGo, err := json.Marshal(&literal, json.Deterministic(true))
	s.Require().NoError(err)
	s.Equal(string(golden), string(fromGo), "the Go literal encodes as the module always did")

	blob, err := os.ReadFile("testdata/rage_blob.json")
	s.Require().NoError(err)
	var b machine.Blob
	s.Require().NoError(json.Unmarshal(blob, &b))
	encoded, err := b.Marshal()
	s.Require().NoError(err)
	s.Equal(string(blob), string(encoded))

	every := `{"fact":{"name":"level","op":"gte","value":9}}`
	var g machine.Guard
	s.Require().NoError(json.Unmarshal([]byte(every), &g))
	out, err := json.Marshal(&g)
	s.Require().NoError(err)
	s.Equal(every, string(out))
	for _, text := range []string{
		`{"all":[{"bag":{"key":"turns_active","op":"gte","value":2}},{"bag":{"key":"mode","op":"eq","value":"fast"}},{"not":{"fact":{"name":"action.melee","op":"eq","value":false}}}]}`,
	} {
		var guard machine.Guard
		s.Require().NoError(json.Unmarshal([]byte(text), &guard))
		out, err := json.Marshal(&guard)
		s.Require().NoError(err)
		s.Equal(text, string(out))
	}
	for _, text := range []string{
		`{"set":{"key":"turns_active","value":2}}`,
		`{"set":{"key":"saw_turn_end","value":true}}`,
		`{"set":{"key":"mode","value":"slow"}}`,
	} {
		var act machine.Act
		s.Require().NoError(json.Unmarshal([]byte(text), &act))
		out, err := json.Marshal(&act)
		s.Require().NoError(err)
		s.Equal(text, string(out))
	}
}
