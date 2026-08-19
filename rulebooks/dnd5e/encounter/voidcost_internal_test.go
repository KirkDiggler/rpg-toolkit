// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// voidcost_internal_test.go is WHAT THE ROCK COSTS (rpg-toolkit#1116).
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
// One run on an AMD 7945HX. Absolute times move with whatever else the machine
// is doing; the RATIOS held across repeats, and they are the finding:
//
//	tomb-shaped, 3 chambers, 8 members     open   84us  rock   53us   1.4-1.6x FASTER
//	20 chambers with gaps, 60 members      open 31.7ms  rock 18.9ms   1.6-1.7x FASTER
//	one 40x40 chamber, void margin, 40     open  3.1ms  rock  4.1ms   1.34x slower
//	20 chambers, canvas tiled, NO void     open 29.7ms  rock 29.6ms   parity
//
// Where there IS void, rock is FASTER, which is not the intuition: the scan is
// arithmetic, and the spatial call it returns before rasterizes, walks
// boundaries, walks blocking entities and then walks a lean lane per neighbour.
// Finding rock early skips all of that. The price is paid by rays that never
// leave the floor, which scan in full and delegate anyway — the third row, and
// the honest worst case. That 1.34x is the number to beat if a caller ever
// forces the floor mask rpg-toolkit#1105's ruling (4) deferred.
//
// The fourth row is the one that was bad, and is the reason fieldHasVoid
// exists. Deleting its check from IsLineOfSightBlocked and re-running that pair
// measures rock at 121ms against open's 30ms — 4.0x, and 46.7 MB of allocation
// against 24.2 MB — every byte of it spent proving there was no void to find on
// a field where rock and open mean the same thing.
// TestRockCostsNothingWhereThereIsNoVoid pins the fix as allocations, so it
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

// benchSightRefresh times one full percept rebuild over the whole roster, which
// is the loop the scan actually runs inside.
func benchSightRefresh(b *testing.B, rooms, dim, stride, anchor, members int, void Void) {
	b.Helper()

	mi := make([]MemberInput, members)
	for i := range mi {
		mi[i] = MemberInput{ID: MemberID(fmt.Sprintf("m%d", i)), Kind: KindPlayer,
			Room:     fmt.Sprintf("r%d", i%rooms),
			Position: spatial.Position{X: float64(i % dim), Y: float64((i / dim) % dim)}}
	}

	enc, err := NewEncounter(&SetupInput{
		Sight: benchSight{1 << 20}, Standing: benchStanding{}, Initiative: benchRoller{},
		Field:   benchField(rooms, dim, stride, anchor, void),
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
func BenchmarkSightTombOpen(b *testing.B) { benchSightRefresh(b, 3, 10, 11, 0, 8, VoidIsOpen()) }
func BenchmarkSightTombRock(b *testing.B) { benchSightRefresh(b, 3, 10, 11, 0, 8, VoidIsRock()) }

// Chambers with gaps between them — the case rock exists for.
func BenchmarkSightGappedOpen(b *testing.B) { benchSightRefresh(b, 20, 20, 21, 0, 60, VoidIsOpen()) }
func BenchmarkSightGappedRock(b *testing.B) { benchSightRefresh(b, 20, 20, 21, 0, 60, VoidIsRock()) }

// One chamber with a void margin and nobody ever looking across it: the scan
// runs in full, finds nothing, and delegates anyway. The worst case.
func BenchmarkSightMarginOpen(b *testing.B) { benchSightRefresh(b, 1, 40, 40, 1, 40, VoidIsOpen()) }
func BenchmarkSightMarginRock(b *testing.B) { benchSightRefresh(b, 1, 40, 40, 1, 40, VoidIsRock()) }

// Chambers that tile their canvas exactly: no void, so nothing to look for.
func BenchmarkSightNoVoidOpen(b *testing.B) { benchSightRefresh(b, 20, 20, 20, 0, 60, VoidIsOpen()) }
func BenchmarkSightNoVoidRock(b *testing.B) { benchSightRefresh(b, 20, 20, 20, 0, 60, VoidIsRock()) }
