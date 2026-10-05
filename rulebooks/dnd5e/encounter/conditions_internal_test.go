// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeliveredSightPayload(t *testing.T) {
	t.Run("conditions are removed and nothing else changes", func(t *testing.T) {
		without, err := EncodeSightTestimony(SightTestimony{State: LocationKnown})
		require.NoError(t, err)
		with, err := EncodeSightTestimony(SightTestimony{
			State: LocationKnown, Conditions: &ConditionSet{Conditions: []ConditionKey{{ConditionRef: "r", SourceID: "s"}}},
		})
		require.NoError(t, err)
		got, err := deliveredSightPayload(with)
		require.NoError(t, err)
		require.Equal(t, string(without), string(got))
	})
	t.Run("a payload without conditions is returned byte for byte", func(t *testing.T) {
		legacy := []byte(`{"x":1,"y":2}`)
		got, err := deliveredSightPayload(legacy)
		require.NoError(t, err)
		require.Equal(t, string(legacy), string(got))
		got[0] = 'X'
		require.Equal(t, byte('{'), legacy[0], "a copy, not the stored bytes")
	})
	t.Run("an undecodable payload claiming conditions is refused, not relayed", func(t *testing.T) {
		_, err := deliveredSightPayload([]byte(`{"state":"known","x":1,"y":2,"conditions":null}`))
		require.ErrorIs(t, err, ErrInvalidData)
	})
	t.Run("an undecodable payload without conditions passes through", func(t *testing.T) {
		got, err := deliveredSightPayload([]byte(`opaque`))
		require.NoError(t, err)
		require.Equal(t, "opaque", string(got))
	})
}
