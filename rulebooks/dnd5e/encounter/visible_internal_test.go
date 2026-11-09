// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TestAVisitedCellIsNotRecast (rpg-project#879 S2): the walker paces two
// cells; the first visit to each sweeps, every visit after reads the first
// one's answer, and the oracle sees no difference. Mutants: a cache that
// never hits (misses climb with every step); a hit that returns the wrong
// disk (the oracle finds floor unlearned).
func TestAVisitedCellIsNotRecast(t *testing.T) {
	enc := sweepScene(t)
	require.Equal(t, len(sweepMembers), enc.visibleMisses, "first light sweeps every observer once")
	require.Zero(t, enc.visibleHits)

	walkAround(t, enc, 6)

	require.Equal(t, len(sweepMembers)+1, enc.visibleMisses, "one new cell was visited; home was swept at first light")
	require.Equal(t, 5, enc.visibleHits, "every later step reads a visit's answer")
	requireNothingUnlearned(t, enc, "after the walk")
}

// TestADoorChangeDropsTheVisibleCache: a door is in the opacity every key
// carries, so opening one forces a fresh sweep from every observer, the
// open floor is learned, the orphaned entries are freed, and the walk after
// it hits again. Mutants: entries kept across the open (the cache holds
// more than one sweep per observer after it); a hit served across the open
// (the east room stays unknown, which the oracle catches).
func TestADoorChangeDropsTheVisibleCache(t *testing.T) {
	enc := sweepScene(t)
	misses := enc.visibleMisses
	farEast := enc.field.cellAt(spatial.Position{X: 9, Y: 1})

	_, err := enc.OpenDoor(&OpenDoorInput{Door: sweepNeckDor})
	require.NoError(t, err)

	require.Equal(t, misses+len(sweepMembers), enc.visibleMisses, "every observer sweeps again against the open door")
	require.Len(t, enc.visibleCache, len(sweepMembers), "the shut door's entries are freed, not kept beside the open door's")
	require.True(t, enc.world.knownCells(sweepWatchA)[farEast], "and sees down the neck")
	hits := enc.visibleHits
	walkAround(t, enc, 2)
	require.Greater(t, enc.visibleHits, hits, "the cells visited after the open are cached again")
	requireNothingUnlearned(t, enc, "after the open and the walk")
}

// TestTheVisibleCacheIsBounded: at the bound the cache is dropped and refills
// by use, costing one sweep per key again and no correctness. Mutant: a cache
// that grows without bound (entries exceed the bound).
func TestTheVisibleCacheIsBounded(t *testing.T) {
	prev := visibleCacheEntries
	visibleCacheEntries = 3
	defer func() { visibleCacheEntries = prev }()
	enc := sweepScene(t)
	for _, to := range []spatial.Position{{X: 1, Y: 2}, {X: 2, Y: 2}, {X: 3, Y: 2}, {X: 2, Y: 2}} {
		_, err := enc.Step(&StepInput{Member: sweepWalker, To: enc.field.cellAt(to), EndWalk: true})
		require.NoError(t, err)
		require.LessOrEqual(t, len(enc.visibleCache), visibleCacheEntries, "the cache never exceeds its bound")
	}
	requireNothingUnlearned(t, enc, "after the bounded walk")
}
