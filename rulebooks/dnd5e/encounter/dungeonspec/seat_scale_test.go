// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

// seat_scale_test.go is WHAT A LARGE OPEN FLOOR COSTS TO COMPILE
// (rpg-toolkit#2047). An open floor puts every cell in the party start's
// partition, so it is the worst case for seat derivation: every cell is a
// candidate seat and every candidate is asked whether it can be stood on.
// Re-run rather than quoted:
// `go test ./dungeonspec/ -run '^$' -bench CompileSingleRoom_Keep -benchtime 3x`.

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// openFloorSpec is the workshop fixture with its floor replaced by a w×h
// odd-r rectangle centred on the origin, as one region — so the start, the
// monster and the props all still stand on floor. The canvas is unbounded
// (rpg-project#725), so nothing else about the fixture has to grow with it.
func openFloorSpec(tb testing.TB, w, h int) *SingleRoomSpec {
	tb.Helper()
	floor := make([]RoomCell, 0, w*h)
	for r := -(h / 2); r < -(h/2)+h; r++ {
		for col := -(w / 2); col < -(w/2)+w; col++ {
			// odd-r offset to axial: q = col - floor(r/2).
			floor = append(floor, RoomCell{Q: col - (r-((r%2)+2)%2)/2, R: r})
		}
	}
	raw, err := os.ReadFile("testdata/world-builder-v3.yaml")
	require.NoError(tb, err)
	decoded, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: raw})
	require.NoError(tb, err)
	spec := decoded.Spec
	SetFloor(&spec.Room.Gameplay, floor)

	return spec
}

func BenchmarkCompileSingleRoom_Keep(b *testing.B) {
	for _, size := range []struct{ w, h int }{{73, 48}, {146, 96}} {
		spec := openFloorSpec(b, size.w, size.h)
		b.Run(fmt.Sprintf("hexes=%d", size.w*size.h), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				compiled, err := CompileSingleRoom(CompileSingleRoomInput{Spec: spec})
				if err != nil {
					b.Fatal(err)
				}
				if len(compiled.PartyStart) < size.w*size.h/2 {
					b.Fatalf("open floor derived only %d seats", len(compiled.PartyStart))
				}
			}
		})
	}
}
