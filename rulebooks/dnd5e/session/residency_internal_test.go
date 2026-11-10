// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// resident is a copy of n estimated bytes, for the bound alone.
func resident(n int64) *Resident { return &Resident{bytes: n} }

// TestMemoryResidencyBoundsByCountAndBytes: Take removes; Keep replaces a
// session's copy; past either bound the least recently kept is evicted; a
// copy larger than the byte bound is not kept; a negative bound keeps none.
func TestMemoryResidencyBoundsByCountAndBytes(t *testing.T) {
	r := NewResidency(&ResidencyConfig{MaxEntries: 2, MaxBytes: 100})
	r.Keep("a", resident(10))
	r.Keep("b", resident(10))
	r.Keep("c", resident(10))
	require.Equal(t, 2, r.Len(), "the count bound holds")
	require.Nil(t, r.Take("a"), "the oldest kept went first")
	require.NotNil(t, r.Take("b"))
	require.Nil(t, r.Take("b"), "Take removes")
	require.Equal(t, 1, r.Len())

	r.Keep("c", resident(60))
	require.Equal(t, int64(60), r.Bytes(), "Keep replaces the session's copy")
	r.Keep("d", resident(30))
	r.Keep("c", resident(60)) // c is now the most recently kept
	r.Keep("e", resident(30))
	require.Nil(t, r.Take("d"), "past the byte bound the oldest kept went first")
	require.Equal(t, int64(90), r.Bytes())

	r.Keep("f", resident(101))
	require.Nil(t, r.Take("f"), "a copy over the byte bound is not kept")

	for _, cfg := range []*ResidencyConfig{{MaxEntries: -1}, {MaxBytes: -1}} {
		none := NewResidency(cfg)
		none.Keep("a", resident(1))
		require.Zero(t, none.Len(), "a negative bound keeps none")
	}
	def := NewResidency(nil)
	require.Equal(t, DefaultResidentEntries, def.maxEntries)
	require.Equal(t, int64(DefaultResidentMaxBytes), def.maxBytes)
}
