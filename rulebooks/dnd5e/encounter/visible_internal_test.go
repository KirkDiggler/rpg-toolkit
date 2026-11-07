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

// TestADoorChangeDropsTheVisibleCache: a door is the cache's input besides
// the key, so opening one changes the epoch and every observer sweeps again
// against the open door; the walk after that hits again. Mutant: an epoch
// blind to doors (the east room stays unknown, which the sweep test above
// catches, and the misses do not climb, which this one pins).
func TestADoorChangeDropsTheVisibleCache(t *testing.T) {
	enc := sweepScene(t)
	epoch, misses := enc.visibleEpoch, enc.visibleMisses
	farEast := enc.field.cellAt(spatial.Position{X: 9, Y: 1})

	_, err := enc.OpenDoor(&OpenDoorInput{Door: sweepNeckDor})
	require.NoError(t, err)

	require.NotEqual(t, epoch, enc.visibleEpoch, "the open changes the epoch")
	require.Equal(t, misses+len(sweepMembers), enc.visibleMisses, "every observer sweeps again against the open door")
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
