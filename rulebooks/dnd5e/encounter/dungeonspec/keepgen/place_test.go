// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package keepgen

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// wallPieceLength is the studio's render piece for the plan's wall (plan
// G13): the `boundsMeters` width of dnd5e:env:dark-fortress:45_wall_01 in
// rpg-dnd5e-web's pinned src/generated/worldAssetCatalog.ts, which the
// provider already measures at the shared Synty scale, so the studio divides
// a span by it as it stands (structuralWallEditing.ts layoutWallPieces).
const wallPieceLength = 2.7868884801864624

// maxWallPieces is MAX_WALL_PIECES_PER_WALL in the web's
// structuralWallEditing.ts: a wall past it refuses to render.
const maxWallPieces = 200

func pairKey(a, b string) string {
	if a > b {
		a, b = b, a
	}
	return a + "|" + b
}

// TestPlace_Walls is design R2 and B3 at both sizes: every pair of regions
// that share an edge has exactly one wall and no other pair has one, except
// the two grounds, which meet open; no wall's blocker covers a cell centre;
// every plan door is one opening with an attached door bound open; no wall
// lays more than the studio's 200 pieces; and with every door shut the walls
// make each room its own partition.
func TestPlace_Walls(t *testing.T) {
	for _, hexes := range []int{10000, 30000} {
		t.Run(fmt.Sprint(hexes), func(t *testing.T) {
			g := keepAt(t, hexes)
			floor := floorCells(g.compiled)
			open := map[string]bool{}
			for _, r := range g.layout.regions {
				if r.open {
					open[r.id] = true
				}
			}

			// Region pairs by how many crossings join them. A shared EDGE is
			// many crossings; two rectangles that meet only at a corner share
			// one diagonal crossing, which the ends of the walls either side
			// cut, and carry no wall of their own.
			crossings := map[string]int{}
			for c, id := range floor {
				for _, nb := range c.neighbours() {
					if other, ok := floor[nb]; ok && other != id {
						crossings[pairKey(id, other)]++ // each crossing counted from both ends
					}
				}
			}
			walls := map[string]int{}
			for _, w := range g.site.walls {
				walls[pairKey(g.layout.regions[w.a].id, g.layout.regions[w.b].id)]++
			}
			for pair, n := range crossings {
				if n/2 < 2 {
					require.Zero(t, walls[pair], "%s meet at a corner and carry no wall", pair)
					continue
				}
				a, b, _ := strings.Cut(pair, "|")
				if open[a] && open[b] {
					require.Zero(t, walls[pair], "%s are both open ground and meet without a wall", pair)
					continue
				}
				require.Equal(t, 1, walls[pair], "%s share an edge: one wall", pair)
			}
			for pair, n := range walls {
				require.Equal(t, 1, n, pair)
				require.GreaterOrEqual(t, crossings[pair]/2, 2, "%s has a wall and shares an edge", pair)
			}

			// No blocker covers a centre: every floor centre is more than
			// half the blocker's depth from every wall line.
			for _, w := range g.site.walls {
				minX, maxX := math.Min(w.sx, w.ex)-1, math.Max(w.sx, w.ex)+1
				minZ, maxZ := math.Min(w.sz, w.ez)-1, math.Max(w.sz, w.ez)+1
				for c := range floor {
					x, z := c.centre()
					if x < minX || x > maxX || z < minZ || z > maxZ {
						continue
					}
					require.Greater(t, segmentDistance(x, z, w), blockerDepth/2, "wall %s covers the centre of %v", w.id, c)
				}
			}

			// One door per plan door, every one open.
			require.Len(t, g.compiled.Field.Doors, len(DefaultPlan().Doors))
			for _, d := range g.compiled.Field.Doors {
				require.Equal(t, encounter.DoorOpen, d.State.Kind(), "door %s is bound open", d.ID)
			}

			// The studio's piece cap.
			for _, w := range g.site.walls {
				spans := []float64{w.length}
				if w.door != nil {
					spans = []float64{w.door.position - doorWidth/2, w.length - w.door.position - doorWidth/2}
				}
				pieces := 0
				for _, span := range spans {
					pieces += max(1, int(math.Round(span/wallPieceLength)))
				}
				require.LessOrEqual(t, pieces, maxWallPieces, "wall %s", w.id)
			}

			// With every door shut each room is its own partition; the two
			// grounds are one.
			parts, err := encounter.PartitionFloor(encounter.PartitionFloorInput{Field: g.compiled.Field})
			require.NoError(t, err)
			require.Len(t, parts.Components, len(g.layout.regions)-1)
			for _, component := range parts.Components {
				in := map[string]bool{}
				for _, p := range component {
					in[floor[cell{int(p.X), int(p.Y)}]] = true
				}
				if len(in) == 2 {
					require.True(t, in["west-grounds"] && in["south-grounds"], "only the grounds share a partition: %v", in)
					continue
				}
				require.Len(t, in, 1, "a partition is one room: %v", in)
			}
		})
	}
}

// segmentDistance is the distance from (x, z) to the wall's line segment.
func segmentDistance(x, z float64, w wall) float64 {
	dx, dz := w.ex-w.sx, w.ez-w.sz
	t := ((x-w.sx)*dx + (z-w.sz)*dz) / (dx*dx + dz*dz)
	t = math.Max(0, math.Min(1, t))
	return math.Hypot(x-(w.sx+t*dx), z-(w.sz+t*dz))
}

// TestPlace_KeepsThresholdsClear is B4 at both sizes, read against the
// compiled floor: no prop's footprint covers a door threshold (either cell
// of a crossing inside an opening), the party start, an exit or a monster's
// cell; every footprint stays inside the prop's own region (plan G2); and no
// two props share a cell. Footprints are recomputed here by brute force over
// the floor, not taken from the placement.
func TestPlace_KeepsThresholdsClear(t *testing.T) {
	for _, hexes := range []int{10000, 30000} {
		t.Run(fmt.Sprint(hexes), func(t *testing.T) {
			g := keepAt(t, hexes)
			floor := floorCells(g.compiled)
			clear := map[cell]string{}
			for _, w := range g.site.walls {
				if w.door == nil {
					continue
				}
				a, b := g.layout.regions[w.a].id, g.layout.regions[w.b].id
				found := 0
				for c, id := range floor {
					if id != a {
						continue
					}
					for _, nb := range c.neighbours() {
						if floor[nb] != b {
							continue
						}
						x1, z1 := c.centre()
						x2, z2 := nb.centre()
						if math.Hypot((x1+x2)/2-w.door.cx, (z1+z2)/2-w.door.cz) <= doorWidth/2 {
							clear[c], clear[nb] = "threshold of "+w.door.id, "threshold of "+w.door.id
							found++
						}
					}
				}
				require.Positive(t, found, "door %s opens a crossing between %s and %s", w.door.id, a, b)
			}
			start := g.compiled.PartyStart[0].At
			clear[cell{int(start.X), int(start.Y)}] = "the party start"
			require.Len(t, g.compiled.Field.Exits, 2)
			for _, ex := range g.compiled.Field.Exits {
				clear[cell{int(ex.At.X), int(ex.At.Y)}] = "exit " + string(ex.ID)
			}
			require.Len(t, g.compiled.Monsters, len(g.site.monsters))
			for _, m := range g.compiled.Monsters {
				clear[cell{int(m.At.X), int(m.At.Y)}] = "monster " + m.MemberID
			}

			byRow := map[int][]cell{}
			for c := range floor {
				byRow[c.row] = append(byRow[c.row], c)
			}
			claimed := map[cell]string{}
			for _, pr := range g.site.props {
				hx, hz := pr.entry.Width/2, pr.entry.Depth/2
				if pr.rotation != 0 {
					hx, hz = hz, hx
				}
				cx, cz := pr.at.centre()
				// Every floor cell in the rows the footprint can reach (no
				// palette side exceeds 12, so eight rows either way is all).
				var covered []cell
				for row := pr.at.row - 8; row <= pr.at.row+8; row++ {
					for _, c := range byRow[row] {
						x, z := c.centre()
						if math.Abs(x-cx) <= hx+1e-6 && math.Abs(z-cz) <= hz+1e-6 {
							covered = append(covered, c)
						}
					}
				}
				require.NotEmpty(t, covered, pr.id)
				require.ElementsMatch(t, pr.covered, covered, "%s covers what the placement says", pr.id)
				own := g.layout.regions[pr.region].id
				for _, c := range covered {
					require.Equal(t, own, floor[c], "%s stays inside %s", pr.id, own)
					require.Empty(t, clear[c], "%s covers %v, %s", pr.id, c, clear[c])
					require.Empty(t, claimed[c], "%s and %s share %v", pr.id, claimed[c], c)
					claimed[c] = pr.id
				}
			}
		})
	}
}

// TestPlace_Densities is the design's Defaults at 10,000 hexes: props per
// palette kind within 10% of the density over that kind's floor, two thirds
// of them blocking, and monsters within 10% of one per 100 floor hexes.
func TestPlace_Densities(t *testing.T) {
	g := keepAt(t, 10000)
	p := DefaultPlan()
	expected, got := map[string]float64{}, map[string]int{}
	for _, r := range g.layout.regions {
		if len(p.Palettes[r.kind].Props) == 0 {
			continue
		}
		d := g.opts.PropDensity
		if r.open {
			d = g.opts.GroundsDensity
		}
		expected[r.kind] += float64(r.size()) * d / 100
	}
	blocking := 0
	for _, pr := range g.site.props {
		got[g.layout.regions[pr.region].kind]++
		if pr.blocking {
			blocking++
			require.True(t, *pr.entry.BlocksMovement)
		}
	}
	require.NotEmpty(t, expected)
	for kind, want := range expected {
		require.InDelta(t, want, float64(got[kind]), want*0.1, "kind %s: %d props for %.1f expected", kind, got[kind], want)
	}
	require.Zero(t, got[KindPassage], "passages have no palette and stay clear")
	require.InDelta(t, 2.0/3, float64(blocking)/float64(len(g.site.props)), 0.05, "two thirds block")
	require.InDelta(t, 100, float64(len(g.site.monsters)), 10, "one monster per 100 floor hexes")

	require.Equal(t, len(g.site.props), g.report.Props)
	require.Equal(t, blocking, g.report.Blocking)
	require.Equal(t, len(g.site.monsters), g.report.Monsters)
	t.Logf("10k: props %d (blocking %d), monsters %d, per kind %v", len(g.site.props), blocking, len(g.site.monsters), got)
}

// TestPlace_MonstersFromTheirPalette: every monster's ref is its region's
// kind's (plan G7), the prison holds prisoners, and no two stand together.
func TestPlace_MonstersFromTheirPalette(t *testing.T) {
	g := keepAt(t, 10000)
	p := DefaultPlan()
	at := map[cell]bool{}
	prisoners := 0
	for _, m := range g.site.monsters {
		r := g.layout.regions[m.region]
		require.Contains(t, p.Palettes[r.kind].Monsters, m.ref, m.id)
		require.False(t, at[m.at], "%s shares a cell", m.id)
		at[m.at] = true
		if r.id == "prison" {
			require.Equal(t, "dnd5e:monsters:prisoner", m.ref)
			prisoners++
		}
	}
	require.Positive(t, prisoners)
}
