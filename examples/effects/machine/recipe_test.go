// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine_test

import (
	"encoding/json/v2"
	"reflect"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/examples/effects/machine"
)

type roundTripSuite struct {
	suite.Suite
}

func (s *roundTripSuite) TestRageRecipeSurvivesJSON() {
	in := rageRecipe()
	data, err := json.Marshal(&in, json.Deterministic(true))
	s.Require().NoError(err)

	var out machine.Recipe
	s.Require().NoError(json.Unmarshal(data, &out))
	s.True(reflect.DeepEqual(in, out), "recipe changed across the round trip:\n%s", data)
	s.Require().NoError(machine.Validate(out, rageVocabulary()), "the decoded recipe still validates")
}

func (s *roundTripSuite) TestRefIsTheStringForm() {
	in := rageRecipe()
	data, err := json.Marshal(&in)
	s.Require().NoError(err)
	s.Contains(string(data), `"ref":"dnd5e:conditions:raging"`)
}

// A guard decoded on its own, as the composer decodes an output's, lands
// whole numbers as int too.
func (s *roundTripSuite) TestGuardLeavesDecodeWholeNumbersAsInt() {
	var g machine.Guard
	s.Require().NoError(json.Unmarshal([]byte(`{"fact":{"name":"level","op":"gte","value":9}}`), &g))
	s.Equal(9, g.Fact.Value)

	var a machine.Act
	s.Require().NoError(json.Unmarshal([]byte(`{"set":{"key":"turns_active","value":2}}`), &a))
	s.Equal(2, a.Set.Value)
}

func TestRecipeRoundTrip(t *testing.T) {
	suite.Run(t, new(roundTripSuite))
}
