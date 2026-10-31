// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// reach_scale_test.go is WHAT A READ COSTS ON THE KEEP (rpg-project#753, S2):
// the whole field's projection, and the one-prop option read the speak menu
// asks per prop. Re-run rather than quoted:
//
//	go test . -run '^$' -bench 'Keep(10|30)k' -benchtime 5x

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// BenchmarkAtlas_Keep30k is one whole-field projection of the 30k keep.
func BenchmarkAtlas_Keep30k(b *testing.B) {
	enc, _ := openSite(b, 30000, false)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := enc.Atlas(); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPropOptionsFor_Keep10k is one prop's options for the first
// player, on a placed prop standing on the 10k keep's floor.
func BenchmarkPropOptionsFor_Keep10k(b *testing.B) {
	enc, _ := openSite(b, 10000, false)
	atlas, err := enc.Atlas()
	require.NoError(b, err)
	require.NotEmpty(b, atlas.Placed)
	prop := atlas.Placed[0].ID
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := enc.PropOptionsFor(keepPlayers[0], prop)
		if err != nil && !isNoProp(err) {
			b.Fatal(err)
		}
	}
}

func isNoProp(err error) bool { return err != nil && errors.Is(err, encounter.ErrNoProp) }

// BenchmarkWithinReach_Keep10k is the first player's reach read on the 10k
// keep: what stands within three hexes and is seen now.
func BenchmarkWithinReach_Keep10k(b *testing.B) {
	enc, _ := openSite(b, 10000, false)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := enc.WithinReach(keepPlayers[0], speakReach); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkAtlasFor_Keep10k is one member's projection of the 10k keep.
func BenchmarkAtlasFor_Keep10k(b *testing.B) {
	enc, _ := openSite(b, 10000, false)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := enc.AtlasFor(keepPlayers[0]); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPlacementIndex_Keep30k is what deriving the placement index costs
// every construction and every load of the 30k keep.
func BenchmarkPlacementIndex_Keep30k(b *testing.B) {
	enc, _ := openSite(b, 30000, false)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		enc.ExportedRebuildPlacementIndex()
	}
}
