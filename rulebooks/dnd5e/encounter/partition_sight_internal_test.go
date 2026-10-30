// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TestPartitionSightAnswersWhatTheCanvasAnswers is the index's whole claim
// (partition_sight.go): for every edge between two floor cells and for every
// cell, the indexed read gives the canvas's own answer —
// [canvasRoom.IsLineOfSightBlocked] and [canvasSightObstructions.At] — on a
// field crowded with every kind of thing that blocks sight: placed rectangles
// of every size and facing (some reaching across many buckets, some a sliver
// between two centres), occluding cell props, sight-blocking edge walls, a
// shut edge door, a movable prop the pass leaves out, and a wall line with an
// opening a shut FOOTPRINT door fills. Seed 0 carries only that wall and its
// door, so the door is the one thing on its crossing.
func TestPartitionSightAnswersWhatTheCanvasAnswers(t *testing.T) {
	for _, seed := range []int64{0, 1, 2, 3, 4, 5} {
		t.Run(fmt.Sprintf("seed %d", seed), func(t *testing.T) {
			rng := rand.New(rand.NewSource(seed))
			in := FieldInput{
				Canvas:  openAir(),
				Regions: []RegionInput{rectRegion("hall", 0, 0, 16, 12)},
			}
			f, err := compileField(in)
			require.NoError(t, err)
			centre := func(col, row int) spatial.Point {
				return f.plane.CellCentre(f.cellAt(spatial.Position{X: float64(col), Y: float64(row)}))
			}

			// THE WALL WITH A DOOR IN ITS OPENING: a thin sight-blocking span
			// either side of the crossing between [10,6] and [11,6], and a
			// shut footprint door standing across that crossing.
			a, b := centre(10, 6), centre(11, 6)
			mid := spatial.Point{X: (a.X + b.X) / 2, Y: (a.Y + b.Y) / 2}
			facing := math.Atan2(b.Y-a.Y, b.X-a.X)*180/math.Pi + 90
			ux, uy := math.Cos(facing*math.Pi/180), math.Sin(facing*math.Pi/180)
			span := func(id string, along float64) PlacedPropInput {
				return PlacedPropInput{ID: PropID(id), BlocksMovement: true, BlocksLineOfSight: true,
					Placement: spatial.FootprintPlacement{
						Footprint: spatial.Footprint{Box: &spatial.Box{W: 0.4, D: 8}},
						Origin:    spatial.Point{X: mid.X + ux*along, Y: mid.Y + uy*along}, Facing: facing,
					}}
			}
			in.Placed = append(in.Placed, span("wall-north", -6.5), span("wall-south", 6.5))
			doorAt := spatial.FootprintPlacement{
				Footprint: spatial.Footprint{Box: &spatial.Box{W: 0.4, D: 4.5}}, Origin: mid, Facing: facing,
			}
			in.Doors = []DoorInput{{ID: "footprint", State: DoorIsClosed(), Placement: &doorAt}}

			for i := 0; seed != 0 && i < 40; i++ {
				at := centre(rng.Intn(16), rng.Intn(12))
				at.X += (rng.Float64() - 0.5) * FeetPerCell
				at.Y += (rng.Float64() - 0.5) * FeetPerCell
				w, d := 0.2+rng.Float64()*3, 0.5+rng.Float64()*40
				if i%4 == 0 {
					w, d = 0.5+rng.Float64()*6, 0.5+rng.Float64()*6
				}
				in.Placed = append(in.Placed, PlacedPropInput{
					ID: PropID(fmt.Sprintf("p%d", i)),
					Placement: spatial.FootprintPlacement{
						Footprint: spatial.Footprint{Box: &spatial.Box{W: w, D: d}},
						Origin:    at, Facing: rng.Float64() * 360,
					},
					BlocksMovement: rng.Intn(2) == 0, BlocksLineOfSight: rng.Intn(4) != 0,
					Holdable: i == 7,
				})
			}
			yes := true
			for i := 0; seed != 0 && i < 6; i++ {
				in.Props = append(in.Props, PropInput{ID: PropID(fmt.Sprintf("rubble%d", i)), Ref: "dnd5e:props:rubble",
					At: spatial.Position{X: float64(rng.Intn(16)), Y: float64(rng.Intn(12))}, BlocksMovement: &yes, BlocksLineOfSight: &yes})
			}
			seen := map[spatial.Position]bool{}
			props := in.Props[:0]
			for _, p := range in.Props {
				if !seen[p.At] {
					seen[p.At] = true
					props = append(props, p)
				}
			}
			in.Props = props
			for row := 0; row < 12; row++ {
				in.Walls = append(in.Walls, WallInput{Boundary: spatial.Boundary{
					From: spatial.Position{X: 7, Y: float64(row)}, To: spatial.Position{X: 8, Y: float64(row)},
					BlocksMovement: true, BlocksLineOfSight: row != 5,
				}})
			}
			in.Doors = append(in.Doors, DoorInput{ID: "shut", State: DoorIsClosed(), Edges: []DoorEdge{{
				From: HexCellAt(HexesArePointyTop(), 3, 4), To: HexCellAt(HexesArePointyTop(), 4, 4),
			}}})

			f, err = compileField(in)
			require.NoError(t, err)
			require.NoError(t, validateDoorInputs(f, in.Doors))
			canvas, err := f.partitionCanvas(in.Doors)
			require.NoError(t, err)
			sight := newPartitionSight(canvas)
			obstructions := canvasSightObstructions{canvas: canvas}

			blockedEdges, opaqueCells := 0, 0
			for cell := range f.owner {
				want, err := obstructions.At(spatial.SightCellInput{At: cell})
				require.NoError(t, err)
				got, err := sight.opaqueAt(cell)
				require.NoError(t, err)
				require.Equal(t, want.Blocked, got, "cell %v", cell)
				if got {
					opaqueCells++
				}
				for _, n := range canvas.GetGrid().GetNeighbors(cell) {
					if _, owned := f.owner[n]; !owned {
						continue
					}
					want := canvas.IsLineOfSightBlocked(cell, n)
					require.Equal(t, want, sight.edgeBlocked(cell, n), "edge %v -> %v", cell, n)
					if want {
						blockedEdges++
					}
				}
			}
			require.Positive(t, blockedEdges, "the fixture blocks some edges")
			door := f.cellAt(spatial.Position{X: 10, Y: 6})
			beyond := f.cellAt(spatial.Position{X: 11, Y: 6})
			require.True(t, canvas.IsLineOfSightBlocked(door, beyond), "precondition: the shut door closes its crossing")
			require.True(t, sight.edgeBlocked(door, beyond), "and the index reads the door")
			require.Positive(t, opaqueCells, "and covers some centres")
		})
	}
}
