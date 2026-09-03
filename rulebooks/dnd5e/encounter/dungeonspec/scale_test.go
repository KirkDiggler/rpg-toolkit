// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// scale_test.go is WHAT THE WALL DERIVATIONS COST, measured rather than
// reasoned about (rpg-project#360, review round on rpg-toolkit#1477).
//
// Two shapes in this wave are quadratic on paper. deriveWalls tests every
// floor cell against every wall, and maskHeight scans the segment list per
// masked boundary. Both are the simplest version that is true, and both have
// an obvious narrower form — an axial bounding range for candidate cells, an
// index built once per projection — that costs a second representation to keep
// in step with the first.
//
// THE STANCE IS TO OPTIMISE AFTER IT IS A PROBLEM, so the question is not
// whether the shape is quadratic but whether the numbers matter at the size a
// dungeon actually is. These benchmarks are the answer, and they are here to be
// re-run rather than quoted: `go test ./dungeonspec/ -bench Scale -benchtime
// 20x`. The precedent is voidcost_internal_test.go, which holds the sight
// scan's own measured ratios for the same reason.
//
// The yardstick is TEN TIMES THE REFERENCE TOMB — eleven rooms, ten seams,
// thousands of floor cells, and a concealed room at the end so the masquerade
// runs — which is far larger than any dungeon anyone has authored by hand.

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// readFixture reads an authored dungeon for a benchmark or a test alike —
// [tombYAML] takes a *testing.T, and a benchmark has a *testing.B.
func readFixture(tb testing.TB, path string) []byte {
	tb.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(tb, err)

	return raw
}

// bigDungeon is `rooms` rectangles side by side, each `cols` wide and `rows`
// tall, separated by quarter-line seams with one door each — the reference
// tomb's own shape, repeated.
//
// The last room is CONCEALED and its door with it, so a projection over this
// dungeon has hidden space to withhold and a long boundary to mask. Every
// other room is reachable from the start through plain doorways, which is what
// keeps the coherence check happy.
func bigDungeon(rooms, cols, rows int) string {
	var b strings.Builder
	b.WriteString("version: 2\nkey: scale\norientation: pointy\nvoid: opaque\nregions:\n")
	for r := 0; r < rooms; r++ {
		fmt.Fprintf(&b, "  - id: room-%d\n    archetype: crypt\n    lighting: { intensity: 1 }\n", r)
		if r == rooms-1 {
			b.WriteString("    concealed: true\n")
		}
		b.WriteString("    cells:\n")
		for row := 0; row < rows; row++ {
			b.WriteString("      - [")
			for c := 0; c < cols; c++ {
				if c > 0 {
					b.WriteString(",")
				}
				fmt.Fprintf(&b, "[%d,%d]", r*cols+c, row)
			}
			b.WriteString("]\n")
		}
	}
	b.WriteString("start: [0, 0]\nwalls:\n")
	for r := 1; r < rooms; r++ {
		west := r*cols - 1
		fmt.Fprintf(&b, "  - start: { cell: [%d,%d], offset: [0.25, 0.375] }\n", west, rows-1)
		fmt.Fprintf(&b, "    end:   { cell: [%d,0], offset: [-0.25, -0.375] }\n", west+1)
		fmt.Fprintf(&b, "    name: seam %d\n", r)
	}
	// Halfway down, on an EVEN row: the quarter line passes through the
	// slanted midpoints of the even-row cells of the eastern column, exactly
	// as the reference tomb's own doors do.
	doorRow := rows / 2
	if doorRow%2 == 1 {
		doorRow--
	}
	b.WriteString("doors:\n")
	for r := 1; r < rooms; r++ {
		fmt.Fprintf(&b, "  - id: door-%d\n", r)
		fmt.Fprintf(&b, "    at: { cell: [%d,%d], offset: [-0.25, -0.375] }\n", r*cols, doorRow)
		b.WriteString("    closed: true\n")
		if r == rooms-1 {
			b.WriteString("    concealed: [{ ability: perception, dc: 15 }]\n")
		}
	}

	return b.String()
}

// tenTimesTheTomb is the yardstick: 11 rooms of 12 by 20, so 2640 floor cells
// against the reference tomb's 224, with ten seams against its two.
func tenTimesTheTomb() string { return bigDungeon(11, 12, 20) }

// TestTheScaleFixtureIsWhatItClaims — a benchmark on a dungeon that quietly
// failed to be big would report a comfortable number about nothing.
func TestTheScaleFixtureIsWhatItClaims(t *testing.T) {
	compiled, err := dungeonspec.Load([]byte(tenTimesTheTomb()))
	require.NoError(t, err)

	cells := 0
	for _, r := range compiled.Field.Regions {
		cells += len(r.Cells)
	}
	require.Equal(t, 2640, cells, "eleven rooms of twelve by twenty")
	require.GreaterOrEqual(t, cells, 224*10, "at least ten times the reference tomb")
	require.Len(t, compiled.Field.Segments, 10, "ten seams")
	require.Len(t, compiled.Field.Doors, 10)
	require.Len(t, compiled.Field.Walls, 380,
		"ten seams of 39 crossings each, less the one crossing each door opens")
	require.Empty(t, compiled.Field.Sealed, "quarter lines seal nothing, at any size")

	// The last room really is hidden, so a projection over this has work to do.
	require.True(t, compiled.Field.Regions[10].Concealed)
}

// nobodyFindsAnything is the resolver a concealed door needs to be authorable.
// No scene here searches, so it is never actually asked.
type nobodyFindsAnything struct{}

func (nobodyFindsAnything) ResolveCheck(*encounter.ResolveCheckInput) (*encounter.ResolveCheckOutput, error) {
	return &encounter.ResolveCheckOutput{}, nil
}

// nobodyIsWatching reports no perceiver for any door — the concealed room
// stays concealed, which is the state these benchmarks want it in.
type nobodyIsWatching struct{}

func (nobodyIsWatching) Perceivers(*encounter.PerceiversInput) ([]encounter.MemberID, error) {
	return nil, nil
}

// scaleEncounter builds the encounter a projection benchmark runs over.
func scaleEncounter(tb testing.TB, raw string) (*encounter.Encounter, encounter.MemberID) {
	tb.Helper()
	compiled, err := dungeonspec.Load([]byte(raw))
	require.NoError(tb, err)

	watcher := core.EntityID("watcher")
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: noAttacksExpected{}, Announcer: quietAnnouncer{},
		CheckResolver: nobodyFindsAnything{}, Witness: nobodyIsWatching{},
		Field: compiled.Field,
		Members: []encounter.MemberInput{
			{ID: watcher, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 0}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	require.NoError(tb, err)

	return enc, watcher
}

// TestTheScaleProjectionHasSomethingToMask — same reason as above: an AtlasFor
// benchmark over a dungeon with nothing hidden measures the fast path and says
// nothing about maskHeight at all.
func TestTheScaleProjectionHasSomethingToMask(t *testing.T) {
	enc, watcher := scaleEncounter(t, tenTimesTheTomb())

	full, err := enc.Atlas()
	require.NoError(t, err)
	scoped, err := enc.AtlasFor(watcher)
	require.NoError(t, err)

	require.Len(t, full.Regions, 11, "the whole dungeon")
	require.Len(t, scoped.Regions, 10, "and one room the watcher cannot see")
	require.Less(t, len(scoped.Cells), len(full.Cells), "its floor is withheld with it")
	require.NotEmpty(t, scoped.Boundaries,
		"and the masquerade stands walls where the secret borders visible space")
}

func BenchmarkScale_LoadReferenceTomb(b *testing.B) {
	raw := readFixture(b, tombPath)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := dungeonspec.Load(raw); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkScale_LoadTenTimesTheTomb(b *testing.B) {
	raw := []byte(tenTimesTheTomb())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := dungeonspec.Load(raw); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkScale_AtlasForReferenceTomb(b *testing.B) {
	enc, watcher := scaleEncounter(b, string(readFixture(b, tombPath)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := enc.AtlasFor(watcher); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkScale_AtlasForTenTimesTheTomb(b *testing.B) {
	enc, watcher := scaleEncounter(b, tenTimesTheTomb())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := enc.AtlasFor(watcher); err != nil {
			b.Fatal(err)
		}
	}
}
