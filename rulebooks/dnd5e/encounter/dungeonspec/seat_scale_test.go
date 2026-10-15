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
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// openFloorSpec is the v3 workshop fixture with its floor replaced by a w×h
// odd-r rectangle centred on the origin — the studio's own workspace shape —
// so the start, the monster and the props all still stand on floor.
func openFloorSpec(tb testing.TB, w, h int) *SingleRoomSpec {
	tb.Helper()
	floor := make([]RoomCell, 0, w*h)
	radius := 0
	for r := -(h / 2); r < -(h/2)+h; r++ {
		for col := -(w / 2); col < -(w/2)+w; col++ {
			// odd-r offset to axial: q = col - floor(r/2).
			c := RoomCell{Q: col - (r-((r%2)+2)%2)/2, R: r}
			floor = append(floor, c)
			radius = max(radius, cubeDistance(c))
		}
	}
	raw, err := os.ReadFile("testdata/world-builder-v3.yaml")
	require.NoError(tb, err)
	const workspace = "workspace: {hexRadius: 6, horizontalLimit: 12}"
	require.Contains(tb, string(raw), workspace, "the fixture's workspace line moved")
	raw = []byte(strings.Replace(string(raw), workspace,
		fmt.Sprintf("workspace: {hexRadius: %d, horizontalLimit: %d}", radius, 2*radius+12), 1))
	decoded, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: raw})
	require.NoError(tb, err)
	spec := decoded.Spec
	spec.Room.Gameplay.WalkableHexes = floor

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
