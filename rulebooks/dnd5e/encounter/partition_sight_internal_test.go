// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
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
// shut edge door and a movable prop the pass leaves out.
func TestPartitionSightAnswersWhatTheCanvasAnswers(t *testing.T) {
	for _, seed := range []int64{1, 2, 3, 4, 5} {
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

			for i := 0; i < 40; i++ {
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
			for i := 0; i < 6; i++ {
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
			in.Doors = []DoorInput{{ID: "shut", State: DoorIsClosed(), Edges: []DoorEdge{{
				From: HexCellAt(HexesArePointyTop(), 3, 4), To: HexCellAt(HexesArePointyTop(), 4, 4),
			}}}}

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
			require.Positive(t, opaqueCells, "and covers some centres")
		})
	}
}
