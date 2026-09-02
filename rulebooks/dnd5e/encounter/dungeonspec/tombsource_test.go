// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// tombsource_test.go holds the authored dungeon every test in this package is
// about, in one place, because very different suites need the same bytes:
// dialect_test.go edits it to check what gets refused and where the refusal
// points, golden_test.go compiles it against the atlas version 1 produced, and
// tomb_test.go walks it.
//
// The bytes live in testdata/reference-tomb.yaml rather than in a Go string
// since rpg-project#256: the file IS the fixture rpg-api ships as
// content/reference-tomb.yaml and the builder round-trips, so it has to be a
// file somebody can open. testdata/tomb-second-skeleton.yaml is the
// rpg-project#254 variant — the same tomb with a wall across the hall and the
// second skeleton behind it — so slice 2's fixture lives here once.

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	tombPath           = "testdata/reference-tomb.yaml"
	secondSkeletonPath = "testdata/tomb-second-skeleton.yaml"
)

// tombYAML reads the reference tomb's authored bytes.
func tombYAML(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(tombPath)
	require.NoError(t, err)
	return string(raw)
}

// tombDoorEdges are the two crossings the tomb's doors stand in, each written
// exactly as the `walls:` list writes an edge. The tomb's two wall lines run
// straight past these cells with a hole punched at row 4 — which is what makes
// it the fixture for "a door stands in a wall" (rpg-project#355): filling the
// holes back in is a real edit with a visible consequence, not a no-op.
var tombDoorEdges = map[string]string{
	"  - [[5,3],[6,4]]":   "  - [[5,4],[6,4]]",
	"  - [[15,3],[16,4]]": "  - [[15,4],[16,4]]",
}

// regroupedTomb rewrites the tomb's flat `walls:` list as `runs` grouped
// entries — THE SAME EDGES IN THE SAME ORDER, only bracketed differently — so
// a caller can pin that regrouping changes nothing about what compiles.
//
// With withDoors, each wall line also reclaims the edge its door stands in,
// which is the shape an author actually draws: one unbroken run, the door
// listed separately. The compiler subtracts those edges again, so BOTH forms
// must produce the identical wall set — and the spec in between genuinely
// carries more edges than the atlas does, which is the contrast that lets the
// subtraction be observed at all.
func regroupedTomb(t *testing.T, tomb string, runs int, withDoors bool) string {
	t.Helper()
	lines := strings.Split(tomb, "\n")
	at := -1
	for i, l := range lines {
		if l == "walls:" {
			at = i
			break
		}
	}
	require.GreaterOrEqual(t, at, 0, "the tomb has a walls: block")

	var edges []string
	end := at + 1
	for ; end < len(lines) && strings.HasPrefix(lines[end], "  - [["); end++ {
		edges = append(edges, strings.TrimPrefix(lines[end], "  - "))
		if door, ok := tombDoorEdges[lines[end]]; withDoors && ok {
			edges = append(edges, strings.TrimPrefix(door, "  - "))
		}
	}
	require.NotEmpty(t, edges, "the tomb's walls are a flat list of edges")
	require.LessOrEqual(t, runs, len(edges))

	out := append([]string{}, lines[:at+1]...)
	per := (len(edges) + runs - 1) / runs
	for i, run := 0, 0; i < len(edges); i, run = i+per, run+1 {
		out = append(out, fmt.Sprintf("  - name: run %d", run), "    edges:")
		for _, e := range edges[i:min(i+per, len(edges))] {
			out = append(out, "      - "+e)
		}
	}
	return strings.Join(append(out, lines[end:]...), "\n")
}
