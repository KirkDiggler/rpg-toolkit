// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine_test

import (
	"encoding/json/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

func TestRecipe_JSONRoundTrip(t *testing.T) {
	in := guardRecipe()
	in.Bag["count"] = 2
	in.States["alert"].Transitions[1].When = fact("wounds", machine.OpGte, 1)
	posted := in.States["posted"].Transitions
	posted[0].Do = append(posted[0].Do, set("count", 3))

	data, err := json.Marshal(&in, json.Deterministic(true))
	require.NoError(t, err)
	assert.Contains(t, string(data), `"ref":"test:machines:guard"`, "the ref is its string form")

	var out machine.Recipe
	require.NoError(t, json.Unmarshal(data, &out))
	assert.Equal(t, in, out, "recipe changed across the round trip:\n%s", data)
	assert.Equal(t, 2, out.Bag["count"], "a whole number lands as int")

	_, err = out.Compile(testVocabulary())
	require.NoError(t, err, "the decoded recipe still compiles")

	// By value, the ref is still its string form.
	byValue, err := json.Marshal(in, json.Deterministic(true))
	require.NoError(t, err)
	assert.JSONEq(t, string(data), string(byValue))
}

func TestRecipe_DecodesTheStudioShape(t *testing.T) {
	src := `{
	  "ref": "test:machines:guard",
	  "initial": "posted",
	  "states": {
	    "posted": {"transitions": [
	      {"on": "time", "do": [{"pick": "time"}]},
	      {"on": "heard", "when": {"scope": {"within": 6}}, "to": "alert", "do": [{"anchor": true}, {"wake_in": 4}]}
	    ]},
	    "alert": {"transitions": [
	      {"on": "time", "when": {"rounds": {"op": "gte", "n": 3}}, "to": "posted"}
	    ]}
	  }
	}`
	var r machine.Recipe
	require.NoError(t, json.Unmarshal([]byte(src), &r))
	p, err := r.Compile(testVocabulary())
	require.NoError(t, err)
	b := p.NewBlob()
	got, err := p.Step(&b, beatHeard, sound{Hexes: 1}, nil, 2)
	require.NoError(t, err)
	assert.Equal(t, "alert", got.To)
	assert.Equal(t, uint64(6), *b.WakeAt)
}
