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

func TestBlob_JSONRoundTrip(t *testing.T) {
	in := machine.Blob{
		State:  "alert",
		Bag:    machine.Bag{"alarmed": true, "count": 2, "seen": "orc"},
		WakeAt: u64(12),
		Anchor: u64(9),
	}
	data, err := json.Marshal(&in, json.Deterministic(true))
	require.NoError(t, err)
	want := `{"state":"alert","bag":{"alarmed":true,"count":2,"seen":"orc"},"wake_at":12,"anchor":9}`
	assert.JSONEq(t, want, string(data))

	var out machine.Blob
	require.NoError(t, json.Unmarshal(data, &out))
	assert.Equal(t, in, out)
	assert.Equal(t, 2, out.Bag["count"], "a whole number lands as int")
}

func TestBlob_NewIsTheInitialStateAndNothingElse(t *testing.T) {
	p := mustCompile(guardRecipe())
	b := p.NewBlob()
	assert.Equal(t, machine.Blob{State: "posted"}, b)

	data, err := json.Marshal(&b)
	require.NoError(t, err)
	assert.JSONEq(t, `{"state":"posted"}`, string(data), "a fresh blob carries only its state")
}

func TestBlob_ZeroWakeIsKept(t *testing.T) {
	in := machine.Blob{State: "s", WakeAt: u64(0)}
	data, err := json.Marshal(&in)
	require.NoError(t, err)
	var out machine.Blob
	require.NoError(t, json.Unmarshal(data, &out))
	require.NotNil(t, out.WakeAt, "a wake at round zero is a wake, not an absence")
}
