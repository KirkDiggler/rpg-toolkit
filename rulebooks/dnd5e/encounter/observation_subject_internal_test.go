// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/stretchr/testify/require"
)

func TestLegacySightSubjectsUpgradeWithoutRefreshing(t *testing.T) {
	// The old name deliberately looks qualified. A version, rather than a prefix
	// guess, determines migration; no character name is reserved by the store.
	raw := []byte(`{"intel":{"holdings":{"prop|idol":{"member|guard":{"payload":"eyJzdGF0ZSI6Imtub3duIiwieCI6MCwieSI6MH0=","channel":"sight","observed":2,"confirmed":4},"deeds|guard|attack":{"payload":"e30=","channel":"deeds","observed":3,"confirmed":3}}}}}`)
	var old perception.Data
	require.NoError(t, json.Unmarshal(raw, &old))
	before, err := json.Marshal(old)
	require.NoError(t, err)
	upgraded, err := normalizePerceptionSubjects(old, 0)
	require.NoError(t, err)
	store, err := perception.Load(upgraded)
	require.NoError(t, err)
	held, err := store.On(sightMember("prop|idol"), sightMember("member|guard"))
	require.NoError(t, err)
	require.Equal(t, uint64(2), held.Observed)
	require.Equal(t, uint64(4), held.Confirmed)
	require.Empty(t, held.CurrentVia)
	require.JSONEq(t, `{"state":"known","x":0,"y":0}`, string(held.Payload))
	unchanged, err := json.Marshal(old)
	require.NoError(t, err)
	require.Equal(t, before, unchanged, "the loader does not mutate the supplied save")
	_, err = normalizePerceptionSubjects(old, 99)
	require.ErrorIs(t, err, ErrInvalidData)
}
