// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// voidcost_internal_test.go is WHAT OPAQUE VOID COSTS (rpg-toolkit#1116).
//
// [canvasRoom.IsLineOfSightBlocked] scans the ray for cells no chamber owns,
// and Copilot's review of PR #1124 asked the obvious question: that scan is
// O(rayLen x rooms) inside percept building's O(members^2) loop. These
// benchmarks are the answer, and they are here rather than in a comment because
// every number quoted in that method's godoc — and the case for the floor mask
// rpg-toolkit#1105's ruling (4) deferred — has to stay re-runnable:
//
//	go test -run XXX -bench BenchmarkSight -benchtime 30x .
//
// One machine, an AMD 7945HX, re-measured for rpg-toolkit#1127 over -count=3
// at 300x. Absolute times move with whatever else the machine is doing; the
// RATIOS held across repeats, and they are the finding:
//
//	SQUARE, 3 chambers, 8 members        transparent  80-86us  opaque  59-71us  1.2-1.4x FASTER
//	20 chambers with gaps, 60 members    transparent   37.4ms  opaque   22.2ms  1.7x FASTER
//	one 40x40 chamber, void margin, 40   transparent 3.0-3.2ms opaque 4.3-4.4ms 1.35-1.43x slower
//	20 chambers, canvas tiled, NO void   transparent   31.3ms  opaque   29.3ms  parity
//	HEX reference tomb, pointy, 8        transparent  67-69us  opaque  92-94us  1.34-1.40x slower
//	HEX reference tomb, flat, 8          (same field) opaque 97-108us  1.06-1.16x the pointy one
//
// Where there IS void BETWEEN the chambers, opaque is FASTER, which is not the
// intuition: the scan is arithmetic, and the spatial call it returns before
// rasterizes, walks boundaries, walks blocking entities and then walks a lean
// lane per neighbour. Finding void early skips all of that. The price is paid
// by rays that never leave the floor, which scan in full and delegate anyway.
//
// THE LAST TWO ROWS ARE THE ONES THAT MATTER NOW, and they were not here
// before rpg-toolkit#1127 because the field they measure could not be built.
// The reference tomb is hex, and a hex canvas holding a sheared rectangle is
// 90.5% void pointy-top, 93.5% flat-top — yet opaque is SLOWER on it, not
// faster, because a party of eight in three chambers looks mostly at people in
// the same chamber, and those rays never leave the floor. So the worst case is
// no longer a contrived margin fixture: it is the shape the game actually runs,
// at 1.34-1.40x, and it is the number to beat if the floor mask is ever asked
// to go faster. Flat-top costs a further 6-16% for the same 224 cells of
// floor — the price of the bigger canvas, and worth knowing before an author
// picks a layout for looks.
//
// The fourth row is the one that was bad, and is the reason fieldHasVoid
// exists. Deleting its check from IsLineOfSightBlocked and re-running that pair
// measures opaque at 121ms against transparent's 30ms — 4.0x, and 46.7 MB of
// against 24.2 MB — every byte of it spent proving there was no void to find on
// a field where opaque and transparent mean the same thing.
// TestOpaqueCostsNothingWhereThereIsNoVoid pins the fix as allocations, so it
// holds without a stopwatch.

type benchSight struct{ cells int }

func (b benchSight) Sight(members []MemberID) (map[MemberID]int, error) {
	out := make(map[MemberID]int, len(members))
	for _, m := range members {
		out[m] = b.cells
	}
	return out, nil
}

type benchStanding struct{}

func (benchStanding) Standing(_ []MemberID) ([]MemberID, error) { return nil, nil }

type benchRoller struct{}

func (benchRoller) RollInitiative(members []MemberID) ([]MemberID, error) { return members, nil }

// benchField lays `rooms` chambers of dim x dim in a row, `stride` apart:
// stride == dim makes them tile the canvas with no void at all, stride == dim+1
// leaves a one-cell gap between each pair. `anchor` shifts the whole field off
// the canvas origin, which is how a single-chamber field gets a void margin.
func benchField(rooms, dim, stride, anchor int, void Void) FieldInput {
	ri := make([]RoomInput, rooms)
	for i := range ri {
		ri[i] = RoomInput{ID: fmt.Sprintf("r%d", i), Width: dim, Height: dim,
			Origin: spatial.Position{X: float64(anchor + i*stride), Y: float64(anchor)}}
	}
	return FieldInput{Canvas: CanvasInput{Void: void}, Rooms: ri}
}

// hexTombField is the REFERENCE TOMB'S OWN SHAPE, in the family the game
// actually runs on: three hex chambers 6, 10 and 12 columns wide and 8 rows
// tall, laid left to right (rpg-toolkit#1127).
//
// It is here because the mask made this shape possible AND made it expensive
// in a way no square field is. A chamber is an authored rectangle, which shears
// into a parallelogram in axial space, while the canvas that must hold it is an
// origin-centred axial span — so the canvas is far bigger than the floor.
// Measured: 90.5% void pointy-top, 93.5% flat-top, against 43.2% for the same
// three chambers read as rhombi. Every square fixture in this file is under
// 10%, so the ratios above were measured on a field nothing in the game looks
// like.
func hexTombField(void Void, o Orientation) FieldInput {
	widths := []int{6, 10, 12}
	ri := make([]RoomInput, len(widths))
	col := 0
	for i, w := range widths {
		ri[i] = RoomInput{ID: fmt.Sprintf("r%d", i), Grid: spatial.GridShapeHex,
			Width: w, Height: 8, Origin: spatial.Position{X: float64(col), Y: 0}}
		col += w
	}
	return FieldInput{Canvas: CanvasInput{Void: void, Orientation: o}, Rooms: ri}
}

// benchSightRefresh times one full percept rebuild over the whole roster, which
// is the loop the scan actually runs inside.
func benchSightRefresh(b *testing.B, rooms, dim, stride, anchor, members int, void Void) {
	b.Helper()
	benchSightRefreshOn(b, benchField(rooms, dim, stride, anchor, void), rooms, dim, members)
}

// benchSightRefreshOn is benchSightRefresh over a field somebody else built —
// the hex tomb, whose chambers are not all one size and so cannot be described
// by benchField's rooms/dim/stride triple.
func benchSightRefreshOn(b *testing.B, field FieldInput, rooms, dim, members int) {
	b.Helper()

	mi := make([]MemberInput, members)
	for i := range mi {
		mi[i] = MemberInput{ID: MemberID(fmt.Sprintf("m%d", i)), Kind: KindPlayer,
			Room:     fmt.Sprintf("r%d", i%rooms),
			Position: spatial.Position{X: float64(i % dim), Y: float64((i / dim) % dim)}}
	}

	enc, err := NewEncounter(&SetupInput{
		Sight: benchSight{1 << 20}, Standing: benchStanding{}, Initiative: benchRoller{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field:   field,
		Members: mi,
		Endings: []EndingInput{{Key: "done", Trigger: TriggerExternal{}}},
	})
	if err != nil {
		b.Fatal(err)
	}

	roster := enc.rosterIDs()
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := enc.rebuildPercepts(roster); err != nil {
			b.Fatal(err)
		}
	}
}

// The reference tomb's shape: three chambers in a chain, a party of eight.
func BenchmarkSightTombTransparent(b *testing.B) {
	benchSightRefresh(b, 3, 10, 11, 0, 8, VoidIsTransparent())
}
func BenchmarkSightTombOpaque(b *testing.B) { benchSightRefresh(b, 3, 10, 11, 0, 8, VoidIsOpaque()) }

// Chambers with gaps between them — the case an opaque declaration exists for.
func BenchmarkSightGappedTransparent(b *testing.B) {
	benchSightRefresh(b, 20, 20, 21, 0, 60, VoidIsTransparent())
}
func BenchmarkSightGappedOpaque(b *testing.B) {
	benchSightRefresh(b, 20, 20, 21, 0, 60, VoidIsOpaque())
}

// One chamber with a void margin and nobody ever looking across it: the scan
// runs in full, finds nothing, and delegates anyway. The worst case.
func BenchmarkSightMarginTransparent(b *testing.B) {
	benchSightRefresh(b, 1, 40, 40, 1, 40, VoidIsTransparent())
}
func BenchmarkSightMarginOpaque(b *testing.B) { benchSightRefresh(b, 1, 40, 40, 1, 40, VoidIsOpaque()) }

// Chambers that tile their canvas exactly: no void, so nothing to look for.
func BenchmarkSightNoVoidTransparent(b *testing.B) {
	benchSightRefresh(b, 20, 20, 20, 0, 60, VoidIsTransparent())
}
func BenchmarkSightNoVoidOpaque(b *testing.B) {
	benchSightRefresh(b, 20, 20, 20, 0, 60, VoidIsOpaque())
}

// The reference tomb in its own family: three hex chambers, a party of eight,
// on a canvas that is 90% void because a sheared rectangle needs one that big
// (rpg-toolkit#1127). See hexTombField.
func BenchmarkSightHexTombTransparent(b *testing.B) {
	benchSightRefreshOn(b, hexTombField(VoidIsTransparent(), HexesArePointyTop()), 3, 6, 8)
}
func BenchmarkSightHexTombOpaque(b *testing.B) {
	benchSightRefreshOn(b, hexTombField(VoidIsOpaque(), HexesArePointyTop()), 3, 6, 8)
}

// The same tomb flat-top, which is a different canvas (93.5% void) for the same
// 224 cells of floor — the price of the other layout, measured rather than
// assumed.
func BenchmarkSightHexTombFlatOpaque(b *testing.B) {
	benchSightRefreshOn(b, hexTombField(VoidIsOpaque(), HexesAreFlatTop()), 3, 6, 8)
}
