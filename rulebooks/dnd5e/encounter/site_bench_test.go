// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"context"
	"encoding/json"
	"os"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// BenchmarkGarrisonWalkLine is F23's shape (rpg-project#879 S5): the player
// walks eight cells out along a line from the party start and eight back on
// the posts-asleep garrison keep, one walk per op, timed per walking step
// (steps that raise the world clock are excluded, as F23 excluded them). Three
// variants:
//
//   - open: the keep stays open across ops, the bench's old shape;
//
//   - reloaded-shared: the keep is loaded from its checkpoint before every op
//     (outside the timer) on a provider that keeps the site resident, which is
//     what a live server's per-verb load is now;
//
//   - reloaded-cold: the same load on a fresh provider every op, the site
//     compiled from its piece — what every load was before the site tier.
//
//     go test -run XXX -bench BenchmarkGarrisonWalkLine -benchtime 10x .
func BenchmarkGarrisonWalkLine(b *testing.B) {
	for _, variant := range []string{"open", "reloaded-shared", "reloaded-cold"} {
		b.Run(variant, func(b *testing.B) { benchWalkLine(b, variant) })
	}
}

func benchWalkLine(b *testing.B, variant string) {
	b.StopTimer()
	sites := encounter.NewMemorySites(nil)
	enc := openGarrisonKeepOn(b, true, sites)
	player := keepPlayers[0]
	line := walkLine(b, enc, sites, player, 8)
	walk := append(append([]spatial.Position(nil), line[1:]...), reversed(line[:len(line)-1])...)
	checkpoint := enc.ToData()
	piece := enc.Site().Data()

	var walkNs time.Duration
	var walkSteps int
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		switch variant {
		case "reloaded-shared":
			enc = loadGarrison(b, checkpoint, sites, nil)
		case "reloaded-cold":
			enc = loadGarrison(b, checkpoint, encounter.NewMemorySites(nil), &piece)
		}
		before := enc.WorldRound()
		b.StartTimer()
		for j, cell := range walk {
			started := time.Now()
			if _, err := enc.Step(&encounter.StepInput{Member: player, To: cell, EndWalk: j == len(walk)-1}); err != nil {
				b.Fatalf("op %d step %d to %v: %v", i, j, cell, err)
			}
			took := time.Since(started)
			b.StopTimer()
			if after := enc.WorldRound(); after == before {
				walkNs += took
				walkSteps++
			} else {
				before = after
			}
			b.StartTimer()
		}
		b.StopTimer()
	}
	if walkSteps > 0 {
		b.ReportMetric(float64(walkNs)/float64(walkSteps)/float64(time.Millisecond), "walk-step-ms")
	}
}

func loadGarrison(
	b *testing.B, data encounter.EncounterData, sites encounter.SiteProvider, piece *encounter.SiteData,
) *encounter.Encounter {
	b.Helper()
	enc, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data, Sites: sites, Site: piece, Capabilities: garrisonCapabilities(),
	})
	if err != nil {
		b.Fatalf("load the garrison: %v", err)
	}
	return enc
}

// walkLine is n+1 cells from where player stands, each a step from the last
// and never one already on the line, found by stepping a probe copy of the
// keep (loaded from enc's checkpoint, outside any timer). It keeps going the
// way it went when it can, so the line runs straight where the floor lets it.
func walkLine(
	b *testing.B, enc *encounter.Encounter, sites encounter.SiteProvider, player encounter.MemberID, n int,
) []spatial.Position {
	b.Helper()
	probe := loadGarrison(b, enc.ToData(), sites, nil)
	at := whereIs(b, probe, player)
	line := []spatial.Position{at}
	on := map[spatial.Position]bool{at: true}
	dirs := []spatial.Position{{X: 1, Y: 0}, {X: 0, Y: 1}, {X: -1, Y: 1}, {X: -1, Y: 0}, {X: 0, Y: -1}, {X: 1, Y: -1}}
	last := 0
	for len(line) <= n {
		moved := false
		for k := 0; k < len(dirs); k++ {
			d := dirs[(last+k)%len(dirs)]
			next := spatial.Position{X: at.X + d.X, Y: at.Y + d.Y}
			if on[next] {
				continue
			}
			if _, err := probe.Step(&encounter.StepInput{Member: player, To: next, EndWalk: true}); err != nil {
				continue
			}
			at, last, moved = next, (last+k)%len(dirs), true
			line = append(line, at)
			on[at] = true
			break
		}
		if !moved {
			b.Fatalf("no line of %d cells from %v", n, line[0])
		}
	}
	return line
}

func reversed(cells []spatial.Position) []spatial.Position {
	out := make([]spatial.Position, len(cells))
	for i, c := range cells {
		out[len(cells)-1-i] = c
	}
	return out
}

// TestMeasureGarrisonSite is the S5 run record's measurement (rpg-project#879
// F8, F21), run only when RPG_MEASURE_SITE is set: the heap a compiled site
// holds against the heap of the whole built keep, and LoadEncounter on the
// garrison's checkpoint with the site resident against compiled from its
// piece. It asserts nothing about time; it logs.
//
//	RPG_MEASURE_SITE=1 go test -run TestMeasureGarrisonSite -count=1 -v .
func TestMeasureGarrisonSite(t *testing.T) {
	if os.Getenv("RPG_MEASURE_SITE") == "" {
		t.Skip("RPG_MEASURE_SITE unset: the S5 measurement runs on request")
	}
	heap := func() uint64 {
		runtime.GC()
		runtime.GC()
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return m.HeapInuse
	}
	for _, listen := range []bool{false, true} {
		sites := encounter.NewMemorySites(nil)
		warm := openGarrisonKeepOn(t, listen, sites)
		piece := warm.Site().Data()

		base := heap()
		siteOut, err := encounter.CompileSite(&encounter.CompileSiteInput{Data: &piece})
		require.NoError(t, err)
		siteHeap := heap() - base
		own := encounter.NewMemorySites(nil)
		_, err = own.Site(context.Background(), &encounter.SiteInput{Data: &piece})
		require.NoError(t, err)
		base = heap()
		inst := openGarrisonKeepOn(t, listen, own)
		instHeap := heap() - base
		runtime.KeepAlive(siteOut)

		data := inst.ToData()
		blob, err := json.Marshal(data)
		require.NoError(t, err)
		pieceRaw, err := json.Marshal(piece)
		require.NoError(t, err)

		const n = 20
		timeLoad := func(sites encounter.SiteProvider, withPiece bool) time.Duration {
			var took []time.Duration
			for i := 0; i < n; i++ {
				var p *encounter.SiteData
				provider := sites
				if withPiece {
					cp := piece
					p, provider = &cp, encounter.NewMemorySites(nil)
				}
				started := time.Now()
				_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
					Data: data, Sites: provider, Site: p, Capabilities: garrisonCapabilities(),
				})
				took = append(took, time.Since(started))
				require.NoError(t, err)
			}
			sort.Slice(took, func(i, j int) bool { return took[i] < took[j] })
			return took[n/2]
		}
		compileTook := func() time.Duration {
			var took []time.Duration
			for i := 0; i < n; i++ {
				cp := piece
				started := time.Now()
				_, err := encounter.CompileSite(&encounter.CompileSiteInput{Data: &cp})
				took = append(took, time.Since(started))
				require.NoError(t, err)
			}
			sort.Slice(took, func(i, j int) bool { return took[i] < took[j] })
			return took[n/2]
		}()
		t.Logf("posts-listen=%v site heap %.2f MB, instance heap %.2f MB (site share %.0f%%)",
			listen, float64(siteHeap)/1e6, float64(instHeap)/1e6, 100*float64(siteHeap)/float64(siteHeap+instHeap))
		t.Logf("posts-listen=%v checkpoint %d bytes, site piece %d bytes", listen, len(blob), len(pieceRaw))
		t.Logf("posts-listen=%v LoadEncounter p50: warm %v, cold %v; CompileSite p50 %v",
			listen, timeLoad(own, false), timeLoad(nil, true), compileTook)
	}
}
