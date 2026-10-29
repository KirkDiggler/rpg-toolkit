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

func TestRecipeRoundTrip(t *testing.T) {
	suite.Run(t, new(roundTripSuite))
}
